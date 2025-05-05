package crdtstorage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"

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
		PubSubType:                "memory",
		RedisAddr:                 "localhost:6379",
		RedisPassword:             "",
		RedisDB:                   0,
		KeyPrefix:                 "luvjson",
		SyncInterval:              time.Minute,
		AutoSave:                  true,
		AutoSaveInterval:          time.Minute * 5,
		PersistenceType:           "memory",
		PersistencePath:           "",
		EnableDistributedLock:     false,
		DistributedLockTimeout:    time.Minute,
		EnableTransactionTracking: false,
		SyncMethod:                "pubsub",
		MaxStreamLength:           10000,
		EnableSnapshots:           true,
		SnapshotInterval:          time.Hour,
		MaxSnapshots:              10,
		SnapshotOnSave:            false,
		SnapshotTableName:         "document_snapshots",
	}
}

// DefaultDocumentOptions는 기본 문서 옵션을 반환합니다.
func DefaultDocumentOptions() *DocumentOptions {
	return &DocumentOptions{
		AutoSave:               true,
		AutoSaveInterval:       time.Minute * 5,
		Metadata:               make(map[string]interface{}),
		OptimisticConcurrency:  false,
		MaxTransactionRetries:  3,
		RequireDistributedLock: false,
	}
}

// storageImpl은 Storage 인터페이스의 구현체입니다.
type storageImpl struct {
	// options는 저장소 옵션입니다.
	options *StorageOptions

	// pubsub은 PubSub 인스턴스입니다.
	pubsub crdtpubsub.PubSub

	// persistence는 영구 저장소 어댑터입니다.
	persistence PersistenceAdapter

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

	// lockManager는 분산 락 관리자입니다.
	// 분산 환경에서 트랜잭션을 보장하는 데 사용됩니다.
	lockManager DistributedLockManager

	// transactionManager는 트랜잭션 관리자입니다.
	// 분산 환경에서 트랜잭션을 추적하고 관리하는 데 사용됩니다.
	transactionManager TransactionManager

	// serializer는 문서 직렬화/역직렬화를 담당합니다.
	serializer DocumentSerializer

	// syncManagerRegistry는 동기화 매니저 레지스트리입니다.
	// 여러 문서의 동기화를 관리합니다.
	syncManagerRegistry *SyncManagerRegistry
}

// NewStorage는 새 저장소를 생성합니다.
func NewStorage(ctx context.Context, options *StorageOptions) (Storage, error) {
	return NewStorageWithCustomPersistence(ctx, options, nil)
}

// NewStorageWithCustomPersistence는 사용자 정의 영구 저장소를 사용하여 새 저장소를 생성합니다.
func NewStorageWithCustomPersistence(ctx context.Context, options *StorageOptions, customPersistence PersistenceAdapter) (Storage, error) {
	if options == nil {
		options = DefaultStorageOptions()
	}

	// 컨텍스트 생성
	storageCtx, cancel := context.WithCancel(ctx)

	// 저장소 인스턴스 생성
	storage := &storageImpl{
		options:    options,
		documents:  make(map[string]*Document),
		ctx:        storageCtx,
		cancel:     cancel,
		serializer: NewDefaultDocumentSerializer(),
	}

	// PubSub 생성
	pubsub, err := createPubSub(storageCtx, options)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create PubSub: %w", err)
	}
	storage.pubsub = pubsub

	// 영구 저장소 생성
	persistence, err := createPersistenceAdapter(storageCtx, options, customPersistence)
	if err != nil {
		cancel()
		pubsub.Close()
		return nil, fmt.Errorf("failed to create persistence adapter: %w", err)
	}
	storage.persistence = persistence

	// Redis 클라이언트 저장 (락 관리자와 트랜잭션 관리자에서 사용)
	if options.PubSubType == "redis" || options.PersistenceType == "redis" {
		// Redis 클라이언트 생성
		redisClient := redis.NewClient(&redis.Options{
			Addr:     options.RedisAddr,
			Password: options.RedisPassword,
			DB:       options.RedisDB,
		})

		// Redis 연결 테스트
		if err := redisClient.Ping(storageCtx).Err(); err != nil {
			cancel()
			pubsub.Close()
			persistence.Close()
			return nil, fmt.Errorf("failed to connect to Redis: %w", err)
		}

		storage.redisClient = redisClient

		// 분산 락 관리자 생성 (필요한 경우)
		if options.EnableDistributedLock {
			storage.lockManager = createLockManager(redisClient, options)
		}

		// 트랜잭션 관리자 생성 (필요한 경우)
		if options.EnableTransactionTracking {
			storage.transactionManager = createTransactionManager(redisClient, storage.lockManager, options)
		}
	} else {
		// 분산 락이나 트랜잭션 추적이 필요한 경우 더미 구현체 사용
		if options.EnableDistributedLock {
			storage.lockManager = NewNoOpDistributedLockManager()
		}

		if options.EnableTransactionTracking {
			storage.transactionManager = NewNoOpTransactionManager()
		}
	}

	// 동기화 매니저 레지스트리 생성
	syncOptions := crdtsync.DefaultSyncOptions()

	// PubSub 유형에 따라 동기화 유형 설정
	switch options.PubSubType {
	case "memory":
		syncOptions.SyncType = crdtsync.SyncTypeMemory
	case "redis":
		if options.SyncMethod == "streams" {
			syncOptions.SyncType = crdtsync.SyncTypeRedisStreams
		} else {
			syncOptions.SyncType = crdtsync.SyncTypeRedisPubSub
		}
	default:
		syncOptions.SyncType = crdtsync.SyncTypeMemory
	}

	// Redis 설정
	if storage.redisClient != nil {
		syncOptions.RedisAddr = options.RedisAddr
		syncOptions.RedisPassword = options.RedisPassword
		syncOptions.RedisDB = options.RedisDB
	}

	// 인코딩 형식 설정
	syncOptions.EncodingFormat = crdtpubsub.EncodingFormatJSON

	// 최대 스트림 길이 설정
	syncOptions.MaxStreamLength = options.MaxStreamLength

	// 동기화 매니저 레지스트리 생성
	syncManagerRegistry, err := NewSyncManagerRegistry(storageCtx, syncOptions)
	if err != nil {
		cancel()
		pubsub.Close()
		persistence.Close()
		if storage.redisClient != nil {
			storage.redisClient.Close()
		}
		return nil, fmt.Errorf("failed to create sync manager registry: %w", err)
	}
	storage.syncManagerRegistry = syncManagerRegistry

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
			Addr:     options.RedisAddr,
			Password: options.RedisPassword,
			DB:       options.RedisDB,
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

// 이 함수는 persistence_factory.go 파일로 이동되었습니다.

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

	// 패치 빌더 생성
	patchBuilder := crdtpatch.NewPatchBuilder(sessionID, crdtDoc.NextTimestamp().Counter)

	// 문서 컨텍스트 생성
	docCtx, docCancel := context.WithCancel(s.ctx)

	// 문서 인스턴스 생성
	doc := &Document{
		ID:                documentID,
		CRDTDoc:           crdtDoc,
		PatchBuilder:      patchBuilder,
		SessionID:         sessionID,
		LastModified:      time.Now(),
		Metadata:          make(map[string]interface{}),
		ctx:               docCtx,
		cancel:            docCancel,
		autoSave:          s.options.AutoSave,
		autoSaveInterval:  s.options.AutoSaveInterval,
		onChangeCallbacks: make([]func(*Document, *crdtpatch.Patch), 0),
		// 뮤텍스 초기화는 기본값으로 자동 설정됨
		lockManager:        s.lockManager,
		transactionManager: s.transactionManager,
		Version:            1,
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
	if err := s.SaveDocument(ctx, doc); err != nil {
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

	// 패치 빌더 생성
	patchBuilder := crdtpatch.NewPatchBuilder(sessionID, crdtDoc.NextTimestamp().Counter)

	// 문서 컨텍스트 생성
	docCtx, docCancel := context.WithCancel(s.ctx)

	// 문서 인스턴스 생성
	doc = &Document{
		ID:                documentID,
		CRDTDoc:           crdtDoc,
		PatchBuilder:      patchBuilder,
		SessionID:         sessionID,
		LastModified:      time.Now(),
		Metadata:          make(map[string]interface{}),
		ctx:               docCtx,
		cancel:            docCancel,
		autoSave:          s.options.AutoSave,
		autoSaveInterval:  s.options.AutoSaveInterval,
		onChangeCallbacks: make([]func(*Document, *crdtpatch.Patch), 0),
		// 뮤텍스 초기화는 기본값으로 자동 설정됨
		lockManager:        s.lockManager,
		transactionManager: s.transactionManager,
		Version:            1,
	}

	// 문서 데이터 역직렬화
	if err := s.serializer.Deserialize(doc, data); err != nil {
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
	_, err := s.GetDocument(ctx, documentID)
	if err != nil {
		return fmt.Errorf("failed to get document: %w", err)
	}

	// 동기화 매니저 레지스트리를 통해 문서 동기화
	return s.syncManagerRegistry.SyncDocument(ctx, documentID, peerID)
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
		// 문서가 이미 로드되어 있는지 확인
		s.mutex.RLock()
		_, exists := s.documents[documentID]
		s.mutex.RUnlock()

		// 로드되어 있지 않은 경우 로드
		if !exists {
			if _, err := s.GetDocument(ctx, documentID); err != nil {
				syncErrors = append(syncErrors, fmt.Errorf("failed to load document %s: %w", documentID, err))
				continue
			}
		}

		// 동기화 매니저 레지스트리를 통해 문서 동기화
		if err := s.syncManagerRegistry.SyncDocument(ctx, documentID, peerID); err != nil {
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
		return fmt.Errorf("%s", errMsg)
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

	// 동기화 매니저 레지스트리 닫기
	if s.syncManagerRegistry != nil {
		if err := s.syncManagerRegistry.Close(); err != nil {
			return fmt.Errorf("failed to close sync manager registry: %w", err)
		}
	}

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

// SaveDocument는 문서를 저장합니다.
func (s *storageImpl) SaveDocument(ctx context.Context, doc *Document) error {
	// 마지막 수정 시간 업데이트
	doc.LastModified = time.Now()

	// 영구 저장소에 저장
	// Document 객체를 직접 전달하여 사용자가 필요에 맞게 데이터를 인덱싱하고 저장 쿼리를 작성할 수 있도록 함
	return s.persistence.SaveDocument(ctx, doc)
}

// setupSyncManager는 문서의 동기화 매니저를 설정합니다.
func (s *storageImpl) setupSyncManager(doc *Document) error {
	// 문서를 동기화 매니저 레지스트리에 등록
	if err := s.syncManagerRegistry.RegisterDocument(doc); err != nil {
		return fmt.Errorf("failed to register document with sync manager registry: %w", err)
	}

	return nil
}

// createLockManager는 분산 락 관리자를 생성합니다.
func createLockManager(redisClient *redis.Client, options *StorageOptions) DistributedLockManager {
	// Redis 클라이언트를 RedisClient 인터페이스로 래핑
	client := NewRedisClientAdapter(redisClient)

	// Redis 분산 락 관리자 생성
	return NewRedisDistributedLockManager(client)
}

// createTransactionManager는 트랜잭션 관리자를 생성합니다.
func createTransactionManager(redisClient *redis.Client, lockManager DistributedLockManager, options *StorageOptions) TransactionManager {
	// Redis 클라이언트를 RedisClient 인터페이스로 래핑
	client := NewRedisClientAdapter(redisClient)

	// Redis 트랜잭션 관리자 생성
	return NewRedisTransactionManager(client, lockManager, options.KeyPrefix)
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
