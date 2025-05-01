package crdtstorage

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-redis/redis/v8"
)

// RedisAdapter는 Redis 기반 영구 저장소 어댑터입니다.
type RedisAdapter struct {
	// client는 Redis 클라이언트입니다.
	client *redis.Client

	// keyPrefix는 Redis 키 접두사입니다.
	keyPrefix string

	// mutex는 Redis 작업에 대한 동시 접근을 보호합니다.
	mutex sync.RWMutex

	// serializer는 문서 직렬화/역직렬화를 담당합니다.
	serializer DocumentSerializer
}

// NewRedisAdapter는 새 Redis 어댑터를 생성합니다.
func NewRedisAdapter(client *redis.Client, keyPrefix string) *RedisAdapter {
	return &RedisAdapter{
		client:     client,
		keyPrefix:  keyPrefix,
		serializer: NewDefaultDocumentSerializer(),
	}
}

// getDocumentKey는 문서 ID에 대한 Redis 키를 반환합니다.
func (a *RedisAdapter) getDocumentKey(documentID string) string {
	return fmt.Sprintf("%s:doc:%s", a.keyPrefix, documentID)
}

// getDocumentListKey는 문서 목록에 대한 Redis 키를 반환합니다.
func (a *RedisAdapter) getDocumentListKey() string {
	return fmt.Sprintf("%s:docs", a.keyPrefix)
}

// SaveDocument는 문서를 Redis에 저장합니다.
func (a *RedisAdapter) SaveDocument(ctx context.Context, doc *Document) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 문서 직렬화
	data, err := a.serializer.Serialize(doc)
	if err != nil {
		return fmt.Errorf("failed to serialize document: %w", err)
	}

	// 문서 키 가져오기
	docKey := a.getDocumentKey(doc.ID)

	// 문서 저장
	if err := a.client.Set(ctx, docKey, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}

	// 문서 목록에 추가
	if err := a.client.SAdd(ctx, a.getDocumentListKey(), doc.ID).Err(); err != nil {
		return fmt.Errorf("failed to add document to list: %w", err)
	}

	return nil
}

// LoadDocument는 문서를 Redis에서 로드합니다.
func (a *RedisAdapter) LoadDocument(ctx context.Context, documentID string) ([]byte, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 문서 키 가져오기
	docKey := a.getDocumentKey(documentID)

	// 문서 가져오기
	data, err := a.client.Get(ctx, docKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("document not found: %s", documentID)
		}
		return nil, fmt.Errorf("failed to get document: %w", err)
	}

	return data, nil
}

// ListDocuments는 모든 문서 목록을 반환합니다.
func (a *RedisAdapter) ListDocuments(ctx context.Context) ([]string, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 문서 목록 가져오기
	members, err := a.client.SMembers(ctx, a.getDocumentListKey()).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get document list: %w", err)
	}

	return members, nil
}

// DeleteDocument는 문서를 Redis에서 삭제합니다.
func (a *RedisAdapter) DeleteDocument(ctx context.Context, documentID string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 문서 키 가져오기
	docKey := a.getDocumentKey(documentID)

	// 문서 삭제
	if err := a.client.Del(ctx, docKey).Err(); err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	// 문서 목록에서 제거
	if err := a.client.SRem(ctx, a.getDocumentListKey(), documentID).Err(); err != nil {
		return fmt.Errorf("failed to remove document from list: %w", err)
	}

	return nil
}

// Close는 Redis 어댑터를 닫습니다.
func (a *RedisAdapter) Close() error {
	// Redis 클라이언트는 외부에서 관리하므로 여기서 닫지 않음
	return nil
}
