# EventSourcedStorage

EventSourcedStorage는 이벤트 소싱 패턴을 구현한 저장소로, 모든 변경 사항을 이벤트로 저장하고 이를 통해 상태를 재구성할 수 있는 기능을 제공합니다.

## 개요

EventSourcedStorage는 nodestorage.Storage 인터페이스와 유사한 메서드 이름을 사용하지만, 클라이언트 ID를 추가 인자로 받는 이벤트 소싱에 특화된 인터페이스를 제공합니다. 이를 통해 각 작업마다 클라이언트 ID를 지정할 수 있어 다양한 클라이언트의 요청을 처리할 수 있습니다.

## 주요 기능

- **클라이언트별 이벤트 생성**: 각 작업마다 클라이언트 ID를 지정하여 이벤트 생성
- **버전 기반 이벤트 관리**: 문서의 버전 필드를 사용하여 이벤트 순서 관리
- **이벤트 소싱 패턴 구현**: 모든 변경 사항을 이벤트로 저장하고 이를 통해 상태 재구성
- **누락된 이벤트 조회**: 클라이언트의 마지막 버전을 기반으로 누락된 이벤트 조회

## 구현 방식

### 1. 인터페이스 설계

EventSourcedStorage는 nodestorage.Storage 인터페이스와 유사한 메서드 이름을 사용하지만, 클라이언트 ID를 추가 인자로 받는 이벤트 소싱에 특화된 인터페이스를 제공합니다.

```go
// EventSourcedStorage는 이벤트 소싱 패턴을 구현한 저장소입니다.
type EventSourcedStorage[T nodestorage.Cachable[T]] struct {
    storage    nodestorage.Storage[T] // 내부적으로 실제 nodestorage 인스턴스 사용
    eventStore EventStore             // 이벤트 저장소
    logger     *zap.Logger
}
```

### 2. 클라이언트별 이벤트 생성

각 작업마다 클라이언트 ID를 지정하여 이벤트를 생성합니다.

```go
// FindOneAndUpsert는 문서를 생성하거나 이미 존재하는 경우 반환하고 이벤트를 저장합니다.
func (s *EventSourcedStorage[T]) FindOneAndUpsert(ctx context.Context, data T, clientID string) (T, error) {
    // ...
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
    // ...
}
```

### 3. 버전 기반 이벤트 관리

문서의 버전 필드를 사용하여 이벤트 순서를 관리합니다.

```go
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
        // ...
    }
    // ...
}
```

### 4. 누락된 이벤트 조회

클라이언트의 마지막 버전을 기반으로 누락된 이벤트를 조회합니다.

```go
// GetMissingEvents는 클라이언트의 마지막 버전 이후의 이벤트를 조회합니다.
func (s *EventSourcedStorage[T]) GetMissingEvents(ctx context.Context, documentID primitive.ObjectID, afterVersion int64) ([]*Event, error) {
    return s.eventStore.GetEventsAfterVersion(ctx, documentID, afterVersion)
}
```

## 사용 방법

### 1. EventSourcedStorage 생성

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

// EventSourcedStorage 생성
eventSourcedStorage := eventsync.NewEventSourcedStorage(
    storage,
    eventStore,
    logger,
)
```

### 2. 문서 생성

```go
// 게임 상태 생성 (game-server 클라이언트로)
createdGame, err := eventSourcedStorage.FindOneAndUpsert(ctx, game, "game-server")
if err != nil {
    logger.Fatal("게임 상태 생성 실패", zap.Error(err))
}
```

### 3. 문서 업데이트

```go
// 게임 상태 업데이트 (admin-panel 클라이언트로)
updateFn := func(g *GameState) (*GameState, error) {
    g.Gold += 50
    g.Experience += 100
    g.LastUpdated = time.Now()
    g.Version++
    return g, nil
}

updatedGame, diff, err := eventSourcedStorage.FindOneAndUpdate(ctx, gameID, updateFn, "admin-panel")
if err != nil {
    logger.Fatal("게임 상태 업데이트 실패", zap.Error(err))
}
```

### 4. 문서 삭제

```go
// 게임 상태 삭제 (admin-panel 클라이언트로)
err = eventSourcedStorage.DeleteOne(ctx, gameID, "admin-panel")
if err != nil {
    logger.Fatal("게임 상태 삭제 실패", zap.Error(err))
}
```

### 5. 누락된 이벤트 조회

```go
// 모바일 클라이언트의 마지막 버전 시뮬레이션
lastReceivedVersion := int64(1) // 버전 1까지 받음

// 누락된 이벤트 조회
events, err := eventSourcedStorage.GetMissingEvents(ctx, gameID, lastReceivedVersion)
if err != nil {
    logger.Fatal("누락된 이벤트 조회 실패", zap.Error(err))
}
```

## 이점

1. **클라이언트별 이벤트 생성**: 각 작업마다 클라이언트 ID를 지정할 수 있어 다양한 클라이언트의 요청을 처리할 수 있습니다.
2. **버전 기반 이벤트 관리**: 문서의 버전 필드를 사용하여 이벤트 순서를 관리하므로 단순하고 효율적입니다.
3. **이벤트 소싱 패턴 구현**: 모든 변경 사항을 이벤트로 저장하고 이를 통해 상태를 재구성할 수 있어 데이터의 이력을 추적할 수 있습니다.
4. **누락된 이벤트 조회**: 클라이언트의 마지막 버전을 기반으로 누락된 이벤트를 조회할 수 있어 클라이언트의 상태를 최신 상태로 유지할 수 있습니다.

## 주의 사항

1. **이벤트 저장 실패 처리**: 이벤트 저장에 실패하면 에러를 반환합니다. 이는 이벤트 소싱의 핵심인 모든 변경 사항을 이벤트로 저장하는 원칙을 지키기 위함입니다.
2. **버전 관리**: 버전은 문서별로 관리되며, 이를 통해 이벤트의 순서와 누락된 이벤트를 식별할 수 있습니다.
3. **이벤트 압축 및 스냅샷**: 이벤트 수가 계속 증가하므로 주기적인 스냅샷 생성과 이벤트 압축을 고려해야 합니다.

## 예제

전체 예제는 `examples/eventsync/event_sourced_storage_example` 디렉토리에서 확인할 수 있습니다.
