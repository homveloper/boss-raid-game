# EventSourcedStorage 예제

이 예제는 EventSourcedStorage를 사용하여 이벤트 소싱 패턴을 구현하는 방법을 보여줍니다.

## 개요

EventSourcedStorage는 이벤트 소싱 패턴을 구현한 저장소로, 모든 변경 사항을 이벤트로 저장하고 이를 통해 상태를 재구성할 수 있는 기능을 제공합니다. 이 예제에서는 다양한 클라이언트가 게임 상태를 수정하는 시나리오를 시뮬레이션합니다.

## 주요 기능

- **클라이언트별 이벤트 생성**: 각 작업마다 클라이언트 ID를 지정하여 이벤트 생성
- **벡터 시계 자동 관리**: 문서별, 클라이언트별 벡터 시계 자동 관리
- **이벤트 소싱 패턴 구현**: 모든 변경 사항을 이벤트로 저장하고 이를 통해 상태 재구성
- **누락된 이벤트 조회**: 클라이언트의 벡터 시계를 기반으로 누락된 이벤트 조회

## 실행 방법

### 사전 요구사항

- MongoDB 인스턴스 (localhost:27017)
- Go 개발 환경

### 실행 명령어

```bash
# 예제 디렉토리로 이동
cd examples/eventsync/event_sourced_storage_example

# 예제 실행
go run main.go
```

## 예제 설명

이 예제는 다음과 같은 단계로 구성되어 있습니다:

1. MongoDB 연결 설정
2. nodestorage 설정
3. 이벤트 저장소 설정
4. 벡터 시계 관리자 설정
5. EventSourcedStorage 생성
6. 게임 상태 생성 (game-server 클라이언트로)
7. 게임 상태 업데이트 (admin-panel 클라이언트로)
8. 레벨 업 처리 (mobile-client 클라이언트로)
9. 누락된 이벤트 조회

## 코드 구조

- `main.go`: 예제 애플리케이션 코드
- `GameState`: 게임 상태를 나타내는 구조체 (nodestorage.Cachable 인터페이스 구현)

## 주요 코드 설명

### EventSourcedStorage 생성

```go
// EventSourcedStorage 생성
eventSourcedStorage := eventsync.NewEventSourcedStorage(
    storage,
    eventStore,
    logger,
    eventsync.WithVectorClockManager(vectorClockManager),
)
```

### 다양한 클라이언트 시뮬레이션

```go
// 다양한 클라이언트 ID 시뮬레이션
clientIDs := []string{"game-server", "admin-panel", "mobile-client"}

// 게임 상태 생성 (game-server 클라이언트로)
createdGame, err := eventSourcedStorage.CreateDocument(ctx, game, clientIDs[0])

// 게임 상태 업데이트 (admin-panel 클라이언트로)
updatedGame, diff, err := eventSourcedStorage.UpdateDocument(ctx, gameID, updateFn, clientIDs[1])

// 레벨 업 처리 (mobile-client 클라이언트로)
leveledUpGame, levelDiff, err := eventSourcedStorage.UpdateDocument(ctx, gameID, levelUpFn, clientIDs[2])
```

### 누락된 이벤트 조회

```go
// 모바일 클라이언트의 벡터 시계 시뮬레이션
mobileClientVectorClock := map[string]int64{
    clientIDs[0]: 1, // game-server의 이벤트 1번까지 받음
    // admin-panel과 mobile-client의 이벤트는 아직 받지 못함
}

// 누락된 이벤트 조회
events, err := eventSourcedStorage.GetMissingEvents(ctx, gameID, mobileClientVectorClock)
```

## 참고 사항

- 이 예제는 MongoDB가 localhost:27017에서 실행 중이라고 가정합니다.
- 실제 프로덕션 환경에서는 적절한 에러 처리와 로깅을 추가해야 합니다.
- 이벤트 저장소의 이벤트 수가 계속 증가하므로 주기적인 스냅샷 생성과 이벤트 압축을 고려해야 합니다.
