package crdtsync

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisPeerDiscovery는 Redis를 사용하는 피어 발견 구현입니다.
type RedisPeerDiscovery struct {
	// client는 Redis 클라이언트입니다.
	client *redis.Client

	// keyPrefix는 Redis 키 접두사입니다.
	keyPrefix string

	// peerID는 이 노드의 피어 ID입니다.
	peerID string

	// ttl은 피어 등록의 TTL입니다.
	ttl time.Duration

	// heartbeatInterval은 하트비트 간격입니다.
	heartbeatInterval time.Duration

	// ctx는 피어 발견의 컨텍스트입니다.
	ctx context.Context

	// cancel은 컨텍스트 취소 함수입니다.
	cancel context.CancelFunc

	// running은 피어 발견이 실행 중인지 여부를 나타냅니다.
	running bool
}

// NewRedisPeerDiscovery는 새 Redis 피어 발견을 생성합니다.
func NewRedisPeerDiscovery(client *redis.Client, keyPrefix string, peerID string) *RedisPeerDiscovery {
	return &RedisPeerDiscovery{
		client:           client,
		keyPrefix:        keyPrefix,
		peerID:           peerID,
		ttl:              time.Minute * 5,
		heartbeatInterval: time.Minute,
	}
}

// Start는 피어 발견을 시작합니다.
func (pd *RedisPeerDiscovery) Start(ctx context.Context) error {
	if pd.running {
		return fmt.Errorf("peer discovery is already running")
	}

	// 컨텍스트 생성
	pd.ctx, pd.cancel = context.WithCancel(ctx)

	// 자신을 등록
	if err := pd.RegisterPeer(pd.ctx, pd.peerID); err != nil {
		return fmt.Errorf("failed to register self: %w", err)
	}

	// 하트비트 시작
	go pd.heartbeat()

	pd.running = true
	return nil
}

// DiscoverPeers는 사용 가능한 피어를 발견합니다.
func (pd *RedisPeerDiscovery) DiscoverPeers(ctx context.Context) ([]string, error) {
	// 모든 피어 키 가져오기
	pattern := fmt.Sprintf("%s:peers:*", pd.keyPrefix)
	keys, err := pd.client.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get peer keys: %w", err)
	}

	// 피어 ID 추출
	peers := make([]string, 0, len(keys))
	for _, key := range keys {
		// 키에서 피어 ID 추출
		peerID := key[len(pd.keyPrefix)+7:] // ":peers:" 길이는 7
		
		// 자신은 제외
		if peerID != pd.peerID {
			peers = append(peers, peerID)
		}
	}

	return peers, nil
}

// RegisterPeer는 피어를 등록합니다.
func (pd *RedisPeerDiscovery) RegisterPeer(ctx context.Context, peerID string) error {
	// 피어 키 생성
	key := fmt.Sprintf("%s:peers:%s", pd.keyPrefix, peerID)

	// 현재 시간 저장
	now := time.Now().Unix()
	
	// 피어 등록
	if err := pd.client.Set(ctx, key, now, pd.ttl).Err(); err != nil {
		return fmt.Errorf("failed to register peer: %w", err)
	}

	return nil
}

// UnregisterPeer는 피어 등록을 해제합니다.
func (pd *RedisPeerDiscovery) UnregisterPeer(ctx context.Context, peerID string) error {
	// 피어 키 생성
	key := fmt.Sprintf("%s:peers:%s", pd.keyPrefix, peerID)
	
	// 피어 등록 해제
	if err := pd.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to unregister peer: %w", err)
	}

	return nil
}

// Close는 피어 발견 서비스를 종료합니다.
func (pd *RedisPeerDiscovery) Close() error {
	if !pd.running {
		return nil
	}

	// 컨텍스트 취소
	if pd.cancel != nil {
		pd.cancel()
	}

	// 자신의 등록 해제
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	
	if err := pd.UnregisterPeer(ctx, pd.peerID); err != nil {
		return fmt.Errorf("failed to unregister self: %w", err)
	}

	pd.running = false
	return nil
}

// heartbeat는 주기적으로 자신의 등록을 갱신합니다.
func (pd *RedisPeerDiscovery) heartbeat() {
	ticker := time.NewTicker(pd.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pd.ctx.Done():
			return
		case <-ticker.C:
			// 자신 재등록
			if err := pd.RegisterPeer(pd.ctx, pd.peerID); err != nil {
				fmt.Printf("Error in heartbeat: %v\n", err)
			}
		}
	}
}
