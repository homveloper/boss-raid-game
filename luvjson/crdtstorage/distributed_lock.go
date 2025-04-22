package crdtstorage

import (
	"context"
	"fmt"
	"time"
)

// DistributedLock은 분산 락 인터페이스입니다.
// 이 인터페이스는 분산 환경에서 리소스에 대한 독점적 접근을 제공합니다.
type DistributedLock interface {
	// Acquire는 락을 획득합니다.
	// 성공하면 true, 실패하면 false를 반환합니다.
	// 타임아웃이 발생하면 오류를 반환합니다.
	Acquire(ctx context.Context, timeout time.Duration) (bool, error)

	// Release는 락을 해제합니다.
	// 락이 성공적으로 해제되면 true, 그렇지 않으면 false를 반환합니다.
	Release(ctx context.Context) (bool, error)

	// Refresh는 락의 만료 시간을 갱신합니다.
	// 락이 성공적으로 갱신되면 true, 그렇지 않으면 false를 반환합니다.
	Refresh(ctx context.Context, ttl time.Duration) (bool, error)
}

// DistributedLockManager는 분산 락 관리자 인터페이스입니다.
// 이 인터페이스는 분산 락을 생성하고 관리하는 기능을 제공합니다.
type DistributedLockManager interface {
	// GetLock은 지정된 리소스에 대한 분산 락을 반환합니다.
	GetLock(resourceID string, ownerID string) DistributedLock

	// Close는 락 관리자를 닫습니다.
	Close() error
}

// RedisDistributedLock은 Redis 기반 분산 락 구현체입니다.
type RedisDistributedLock struct {
	// client는 Redis 클라이언트입니다.
	client RedisClient

	// resourceID는 락을 획득할 리소스의 ID입니다.
	resourceID string

	// ownerID는 락 소유자의 ID입니다.
	ownerID string

	// lockKey는 Redis에서 사용할 락 키입니다.
	lockKey string

	// acquired는 락 획득 여부입니다.
	acquired bool

	// refreshTicker는 락 갱신을 위한 타이커입니다.
	refreshTicker *time.Ticker

	// stopRefresh는 락 갱신을 중지하기 위한 채널입니다.
	stopRefresh chan struct{}
}

// RedisClient는 Redis 클라이언트 인터페이스입니다.
// 이 인터페이스는 Redis 작업에 필요한 메서드를 정의합니다.
type RedisClient interface {
	// SetNX는 키가 존재하지 않는 경우에만 값을 설정합니다.
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)

	// Eval은 Lua 스크립트를 실행합니다.
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)

	// Del은 키를 삭제합니다.
	Del(ctx context.Context, keys ...string) (int64, error)

	// Expire는 키의 만료 시간을 설정합니다.
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)

	// Get은 키의 값을 가져옵니다.
	Get(ctx context.Context, key string) (string, error)

	// Set은 키에 값을 설정합니다.
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
}

// NewRedisDistributedLock은 새 Redis 분산 락을 생성합니다.
func NewRedisDistributedLock(client RedisClient, resourceID, ownerID string) *RedisDistributedLock {
	return &RedisDistributedLock{
		client:      client,
		resourceID:  resourceID,
		ownerID:     ownerID,
		lockKey:     fmt.Sprintf("lock:%s", resourceID),
		acquired:    false,
		stopRefresh: make(chan struct{}),
	}
}

// Acquire는 락을 획득합니다.
func (l *RedisDistributedLock) Acquire(ctx context.Context, timeout time.Duration) (bool, error) {
	// 이미 락을 획득한 경우
	if l.acquired {
		return true, nil
	}

	// 락 획득 시도
	acquired, err := l.client.SetNX(ctx, l.lockKey, l.ownerID, timeout)
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	if !acquired {
		return false, nil
	}

	// 락 획득 성공
	l.acquired = true

	// 락 자동 갱신 시작 (데드락 방지)
	l.startAutoRefresh(ctx, timeout)

	return true, nil
}

// Release는 락을 해제합니다.
func (l *RedisDistributedLock) Release(ctx context.Context) (bool, error) {
	// 락을 획득하지 않은 경우
	if !l.acquired {
		return true, nil
	}

	// 자동 갱신 중지
	if l.refreshTicker != nil {
		l.stopRefresh <- struct{}{}
	}

	// 락 해제 (소유자 확인 후 삭제)
	result, err := l.client.Eval(ctx, `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`, []string{l.lockKey}, l.ownerID)

	if err != nil {
		return false, fmt.Errorf("failed to release lock: %w", err)
	}

	// 결과 확인
	success := false
	if val, ok := result.(int64); ok && val > 0 {
		success = true
	}

	l.acquired = false
	return success, nil
}

// Refresh는 락의 만료 시간을 갱신합니다.
func (l *RedisDistributedLock) Refresh(ctx context.Context, ttl time.Duration) (bool, error) {
	// 락을 획득하지 않은 경우
	if !l.acquired {
		return false, nil
	}

	// 락 갱신 (소유자 확인 후 만료 시간 설정)
	result, err := l.client.Eval(ctx, `
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("EXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`, []string{l.lockKey}, l.ownerID, int(ttl.Seconds()))

	if err != nil {
		return false, fmt.Errorf("failed to refresh lock: %w", err)
	}

	// 결과 확인
	success := false
	if val, ok := result.(int64); ok && val > 0 {
		success = true
	}

	return success, nil
}

// startAutoRefresh는 락 자동 갱신을 시작합니다.
func (l *RedisDistributedLock) startAutoRefresh(ctx context.Context, ttl time.Duration) {
	// 이전 타이커 중지
	if l.refreshTicker != nil {
		l.stopRefresh <- struct{}{}
	}

	// 갱신 간격은 TTL의 1/3로 설정
	refreshInterval := ttl / 3
	if refreshInterval < time.Second {
		refreshInterval = time.Second
	}

	l.refreshTicker = time.NewTicker(refreshInterval)
	go func() {
		for {
			select {
			case <-l.stopRefresh:
				l.refreshTicker.Stop()
				return
			case <-l.refreshTicker.C:
				// 컨텍스트 타임아웃 설정
				refreshCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, err := l.Refresh(refreshCtx, ttl)
				cancel()
				if err != nil {
					// 갱신 실패 로깅
					fmt.Printf("Failed to refresh lock for resource %s: %v\n", l.resourceID, err)
				}
			}
		}
	}()
}

// RedisDistributedLockManager는 Redis 기반 분산 락 관리자 구현체입니다.
type RedisDistributedLockManager struct {
	// client는 Redis 클라이언트입니다.
	client RedisClient
}

// NewRedisDistributedLockManager는 새 Redis 분산 락 관리자를 생성합니다.
func NewRedisDistributedLockManager(client RedisClient) *RedisDistributedLockManager {
	return &RedisDistributedLockManager{
		client: client,
	}
}

// GetLock은 지정된 리소스에 대한 분산 락을 반환합니다.
func (m *RedisDistributedLockManager) GetLock(resourceID string, ownerID string) DistributedLock {
	return NewRedisDistributedLock(m.client, resourceID, ownerID)
}

// Close는 락 관리자를 닫습니다.
func (m *RedisDistributedLockManager) Close() error {
	// Redis 클라이언트는 외부에서 관리되므로 여기서 닫지 않음
	return nil
}

// NoOpDistributedLockManager는 아무 작업도 수행하지 않는 분산 락 관리자 구현체입니다.
// 테스트나 단일 노드 환경에서 사용할 수 있습니다.
type NoOpDistributedLockManager struct{}

// NoOpDistributedLock은 아무 작업도 수행하지 않는 분산 락 구현체입니다.
type NoOpDistributedLock struct{}

// NewNoOpDistributedLockManager는 새 NoOp 분산 락 관리자를 생성합니다.
func NewNoOpDistributedLockManager() *NoOpDistributedLockManager {
	return &NoOpDistributedLockManager{}
}

// GetLock은 지정된 리소스에 대한 분산 락을 반환합니다.
func (m *NoOpDistributedLockManager) GetLock(resourceID string, ownerID string) DistributedLock {
	return &NoOpDistributedLock{}
}

// Close는 락 관리자를 닫습니다.
func (m *NoOpDistributedLockManager) Close() error {
	return nil
}

// Acquire는 락을 획득합니다.
func (l *NoOpDistributedLock) Acquire(ctx context.Context, timeout time.Duration) (bool, error) {
	return true, nil
}

// Release는 락을 해제합니다.
func (l *NoOpDistributedLock) Release(ctx context.Context) (bool, error) {
	return true, nil
}

// Refresh는 락의 만료 시간을 갱신합니다.
func (l *NoOpDistributedLock) Refresh(ctx context.Context, ttl time.Duration) (bool, error) {
	return true, nil
}
