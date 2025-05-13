# EventSyncStorage와 EventSourcedStorage 비교

이 문서는 EventSyncStorage와 EventSourcedStorage의 차이점을 설명합니다.

## 개요

eventsync 패키지는 두 가지 저장소 구현을 제공합니다:

1. **EventSyncStorage**: nodestorage.Storage 인터페이스를 구현하여 기존 코드와의 호환성을 유지하면서 이벤트 소싱 기능을 추가합니다.
2. **EventSourcedStorage**: nodestorage.Storage 인터페이스를 구현하지 않고, 이벤트 소싱에 특화된 인터페이스를 제공합니다.

## 주요 차이점

### 1. 인터페이스 호환성

- **EventSyncStorage**: nodestorage.Storage 인터페이스를 구현하여 기존 코드와의 호환성을 유지합니다.
- **EventSourcedStorage**: nodestorage.Storage 인터페이스와 유사한 메서드 이름을 사용하지만, 클라이언트 ID를 추가 인자로 받는 이벤트 소싱에 특화된 인터페이스를 제공합니다.

### 2. 클라이언트 ID 처리

- **EventSyncStorage**: 생성 시 클라이언트 ID를 고정하여 모든 이벤트에 동일한 클라이언트 ID를 사용합니다.
- **EventSourcedStorage**: 각 작업마다 클라이언트 ID를 지정할 수 있어 다양한 클라이언트의 요청을 처리할 수 있습니다.

### 3. 메서드 시그니처

- **EventSyncStorage**: nodestorage.Storage 인터페이스의 메서드 시그니처를 그대로 사용합니다.
- **EventSourcedStorage**: nodestorage와 유사한 메서드 이름을 사용하지만, 클라이언트 ID를 추가 인자로 받습니다 (예: FindOneAndUpsert, FindOneAndUpdate, DeleteOne).

### 4. 에러 처리

- **EventSyncStorage**: 이벤트 저장에 실패하더라도 원래 작업은 성공한 것으로 처리합니다.
- **EventSourcedStorage**: 이벤트 저장에 실패하면 에러를 반환합니다.

## 사용 시나리오

### EventSyncStorage 사용 시나리오

- 기존 nodestorage.Storage를 사용하는 코드가 있고, 최소한의 수정으로 이벤트 소싱 기능을 추가하고 싶을 때
- 단일 서버 또는 단일 클라이언트 환경에서 사용할 때
- 이벤트 저장 실패가 전체 작업 실패로 이어지지 않아야 할 때

```go
// EventSyncStorage 생성
eventSyncStorage := eventsync.NewEventSyncStorage(
    storage,
    eventStore,
    logger,
    eventsync.WithClientID("server"),
)

// 사용 예시
updatedDoc, diff, err := eventSyncStorage.FindOneAndUpdate(ctx, id, updateFn)
```

### EventSourcedStorage 사용 시나리오

- 이벤트 소싱 패턴을 처음부터 구현하고 싶을 때
- 다양한 클라이언트의 요청을 처리해야 할 때
- 이벤트 저장이 작업의 핵심이고, 이벤트 저장 실패가 전체 작업 실패로 이어져야 할 때

```go
// EventSourcedStorage 생성
eventSourcedStorage := eventsync.NewEventSourcedStorage(
    storage,
    eventStore,
    logger,
)

// 사용 예시
updatedDoc, diff, err := eventSourcedStorage.FindOneAndUpdate(ctx, id, updateFn, "mobile-client")
```

## 선택 가이드

다음 질문에 답하여 어떤 저장소를 사용할지 결정할 수 있습니다:

1. **기존 코드와의 호환성이 중요한가?**
   - 예: EventSyncStorage
   - 아니오: EventSourcedStorage

2. **다양한 클라이언트의 요청을 처리해야 하는가?**
   - 예: EventSourcedStorage
   - 아니오: EventSyncStorage

3. **이벤트 저장 실패가 전체 작업 실패로 이어져야 하는가?**
   - 예: EventSourcedStorage
   - 아니오: EventSyncStorage

4. **이벤트 소싱 패턴을 처음부터 구현하고 싶은가?**
   - 예: EventSourcedStorage
   - 아니오: EventSyncStorage

## 결론

- **EventSyncStorage**는 기존 코드와의 호환성을 유지하면서 이벤트 소싱 기능을 추가하고 싶을 때 적합합니다.
- **EventSourcedStorage**는 이벤트 소싱 패턴을 처음부터 구현하고, 다양한 클라이언트의 요청을 처리해야 할 때 적합합니다.

두 저장소 모두 eventsync 패키지의 핵심 기능인 이벤트 소싱과 벡터 시계 관리를 제공하지만, 사용 시나리오와 요구사항에 따라 적절한 저장소를 선택해야 합니다.
