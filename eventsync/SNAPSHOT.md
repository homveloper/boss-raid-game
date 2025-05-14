# 스냅샷 저장소 (SnapshotStore)

스냅샷 저장소는 특정 시점의 문서 상태를 저장하여 이벤트 소싱 시스템의 성능을 최적화하는 컴포넌트입니다. 이벤트가 많아질수록 모든 이벤트를 처음부터 적용하는 것은 비효율적이므로, 주기적으로 스냅샷을 생성하여 상태 복원 시간을 단축합니다.

## 개요

스냅샷 저장소는 다음과 같은 기능을 제공합니다:

- 특정 시점의 문서 상태를 스냅샷으로 저장
- 버전 및 서버 시퀀스 번호 기반 스냅샷 관리
- 자동 또는 수동 스냅샷 생성
- 스냅샷 조회 및 관리
- 오래된 스냅샷 정리

## 주요 인터페이스

```go
// SnapshotStore 인터페이스는 스냅샷 저장소의 기능을 정의합니다.
type SnapshotStore interface {
    // CreateSnapshot은 새로운 스냅샷을 생성합니다.
    CreateSnapshot(ctx context.Context, documentID primitive.ObjectID, state map[string]interface{}, version int64, serverSeq int64) (*Snapshot, error)

    // GetLatestSnapshot은 문서의 최신 스냅샷을 조회합니다.
    GetLatestSnapshot(ctx context.Context, documentID primitive.ObjectID) (*Snapshot, error)

    // GetSnapshotByServerSeq는 특정 서버 시퀀스 이전의 가장 최근 스냅샷을 조회합니다.
    GetSnapshotByServerSeq(ctx context.Context, documentID primitive.ObjectID, maxServerSeq int64) (*Snapshot, error)

    // DeleteSnapshots는 특정 서버 시퀀스 이전의 모든 스냅샷을 삭제합니다.
    DeleteSnapshots(ctx context.Context, documentID primitive.ObjectID, maxServerSeq int64) (int64, error)

    // Close는 스냅샷 저장소를 닫습니다.
    Close() error
}
```

## 스냅샷 구조체

```go
// Snapshot 구조체는 특정 시점의 문서 상태를 나타냅니다.
type Snapshot struct {
    ID          primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
    DocumentID  primitive.ObjectID     `bson:"document_id" json:"documentId"`
    State       map[string]interface{} `bson:"state" json:"state"`
    Version     int64                  `bson:"version" json:"version"`
    ServerSeq   int64                  `bson:"server_seq" json:"serverSeq"`
    CreatedAt   time.Time              `bson:"created_at" json:"createdAt"`
}
```

## MongoDB 기반 구현

```go
// MongoSnapshotStore는 MongoDB 기반 스냅샷 저장소 구현체입니다.
type MongoSnapshotStore struct {
    collection *mongo.Collection
    eventStore EventStore
    logger     *zap.Logger
}

// NewMongoSnapshotStore는 새로운 MongoDB 스냅샷 저장소를 생성합니다.
func NewMongoSnapshotStore(ctx context.Context, client *mongo.Client, database, collection string, eventStore EventStore, logger *zap.Logger) (*MongoSnapshotStore, error) {
    // 컬렉션 가져오기
    coll := client.Database(database).Collection(collection)

    // 인덱스 생성
    indexModels := []mongo.IndexModel{
        {
            Keys: bson.D{
                {Key: "document_id", Value: 1},
                {Key: "server_seq", Value: -1},
            },
        },
        {
            Keys: bson.D{
                {Key: "created_at", Value: 1},
            },
        },
    }

    _, err := coll.Indexes().CreateMany(ctx, indexModels)
    if err != nil {
        return nil, fmt.Errorf("failed to create indexes: %w", err)
    }

    return &MongoSnapshotStore{
        collection: coll,
        eventStore: eventStore,
        logger:     logger,
    }, nil
}
```

## EventSourcedStorage와의 통합

EventSourcedStorage는 스냅샷 저장소를 활용하여 다음과 같은 기능을 제공합니다:

### 1. 수동 스냅샷 생성

```go
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
```

### 2. 자동 스냅샷 생성

```go
// FindOneAndUpdate는 문서를 수정하고 이벤트를 저장합니다.
func (s *EventSourcedStorage[T]) FindOneAndUpdate(ctx context.Context, id primitive.ObjectID, updateFn nodestorage.EditFunc[T], clientID string) (T, *nodestorage.Diff, error) {
    // ... 기존 코드 ...

    // 자동 스냅샷 생성 (설정된 경우)
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

    // ... 기존 코드 ...
}
```

### 3. 스냅샷과 이벤트 함께 조회

```go
// GetEventsWithSnapshot은 스냅샷과 그 이후의 이벤트를 함께 조회합니다.
func (s *EventSourcedStorage[T]) GetEventsWithSnapshot(ctx context.Context, documentID primitive.ObjectID) (*Snapshot, []*Event, error) {
    if s.snapshotStore == nil {
        // 스냅샷 저장소가 없으면 모든 이벤트만 반환
        events, err := s.eventStore.GetEventsAfterVersion(ctx, documentID, 0)
        return nil, events, err
    }

    return GetEventsWithSnapshot(ctx, documentID, s.snapshotStore, s.eventStore)
}
```

## 사용 예시

```go
// 스냅샷 저장소 설정
snapshotStore, err := eventsync.NewMongoSnapshotStore(ctx, client, dbName, "snapshots", eventStore, logger)
if err != nil {
    logger.Fatal("SnapshotStore 생성 실패", zap.Error(err))
}

// EventSourcedStorage 생성 (자동 스냅샷 활성화)
eventSourcedStorage := eventsync.NewEventSourcedStorage(
    storage,
    eventStore,
    logger,
    eventsync.WithSnapshotStore(snapshotStore),
    eventsync.WithAutoSnapshot(true),
    eventsync.WithSnapshotInterval(5), // 5개 이벤트마다 스냅샷 생성
)

// 수동으로 스냅샷 생성
snapshot, err := eventSourcedStorage.CreateSnapshot(ctx, documentID)
if err != nil {
    logger.Fatal("스냅샷 생성 실패", zap.Error(err))
}

// 스냅샷과 이벤트 함께 조회
latestSnapshot, eventsAfterSnapshot, err := eventSourcedStorage.GetEventsWithSnapshot(ctx, documentID)
if err != nil {
    logger.Fatal("스냅샷과 이벤트 조회 실패", zap.Error(err))
}

// 스냅샷 적용 후 이벤트 순차 적용
if latestSnapshot != nil {
    // 스냅샷 상태로 초기화
    state := latestSnapshot.State
    
    // 이벤트 순차 적용
    for _, event := range eventsAfterSnapshot {
        // 이벤트 적용 로직
        applyEvent(state, event)
    }
}
```

## 스냅샷 관리 전략

### 1. 스냅샷 생성 전략

- **이벤트 개수 기반**: 특정 개수의 이벤트마다 스냅샷 생성 (예: 100개 이벤트마다)
- **시간 기반**: 특정 시간 간격으로 스냅샷 생성 (예: 1시간마다)
- **하이브리드**: 이벤트 개수와 시간 중 먼저 도달하는 조건에 스냅샷 생성

### 2. 스냅샷 정리 전략

- **최신 N개 유지**: 문서별로 최신 N개의 스냅샷만 유지
- **특정 기간 이전 삭제**: 특정 기간(예: 30일) 이전의 스냅샷 삭제
- **특정 버전 이전 삭제**: 특정 버전 이전의 스냅샷 삭제

### 3. 스냅샷 저장 최적화

- **압축**: 스냅샷 데이터 압축 저장
- **증분 스냅샷**: 전체 상태 대신 변경된 부분만 저장
- **스토리지 계층화**: 최신 스냅샷은 빠른 스토리지에, 오래된 스냅샷은 저렴한 스토리지에 저장
