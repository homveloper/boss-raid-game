package crdtstorage

import (
	"context"
	"fmt"
	"sync"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtsync"
)

// SyncManagerRegistry는 여러 문서에 대한 동기화를 관리하는 레지스트리입니다.
// 이 레지스트리는 단일 SyncManager 인스턴스를 사용하여 여러 문서의 동기화를 관리합니다.
type SyncManagerRegistry struct {
	// syncManager는 공유 동기화 매니저입니다.
	syncManager crdtsync.SyncManager

	// documents는 문서 ID와 CRDT 문서의 맵입니다.
	documents map[string]*registeredDocument

	// mutex는 레지스트리에 대한 동시 접근을 보호합니다.
	mutex sync.RWMutex

	// ctx는 레지스트리의 컨텍스트입니다.
	ctx context.Context

	// cancel은 컨텍스트 취소 함수입니다.
	cancel context.CancelFunc

	// syncOptions는 동기화 옵션입니다.
	syncOptions *crdtsync.SyncOptions
}

// registeredDocument는 레지스트리에 등록된 문서 정보를 나타냅니다.
type registeredDocument struct {
	// doc은 CRDT 문서입니다.
	doc *crdt.Document

	// documentID는 문서 ID입니다.
	documentID string

	// crdtDoc은 문서 인스턴스에 대한 참조입니다.
	crdtDoc *Document
}

// NewSyncManagerRegistry는 새 동기화 매니저 레지스트리를 생성합니다.
func NewSyncManagerRegistry(ctx context.Context, syncOptions *crdtsync.SyncOptions) (*SyncManagerRegistry, error) {
	if syncOptions == nil {
		syncOptions = crdtsync.DefaultSyncOptions()
	}

	// 컨텍스트 생성
	registryCtx, cancel := context.WithCancel(ctx)

	// 레지스트리 인스턴스 생성
	registry := &SyncManagerRegistry{
		documents:   make(map[string]*registeredDocument),
		ctx:         registryCtx,
		cancel:      cancel,
		syncOptions: syncOptions,
	}

	// 공유 동기화 매니저 생성
	// 여기서는 더미 문서를 사용하여 초기화하고, 나중에 실제 문서를 등록합니다.
	dummyDoc := crdt.NewDocument(common.NewSessionID())
	syncManager, err := crdtsync.CreateSyncManager(
		registryCtx,
		dummyDoc,
		"registry",
		syncOptions,
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create sync manager: %w", err)
	}

	// 동기화 매니저 시작
	if err := syncManager.Start(registryCtx); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start sync manager: %w", err)
	}

	registry.syncManager = syncManager

	return registry, nil
}

// RegisterDocument는 문서를 레지스트리에 등록합니다.
func (r *SyncManagerRegistry) RegisterDocument(doc *Document) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 이미 등록된 문서인지 확인
	if _, exists := r.documents[doc.ID]; exists {
		return fmt.Errorf("document already registered: %s", doc.ID)
	}

	// 등록된 문서 정보 생성
	regDoc := &registeredDocument{
		doc:        doc.CRDTDoc,
		documentID: doc.ID,
		crdtDoc:    doc,
	}

	// 문서 맵에 추가
	r.documents[doc.ID] = regDoc

	// 문서에 동기화 매니저 설정
	// 여기서는 공유 동기화 매니저를 사용하지만, 문서별 동작을 위한 어댑터를 제공합니다.
	doc.SyncManager = &documentSyncManagerAdapter{
		registry:   r,
		documentID: doc.ID,
	}

	return nil
}

// UnregisterDocument는 문서를 레지스트리에서 등록 해제합니다.
func (r *SyncManagerRegistry) UnregisterDocument(documentID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 등록된 문서인지 확인
	if _, exists := r.documents[documentID]; !exists {
		return fmt.Errorf("document not registered: %s", documentID)
	}

	// 문서 맵에서 제거
	delete(r.documents, documentID)

	return nil
}

// GetDocument는 문서 ID로 등록된 문서를 가져옵니다.
func (r *SyncManagerRegistry) GetDocument(documentID string) (*registeredDocument, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// 등록된 문서인지 확인
	doc, exists := r.documents[documentID]
	if !exists {
		return nil, fmt.Errorf("document not registered: %s", documentID)
	}

	return doc, nil
}

// SyncDocument는 특정 문서를 동기화합니다.
func (r *SyncManagerRegistry) SyncDocument(ctx context.Context, documentID string, peerID string) error {
	// 등록된 문서 가져오기
	_, err := r.GetDocument(documentID)
	if err != nil {
		return err
	}

	// 특정 피어가 지정된 경우 해당 피어와만 동기화
	if peerID != "" {
		return r.syncManager.SyncWithPeer(ctx, peerID)
	}

	// 모든 피어와 동기화
	return r.syncManager.SyncWithAllPeers(ctx)
}

// ApplyPatch는 패치를 적용하고 브로드캐스트합니다.
func (r *SyncManagerRegistry) ApplyPatch(ctx context.Context, documentID string, patch *crdtpatch.Patch) error {
	// 등록된 문서 가져오기
	doc, err := r.GetDocument(documentID)
	if err != nil {
		return err
	}

	// 패치 적용
	if err := patch.Apply(doc.doc); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	// 패치 브로드캐스트
	return r.syncManager.ApplyPatch(ctx, patch)
}

// Close는 레지스트리를 닫습니다.
func (r *SyncManagerRegistry) Close() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// 컨텍스트 취소
	r.cancel()

	// 동기화 매니저 중지
	if r.syncManager != nil {
		if err := r.syncManager.Stop(); err != nil {
			return fmt.Errorf("failed to stop sync manager: %w", err)
		}
	}

	// 문서 맵 초기화
	r.documents = make(map[string]*registeredDocument)

	return nil
}

// documentSyncManagerAdapter는 문서별 동기화 매니저 어댑터입니다.
// 이 어댑터는 공유 동기화 매니저를 사용하지만, 문서별 동작을 제공합니다.
type documentSyncManagerAdapter struct {
	registry   *SyncManagerRegistry
	documentID string
}

// Start는 동기화 매니저를 시작합니다.
func (a *documentSyncManagerAdapter) Start(ctx context.Context) error {
	// 이미 시작되어 있으므로 아무것도 하지 않음
	return nil
}

// Stop은 동기화 매니저를 중지합니다.
func (a *documentSyncManagerAdapter) Stop() error {
	// 공유 동기화 매니저를 중지하지 않고, 등록 해제만 수행
	return a.registry.UnregisterDocument(a.documentID)
}

// ApplyPatch는 패치를 적용하고 브로드캐스트합니다.
func (a *documentSyncManagerAdapter) ApplyPatch(ctx context.Context, patch *crdtpatch.Patch) error {
	return a.registry.ApplyPatch(ctx, a.documentID, patch)
}

// SyncWithPeer는 특정 피어와 동기화합니다.
func (a *documentSyncManagerAdapter) SyncWithPeer(ctx context.Context, peerID string) error {
	return a.registry.SyncDocument(ctx, a.documentID, peerID)
}

// GetDocument는 CRDT 문서를 반환합니다.
func (a *documentSyncManagerAdapter) GetDocument() *crdt.Document {
	doc, err := a.registry.GetDocument(a.documentID)
	if err != nil {
		return nil
	}
	return doc.doc
}

// GetStateVector는 현재 상태 벡터를 반환합니다.
func (a *documentSyncManagerAdapter) GetStateVector() map[string]uint64 {
	return a.registry.syncManager.GetStateVector()
}

// GetPeerDiscovery는 피어 발견 기능을 반환합니다.
func (a *documentSyncManagerAdapter) GetPeerDiscovery() crdtsync.PeerDiscovery {
	return a.registry.syncManager.GetPeerDiscovery()
}

// SyncWithAllPeers는 모든 사용 가능한 피어와 동기화를 수행합니다.
func (a *documentSyncManagerAdapter) SyncWithAllPeers(ctx context.Context) error {
	return a.registry.SyncDocument(ctx, a.documentID, "")
}
