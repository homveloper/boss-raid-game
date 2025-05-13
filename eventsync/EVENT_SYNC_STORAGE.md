# EventSyncStorage

EventSyncStorage는 nodestorage와 eventsync를 효과적으로 통합하기 위한 핵심 컴포넌트입니다. 이 컴포넌트는 nodestorage.Storage 인터페이스를 구현하면서 동시에 nodestorage에서 생성된 Diff를 자동으로 이벤트 저장소에 저장합니다.

## 개요

기존 eventsync 아키텍처에서는 nodestorage의 FindOneAndUpdate로 생성된 Diff를 이벤트 저장소에 저장하는 과정이 자동화되어 있지 않았습니다. 이로 인해 애플리케이션 코드에서 수동으로 Diff를 이벤트로 변환하고 저장해야 했습니다.

EventSyncStorage는 이 문제를 해결하기 위해 nodestorage.Storage 인터페이스를 구현하면서 내부적으로 실제 nodestorage 인스턴스를 사용하고, 모든 변경 작업에서 생성된 Diff를 자동으로 이벤트로 변환하여 저장합니다.

## 주요 기능

- nodestorage.Storage 인터페이스 구현으로 기존 코드와의 호환성 유지
- nodestorage의 FindOneAndUpdate 호출 시 생성된 Diff를 자동으로 이벤트로 변환하여 저장
- 낙관적 동시성 제어 기능 제공
- 이벤트 저장소와의 통합을 통한 실시간 동기화 지원

## 구현 방식

### 1. 인터페이스 호환성

EventSyncStorage는 nodestorage.Storage 인터페이스를 구현하여 기존 코드와의 호환성을 유지합니다.

```go
// EventSyncStorage는 nodestorage.Storage 인터페이스를 구현하여 기존 코드와의 호환성을 유지합니다.
type EventSyncStorage[T nodestorage.Cachable[T]] struct {
    storage    nodestorage.Storage[T]  // 내부적으로 실제 nodestorage 인스턴스 사용
    eventStore EventStore              // 이벤트 저장소
    logger     *zap.Logger
}
```

### 2. Diff 자동 캡처 및 이벤트 저장

FindOneAndUpdate 메서드는 nodestorage의 동일 메서드를 호출하고 생성된 Diff를 이벤트로 저장합니다.

```go
// FindOneAndUpdate는 문서를 수정하고 변경 사항을 이벤트로 저장합니다.
func (s *EventSyncStorage[T]) FindOneAndUpdate(ctx context.Context, id primitive.ObjectID, updateFn nodestorage.EditFunc[T], opts ...nodestorage.EditOption) (T, *nodestorage.Diff, error) {
    // 1. nodestorage의 FindOneAndUpdate 호출
    updatedDoc, diff, err := s.storage.FindOneAndUpdate(ctx, id, updateFn, opts...)

    // 2. 에러가 없고 변경사항이 있는 경우에만 이벤트 저장
    if err == nil && diff != nil && diff.HasChanges {
        // 3. Diff를 이벤트로 변환
        event := &Event{
            ID:          primitive.NewObjectID(),
            DocumentID:  id,
            Timestamp:   time.Now(),
            Operation:   "update",
            Diff:        diff,
            ClientID:    "server",
            VectorClock: map[string]int64{"server": 1},
        }

        // 4. 이벤트 저장
        if storeErr := s.eventStore.StoreEvent(ctx, event); storeErr != nil {
            // 이벤트 저장 실패 로깅 (하지만 원래 작업은 성공했으므로 에러 반환하지 않음)
            s.logger.Error("Failed to store update event",
                zap.String("document_id", id.Hex()),
                zap.Error(storeErr))
        }
    }

    return updatedDoc, diff, err
}
```

### 3. 다른 작업에 대한 이벤트 처리

DeleteOne, FindOneAndUpsert 등 다른 메서드들도 유사한 방식으로 이벤트를 생성하고 저장합니다.

```go
// DeleteOne은 문서를 삭제하고 삭제 이벤트를 저장합니다.
func (s *EventSyncStorage[T]) DeleteOne(ctx context.Context, id primitive.ObjectID) error {
    // 1. 삭제 전 문서 조회 (이벤트에 포함시키기 위함)
    doc, err := s.storage.FindOne(ctx, id)
    if err != nil && err != mongo.ErrNoDocuments {
        return err
    }

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
        ID:          primitive.NewObjectID(),
        DocumentID:  id,
        Timestamp:   time.Now(),
        Operation:   "delete",
        ClientID:    "server",
        VectorClock: map[string]int64{"server": 1},
        Metadata:    metadata,
    }

    if storeErr := s.eventStore.StoreEvent(ctx, event); storeErr != nil {
        s.logger.Error("Failed to store delete event",
            zap.String("document_id", id.Hex()),
            zap.Error(storeErr))
    }

    return nil
}
```

## 사용 방법

### 1. EventSyncStorage 생성

```go
// nodestorage 설정
memCache := cache.NewMemoryCache[*GameState](nil)
storageOptions := &nodestorage.Options{
    VersionField:      "version",
    CacheTTL:          time.Minute * 10,
    WatchEnabled:      true,
    WatchFullDocument: "updateLookup",
}

storage, err := nodestorage.NewStorage[*GameState](ctx, gameCollection, memCache, storageOptions)
if err != nil {
    logger.Fatal("Storage 생성 실패", zap.Error(err))
}

// 이벤트 저장소 설정
eventStore, err := eventsync.NewMongoEventStore(ctx, client, dbName, "events", logger)
if err != nil {
    logger.Fatal("EventStore 생성 실패", zap.Error(err))
}

// 벡터 시계 관리자 설정 (MongoDB 기반)
vectorClockManager, err := eventsync.NewMongoVectorClockManager(ctx, client, dbName, "vector_clocks", logger)
if err != nil {
    logger.Fatal("벡터 시계 관리자 생성 실패", zap.Error(err))
}

// EventSyncStorage 생성 (옵션 설정)
eventSyncStorage := eventsync.NewEventSyncStorage(
    storage,
    eventStore,
    logger,
    eventsync.WithClientID("game-server"),
    eventsync.WithVectorClockManager(vectorClockManager),
)
```

### 2. EventSyncStorage 사용

기존 nodestorage.Storage를 사용하는 방식과 동일하게 사용할 수 있습니다.

```go
// 게임 상태 업데이트
updateFn := func(g *GameState) (*GameState, error) {
    g.Gold += 50
    g.Experience += 100
    g.LastUpdated = time.Now()
    g.Version++
    return g, nil
}

updatedGame, diff, err := eventSyncStorage.FindOneAndUpdate(ctx, gameID, updateFn)
if err != nil {
    logger.Fatal("게임 상태 업데이트 실패", zap.Error(err))
}
```

## 이점

1. **코드 수정 최소화**: 기존 nodestorage.Storage를 사용하는 코드를 EventSyncStorage로 대체하기만 하면 됩니다.
2. **자동화된 이벤트 저장**: 모든 변경 작업에서 생성된 Diff가 자동으로 이벤트로 변환되어 저장됩니다.
3. **일관된 인터페이스**: nodestorage.Storage 인터페이스와 동일한 인터페이스를 제공하므로 기존 코드와의 호환성이 유지됩니다.
4. **실시간 동기화 지원**: 저장된 이벤트를 기반으로 클라이언트와의 실시간 동기화가 가능합니다.

## 벡터 시계 관리

EventSyncStorage는 벡터 시계를 관리하기 위한 두 가지 구현을 제공합니다:

### 1. DefaultVectorClockManager

메모리 기반 벡터 시계 관리자로, 서버 재시작 시 데이터가 손실됩니다. 간단한 테스트나 개발 환경에 적합합니다.

```go
// 기본 벡터 시계 관리자 생성
vectorClockManager := eventsync.NewDefaultVectorClockManager()
```

### 2. MongoVectorClockManager

MongoDB 기반 벡터 시계 관리자로, 벡터 시계를 영구적으로 저장합니다. 프로덕션 환경에 적합합니다.

```go
// MongoDB 기반 벡터 시계 관리자 생성
vectorClockManager, err := eventsync.NewMongoVectorClockManager(ctx, client, dbName, "vector_clocks", logger)
if err != nil {
    logger.Fatal("벡터 시계 관리자 생성 실패", zap.Error(err))
}
```

### 벡터 시계 관리 인터페이스

사용자 정의 벡터 시계 관리자를 구현하려면 다음 인터페이스를 구현하면 됩니다:

```go
// VectorClockManager는 벡터 시계를 관리하는 인터페이스입니다.
type VectorClockManager interface {
    // GetVectorClock은 지정된 문서 ID에 대한 현재 벡터 시계를 반환합니다.
    GetVectorClock(ctx context.Context, documentID primitive.ObjectID) (map[string]int64, error)

    // UpdateVectorClock은 지정된 문서 ID에 대한 벡터 시계를 업데이트합니다.
    UpdateVectorClock(ctx context.Context, documentID primitive.ObjectID, clientID string, sequenceNum int64) error
}
```

## 클라이언트 ID 설정

EventSyncStorage는 이벤트를 생성할 때 사용할 클라이언트 ID를 설정할 수 있습니다. 기본값은 "server"입니다.

```go
// 클라이언트 ID 설정
eventSyncStorage := eventsync.NewEventSyncStorage(
    storage,
    eventStore,
    logger,
    eventsync.WithClientID("game-server"),
)
```

## 주의 사항

1. **이벤트 저장 실패 처리**: 이벤트 저장에 실패하더라도 원래 작업은 성공한 것으로 처리됩니다. 이는 이벤트 저장이 실패해도 데이터 일관성을 유지하기 위함입니다.
2. **벡터 시계 관리**: 벡터 시계는 문서별로 관리되며, 각 클라이언트(또는 서버)의 시퀀스 번호를 추적합니다. 이를 통해 이벤트의 순서와 누락된 이벤트를 식별할 수 있습니다.
3. **이벤트 압축 및 스냅샷**: 이벤트 수가 계속 증가하므로 주기적인 스냅샷 생성과 이벤트 압축을 고려해야 합니다.

## 예제

전체 예제는 `examples/eventsync/event_sync_storage_example` 디렉토리에서 확인할 수 있습니다.
