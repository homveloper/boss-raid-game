package crdtsync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
)

// syncManagerImpl은 SyncManager 인터페이스의 구현입니다.
type syncManagerImpl struct {
	// doc은 CRDT 문서입니다.
	doc *crdt.Document

	// broadcaster는 패치 브로드캐스팅에 사용됩니다.
	broadcaster Broadcaster

	// syncer는 상태 동기화에 사용됩니다.
	syncer Syncer

	// peerDiscovery는 피어 발견에 사용됩니다.
	peerDiscovery PeerDiscovery

	// patchStore는 패치 저장에 사용됩니다.
	patchStore PatchStore

	// stateVector는 이 노드의 상태 벡터입니다.
	stateVector *StateVector

	// syncInterval은 자동 동기화 간격입니다.
	syncInterval time.Duration

	// ctx는 동기화 매니저의 컨텍스트입니다.
	ctx context.Context

	// cancel은 컨텍스트 취소 함수입니다.
	cancel context.CancelFunc

	// mutex는 동기화 매니저에 대한 동시 접근을 보호합니다.
	mutex sync.RWMutex

	// running은 동기화 매니저가 실행 중인지 여부를 나타냅니다.
	running bool
}

// NewSyncManager는 새 동기화 매니저를 생성합니다.
func NewSyncManager(doc *crdt.Document, broadcaster Broadcaster, syncer Syncer, peerDiscovery PeerDiscovery, patchStore PatchStore) SyncManager {
	return &syncManagerImpl{
		doc:           doc,
		broadcaster:   broadcaster,
		syncer:        syncer,
		peerDiscovery: peerDiscovery,
		patchStore:    patchStore,
		stateVector:   NewStateVector(),
		syncInterval:  time.Minute,
	}
}

// Start는 동기화 매니저를 시작합니다.
func (sm *syncManagerImpl) Start(ctx context.Context) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if sm.running {
		return fmt.Errorf("sync manager is already running")
	}

	// 컨텍스트 생성
	sm.ctx, sm.cancel = context.WithCancel(ctx)

	// 브로드캐스트 리스너 시작
	go sm.listenForBroadcasts()

	// 주기적 동기화 시작
	go sm.periodicSync()

	sm.running = true
	return nil
}

// Stop은 동기화 매니저를 중지합니다.
func (sm *syncManagerImpl) Stop() error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	if !sm.running {
		return nil
	}

	// 컨텍스트 취소
	if sm.cancel != nil {
		sm.cancel()
	}

	sm.running = false
	return nil
}

// ApplyPatch는 패치를 적용하고 브로드캐스트합니다.
func (sm *syncManagerImpl) ApplyPatch(ctx context.Context, patch *crdtpatch.Patch) error {
	// 패치 적용
	if err := patch.Apply(sm.doc); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	// 상태 벡터 업데이트
	sm.stateVector.Update(patch.ID())

	// 패치 저장
	if err := sm.patchStore.StorePatch(patch); err != nil {
		return fmt.Errorf("failed to store patch: %w", err)
	}

	// 패치 브로드캐스트
	if err := sm.broadcaster.Broadcast(ctx, patch); err != nil {
		return fmt.Errorf("failed to broadcast patch: %w", err)
	}

	return nil
}

// SyncWithPeer는 특정 피어와 동기화합니다.
func (sm *syncManagerImpl) SyncWithPeer(ctx context.Context, peerID string) error {
	return sm.syncer.Sync(ctx, peerID)
}

// GetDocument는 CRDT 문서를 반환합니다.
func (sm *syncManagerImpl) GetDocument() *crdt.Document {
	return sm.doc
}

// GetStateVector는 현재 상태 벡터를 반환합니다.
func (sm *syncManagerImpl) GetStateVector() map[string]uint64 {
	return sm.stateVector.Get()
}

// GetPeerDiscovery는 피어 발견 기능을 반환합니다.
func (sm *syncManagerImpl) GetPeerDiscovery() PeerDiscovery {
	return sm.peerDiscovery
}

// SyncWithAllPeers는 모든 사용 가능한 피어와 동기화를 수행합니다.
func (sm *syncManagerImpl) SyncWithAllPeers(ctx context.Context) error {
	// 피어 발견이 없는 경우 오류 반환
	if sm.peerDiscovery == nil {
		return fmt.Errorf("peer discovery is not available")
	}

	// 사용 가능한 피어 발견
	peers, err := sm.peerDiscovery.DiscoverPeers(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover peers: %w", err)
	}

	// 발견된 피어가 없는 경우
	if len(peers) == 0 {
		return nil // 오류가 아니라 성공으로 처리
	}

	// 각 피어와 동기화
	var syncErrors []error
	for _, peer := range peers {
		if err := sm.SyncWithPeer(ctx, peer); err != nil {
			// 하나의 피어 동기화 실패를 전체 실패로 처리하지 않고 계속 진행
			syncErrors = append(syncErrors, fmt.Errorf("failed to sync with peer %s: %w", peer, err))
		}
	}

	// 오류가 있는 경우 요약하여 반환
	if len(syncErrors) > 0 {
		errMsg := fmt.Sprintf("failed to sync with %d peers:", len(syncErrors))
		for i, err := range syncErrors {
			errMsg += fmt.Sprintf("\n  %d. %v", i+1, err)
		}
		return fmt.Errorf(errMsg)
	}

	return nil
}

// listenForBroadcasts는 브로드캐스트된 패치를 수신합니다.
func (sm *syncManagerImpl) listenForBroadcasts() {
	for {
		select {
		case <-sm.ctx.Done():
			return
		default:
			// 다음 패치 수신
			patch, err := sm.broadcaster.Next(sm.ctx)
			if err != nil {
				// 컨텍스트 취소 에러는 무시
				if sm.ctx.Err() != nil {
					return
				}
				// 다른 에러는 로깅하고 계속 진행
				fmt.Printf("Error receiving broadcast: %v\n", err)
				continue
			}

			// 패치 적용
			if err := patch.Apply(sm.doc); err != nil {
				fmt.Printf("Error applying broadcast patch: %v\n", err)
				continue
			}

			// 상태 벡터 업데이트
			sm.stateVector.Update(patch.ID())

			// 패치 저장
			if err := sm.patchStore.StorePatch(patch); err != nil {
				fmt.Printf("Error storing broadcast patch: %v\n", err)
				continue
			}
		}
	}
}

// periodicSync는 주기적으로 피어와 동기화합니다.
func (sm *syncManagerImpl) periodicSync() {
	ticker := time.NewTicker(sm.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			// 피어 발견
			peers, err := sm.peerDiscovery.DiscoverPeers(sm.ctx)
			if err != nil {
				fmt.Printf("Error discovering peers: %v\n", err)
				continue
			}

			// 각 피어와 동기화
			for _, peerID := range peers {
				if err := sm.SyncWithPeer(sm.ctx, peerID); err != nil {
					fmt.Printf("Error syncing with peer %s: %v\n", peerID, err)
				}
			}
		}
	}
}
