# CQRS 초보자 가이드

이 문서는 CQRS(Command Query Responsibility Segregation) 패턴을 처음 접하는 개발자를 위한 가이드입니다. 복잡한 이론보다는 실용적인 관점에서 CQRS의 장점을 최대한 활용하는 방법을 설명합니다.

## CQRS란 무엇인가?

CQRS는 간단히 말해 **쓰기 작업(Command)과 읽기 작업(Query)을 분리**하는 패턴입니다. 이렇게 분리함으로써 각각의 작업을 독립적으로 최적화할 수 있습니다.

### 전통적인 CRUD 모델 vs CQRS 모델

**전통적인 CRUD 모델:**
```
사용자 → 단일 모델 → 데이터베이스
```

**CQRS 모델:**
```
사용자 → 명령 모델 → 데이터베이스
      ↘ 조회 모델 ↗
```

## CQRS의 핵심 개념

### 1. Command (명령)

명령은 시스템에 변경을 요청하는 것입니다. 예를 들어:
- 사용자 생성
- 이메일 변경
- 주문 취소

명령은 항상 의도(intent)를 나타내며, 과거형이 아닌 명령형으로 표현합니다.

### 2. Query (조회)

조회는 시스템에서 데이터를 읽어오는 것입니다. 예를 들어:
- 사용자 정보 조회
- 주문 목록 조회
- 대시보드 통계 조회

조회는 시스템의 상태를 변경하지 않습니다.

### 3. Event (이벤트)

이벤트는 시스템에서 발생한 사실을 나타냅니다. 예를 들어:
- 사용자가 생성됨
- 이메일이 변경됨
- 주문이 취소됨

이벤트는 항상 과거형으로 표현하며, 이미 발생한 사실을 나타냅니다.

## CQRS의 장점

### 1. 성능 최적화

- **읽기 최적화**: 조회 모델을 읽기에 최적화된 형태로 설계 가능
- **쓰기 최적화**: 명령 모델을 일관성과 비즈니스 규칙에 집중하여 설계 가능
- **확장성**: 읽기와 쓰기를 독립적으로 확장 가능

### 2. 비즈니스 규칙 관리

- **명확한 의도**: 명령은 사용자의 의도를 명확하게 표현
- **규칙 집중**: 비즈니스 규칙을 명령 처리에 집중시켜 관리 용이
- **유지보수성**: 비즈니스 규칙 변경 시 영향 범위 최소화

### 3. 협업 및 동시성

- **충돌 감소**: 낙관적 동시성 제어를 통한 충돌 관리
- **이벤트 기반**: 이벤트를 통한 시스템 간 느슨한 결합
- **실시간 업데이트**: 이벤트를 통한 실시간 데이터 동기화

## EventSourcedStorage로 CQRS 시작하기

EventSourcedStorage는 CQRS 패턴을 쉽게 구현할 수 있도록 도와주는 패키지입니다. 복잡한 이론을 몰라도 간단하게 사용할 수 있습니다.

### 1. 기본 설정

```go
// MongoDB 연결
client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
if err != nil {
    log.Fatalf("Failed to connect to MongoDB: %v", err)
}

// SimpleCQRS 생성
simpleCQRS, err := helper.NewSimpleCQRS(ctx, client, "mydb")
if err != nil {
    log.Fatalf("Failed to create SimpleCQRS: %v", err)
}
```

### 2. 애그리게이트 정의

애그리게이트는 관련된 객체들의 집합으로, 하나의 단위로 처리됩니다.

```go
// UserAggregate 정의
type UserAggregate struct {
    *aggregate.BaseAggregate
    Name  string
    Email string
}

// 애그리게이트 등록
simpleCQRS.RegisterAggregate("User", func(id string) aggregate.Aggregate {
    return NewUserAggregate(id)
})
```

### 3. 명령 처리기 등록

명령을 처리하는 함수를 등록합니다.

```go
// 명령 처리기 등록
simpleCQRS.RegisterCommandHandler("CreateUser", handleCreateUser)
simpleCQRS.RegisterCommandHandler("UpdateUser", handleUpdateUser)
```

### 4. 이벤트 구독

이벤트가 발생했을 때 실행할 함수를 등록합니다.

```go
// 이벤트 구독
simpleCQRS.RegisterEventHandlerFunc("UserCreated", func(ctx context.Context, e event.Event) error {
    log.Printf("새 사용자가 생성되었습니다: %s", e.AggregateID())
    return nil
})
```

### 5. 명령 실행

명령을 실행하여 시스템의 상태를 변경합니다.

```go
// 명령 실행
err = simpleCQRS.ExecuteCommandWithType(ctx, "CreateUser", "user123", "User", map[string]interface{}{
    "name":  "홍길동",
    "email": "hong@example.com",
})
```

## 실제 사용 사례

### 1. 사용자 관리 시스템

```go
// 사용자 생성
simpleCQRS.ExecuteCommand(ctx, "CreateUser", "user123", map[string]interface{}{
    "name":  "홍길동",
    "email": "hong@example.com",
})

// 이메일 변경
simpleCQRS.ExecuteCommand(ctx, "UpdateUserEmail", "user123", map[string]interface{}{
    "email": "hong.gildong@example.com",
})

// 사용자 정보 조회 (별도의 조회 모델 사용)
user := userQueryService.GetUserById("user123")
```

### 2. 주문 관리 시스템

```go
// 주문 생성
simpleCQRS.ExecuteCommand(ctx, "CreateOrder", "order123", map[string]interface{}{
    "customer_id": "customer123",
    "items": []map[string]interface{}{
        {"product_id": "product1", "quantity": 2},
        {"product_id": "product2", "quantity": 1},
    },
})

// 주문 취소
simpleCQRS.ExecuteCommand(ctx, "CancelOrder", "order123", map[string]interface{}{
    "reason": "고객 요청",
})

// 주문 목록 조회 (별도의 조회 모델 사용)
orders := orderQueryService.GetOrdersByCustomerId("customer123")
```

## 자주 묻는 질문

### Q: CQRS는 항상 이벤트 소싱과 함께 사용해야 하나요?
A: 아니요. CQRS와 이벤트 소싱은 별개의 패턴입니다. CQRS는 이벤트 소싱 없이도 구현할 수 있으며, 이벤트 소싱은 CQRS 없이도 사용할 수 있습니다. 다만, 두 패턴을 함께 사용하면 시너지 효과가 있습니다.

### Q: 모든 시스템에 CQRS를 적용해야 하나요?
A: 아니요. CQRS는 복잡한 도메인과 높은 확장성이 필요한 시스템에 적합합니다. 간단한 CRUD 애플리케이션에는 오버엔지니어링이 될 수 있습니다.

### Q: CQRS를 사용하면 시스템이 더 복잡해지지 않나요?
A: 초기에는 복잡성이 증가할 수 있지만, 시스템이 커지고 복잡해질수록 CQRS의 이점이 더 명확해집니다. EventSourcedStorage와 같은 도구를 사용하면 복잡성을 관리하기 쉬워집니다.

### Q: 읽기 모델과 쓰기 모델을 항상 다른 데이터베이스에 저장해야 하나요?
A: 아니요. 같은 데이터베이스를 사용할 수 있습니다. 중요한 것은 모델의 논리적 분리입니다. 필요에 따라 물리적으로도 분리할 수 있습니다.

## 결론

CQRS는 복잡해 보이지만, 핵심 개념은 간단합니다: 쓰기와 읽기를 분리하여 각각 최적화하는 것입니다. EventSourcedStorage를 사용하면 CQRS의 복잡성을 추상화하여 쉽게 시작할 수 있습니다.

CQRS의 모든 이론을 완벽하게 이해하지 않아도, 실용적인 관점에서 그 장점을 활용할 수 있습니다. 시스템의 규모와 복잡성이 증가함에 따라 CQRS의 이점은 더욱 명확해질 것입니다.
