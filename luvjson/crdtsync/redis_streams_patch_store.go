package crdtsync

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtpubsub"
)

// RedisStreamsPatchStore는 Redis Streams를 사용하는 패치 저장소 구현입니다.
// Redis Streams는 내구성 있는 메시지 큐로, 패치를 영구적으로 저장하고 조회할 수 있습니다.
type RedisStreamsPatchStore struct {
	// client는 Redis 클라이언트입니다.
	client *redis.Client

	// streamKey는 패치를 저장하는 데 사용되는 스트림 키입니다.
	streamKey string

	// format은 패치 인코딩 형식입니다.
	format crdtpubsub.EncodingFormat

	// maxLen은 스트림의 최대 길이입니다. 이 값을 초과하면 오래된 메시지가 자동으로 삭제됩니다.
	maxLen int64

	// mutex는 동시 접근을 보호합니다.
	mutex sync.RWMutex

	// patchCache는 패치 캐시입니다. 자주 접근하는 패치를 메모리에 캐싱합니다.
	patchCache map[string]*crdtpatch.Patch
}

// NewRedisStreamsPatchStore는 새 Redis Streams 패치 저장소를 생성합니다.
func NewRedisStreamsPatchStore(
	client *redis.Client,
	streamKey string,
	format crdtpubsub.EncodingFormat,
) (*RedisStreamsPatchStore, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}

	store := &RedisStreamsPatchStore{
		client:     client,
		streamKey:  streamKey,
		format:     format,
		maxLen:     10000, // 기본값으로 10000개 패치 유지
		mutex:      sync.RWMutex{},
		patchCache: make(map[string]*crdtpatch.Patch),
	}

	// 스트림 초기화
	if err := store.initialize(context.Background()); err != nil {
		return nil, err
	}

	return store, nil
}

// initialize는 스트림을 초기화합니다.
func (s *RedisStreamsPatchStore) initialize(ctx context.Context) error {
	// 스트림이 존재하는지 확인
	exists, err := s.client.Exists(ctx, s.streamKey).Result()
	if err != nil {
		return fmt.Errorf("failed to check if stream exists: %w", err)
	}

	// 스트림이 존재하지 않으면 빈 메시지 추가하여 생성
	if exists == 0 {
		_, err = s.client.XAdd(ctx, &redis.XAddArgs{
			Stream: s.streamKey,
			ID:     "*", // 자동 ID 생성
			Values: map[string]interface{}{
				"init": "true",
			},
		}).Result()
		if err != nil {
			return fmt.Errorf("failed to create stream: %w", err)
		}
	}

	return nil
}

// StorePatch는 패치를 저장합니다.
func (s *RedisStreamsPatchStore) StorePatch(patch *crdtpatch.Patch) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 패치 인코딩
	encoder, err := crdtpubsub.GetEncoderDecoder(s.format)
	if err != nil {
		return fmt.Errorf("failed to get encoder: %w", err)
	}

	data, err := encoder.Encode(patch)
	if err != nil {
		return fmt.Errorf("failed to encode patch: %w", err)
	}

	// 패치 ID 생성
	patchID := patch.ID().String()

	// 메시지 필드 생성
	values := map[string]interface{}{
		"id":        patchID,
		"data":      data,
		"format":    string(s.format),
		"timestamp": time.Now().UnixNano(),
		"sid":       patch.ID().SID.String(),
		"seq":       strconv.FormatUint(patch.ID().Counter, 10),
	}

	// 스트림에 메시지 추가
	ctx := context.Background()
	_, err = s.client.XAdd(ctx, &redis.XAddArgs{
		Stream:       s.streamKey,
		MaxLen:       s.maxLen,
		MaxLenApprox: s.maxLen, // 근사치 허용
		ID:           "*",      // 자동 ID 생성
		Values:       values,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to add patch to stream: %w", err)
	}

	// 패치 캐시에 추가
	s.patchCache[patchID] = patch

	return nil
}

// GetPatches는 주어진 상태 벡터 이후의 패치를 반환합니다.
func (s *RedisStreamsPatchStore) GetPatches(stateVector map[string]uint64) ([]*crdtpatch.Patch, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 모든 패치 가져오기
	ctx := context.Background()
	messages, err := s.client.XRange(ctx, s.streamKey, "-", "+").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get patches from stream: %w", err)
	}

	// 필요한 패치 필터링
	var patches []*crdtpatch.Patch
	for _, message := range messages {
		// 초기화 메시지 건너뛰기
		if _, ok := message.Values["init"]; ok {
			continue
		}

		// 패치 ID 필드 추출
		sidStr, ok := message.Values["sid"].(string)
		if !ok {
			continue
		}

		seqStr, ok := message.Values["seq"].(string)
		if !ok {
			continue
		}

		// 시퀀스 번호 파싱
		seq, err := strconv.ParseUint(seqStr, 10, 64)
		if err != nil {
			continue
		}

		// 상태 벡터와 비교
		lastSeq, ok := stateVector[sidStr]
		if ok && seq <= lastSeq {
			// 이미 적용된 패치는 건너뛰기
			continue
		}

		// 패치 데이터 필드 추출
		data, ok := message.Values["data"].(string)
		if !ok {
			continue
		}

		formatStr, ok := message.Values["format"].(string)
		if !ok {
			continue
		}

		// 패치 디코딩
		decoder, err := crdtpubsub.GetEncoderDecoder(crdtpubsub.EncodingFormat(formatStr))
		if err != nil {
			continue
		}

		patch, err := decoder.Decode([]byte(data))
		if err != nil {
			continue
		}

		patches = append(patches, patch)
	}

	return patches, nil
}

// GetPatch는 특정 ID의 패치를 반환합니다.
func (s *RedisStreamsPatchStore) GetPatch(id common.LogicalTimestamp) (*crdtpatch.Patch, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// 패치 ID 생성
	patchID := id.String()

	// 캐시에서 패치 찾기
	if patch, ok := s.patchCache[patchID]; ok {
		return patch, nil
	}

	// 스트림에서 패치 찾기
	ctx := context.Background()
	messages, err := s.client.XRange(ctx, s.streamKey, "-", "+").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get patches from stream: %w", err)
	}

	for _, message := range messages {
		// 초기화 메시지 건너뛰기
		if _, ok := message.Values["init"]; ok {
			continue
		}

		// 패치 ID 필드 추출
		msgID, ok := message.Values["id"].(string)
		if !ok {
			continue
		}

		// ID 비교
		if msgID == patchID {
			// 패치 데이터 필드 추출
			data, ok := message.Values["data"].(string)
			if !ok {
				continue
			}

			formatStr, ok := message.Values["format"].(string)
			if !ok {
				continue
			}

			// 패치 디코딩
			decoder, err := crdtpubsub.GetEncoderDecoder(crdtpubsub.EncodingFormat(formatStr))
			if err != nil {
				continue
			}

			patch, err := decoder.Decode([]byte(data))
			if err != nil {
				continue
			}

			// 패치 캐시에 추가
			s.patchCache[patchID] = patch

			return patch, nil
		}
	}

	return nil, common.ErrNotFound{Message: fmt.Sprintf("patch %s", patchID)}
}

// Close는 패치 저장소를 종료합니다.
func (s *RedisStreamsPatchStore) Close() error {
	// Redis 클라이언트는 외부에서 관리하므로 여기서 닫지 않음
	return nil
}

// SetMaxLen은 스트림의 최대 길이를 설정합니다.
func (s *RedisStreamsPatchStore) SetMaxLen(maxLen int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.maxLen = maxLen
}

// ClearCache는 패치 캐시를 지웁니다.
func (s *RedisStreamsPatchStore) ClearCache() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.patchCache = make(map[string]*crdtpatch.Patch)
}
