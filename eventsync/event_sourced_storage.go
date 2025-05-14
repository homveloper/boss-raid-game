package eventsync

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"nodestorage/v2"
	"nodestorage/v2/cache"
)

// EventSourcedStorage는 이벤트 소싱 패턴을 구현한 저장소입니다.
// nodestorage.Storage 인터페이스와 유사한 메서드 이름을 사용하지만, 클라이언트 ID를 추가 인자로 받는 이벤트 소싱에 특화된 인터페이스를 제공합니다.
type EventSourcedStorage[T nodestorage.Cachable[T]] struct {
	storage       nodestorage.Storage[T]      // 내부적으로 실제 nodestorage 인스턴스 사용
	eventStore    EventStore                  // 이벤트 저장소
	snapshotStore SnapshotStore               // 스냅샷 저장소 (선택적)
	options       *EventSourcedStorageOptions // 옵션
	logger        *zap.Logger
}

// EventSourcedStorageOptions는 EventSourcedStorage의 옵션을 정의합니다.
type EventSourcedStorageOptions struct {
	// 스냅샷 저장소 (선택적)
	SnapshotStore SnapshotStore

	// 자동 스냅샷 생성 여부
	AutoSnapshot bool

	// 자동 스냅샷 생성 간격 (이벤트 개수)
	SnapshotInterval int64
}

// EventSourcedStorageConfig는 EventSourcedStorage 생성을 위한 설정을 정의합니다.
// 이 구조체는 더 이상 사용되지 않으며, 대신 NewEventSourcedStorageWithOptions 함수와 옵션 패턴을 사용하세요.
type EventSourcedStorageConfig struct {
	// 기본 저장소 설정
	StorageProvider StorageProvider // 저장소 제공자 인터페이스
	CollectionName  string          // 컬렉션/테이블 이름

	// 이벤트 저장소 설정
	EventCollectionName string // 이벤트 저장소 컬렉션/테이블 이름

	// 스냅샷 저장소 설정
	EnableSnapshot         bool   // 스냅샷 기능 활성화 여부
	SnapshotCollectionName string // 스냅샷 저장소 컬렉션/테이블 이름
	AutoSnapshot           bool   // 자동 스냅샷 생성 여부
	SnapshotInterval       int64  // 자동 스냅샷 생성 간격 (이벤트 개수)

	// 로깅 설정
	Logger *zap.Logger // 로거
}

// EventSourcedStorageOptions는 EventSourcedStorage 생성을 위한 옵션을 정의합니다.
type EventSourcedStorageFactoryOptions struct {
	// 이벤트 저장소 설정
	EventCollectionName string // 이벤트 저장소 컬렉션/테이블 이름

	// 스냅샷 저장소 설정
	EnableSnapshot         bool   // 스냅샷 기능 활성화 여부
	SnapshotCollectionName string // 스냅샷 저장소 컬렉션/테이블 이름
	AutoSnapshot           bool   // 자동 스냅샷 생성 여부
	SnapshotInterval       int64  // 자동 스냅샷 생성 간격 (이벤트 개수)

	// 로깅 설정
	Logger *zap.Logger // 로거
}

// EventSourcedStorageFactoryOption은 EventSourcedStorage 팩토리 옵션 함수 타입입니다.
type EventSourcedStorageFactoryOption func(*EventSourcedStorageFactoryOptions)

// WithEventCollectionName은 이벤트 저장소 컬렉션 이름을 설정하는 옵션 함수입니다.
func WithEventCollectionName(name string) EventSourcedStorageFactoryOption {
	return func(opts *EventSourcedStorageFactoryOptions) {
		opts.EventCollectionName = name
	}
}

// WithEnableSnapshot은 스냅샷 기능 활성화 여부를 설정하는 옵션 함수입니다.
func WithEnableSnapshot(enable bool) EventSourcedStorageFactoryOption {
	return func(opts *EventSourcedStorageFactoryOptions) {
		opts.EnableSnapshot = enable
	}
}

// WithSnapshotCollectionName은 스냅샷 저장소 컬렉션 이름을 설정하는 옵션 함수입니다.
func WithSnapshotCollectionName(name string) EventSourcedStorageFactoryOption {
	return func(opts *EventSourcedStorageFactoryOptions) {
		opts.SnapshotCollectionName = name
	}
}

// WithFactoryAutoSnapshot은 자동 스냅샷 생성 여부를 설정하는 옵션 함수입니다.
func WithFactoryAutoSnapshot(auto bool) EventSourcedStorageFactoryOption {
	return func(opts *EventSourcedStorageFactoryOptions) {
		opts.AutoSnapshot = auto
	}
}

// WithFactorySnapshotInterval은 자동 스냅샷 생성 간격을 설정하는 옵션 함수입니다.
func WithFactorySnapshotInterval(interval int64) EventSourcedStorageFactoryOption {
	return func(opts *EventSourcedStorageFactoryOptions) {
		opts.SnapshotInterval = interval
	}
}

// WithFactoryLogger는 로거를 설정하는 옵션 함수입니다.
func WithFactoryLogger(logger *zap.Logger) EventSourcedStorageFactoryOption {
	return func(opts *EventSourcedStorageFactoryOptions) {
		opts.Logger = logger
	}
}

// StorageProvider는 저장소 제공자 인터페이스입니다.
// 이 인터페이스를 구현하여 다양한 데이터베이스 시스템을 지원할 수 있습니다.
type StorageProvider interface {
	// GetEventStore는 지정된 컬렉션/테이블에 대한 EventStore 인스턴스를 반환합니다.
	GetEventStore(ctx context.Context, collectionName string) (EventStore, error)

	// GetSnapshotStore는 지정된 컬렉션/테이블에 대한 SnapshotStore 인스턴스를 반환합니다.
	GetSnapshotStore(ctx context.Context, collectionName string, eventStore EventStore) (SnapshotStore, error)
}

// MongoDBStorageProvider는 MongoDB 기반 저장소 제공자 구현체입니다.
type MongoDBStorageProvider struct {
	Client       *mongo.Client
	DatabaseName string
	Logger       *zap.Logger
}

// NewMongoDBStorageProvider는 새로운 MongoDB 저장소 제공자를 생성합니다.
func NewMongoDBStorageProvider(client *mongo.Client, databaseName string, logger *zap.Logger) *MongoDBStorageProvider {
	return &MongoDBStorageProvider{
		Client:       client,
		DatabaseName: databaseName,
		Logger:       logger,
	}
}

// CreateStorage는 지정된 컬렉션에 대한 타입이 지정된 nodestorage.Storage 인스턴스를 생성합니다.
// 제네릭 타입 T를 사용하는 함수로 구현하여 메서드의 타입 파라미터 제한을 우회합니다.
func CreateStorage[T nodestorage.Cachable[T]](ctx context.Context, provider *MongoDBStorageProvider, collectionName string) (nodestorage.Storage[T], error) {
	// 옵션 설정
	storageOpts := &nodestorage.Options{
		VersionField: "Version",
	}

	// 컬렉션 가져오기
	collection := provider.Client.Database(provider.DatabaseName).Collection(collectionName)

	// 메모리 캐시 생성
	memCache := cache.NewMemoryCache[T](nil)

	// nodestorage 생성
	return nodestorage.NewStorage[T](
		ctx,
		collection,
		memCache,
		storageOpts,
	)
}

// GetEventStore는 지정된 컬렉션에 대한 EventStore 인스턴스를 반환합니다.
func (p *MongoDBStorageProvider) GetEventStore(ctx context.Context, collectionName string) (EventStore, error) {
	return NewMongoEventStore(ctx, p.Client, p.DatabaseName, collectionName, p.Logger)
}

// GetSnapshotStore는 지정된 컬렉션에 대한 SnapshotStore 인스턴스를 반환합니다.
func (p *MongoDBStorageProvider) GetSnapshotStore(ctx context.Context, collectionName string, eventStore EventStore) (SnapshotStore, error) {
	return NewMongoSnapshotStore(ctx, p.Client, p.DatabaseName, collectionName, eventStore, p.Logger)
}

// NewEventSourcedStorage는 새로운 EventSourcedStorage 인스턴스를 생성합니다.
func NewEventSourcedStorage[T nodestorage.Cachable[T]](
	storage nodestorage.Storage[T],
	eventStore EventStore,
	logger *zap.Logger,
	options ...func(*EventSourcedStorageOptions),
) *EventSourcedStorage[T] {
	// 기본 옵션 설정
	opts := &EventSourcedStorageOptions{
		AutoSnapshot:     false,
		SnapshotInterval: 100, // 기본값: 100개 이벤트마다 스냅샷 생성
	}

	// 사용자 옵션 적용
	for _, option := range options {
		option(opts)
	}

	return &EventSourcedStorage[T]{
		storage:       storage,
		eventStore:    eventStore,
		snapshotStore: opts.SnapshotStore,
		options:       opts,
		logger:        logger,
	}
}

// WithSnapshotStore는 스냅샷 저장소를 설정하는 옵션 함수입니다.
func WithSnapshotStore(snapshotStore SnapshotStore) func(*EventSourcedStorageOptions) {
	return func(opts *EventSourcedStorageOptions) {
		opts.SnapshotStore = snapshotStore
	}
}

// WithAutoSnapshot은 자동 스냅샷 생성 여부를 설정하는 옵션 함수입니다.
func WithAutoSnapshot(autoSnapshot bool) func(*EventSourcedStorageOptions) {
	return func(opts *EventSourcedStorageOptions) {
		opts.AutoSnapshot = autoSnapshot
	}
}

// WithSnapshotInterval은 자동 스냅샷 생성 간격을 설정하는 옵션 함수입니다.
func WithSnapshotInterval(interval int64) func(*EventSourcedStorageOptions) {
	return func(opts *EventSourcedStorageOptions) {
		opts.SnapshotInterval = interval
	}
}

// NewEventSourcedStorageWithOptions는 필수 인자와 옵션을 기반으로 EventSourcedStorage 인스턴스를 생성합니다.
func NewEventSourcedStorageWithOptions[T nodestorage.Cachable[T]](
	ctx context.Context,
	provider *MongoDBStorageProvider,
	collectionName string,
	options ...EventSourcedStorageFactoryOption,
) (*EventSourcedStorage[T], error) {
	// 기본 옵션 설정
	opts := &EventSourcedStorageFactoryOptions{
		EventCollectionName:    "events",
		EnableSnapshot:         false,
		SnapshotCollectionName: "snapshots",
		AutoSnapshot:           false,
		SnapshotInterval:       100, // 기본값: 100개 이벤트마다 스냅샷 생성
	}

	// 사용자 옵션 적용
	for _, option := range options {
		option(opts)
	}

	// 로거 설정
	logger := opts.Logger
	if logger == nil {
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			return nil, fmt.Errorf("failed to create default logger: %w", err)
		}
	}

	// 기본 저장소 생성
	storage, err := CreateStorage[T](ctx, provider, collectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// 이벤트 저장소 생성
	eventStore, err := provider.GetEventStore(ctx, opts.EventCollectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to create event store: %w", err)
	}

	// EventSourcedStorage 옵션 설정
	storageOptions := []func(*EventSourcedStorageOptions){}

	// 스냅샷 저장소 설정 (활성화된 경우)
	if opts.EnableSnapshot {
		snapshotStore, err := provider.GetSnapshotStore(ctx, opts.SnapshotCollectionName, eventStore)
		if err != nil {
			return nil, fmt.Errorf("failed to create snapshot store: %w", err)
		}
		storageOptions = append(storageOptions, WithSnapshotStore(snapshotStore))

		// 자동 스냅샷 설정
		if opts.AutoSnapshot {
			storageOptions = append(storageOptions, WithAutoSnapshot(true))

			// 스냅샷 간격 설정
			if opts.SnapshotInterval > 0 {
				storageOptions = append(storageOptions, WithSnapshotInterval(opts.SnapshotInterval))
			}
		}
	}

	// EventSourcedStorage 생성
	return NewEventSourcedStorage[T](storage, eventStore, logger, storageOptions...), nil
}

// FindOne은 문서를 ID로 조회합니다.
func (s *EventSourcedStorage[T]) FindOne(ctx context.Context, id primitive.ObjectID, opts ...*options.FindOneOptions) (T, error) {
	return s.storage.FindOne(ctx, id, opts...)
}

// FindMany는 필터에 맞는 여러 문서를 조회합니다.
func (s *EventSourcedStorage[T]) FindMany(ctx context.Context, filter interface{}, opts ...*options.FindOptions) ([]T, error) {
	return s.storage.FindMany(ctx, filter, opts...)
}

// FindOneAndUpsert는 문서를 생성하거나 이미 존재하는 경우 반환하고 이벤트를 저장합니다.
func (s *EventSourcedStorage[T]) FindOneAndUpsert(ctx context.Context, data T, clientID string) (T, error) {
	doc, err := s.storage.FindOneAndUpsert(ctx, data)
	if err != nil {
		return doc, err
	}

	// 문서 ID 추출
	v := primitive.ObjectID{}
	docValue := GetDocumentID(doc)
	if docValue.IsValid() {
		v = docValue.Interface().(primitive.ObjectID)
	}

	// 버전 필드 이름 가져오기
	versionField := s.storage.VersionField()

	// 문서의 버전 가져오기
	version, err := nodestorage.GetVersion(doc, versionField)
	if err != nil {
		s.logger.Error("Failed to get document version",
			zap.String("document_id", v.Hex()),
			zap.Error(err))
		version = 1 // 기본값 사용
	}

	// 생성 이벤트 저장
	event := &Event{
		ID:         primitive.NewObjectID(),
		DocumentID: v,
		Timestamp:  time.Now(),
		Operation:  "create",
		ClientID:   clientID,
		ServerSeq:  version,
		Metadata:   map[string]interface{}{"created_doc": doc},
	}

	if storeErr := s.eventStore.StoreEvent(ctx, event); storeErr != nil {
		s.logger.Error("Failed to store create event",
			zap.String("document_id", v.Hex()),
			zap.Error(storeErr))
		return doc, fmt.Errorf("document created but failed to store event: %w", storeErr)
	}

	return doc, nil
}

// FindOneAndUpdate는 문서를 수정하고 이벤트를 저장합니다.
func (s *EventSourcedStorage[T]) FindOneAndUpdate(ctx context.Context, id primitive.ObjectID, updateFn nodestorage.EditFunc[T], clientID string) (T, *nodestorage.Diff, error) {
	// 1. nodestorage의 FindOneAndUpdate 호출
	updatedDoc, diff, err := s.storage.FindOneAndUpdate(ctx, id, updateFn)
	if err != nil {
		return updatedDoc, diff, err
	}

	// 2. 변경사항이 있는 경우에만 이벤트 저장
	if diff != nil && diff.HasChanges {
		// 3. Diff를 이벤트로 변환 (Diff의 Version 필드 사용)
		event := &Event{
			ID:         primitive.NewObjectID(),
			DocumentID: id,
			Timestamp:  time.Now(),
			Operation:  "update",
			Diff:       diff,
			ClientID:   clientID,
			ServerSeq:  diff.Version,
		}

		// 4. 이벤트 저장
		if storeErr := s.eventStore.StoreEvent(ctx, event); storeErr != nil {
			s.logger.Error("Failed to store update event",
				zap.String("document_id", id.Hex()),
				zap.Error(storeErr))
			return updatedDoc, diff, fmt.Errorf("document updated but failed to store event: %w", storeErr)
		}

		// 5. 자동 스냅샷 생성 (설정된 경우)
		if s.snapshotStore != nil && s.options.AutoSnapshot {
			// 최신 이벤트 수 조회
			latestVersion, err := s.eventStore.GetLatestVersion(ctx, id)
			if err != nil {
				s.logger.Error("Failed to get latest version for auto snapshot",
					zap.String("document_id", id.Hex()),
					zap.Error(err))
			} else if latestVersion%s.options.SnapshotInterval == 0 {
				// 스냅샷 생성 간격에 도달한 경우 스냅샷 생성
				go func() {
					// 백그라운드에서 스냅샷 생성
					snapCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()

					_, err := s.CreateSnapshot(snapCtx, id)
					if err != nil {
						s.logger.Error("Failed to create auto snapshot",
							zap.String("document_id", id.Hex()),
							zap.Error(err))
					}
				}()
			}
		}
	}

	return updatedDoc, diff, nil
}

// DeleteOne은 문서를 삭제하고 이벤트를 저장합니다.
func (s *EventSourcedStorage[T]) DeleteOne(ctx context.Context, id primitive.ObjectID, clientID string) error {
	// 1. 삭제 전 문서 조회 (이벤트에 포함시키기 위함)
	doc, err := s.storage.FindOne(ctx, id)
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	// 버전 필드 이름 가져오기
	versionField := s.storage.VersionField()

	// 문서의 버전 가져오기
	var version int64 = 0
	if err != mongo.ErrNoDocuments {
		version, err = nodestorage.GetVersion(doc, versionField)
		if err != nil {
			s.logger.Error("Failed to get document version",
				zap.String("document_id", id.Hex()),
				zap.Error(err))
			// 오류가 있어도 계속 진행
		}
	}

	// 다음 버전 계산
	nextVersion := version + 1

	// 2. 실제 삭제 수행
	err = s.storage.DeleteOne(ctx, id)
	if err != nil {
		return err
	}

	// 3. 삭제 이벤트 생성 및 저장
	metadata := make(map[string]interface{})
	if err != mongo.ErrNoDocuments {
		metadata["deleted_doc"] = doc
	}

	event := &Event{
		ID:         primitive.NewObjectID(),
		DocumentID: id,
		Timestamp:  time.Now(),
		Operation:  "delete",
		ClientID:   clientID,
		ServerSeq:  nextVersion,
		Metadata:   metadata,
	}

	if storeErr := s.eventStore.StoreEvent(ctx, event); storeErr != nil {
		s.logger.Error("Failed to store delete event",
			zap.String("document_id", id.Hex()),
			zap.Error(storeErr))
		return fmt.Errorf("document deleted but failed to store event: %w", storeErr)
	}

	return nil
}

// GetEvents는 지정된 문서 ID에 대한 이벤트를 조회합니다.
func (s *EventSourcedStorage[T]) GetEvents(ctx context.Context, documentID primitive.ObjectID, afterVersion int64) ([]*Event, error) {
	return s.eventStore.GetEventsAfterVersion(ctx, documentID, afterVersion)
}

// GetMissingEvents는 클라이언트의 마지막 버전 이후의 이벤트를 조회합니다.
func (s *EventSourcedStorage[T]) GetMissingEvents(ctx context.Context, documentID primitive.ObjectID, afterVersion int64) ([]*Event, error) {
	return s.GetEvents(ctx, documentID, afterVersion)
}

// CreateSnapshot은 문서의 현재 상태를 스냅샷으로 저장합니다.
func (s *EventSourcedStorage[T]) CreateSnapshot(ctx context.Context, documentID primitive.ObjectID) (*Snapshot, error) {
	if s.snapshotStore == nil {
		return nil, fmt.Errorf("snapshot store not configured")
	}

	// 문서 조회
	doc, err := s.storage.FindOne(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to find document: %w", err)
	}

	// 버전 필드 이름 가져오기
	versionField := s.storage.VersionField()

	// 문서의 버전 가져오기
	version, err := nodestorage.GetVersion(doc, versionField)
	if err != nil {
		return nil, fmt.Errorf("failed to get document version: %w", err)
	}

	// 문서를 map으로 변환
	docMap, err := convertToMap(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to convert document to map: %w", err)
	}

	// 최신 이벤트의 ServerSeq 조회
	latestEvent, err := s.eventStore.GetLatestVersion(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest event: %w", err)
	}

	// 스냅샷 생성
	snapshot, err := s.snapshotStore.CreateSnapshot(ctx, documentID, docMap, version, latestEvent)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	s.logger.Info("Snapshot created",
		zap.String("document_id", documentID.Hex()),
		zap.Int64("version", version),
		zap.Int64("server_seq", latestEvent))

	return snapshot, nil
}

// GetEventsWithSnapshot은 스냅샷과 그 이후의 이벤트를 함께 조회합니다.
func (s *EventSourcedStorage[T]) GetEventsWithSnapshot(ctx context.Context, documentID primitive.ObjectID) (*Snapshot, []*Event, error) {
	if s.snapshotStore == nil {
		// 스냅샷 저장소가 없으면 모든 이벤트만 반환
		events, err := s.eventStore.GetEventsAfterVersion(ctx, documentID, 0)
		return nil, events, err
	}

	return GetEventsWithSnapshot(ctx, documentID, s.snapshotStore, s.eventStore)
}

// convertToMap은 구조체를 map으로 변환합니다.
func convertToMap(obj interface{}) (map[string]interface{}, error) {
	data, err := bson.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal object: %w", err)
	}

	var result map[string]interface{}
	err = bson.Unmarshal(data, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	return result, nil
}

// Close는 스토리지를 닫습니다.
func (s *EventSourcedStorage[T]) Close() error {
	return s.storage.Close()
}
