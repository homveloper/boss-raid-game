# EventSyncStorage 예제

이 예제는 EventSyncStorage를 사용하여 nodestorage와 eventsync를 통합하는 방법을 보여줍니다.

## 개요

EventSyncStorage는 nodestorage.Storage 인터페이스를 구현하면서 동시에 nodestorage에서 생성된 Diff를 자동으로 이벤트 저장소에 저장하는 컴포넌트입니다. 이를 통해 애플리케이션 코드를 최소한으로 수정하면서 이벤트 소싱 패턴을 구현할 수 있습니다.

## 주요 기능

- nodestorage.Storage 인터페이스 구현으로 기존 코드와의 호환성 유지
- nodestorage의 FindOneAndUpdate 호출 시 생성된 Diff를 자동으로 이벤트로 변환하여 저장
- 낙관적 동시성 제어 기능 제공
- 이벤트 저장소와의 통합을 통한 실시간 동기화 지원

## 실행 방법

### 사전 요구사항

- MongoDB 인스턴스 (localhost:27017)
- Go 개발 환경

### 실행 명령어

```bash
# 예제 디렉토리로 이동
cd examples/eventsync/event_sync_storage_example

# 예제 실행
go run main.go
```

## 예제 설명

이 예제는 다음과 같은 단계로 구성되어 있습니다:

1. MongoDB 연결 설정
2. nodestorage 설정
3. 이벤트 저장소 설정
4. EventSyncStorage 생성
5. 상태 벡터 관리자 설정
6. 동기화 서비스 설정
7. 게임 상태 생성 (FindOneAndUpsert)
8. 게임 상태 업데이트 (FindOneAndUpdate)
9. 레벨 업 처리 (FindOneAndUpdate)
10. 누락된 이벤트 조회 (GetMissingEvents)
11. 벡터 시계 업데이트 (UpdateVectorClock)

## 코드 구조

- `main.go`: 예제 애플리케이션 코드
- `GameState`: 게임 상태를 나타내는 구조체 (nodestorage.Cachable 인터페이스 구현)

## 주요 코드 설명

### EventSyncStorage 생성

```go
// 1. nodestorage 설정
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

// 2. 이벤트 저장소 설정
eventStore, err := eventsync.NewMongoEventStore(ctx, client, dbName, "events", logger)
if err != nil {
    logger.Fatal("EventStore 생성 실패", zap.Error(err))
}

// 3. EventSyncStorage 생성
eventSyncStorage := eventsync.NewEventSyncStorage(storage, eventStore, logger)
```

### 게임 상태 업데이트 및 자동 이벤트 저장

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

위 코드에서 `eventSyncStorage.FindOneAndUpdate`를 호출하면 내부적으로 다음과 같은 작업이 수행됩니다:

1. nodestorage의 FindOneAndUpdate 호출
2. 생성된 Diff를 이벤트로 변환
3. 이벤트를 이벤트 저장소에 저장
4. 업데이트된 문서와 Diff 반환

이 과정은 모두 자동으로 이루어지므로 애플리케이션 코드는 기존 nodestorage를 사용하는 방식과 동일하게 작성할 수 있습니다.

## 참고 사항

- 이 예제는 MongoDB가 localhost:27017에서 실행 중이라고 가정합니다.
- 실제 프로덕션 환경에서는 적절한 에러 처리와 로깅을 추가해야 합니다.
- 이벤트 저장소의 이벤트 수가 계속 증가하므로 주기적인 스냅샷 생성과 이벤트 압축을 고려해야 합니다.
