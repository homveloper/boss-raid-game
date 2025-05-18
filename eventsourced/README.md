# EventSourcedStorage

EventSourcedStorage는 nodestorage를 확장하여 이벤트 소싱 및 CQRS 패턴을 지원하는 Go 패키지입니다. 낙관적 동시성 제어로 문서 수정이 성공할 때 자동으로 이벤트를 발행하는 기능을 제공합니다. 이 패키지는 [jetbasrawi/go.cqrs](https://github.com/jetbasrawi/go.cqrs) 레포지토리의 구조를 참고하여 설계되었습니다.

## 주요 기능

- **낙관적 동시성 제어**: 버전 필드를 통한 동시성 충돌 감지 및 해결
- **자동 이벤트 발행**: 데이터 변경 시 자동으로 이벤트 발행
- **커스텀 이벤트 매핑**: 도메인별 이벤트 매퍼를 통한 세분화된 이벤트 생성
- **다양한 이벤트 버스 지원**: 인메모리, Redis, NATS 등 다양한 이벤트 버스 지원
- **MongoDB 통합**: MongoDB를 기반으로 한 영구 저장소 지원
- **애그리게이트 기반 도메인 모델링**: CQRS 패턴에 따른 애그리게이트 루트 구현

## 빠른 시작 가이드 (CQRS 초보자용)

CQRS는 복잡해 보일 수 있지만, 핵심 개념은 간단합니다: **쓰기 작업과 읽기 작업을 분리하여 각각 최적화**하는 것입니다.

### 1. 기본 개념 이해하기

- **Command (명령)**: 시스템에 변경을 요청하는 것 (예: 사용자 생성, 이메일 변경)
- **Query (조회)**: 시스템에서 데이터를 조회하는 것 (예: 사용자 정보 조회)
- **Event (이벤트)**: 시스템에서 발생한 사실 (예: 사용자가 생성됨, 이메일이 변경됨)

### 2. 간단한 3단계 사용법

1. **모델 정의하기**: 도메인 객체와 이벤트 정의
2. **명령 처리하기**: 변경 요청을 처리하는 핸들러 구현
3. **이벤트 구독하기**: 변경 사항에 반응하는 핸들러 구현

### 3. 코드 예시: 사용자 관리 시스템

```go
// 1. 모델 정의하기
type User struct {
    ID    string
    Name  string
    Email string
}

// 2. 명령 처리하기 - 사용자 생성
createUser := command.NewCommand("CreateUser", "user123", map[string]interface{}{
    "name":  "John Doe",
    "email": "john@example.com",
})
dispatcher.Dispatch(ctx, createUser)

// 3. 이벤트 구독하기 - 사용자 생성 알림
eventBus.Subscribe("UserCreated", func(ctx context.Context, e event.Event) error {
    log.Printf("환영합니다! 새 사용자가 생성되었습니다: %s", e.AggregateID())
    return nil
})
```

이게 전부입니다! EventSourcedStorage가 나머지 복잡한 부분을 처리해줍니다.

## 설치

```bash
go get github.com/yourusername/eventsourced
```

## 사용 방법

### 기본 설정

```go
package main

import (
	"context"
	"log"

	"github.com/yourusername/eventsourced/pkg/storage"
	"github.com/yourusername/eventsourced/pkg/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// MongoDB 연결
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	// 이벤트 버스 생성
	eventBus := event.NewInMemoryEventBus()

	// 이벤트 매퍼 생성
	eventMapper := event.NewDefaultEventMapper()

	// EventSourcedStorage 생성
	storageOpts := &storage.EventSourcedStorageOptions{
		StorageOptions: &storage.StorageOptions{
			VersionField: "version",
		},
		EventBus:    eventBus,
		EventMapper: eventMapper,
	}

	eventSourcedStorage, err := storage.NewEventSourcedStorage(ctx, client, "mydb", storageOpts)
	if err != nil {
		log.Fatalf("Failed to create EventSourcedStorage: %v", err)
	}

	// 이벤트 핸들러 등록
	eventBus.Subscribe("UserCreated", &UserCreatedHandler{})
	eventBus.Subscribe("UserUpdated", &UserUpdatedHandler{})

	// 사용 예시
	_, err = eventSourcedStorage.Update(ctx, "users", "user123", func(doc interface{}) (interface{}, error) {
		user, ok := doc.(*User)
		if !ok {
			// 새 문서 생성
			user = &User{
				ID:    "user123",
				Name:  "John Doe",
				Email: "john@example.com",
			}
		}

		// 문서 업데이트
		user.Email = "john.doe@example.com"
		return user, nil
	})

	if err != nil {
		log.Fatalf("Failed to update user: %v", err)
	}
}

// User 모델
type User struct {
	ID      string `bson:"_id"`
	Name    string `bson:"name"`
	Email   string `bson:"email"`
	Version int    `bson:"version"`
}

// UserCreatedHandler 이벤트 핸들러
type UserCreatedHandler struct{}

func (h *UserCreatedHandler) HandleEvent(ctx context.Context, e event.Event) error {
	log.Printf("User created: %s", e.AggregateID())
	return nil
}

// UserUpdatedHandler 이벤트 핸들러
type UserUpdatedHandler struct{}

func (h *UserUpdatedHandler) HandleEvent(ctx context.Context, e event.Event) error {
	log.Printf("User updated: %s", e.AggregateID())
	return nil
}
```

### 커스텀 이벤트 매퍼 사용

```go
// 커스텀 이벤트 매퍼 생성
type UserEventMapper struct {
	*event.DefaultEventMapper
}

func NewUserEventMapper() *UserEventMapper {
	mapper := &UserEventMapper{
		DefaultEventMapper: event.NewDefaultEventMapper(),
	}

	// 이벤트 타입 등록
	mapper.RegisterCollectionEventTypes("users", event.CollectionEventTypes{
		Created: "UserCreated",
		Updated: "UserUpdated",
		Deleted: "UserDeleted",
	})

	return mapper
}

// MapToEvents 메서드 오버라이드
func (m *UserEventMapper) MapToEvents(collection string, id string, diff *storage.Diff) []event.Event {
	if collection != "users" {
		return m.DefaultEventMapper.MapToEvents(collection, id, diff)
	}

	events := make([]event.Event, 0, 2)

	// 기본 이벤트 생성
	var eventType string
	if diff.IsNew {
		eventType = "UserCreated"
	} else if diff.HasChanges {
		eventType = "UserUpdated"
	} else {
		eventType = "UserDeleted"
	}

	// 기본 이벤트 추가
	events = append(events, event.NewDefaultEvent(
		eventType,
		id,
		"User",
		diff.Version,
		time.Now(),
		map[string]interface{}{
			"diff":        diff,
			"merge_patch": diff.MergePatch,
		},
	))

	// 이메일 변경 시 추가 이벤트 발행
	if mergePatch, ok := diff.MergePatch.(map[string]interface{}); ok {
		if email, ok := mergePatch["email"].(string); ok {
			events = append(events, event.NewDefaultEvent(
				"UserEmailChanged",
				id,
				"User",
				diff.Version,
				time.Now(),
				map[string]interface{}{
					"new_email": email,
				},
			))
		}
	}

	return events
}
```

### Redis 이벤트 버스 사용

```go
// Redis 이벤트 버스 생성
redisClient := redis.NewClient(&redis.Options{
	Addr: "localhost:6379",
})

serializer := event.NewJSONEventSerializer()
deserializer := event.NewJSONEventDeserializer()

redisEventBus := event.NewRedisEventBus(
	redisClient,
	"app:events",
	serializer,
	deserializer,
)

// 이벤트 버스 시작
if err := redisEventBus.Start(); err != nil {
	log.Fatalf("Failed to start Redis event bus: %v", err)
}
defer redisEventBus.Stop()

// EventSourcedStorage에 Redis 이벤트 버스 사용
eventSourcedStorage, err := storage.NewEventSourcedStorage(ctx, client, "mydb", &storage.EventSourcedStorageOptions{
	StorageOptions: &storage.StorageOptions{
		VersionField: "version",
	},
	EventBus:    redisEventBus,
	EventMapper: NewUserEventMapper(),
})
```

## 프로젝트 구조

```
eventsourced/
├── pkg/
│   ├── aggregate/       # 애그리게이트 관련 코드
│   │   ├── aggregate.go       # 애그리게이트 인터페이스 및 기본 구현
│   │   ├── factory.go         # 애그리게이트 팩토리
│   │   └── repository.go      # 애그리게이트 리포지토리
│   │
│   ├── command/         # 커맨드 관련 코드
│   │   ├── command.go         # 커맨드 인터페이스 및 기본 구현
│   │   ├── handler.go         # 커맨드 핸들러
│   │   └── dispatcher.go      # 커맨드 디스패처
│   │
│   ├── event/           # 이벤트 관련 코드
│   │   ├── event.go           # 이벤트 인터페이스 및 기본 구현
│   │   ├── bus.go             # 이벤트 버스 인터페이스
│   │   ├── memory_bus.go      # 인메모리 이벤트 버스 구현
│   │   ├── redis_bus.go       # Redis 이벤트 버스 구현
│   │   ├── handler.go         # 이벤트 핸들러 인터페이스
│   │   └── mapper.go          # 이벤트 매퍼
│   │
│   └── storage/         # 저장소 관련 코드
│       ├── storage.go         # 기본 저장소 인터페이스 및 구현
│       └── event_sourced.go   # 이벤트 소싱 저장소 구현
│
├── examples/           # 예제 코드
│   └── simple/
│       ├── main.go
│       ├── user.go
│       └── handlers.go
│
├── go.mod
├── go.sum
└── README.md
```

## 아키텍처

EventSourcedStorage는 다음과 같은 주요 컴포넌트로 구성됩니다:

1. **Aggregate**: 도메인 객체와 비즈니스 규칙을 캡슐화
2. **Command**: 시스템에 대한 의도를 표현
3. **Event**: 시스템에서 발생한 사실을 표현
4. **Repository**: 애그리게이트의 저장 및 로드 담당
5. **EventBus**: 이벤트 발행 및 구독 처리
6. **Storage**: 기본 데이터 저장소 (MongoDB 기반)

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│    Command      │────▶│   Aggregate     │────▶│     Event       │
│                 │     │                 │     │                 │
└─────────────────┘     └─────────────────┘     └────────┬────────┘
                                │                         │
                                ▼                         ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│   Repository    │◀───▶│    Storage      │     │    EventBus     │
│                 │     │                 │     │                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                                │                         │
                                ▼                         ▼
                        ┌─────────────────┐     ┌─────────────────┐
                        │                 │     │                 │
                        │    MongoDB      │     │  Event Handlers │
                        │                 │     │                 │
                        └─────────────────┘     └─────────────────┘
```

## 장점

- **데이터 일관성**: 데이터 변경과 이벤트 발행이 논리적으로 함께 처리되어 일관성 유지
- **코드 단순화**: 이벤트 발행 코드를 반복 작성할 필요 없음
- **확장성**: 커스텀 이벤트 매퍼를 통해 세분화된 이벤트 생성 가능
- **성능 최적화**: 데이터 조회와 업데이트가 한 번의 작업으로 처리됨
- **테스트 용이성**: 모의 객체를 통한 테스트 용이

## 추가 문서

- [CQRS 초보자 가이드](docs/cqrs_for_beginners.md): CQRS를 처음 접하는 개발자를 위한 가이드
- [알림 패턴](docs/patterns/notification_pattern.md): 클라이언트에게 변경 사항을 효율적으로 알리는 방법
- [낙관적 동시성 제어 패턴](docs/patterns/optimistic_concurrency.md): 동시성 충돌을 효과적으로 관리하는 방법

## 라이센스

MIT
