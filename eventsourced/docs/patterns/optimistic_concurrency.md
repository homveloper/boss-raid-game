# 낙관적 동시성 제어 패턴 (Optimistic Concurrency Control)

CQRS 시스템에서 동시성 충돌을 효과적으로 관리하는 방법을 설명합니다.

## 문제

여러 사용자가 동시에 같은 데이터를 수정하려고 할 때, 한 사용자의 변경 사항이 다른 사용자의 변경 사항을 덮어쓰는 문제가 발생할 수 있습니다.

## 해결책: 낙관적 동시성 제어

낙관적 동시성 제어는 충돌이 드물게 발생한다고 가정하고, 변경 시점에 충돌을 감지하여 처리하는 방식입니다.

### 구현 방법

1. **버전 추적**: 각 데이터에 버전 필드를 추가합니다.
2. **버전 확인**: 데이터 수정 시 현재 버전과 예상 버전이 일치하는지 확인합니다.
3. **버전 증가**: 수정이 성공하면 버전을 증가시킵니다.
4. **충돌 처리**: 버전이 일치하지 않으면 충돌로 간주하고 적절히 처리합니다.

### 코드 예시

#### 서버 측 구현

```go
// 낙관적 동시성 제어를 사용한 업데이트
func (s *UserService) UpdateEmail(ctx context.Context, userID string, email string, expectedVersion int) error {
    // 애그리게이트 로드
    user, err := s.repository.Load(ctx, userID, "User")
    if err != nil {
        return err
    }
    
    // 버전 확인
    if user.Version() != expectedVersion {
        return ErrConcurrencyConflict
    }
    
    // 이메일 업데이트
    if err := user.UpdateEmail(email); err != nil {
        return err
    }
    
    // 애그리게이트 저장 (버전 자동 증가)
    return s.repository.Save(ctx, user)
}
```

#### 클라이언트 측 구현

```javascript
// 사용자 정보 로드
async function loadUser(userId) {
    const response = await fetch(`/api/users/${userId}`);
    return response.json();
}

// 이메일 업데이트
async function updateEmail(userId, email, version) {
    try {
        const response = await fetch(`/api/users/${userId}/email`, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
                'If-Match': version.toString() // 예상 버전을 헤더에 포함
            },
            body: JSON.stringify({ email })
        });
        
        if (response.status === 409) { // 충돌 상태 코드
            // 충돌 처리
            const user = await loadUser(userId);
            alert(`다른 사용자가 이미 데이터를 변경했습니다. 최신 데이터: ${user.email}`);
            return user;
        }
        
        return await response.json();
    } catch (error) {
        console.error('이메일 업데이트 실패:', error);
        throw error;
    }
}

// 사용 예시
async function handleEmailUpdate() {
    const user = await loadUser('user123');
    const newEmail = document.getElementById('email').value;
    
    try {
        const updatedUser = await updateEmail('user123', newEmail, user.version);
        displayUser(updatedUser);
    } catch (error) {
        // 오류 처리
    }
}
```

## 장점

1. **데이터 일관성**: 충돌을 감지하여 데이터 일관성 유지
2. **성능 최적화**: 잠금(lock)을 사용하지 않아 성능 향상
3. **사용자 경험**: 충돌 시 적절한 피드백 제공 가능
4. **확장성**: 분산 시스템에서도 효과적으로 동작

## 구현 팁

1. **자동 재시도**: 충돌 발생 시 자동으로 재시도하는 메커니즘 구현
2. **충돌 해결 UI**: 사용자가 충돌을 해결할 수 있는 인터페이스 제공
3. **버전 관리**: 버전 필드를 숨기고 ETag나 Last-Modified 헤더 활용
4. **부분 업데이트**: 전체 객체가 아닌 변경된 필드만 업데이트하여 충돌 가능성 감소

## 실제 사용 예시

### 1. 협업 문서 편집

여러 사용자가 동시에 문서를 편집할 때, 각 사용자의 변경 사항을 버전 관리하여 충돌을 감지하고 해결합니다.

### 2. 재고 관리 시스템

여러 직원이 동시에 재고를 수정할 때, 낙관적 동시성 제어를 통해 재고 데이터의 일관성을 유지합니다.

### 3. 예약 시스템

여러 사용자가 동시에 같은 자원을 예약하려고 할 때, 버전 확인을 통해 중복 예약을 방지합니다.

## 결론

낙관적 동시성 제어는 CQRS 시스템에서 동시성 충돌을 효과적으로 관리하는 방법입니다. 이 패턴을 사용하면 데이터 일관성을 유지하면서도 성능과 사용자 경험을 향상시킬 수 있습니다.
