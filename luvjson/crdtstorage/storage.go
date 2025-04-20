package crdtstorage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"

	"tictactoe/luvjson/api"
	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtpubsub"
	redispubsub "tictactoe/luvjson/crdtpubsub"
	"tictactoe/luvjson/crdtpubsub/memory"
	"tictactoe/luvjson/crdtsync"
)

// DefaultStorageOptions는 기본 저장소 옵션을 반환합니다.
func DefaultStorageOptions() *StorageOptions {
	return &StorageOptions{
		PubSubType:       "memory",
		RedisAddr:        "localhost:6379",
		KeyPrefix:        "luvjson",
		SyncInterval:     time.Minute,
		AutoSave:         true,
		AutoSaveInterval: time.Minute * 5,
		PersistenceType:  "memory",
		PersistencePath:  "",
	}
}

// DefaultDocumentOptions는 기본 문서 옵션을 반환합니다.
func DefaultDocumentOptions() *DocumentOptions {
	return &DocumentOptions{
		AutoSave:         true,
		AutoSaveInterval: time.Minute * 5,
		Metadata:         make(map[string]interface{}),
	}
}

// storageImpl은 Storage 인터페이스의 구현체입니다.
type storageImpl struct {
	// options는 저장소 옵션입니다.
	options *StorageOptions

	// pubsub은 PubSub 인스턴스입니다.
	pubsub crdtpubsub.PubSub

	// persistence는 영구 저장소 인스턴스입니다.
	persistence PersistenceProvider

	// documents는 현재 로드된 문서 맵입니다.
	documents map[string]*Document

	// redisClient는 Redis 클라이언트입니다.
	redisClient *redis.Client

	// mutex는 문서 맵에 대한 동시 접근을 보호합니다.
	mutex sync.RWMutex

	// ctx는 저장소의 컨텍스트입니다.
	ctx context.Context

	// cancel은 컨텍스트 취소 함수입니다.
	cancel context.CancelFunc
}

// NewStorage는 새 저장소를 생성합니다.
func NewStorage(ctx context.Context, options *StorageOptions) (Storage, error) {
	if options == nil {
		options = DefaultStorageOptions()
	}

	// 컨텍스트 생성
	storageCtx, cancel := context.WithCancel(ctx)

	// 저장소 인스턴스 생성
	storage := &storageImpl{
		options:   options,
		documents: make(map[string]*Document),
		ctx:       storageCtx,
		cancel:    cancel,
	}

	// PubSub 생성
	pubsub, err := createPubSub(storageCtx, options)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create PubSub: %w", err)
	}
	storage.pubsub = pubsub

	// 영구 저장소 생성
	persistence, err := createPersistenceProvider(storageCtx, options)
	if err != nil {
		cancel()
		pubsub.Close()
		return nil, fmt.Errorf("failed to create persistence provider: %w", err)
	}
	storage.persistence = persistence

	return storage, nil
}

// createPubSub은 PubSub 인스턴스를 생성합니다.
func createPubSub(ctx context.Context, options *StorageOptions) (crdtpubsub.PubSub, error) {
	switch options.PubSubType {
	case "memory":
		return memory.NewPubSub()
	case "redis":
		// Redis 클라이언트 생성
		redisClient := redis.NewClient(&redis.Options{
			Addr: options.RedisAddr,
		})

		// Redis 연결 테스트
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("failed to connect to Redis: %w", err)
		}

		// Redis PubSub 생성
		pubsubOptions := crdtpubsub.NewOptions()
		pubsubOptions.DefaultFormat = crdtpubsub.EncodingFormatJSON
		return redispubsub.NewRedisPubSub(redisClient, pubsubOptions)
	default:
		return nil, fmt.Errorf("unsupported PubSub type: %s", options.PubSubType)
	}
}

// createPersistenceProvider는 영구 저장소 인스턴스를 생성합니다.
func createPersistenceProvider(ctx context.Context, options *StorageOptions) (PersistenceProvider, error) {
	switch options.PersistenceType {
	case "memory":
		return NewMemoryPersistence(), nil
	case "file":
		return NewFilePersistence(options.PersistencePath)
	case "redis":
		// Redis 클라이언트 생성
		redisClient := redis.NewClient(&redis.Options{
			Addr: options.RedisAddr,
		})

		// Redis 연결 테스트
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("failed to connect to Redis: %w", err)
		}

		return NewRedisPersistence(redisClient, options.KeyPrefix), nil
	default:
		return nil, fmt.Errorf("unsupported persistence type: %s", options.PersistenceType)
	}
}

// CreateDocument는 새 문서를 생성합니다.
func (s *storageImpl) CreateDocument(ctx context.Context, documentID string) (*Document, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 이미 존재하는 문서인지 확인
	if _, exists := s.documents[documentID]; exists {
		return nil, fmt.Errorf("document already exists: %s", documentID)
	}

	// 세션 ID 생성
	sessionID := common.NewSessionID()

	// CRDT 문서 생성
	crdtDoc := crdt.NewDocument(sessionID)

	// API 모델 생성
	model := api.NewModelWithDocument(crdtDoc)

	// 문서 컨텍스트 생성
	docCtx, docCancel := context.WithCancel(s.ctx)

	// 문서 인스턴스 생성
	doc := &Document{
		ID:                documentID,
		Model:             model,
		SessionID:         sessionID,
		LastModified:      time.Now(),
		Metadata:          make(map[string]interface{}),
		storage:           s,
		ctx:               docCtx,
		cancel:            docCancel,
		autoSave:          s.options.AutoSave,
		autoSaveInterval:  s.options.AutoSaveInterval,
		onChangeCallbacks: make([]func(*Document, *crdtpatch.Patch), 0),
	}

	// 동기화 매니저 설정
	if err := s.setupSyncManager(doc); err != nil {
		docCancel()
		return nil, fmt.Errorf("failed to setup sync manager: %w", err)
	}

	// 자동 저장 설정
	if doc.autoSave {
		go doc.startAutoSave()
	}

	// 문서 저장
	if err := s.saveDocument(ctx, doc); err != nil {
		docCancel()
		return nil, fmt.Errorf("failed to save document: %w", err)
	}

	// 문서 맵에 추가
	s.documents[documentID] = doc

	return doc, nil
}

// GetDocument는 문서 ID로 문서를 가져옵니다.
func (s *storageImpl) GetDocument(ctx context.Context, documentID string) (*Document, error) {
	s.mutex.RLock()
	doc, exists := s.documents[documentID]
	s.mutex.RUnlock()

	// 이미 로드된 문서인 경우 반환
	if exists {
		return doc, nil
	}

	// 문서 로드
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 다시 확인 (다른 고루틴에서 로드했을 수 있음)
	doc, exists = s.documents[documentID]
	if exists {
		return doc, nil
	}

	// 영구 저장소에서 문서 데이터 로드
	data, err := s.persistence.LoadDocument(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to load document: %w", err)
	}

	// 세션 ID 생성
	sessionID := common.NewSessionID()

	// CRDT 문서 생성
	crdtDoc := crdt.NewDocument(sessionID)

	// API 모델 생성
	model := api.NewModelWithDocument(crdtDoc)

	// 문서 컨텍스트 생성
	docCtx, docCancel := context.WithCancel(s.ctx)

	// 문서 인스턴스 생성
	doc = &Document{
		ID:                documentID,
		Model:             model,
		SessionID:         sessionID,
		LastModified:      time.Now(),
		Metadata:          make(map[string]interface{}),
		storage:           s,
		ctx:               docCtx,
		cancel:            docCancel,
		autoSave:          s.options.AutoSave,
		autoSaveInterval:  s.options.AutoSaveInterval,
		onChangeCallbacks: make([]func(*Document, *crdtpatch.Patch), 0),
	}

	// 문서 데이터 역직렬화
	if err := doc.deserialize(data); err != nil {
		docCancel()
		return nil, fmt.Errorf("failed to deserialize document: %w", err)
	}

	// 동기화 매니저 설정
	if err := s.setupSyncManager(doc); err != nil {
		docCancel()
		return nil, fmt.Errorf("failed to setup sync manager: %w", err)
	}

	// 자동 저장 설정
	if doc.autoSave {
		go doc.startAutoSave()
	}

	// 문서 맵에 추가
	s.documents[documentID] = doc

	return doc, nil
}

// ListDocuments는 모든 문서 목록을 반환합니다.
func (s *storageImpl) ListDocuments(ctx context.Context) ([]string, error) {
	return s.persistence.ListDocuments(ctx)
}

// DeleteDocument는 문서를 삭제합니다.
func (s *storageImpl) DeleteDocument(ctx context.Context, documentID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 문서가 로드되어 있는 경우 닫기
	if doc, exists := s.documents[documentID]; exists {
		doc.cancel()
		delete(s.documents, documentID)
	}

	// 영구 저장소에서 문서 삭제
	return s.persistence.DeleteDocument(ctx, documentID)
}

// SyncDocument는 특정 문서를 동기화합니다.
func (s *storageImpl) SyncDocument(ctx context.Context, documentID string, peerID string) error {
	// 문서 가져오기
	doc, err := s.GetDocument(ctx, documentID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// 문서 동기화
	return doc.Sync(ctx, peerID)
}

// SyncAllDocuments는 모든 문서를 동기화합니다.
func (s *storageImpl) SyncAllDocuments(ctx context.Context, peerID string) error {
	// 모든 문서 목록 가져오기
	documentIDs, err := s.ListDocuments(ctx)
	if err != nil {
		return fmt.Errorf("failed to list documents: %w", err)
	}

	// 각 문서 동기화
	var syncErrors []error
	for _, documentID := range documentIDs {
		if err := s.SyncDocument(ctx, documentID, peerID); err != nil {
			// 하나의 문서 동기화 실패를 전체 실패로 처리하지 않고 계속 진행
			syncErrors = append(syncErrors, fmt.Errorf("failed to sync document %s: %w", documentID, err))
		}
	}

	// 오류가 있는 경우 요약하여 반환
	if len(syncErrors) > 0 {
		errMsg := fmt.Sprintf("failed to sync %d documents:", len(syncErrors))
		for i, err := range syncErrors {
			errMsg += fmt.Sprintf("\n  %d. %v", i+1, err)
		}
		return fmt.Errorf(errMsg)
	}

	return nil
}

// Close는 저장소를 닫습니다.
func (s *storageImpl) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// 컨텍스트 취소
	s.cancel()

	// 모든 문서 닫기
	for _, doc := range s.documents {
		doc.cancel()
	}
	s.documents = make(map[string]*Document)

	// PubSub 닫기
	if err := s.pubsub.Close(); err != nil {
		return fmt.Errorf("failed to close PubSub: %w", err)
	}

	// 영구 저장소 닫기
	if err := s.persistence.Close(); err != nil {
		return fmt.Errorf("failed to close persistence: %w", err)
	}

	// Redis 클라이언트 닫기
	if s.redisClient != nil {
		if err := s.redisClient.Close(); err != nil {
			return fmt.Errorf("failed to close Redis client: %w", err)
		}
	}

	return nil
}

// saveDocument는 문서를 영구 저장소에 저장합니다.
func (s *storageImpl) saveDocument(ctx context.Context, doc *Document) error {
	// 영구 저장소에 저장
	// Document 객체를 직접 전달하여 사용자가 필요에 맞게 데이터를 인덱싱하고 저장 쿼리를 작성할 수 있도록 함
	return s.persistence.SaveDocument(ctx, doc)
}

// setupSyncManager는 문서의 동기화 매니저를 설정합니다.
func (s *storageImpl) setupSyncManager(doc *Document) error {
	// 브로드캐스터 생성
	broadcaster := crdtsync.NewPubSubBroadcaster(
		s.pubsub,
		fmt.Sprintf("%s-patches", doc.ID),
		crdtpubsub.EncodingFormatJSON,
		doc.SessionID,
	)

	// 패치 저장소 생성
	patchStore := crdtsync.NewMemoryPatchStore()

	// 상태 벡터 생성
	stateVector := crdtsync.NewStateVector()

	// 피어 발견 생성
	var peerDiscovery crdtsync.PeerDiscovery
	if s.options.PubSubType == "redis" && s.redisClient != nil {
		// Redis 피어 발견 생성
		peerDiscovery = crdtsync.NewRedisPeerDiscovery(s.redisClient, doc.ID, doc.SessionID.String()[:8])
		if err := peerDiscovery.(*crdtsync.RedisPeerDiscovery).Start(doc.ctx); err != nil {
			return fmt.Errorf("failed to start peer discovery: %w", err)
		}
	} else {
		// 더미 피어 발견 생성
		peerDiscovery = &dummyPeerDiscovery{}
	}

	// 싱커 생성
	syncer := crdtsync.NewPubSubSyncer(
		s.pubsub,
		fmt.Sprintf("%s-sync", doc.ID),
		doc.SessionID.String()[:8],
		stateVector,
		patchStore,
		crdtpubsub.EncodingFormatJSON,
	)

	// 동기화 매니저 생성
	syncManager := crdtsync.NewSyncManager(doc.Model.GetDocument(), broadcaster, syncer, peerDiscovery, patchStore)

	// 동기화 매니저 시작
	if err := syncManager.Start(doc.ctx); err != nil {
		return fmt.Errorf("failed to start sync manager: %w", err)
	}

	// 문서에 동기화 매니저 설정
	doc.SyncManager = syncManager

	return nil
}

// dummyPeerDiscovery는 더미 피어 발견 구현입니다.
type dummyPeerDiscovery struct{}

func (d *dummyPeerDiscovery) DiscoverPeers(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (d *dummyPeerDiscovery) RegisterPeer(ctx context.Context, peerID string) error {
	return nil
}

func (d *dummyPeerDiscovery) UnregisterPeer(ctx context.Context, peerID string) error {
	return nil
}

func (d *dummyPeerDiscovery) Close() error {
	return nil
}
