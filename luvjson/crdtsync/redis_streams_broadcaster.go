package crdtsync

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtpubsub"
)

// RedisStreamsBroadcaster는 Redis Streams를 사용하는 브로드캐스터 구현입니다.
// Redis Streams는 내구성 있는 메시지 큐로, 메시지 손실 없이 패치를 전달할 수 있습니다.
type RedisStreamsBroadcaster struct {
	// client는 Redis 클라이언트입니다.
	client *redis.Client

	// streamKey는 패치를 브로드캐스트하는 데 사용되는 스트림 키입니다.
	streamKey string

	// consumerGroup은 소비자 그룹 이름입니다.
	consumerGroup string

	// consumerName은 소비자 이름입니다.
	consumerName string

	// localSessionID는 로컬 세션 ID입니다.
	localSessionID common.SessionID

	// maxLen은 스트림의 최대 길이입니다. 이 값을 초과하면 오래된 메시지가 자동으로 삭제됩니다.
	maxLen int64

	// format은 패치 인코딩 형식입니다.
	format crdtpubsub.EncodingFormat
}

// NewRedisStreamsBroadcaster는 새 Redis Streams 브로드캐스터를 생성합니다.
func NewRedisStreamsBroadcaster(
	client *redis.Client,
	streamKey string,
	format crdtpubsub.EncodingFormat,
	sessionID common.SessionID,
) (*RedisStreamsBroadcaster, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}

	// 소비자 그룹 및 소비자 이름 생성
	consumerGroup := fmt.Sprintf("%s-group", streamKey)
	consumerName := fmt.Sprintf("consumer-%s", sessionID.String()[:8])

	broadcaster := &RedisStreamsBroadcaster{
		client:         client,
		streamKey:      streamKey,
		consumerGroup:  consumerGroup,
		consumerName:   consumerName,
		localSessionID: sessionID,
		maxLen:         1000, // 기본값으로 1000개 메시지 유지
		format:         format,
	}

	// 스트림 및 소비자 그룹 초기화
	if err := broadcaster.initialize(context.Background()); err != nil {
		return nil, err
	}

	return broadcaster, nil
}

// initialize는 스트림 및 소비자 그룹을 초기화합니다.
func (b *RedisStreamsBroadcaster) initialize(ctx context.Context) error {
	// 스트림이 존재하는지 확인
	exists, err := b.client.Exists(ctx, b.streamKey).Result()
	if err != nil {
		return fmt.Errorf("failed to check if stream exists: %w", err)
	}

	// 스트림이 존재하지 않으면 빈 메시지 추가하여 생성
	if exists == 0 {
		_, err = b.client.XAdd(ctx, &redis.XAddArgs{
			Stream: b.streamKey,
			ID:     "*", // 자동 ID 생성
			Values: map[string]interface{}{
				"init": "true",
			},
		}).Result()
		if err != nil {
			return fmt.Errorf("failed to create stream: %w", err)
		}
	}

	// 소비자 그룹 생성 시도
	err = b.client.XGroupCreate(ctx, b.streamKey, b.consumerGroup, "0").Err()
	if err != nil {
		// 이미 존재하는 경우 무시
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return fmt.Errorf("failed to create consumer group: %w", err)
		}
	}

	return nil
}

// Broadcast는 패치를 다른 노드들에게 브로드캐스트합니다.
func (b *RedisStreamsBroadcaster) Broadcast(ctx context.Context, patch *crdtpatch.Patch) error {
	// 패치 인코딩
	encoder, err := crdtpubsub.GetEncoderDecoder(b.format)
	if err != nil {
		return fmt.Errorf("failed to get encoder: %w", err)
	}

	data, err := encoder.Encode(patch)
	if err != nil {
		return fmt.Errorf("failed to encode patch: %w", err)
	}

	// 메시지 필드 생성
	values := map[string]interface{}{
		"data":      data,
		"format":    string(b.format),
		"sessionID": b.localSessionID.String(),
		"timestamp": time.Now().UnixNano(),
	}

	// 스트림에 메시지 추가
	_, err = b.client.XAdd(ctx, &redis.XAddArgs{
		Stream:       b.streamKey,
		MaxLen:       b.maxLen,
		MaxLenApprox: b.maxLen, // 근사치 허용
		ID:           "*",      // 자동 ID 생성
		Values:       values,
	}).Result()

	if err != nil {
		return fmt.Errorf("failed to add message to stream: %w", err)
	}

	return nil
}

// Next는 다음 브로드캐스트된 패치를 수신합니다.
func (b *RedisStreamsBroadcaster) Next(ctx context.Context) (*crdtpatch.Patch, error) {
	for {
		// 새 메시지 읽기 시도
		streams, err := b.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    b.consumerGroup,
			Consumer: b.consumerName,
			Streams:  []string{b.streamKey, ">"}, // ">"는 아직 처리되지 않은 메시지만 가져옴
			Count:    1,                          // 한 번에 하나의 메시지만 가져옴
			Block:    time.Second * 1,            // 1초 동안 블록
		}).Result()

		if err != nil {
			// 컨텍스트 취소 또는 타임아웃인 경우
			if err == context.Canceled || err == context.DeadlineExceeded {
				return nil, err
			}

			// 타임아웃인 경우 다시 시도
			if err == redis.Nil {
				continue
			}

			return nil, fmt.Errorf("failed to read from stream: %w", err)
		}

		// 메시지가 없는 경우 다시 시도
		if len(streams) == 0 || len(streams[0].Messages) == 0 {
			continue
		}

		// 메시지 처리
		message := streams[0].Messages[0]
		messageID := message.ID

		// 메시지 필드 추출
		data, ok := message.Values["data"].(string)
		if !ok {
			// 메시지 확인 처리 후 건너뛰기
			b.client.XAck(ctx, b.streamKey, b.consumerGroup, messageID)
			continue
		}

		formatStr, ok := message.Values["format"].(string)
		if !ok {
			// 메시지 확인 처리 후 건너뛰기
			b.client.XAck(ctx, b.streamKey, b.consumerGroup, messageID)
			continue
		}

		sessionIDStr, ok := message.Values["sessionID"].(string)
		if !ok {
			// 메시지 확인 처리 후 건너뛰기
			b.client.XAck(ctx, b.streamKey, b.consumerGroup, messageID)
			continue
		}

		// 자신이 보낸 메시지는 무시
		if sessionIDStr == b.localSessionID.String() {
			// 메시지 확인 처리 후 건너뛰기
			b.client.XAck(ctx, b.streamKey, b.consumerGroup, messageID)
			continue
		}

		// 패치 디코딩
		decoder, err := crdtpubsub.GetEncoderDecoder(crdtpubsub.EncodingFormat(formatStr))
		if err != nil {
			// 메시지 확인 처리 후 건너뛰기
			b.client.XAck(ctx, b.streamKey, b.consumerGroup, messageID)
			continue
		}

		patch, err := decoder.Decode([]byte(data))
		if err != nil {
			// 메시지 확인 처리 후 건너뛰기
			b.client.XAck(ctx, b.streamKey, b.consumerGroup, messageID)
			continue
		}

		// 메시지 확인 처리
		b.client.XAck(ctx, b.streamKey, b.consumerGroup, messageID)

		return patch, nil
	}
}

// Close는 브로드캐스터를 종료합니다.
func (b *RedisStreamsBroadcaster) Close() error {
	// Redis 클라이언트는 외부에서 관리하므로 여기서 닫지 않음
	return nil
}

// SetMaxLen은 스트림의 최대 길이를 설정합니다.
func (b *RedisStreamsBroadcaster) SetMaxLen(maxLen int64) {
	b.maxLen = maxLen
}
