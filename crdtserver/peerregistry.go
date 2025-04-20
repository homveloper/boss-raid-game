package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

const (
	// Redis 키 접두사
	peerSetKey = "crdt:peers"
	// 피어 TTL (초)
	peerTTL = 60
)

// PeerInfo는 피어 정보를 저장하는 구조체
type PeerInfo struct {
	ID        string    `json:"id"`
	Addrs     []string  `json:"addrs"`
	LastSeen  time.Time `json:"lastSeen"`
	StartTime time.Time `json:"startTime"`
}

// PeerRegistry는 Redis 기반 피어 레지스트리
type PeerRegistry struct {
	client      *redis.Client
	localPeerID string
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewPeerRegistry는 새 피어 레지스트리를 생성
func NewPeerRegistry(redisClient *redis.Client, localPeerID string) (*PeerRegistry, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 연결 테스트
	if err := redisClient.Ping(ctx).Err(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &PeerRegistry{
		client:      redisClient,
		localPeerID: localPeerID,
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

// Close는 레지스트리를 닫고 리소스를 정리
func (r *PeerRegistry) Close() error {
	r.cancel()
	return nil
}

// RegisterPeer는 피어 정보를 Redis에 등록
func (r *PeerRegistry) RegisterPeer(peerID string, addrs []multiaddr.Multiaddr) error {
	addrStrs := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrs[i] = addr.String()
	}

	peerInfo := PeerInfo{
		ID:        peerID,
		Addrs:     addrStrs,
		LastSeen:  time.Now(),
		StartTime: time.Now(),
	}

	data, err := json.Marshal(peerInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal peer info: %w", err)
	}

	// Redis에 피어 정보 저장
	err = r.client.HSet(r.ctx, peerSetKey, peerID, data).Err()
	if err != nil {
		return fmt.Errorf("failed to register peer: %w", err)
	}

	return nil
}

// UpdatePeer는 피어의 LastSeen 시간을 업데이트
func (r *PeerRegistry) UpdatePeer(peerID string, addrs []multiaddr.Multiaddr) error {
	// 기존 정보 조회
	data, err := r.client.HGet(r.ctx, peerSetKey, peerID).Bytes()
	if err != nil {
		if err == redis.Nil {
			// 피어가 없으면 새로 등록
			return r.RegisterPeer(peerID, addrs)
		}
		return fmt.Errorf("failed to get peer info: %w", err)
	}

	var peerInfo PeerInfo
	if err := json.Unmarshal(data, &peerInfo); err != nil {
		return fmt.Errorf("failed to unmarshal peer info: %w", err)
	}

	// 주소 업데이트
	addrStrs := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrs[i] = addr.String()
	}
	peerInfo.Addrs = addrStrs
	peerInfo.LastSeen = time.Now()

	// 업데이트된 정보 저장
	updatedData, err := json.Marshal(peerInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal updated peer info: %w", err)
	}

	err = r.client.HSet(r.ctx, peerSetKey, peerID, updatedData).Err()
	if err != nil {
		return fmt.Errorf("failed to update peer: %w", err)
	}

	return nil
}

// GetPeers는 모든 활성 피어 정보를 조회
func (r *PeerRegistry) GetPeers() ([]peer.AddrInfo, error) {
	// 모든 피어 정보 조회
	data, err := r.client.HGetAll(r.ctx, peerSetKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get peers: %w", err)
	}

	peers := make([]peer.AddrInfo, 0, len(data))
	now := time.Now()

	for _, peerData := range data {
		var peerInfo PeerInfo
		if err := json.Unmarshal([]byte(peerData), &peerInfo); err != nil {
			continue // 잘못된 형식의 데이터는 건너뜀
		}

		// TTL 체크 (마지막 업데이트 후 peerTTL초가 지났으면 비활성)
		if now.Sub(peerInfo.LastSeen) > time.Duration(peerTTL)*time.Second {
			continue
		}

		// 자신은 제외
		if peerInfo.ID == r.localPeerID {
			continue
		}

		// 주소 변환
		addrs := make([]multiaddr.Multiaddr, 0, len(peerInfo.Addrs))
		for _, addrStr := range peerInfo.Addrs {
			addr, err := multiaddr.NewMultiaddr(addrStr)
			if err != nil {
				continue
			}
			addrs = append(addrs, addr)
		}

		// 피어 ID 변환
		pID, err := peer.Decode(peerInfo.ID)
		if err != nil {
			continue
		}

		peers = append(peers, peer.AddrInfo{
			ID:    pID,
			Addrs: addrs,
		})
	}

	return peers, nil
}

// CleanupInactivePeers는 비활성 피어를 제거
func (r *PeerRegistry) CleanupInactivePeers() error {
	// 모든 피어 정보 조회
	data, err := r.client.HGetAll(r.ctx, peerSetKey).Result()
	if err != nil {
		return fmt.Errorf("failed to get peers for cleanup: %w", err)
	}

	now := time.Now()

	for peerID, peerData := range data {
		var peerInfo PeerInfo
		if err := json.Unmarshal([]byte(peerData), &peerInfo); err != nil {
			// 잘못된 형식의 데이터는 삭제
			r.client.HDel(r.ctx, peerSetKey, peerID)
			continue
		}

		// TTL 체크 (마지막 업데이트 후 peerTTL초가 지났으면 비활성)
		if now.Sub(peerInfo.LastSeen) > time.Duration(peerTTL)*time.Second {
			r.client.HDel(r.ctx, peerSetKey, peerID)
		}
	}

	return nil
}

// StartHeartbeat는 주기적인 상태 업데이트 및 정리 작업 시작
func (r *PeerRegistry) StartHeartbeat(h host.Host, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			// 자신의 상태 업데이트
			r.UpdatePeer(h.ID().String(), h.Addrs())

			// 비활성 피어 정리
			r.CleanupInactivePeers()
		}
	}
}
