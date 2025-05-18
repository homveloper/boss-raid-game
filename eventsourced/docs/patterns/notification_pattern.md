# 알림 패턴 (Notification Pattern)

CQRS 시스템에서 클라이언트에게 변경 사항을 효율적으로 알리는 방법을 설명합니다.

## 문제

CQRS 시스템에서 데이터가 변경되면 클라이언트에게 이를 알려야 합니다. 그러나 전체 데이터를 매번 전송하는 것은 네트워크 대역폭 낭비가 될 수 있습니다.

## 해결책: 변경 알림 패턴

변경 알림 패턴은 데이터 자체가 아닌 **변경 사실만 알리고**, 클라이언트가 필요할 때 데이터를 조회하도록 하는 패턴입니다.

### 구현 방법

1. **이벤트 구독**: 클라이언트는 관심 있는 이벤트를 구독합니다.
2. **변경 알림**: 서버는 이벤트 발생 시 최소한의 정보만 포함한 알림을 전송합니다.
3. **데이터 조회**: 클라이언트는 알림을 받으면 필요할 때 데이터를 조회합니다.

### 코드 예시

#### 서버 측 구현

```go
// 이벤트 핸들러 등록
eventBus.Subscribe("UserCreated", func(ctx context.Context, e event.Event) error {
    // 클라이언트에게 알림 전송
    notification := Notification{
        Type:        "change",
        ResourceType: "User",
        ResourceID:   e.AggregateID(),
        Action:       "created",
        Timestamp:    time.Now(),
    }
    
    // WebSocket, SSE 등을 통해 알림 전송
    notificationHub.Broadcast(notification)
    return nil
})
```

#### 클라이언트 측 구현

```javascript
// WebSocket 연결
const socket = new WebSocket('ws://localhost:8080/ws');

// 알림 수신
socket.onmessage = function(event) {
    const notification = JSON.parse(event.data);
    
    if (notification.type === 'change' && notification.resource_type === 'User') {
        // 변경된 사용자 데이터 조회
        fetchUser(notification.resource_id).then(user => {
            // UI 업데이트
            updateUserUI(user);
        });
    }
};

// 사용자 데이터 조회 함수
function fetchUser(userId) {
    return fetch(`/api/users/${userId}`)
        .then(response => response.json());
}
```

## 장점

1. **네트워크 효율성**: 필요한 데이터만 전송하여 대역폭 절약
2. **클라이언트 유연성**: 클라이언트가 필요할 때만 데이터 조회 가능
3. **서버 부하 감소**: 모든 변경 사항을 전송하지 않아도 됨
4. **확장성**: 많은 클라이언트가 있어도 효율적으로 동작

## 구현 팁

1. **알림 필터링**: 클라이언트가 관심 있는 리소스 타입만 구독하도록 설정
2. **버전 정보 포함**: 알림에 버전 정보를 포함하여 클라이언트가 최신 상태인지 확인 가능
3. **배치 처리**: 짧은 시간 내에 여러 변경이 발생하면 하나의 알림으로 묶어서 전송
4. **재연결 처리**: 연결이 끊어졌을 때 놓친 알림을 처리하는 메커니즘 구현

## 실제 사용 예시

### 1. 실시간 협업 문서 편집기

여러 사용자가 동시에 문서를 편집할 때, 변경 사항이 발생하면 다른 사용자에게 알림만 전송하고, 각 사용자는 필요한 부분만 조회합니다.

### 2. 대시보드 모니터링 시스템

시스템 상태가 변경될 때 대시보드에 알림만 전송하고, 대시보드는 필요한 데이터만 조회하여 표시합니다.

### 3. 소셜 미디어 피드

새 게시물이 등록되면 팔로워에게 알림만 전송하고, 사용자가 피드를 열 때 실제 데이터를 조회합니다.

## 결론

변경 알림 패턴은 CQRS 시스템에서 클라이언트에게 변경 사항을 효율적으로 알리는 방법입니다. 이 패턴을 사용하면 네트워크 대역폭을 절약하고, 클라이언트에게 더 나은 사용자 경험을 제공할 수 있습니다.
