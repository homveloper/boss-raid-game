# CQRS 패턴을 활용한 이송 시스템 데모

이 데모 애플리케이션은 CQRS(Command Query Responsibility Segregation) 패턴과 이벤트 소싱을 활용하여 이송 시스템을 구현한 예제입니다.

## 개요

이 데모는 다음과 같은 기능을 제공합니다:

1. 이송 생성 (CreateTransport) - 이송 모집 시작
2. 이송 참가 (JoinTransport) - 이송에 참가
3. 이송 시작 (StartTransport) - 이송 시작 (자동 또는 수동)
4. 이송 조회 (GetTransport) - 이송 정보 조회
5. 활성 이송 목록 조회 (GetActiveTransports) - 활성 상태 이송 목록 조회
6. 이송 약탈 (RaidTransport) - 이송 약탈
7. 이송 방어 (DefendTransport) - 이송 방어
8. 이송 완료 (자동) - 이송 완료 처리

## 아키텍처

이 데모는 다음과 같은 CQRS 아키텍처를 따릅니다:

### 도메인 계층 (Domain Layer)
- 애그리게이트 (TransportAggregate, RaidAggregate)
- 커맨드 (CreateTransportCommand, StartTransportCommand 등)
- 이벤트 (TransportCreatedEvent, TransportStartedEvent 등)

### 인프라 계층 (Infrastructure Layer)
- 이벤트 저장소 (MongoEventStore)
- 이벤트 버스 (RedisEventBus)
- 커맨드 버스 (InMemoryCommandBus)
- 레포지토리 (MongoRepository)

### 애플리케이션 계층 (Application Layer)
- 커맨드 핸들러 (TransportCommandHandler, RaidCommandHandler)
- 프로젝터 (TransportProjector)

### 비즈니스 계층 (Business Layer)
- 비즈니스 서비스 (TransportService, GuildService, TicketService)
- 비즈니스 규칙 검증
- 에러 처리

### API 계층 (API Layer)
- REST API 핸들러
- JSON-RPC 핸들러
- 요청/응답 변환

## 비즈니스 규칙

이 데모에서는 다음과 같은 비즈니스 규칙을 구현했습니다:

### 이송 생성 규칙
1. 이송을 시작하려면 해당 유저가 길드에 가입되어 있어야 합니다.
2. 이송을 시작하려면 해당 유저가 티켓을 보유하고 있어야 합니다.
3. 이송을 시작하려면 대기중인 이송이 아무것도 없어야 합니다.

### 이송 참가 규칙
1. 이송에 참가하려면 해당 이송이 준비 단계에 있어야 합니다.
2. 이송에 참가하려면 해당 유저가 티켓을 보유하고 있어야 합니다.
3. 이송에 참가하려면 해당 이송의 최대 참가자 수를 초과하지 않아야 합니다.
4. 이송에 참가하려면 해당 유저가 이미 참가 중이지 않아야 합니다.

### 이송 시작 규칙
1. 이송은 준비 시간이 만료되면 자동으로 시작됩니다.
2. 이송은 최대 참가자 수에 도달하면 자동으로 시작됩니다.
3. 이송은 수동으로도 시작할 수 있습니다.

## 실행 방법

### 필수 조건
- Go 1.16 이상
- MongoDB
- Redis

### 실행
```bash
go run main.go
```

## API 엔드포인트

이 애플리케이션은 두 가지 API 방식을 제공합니다:

1. **REST API**: 전통적인 HTTP 엔드포인트
2. **JSON-RPC API**: 배치 처리가 가능한 단일 엔드포인트

### REST API 엔드포인트

#### 이송 생성
```
POST /api/transports
```
요청 예시:
```json
{
  "alliance_id": "alliance1",
  "player_id": "player1",
  "player_name": "Player One",
  "mine_id": "mine1",
  "mine_name": "Gold Mine",
  "mine_level": 1,
  "general_id": "general1",
  "gold_amount": 100,
  "max_participants": 5,
  "prep_time": 30,
  "transport_time": 60
}
```

#### 이송 참가
```
POST /api/transports/{id}/join
```
요청 예시:
```json
{
  "player_id": "player2",
  "player_name": "Player Two",
  "gold_amount": 50
}
```

#### 이송 시작
```
POST /api/transports/{id}/start
```

#### 이송 조회
```
GET /api/transports/{id}
```

#### 활성 이송 목록 조회
```
GET /api/transports?alliance_id={alliance_id}
```

#### 이송 약탈
```
POST /api/transports/{id}/raid
```
요청 예시:
```json
{
  "raider_id": "raider1",
  "raider_name": "Raider One"
}
```

#### 이송 방어
```
POST /api/transports/{id}/defend
```
요청 예시:
```json
{
  "defender_id": "defender1",
  "defender_name": "Defender One",
  "successful": true
}
```

### JSON-RPC API 엔드포인트

단일 엔드포인트로 모든 기능을 제공합니다:

```
POST /rpc
```

#### 단일 요청 예시
```json
{
  "jsonrpc": "2.0",
  "method": "createTransport",
  "params": {
    "alliance_id": "alliance1",
    "player_id": "player1",
    "player_name": "Player One",
    "mine_id": "mine1",
    "mine_name": "Gold Mine",
    "mine_level": 1,
    "general_id": "general1",
    "gold_amount": 100,
    "max_participants": 5,
    "prep_time": 30,
    "transport_time": 60
  },
  "id": 1
}
```

#### 배치 요청 예시
```json
[
  {
    "jsonrpc": "2.0",
    "method": "createTransport",
    "params": {
      "alliance_id": "alliance1",
      "player_id": "player1",
      "player_name": "Player One",
      "mine_id": "mine1",
      "mine_name": "Gold Mine",
      "mine_level": 1,
      "general_id": "general1",
      "gold_amount": 100,
      "max_participants": 5,
      "prep_time": 30,
      "transport_time": 60
    },
    "id": 1
  },
  {
    "jsonrpc": "2.0",
    "method": "getActiveTransports",
    "params": {
      "alliance_id": "alliance1"
    },
    "id": 2
  }
]
```

자세한 JSON-RPC API 사용법은 `README_RPC.md` 파일을 참조하세요.

## 테스트

```bash
go test -v
```

## 주요 특징

1. **낙관적 동시성 제어**: 모든 애그리게이트는 버전 필드를 통해 동시성 제어를 수행합니다.
2. **멱등성 보장**: 이벤트 ID를 통한 중복 처리 방지 메커니즘을 구현했습니다.
3. **그룹 기반 알림**: 연합/길드 ID 기반 이벤트 구독 채널을 구성했습니다.
4. **실시간 처리**: 이벤트 버스를 통한 실시간 이벤트 전파 기능을 제공합니다.
5. **레이어 분리**: API 송수신 레이어와 비즈니스 레이어를 명확히 분리했습니다.
6. **배치 처리**: JSON-RPC를 통해 여러 요청을 한 번의 네트워크 통신으로 처리할 수 있습니다.

## 구현 상세

### 이벤트 저장소 (Event Store)
MongoDB를 사용하여 이벤트를 저장하고 조회합니다. 각 이벤트는 애그리게이트 ID, 이벤트 타입, 버전 등의 메타데이터와 함께 저장됩니다.

### 이벤트 버스 (Event Bus)
Redis Streams를 사용하여 이벤트를 발행하고 구독합니다. 그룹 기반 알림을 위해 연합/길드 ID 기반 채널을 구성했습니다.

### 커맨드 버스 (Command Bus)
메모리 내에서 동작하는 커맨드 버스를 구현했습니다. 각 커맨드 타입별로 핸들러를 등록하고, 커맨드가 전달되면 해당 핸들러가 처리합니다.

### 프로젝터 (Projector)
이벤트를 처리하여 읽기 모델을 업데이트하는 프로젝터를 구현했습니다. 낙관적 동시성 제어와 멱등성을 보장하기 위한 메커니즘을 포함합니다.

### 비즈니스 서비스 (Business Services)
비즈니스 규칙을 검증하고 적용하는 서비스를 구현했습니다. 각 서비스는 특정 도메인 영역에 집중합니다.

### API 핸들러 (API Handlers)
REST API와 JSON-RPC API를 모두 지원하는 핸들러를 구현했습니다. JSON-RPC는 배치 처리를 통해 네트워크 효율성을 높입니다.

## 확장 방향

1. **분산 환경 지원**: 현재는 단일 서버에서 동작하지만, 분산 환경에서도 동작할 수 있도록 확장할 수 있습니다.
2. **스케줄링 기능 강화**: 이송 완료, 약탈 완료 등의 시간 기반 이벤트를 처리하는 스케줄링 기능을 강화할 수 있습니다.
3. **실시간 알림 개선**: WebSocket을 통한 실시간 알림 기능을 추가할 수 있습니다.
4. **모니터링 및 로깅**: 시스템 모니터링 및 로깅 기능을 강화할 수 있습니다.
