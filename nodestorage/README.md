# NodeStorage

NodeStorage는 분산 서버 환경에서 낙관적 동시성 제어를 기반으로 데이터 동기화를 제공하는 패키지입니다.

## 주요 기능

- **제네릭 지원**: Go 1.18+ 제네릭을 활용한 타입 안전성
- **낙관적 동시성 제어**: 버전 기반 낙관적 동시성 제어로 데이터 일관성 보장
- **다중 저장소 계층**: MongoDB 영구 저장소와 BadgerDB 캐시 저장소 지원
- **실시간 동기화**: Redis PubSub을 통한 분산 서버 간 실시간 데이터 동기화
- **자동 재시도**: 충돌 발생 시 자동 재시도 및 지수 백오프 지원
- **변경 감지**: 문서 변경 사항에 대한 감시 기능 제공
- **JSON 패치**: 변경 사항을 JSON 패치 형식으로 제공

## 설치

```bash
go get github.com/yourusername/nodestorage
```

## 사용법

### 기본 설정

```go
import (
    "context"
    "log"

    "github.com/yourusername/nodestorage"
)

func main() {
    // 기본 옵션으로 저장소 생성
    options := nodestorage.DefaultOptions()
    options.MongoURI = "mongodb://localhost:27017"
    options.MongoDatabase = "mydb"
    options.BadgerPath = "./badger-data"
    options.RedisAddr = "localhost:6379"

    ctx := context.Background()
    storage, err := nodestorage.NewStorage(ctx, options)
    if err != nil {
        log.Fatalf("저장소 생성 실패: %v", err)
    }
    defer storage.Close()

    // 이제 저장소를 사용할 수 있습니다
}
```

### Cachable 인터페이스 구현

```go
// UserProfile은 Cachable 인터페이스를 구현합니다
type UserProfile struct {
    ID       string    `json:"id"`
    Username string    `json:"username"`
    Email    string    `json:"email"`
    LastSeen time.Time `json:"last_seen"`
    version  int64     `json:"-"`
}

// Copy는 객체의 깊은 복사본을 생성합니다
func (p UserProfile) Copy() UserProfile {
    return UserProfile{
        ID:       p.ID,
        Username: p.Username,
        Email:    p.Email,
        LastSeen: p.LastSeen,
        version:  p.version,
    }
}

// Version은 버전을 가져오거나 설정합니다
func (p UserProfile) Version(v ...int64) int64 {
    if len(v) > 0 {
        p.version = v[0]
    }
    return p.version
}
```

### 문서 생성 및 수정

```go
// 문서 생성
profile := UserProfile{
    ID:       "user123",
    Username: "johndoe",
    Email:    "john@example.com",
    LastSeen: time.Now(),
}

// 직렬화
profileBytes, _ := json.Marshal(profile)

// 문서 저장
docID := "user:" + profile.ID
err = storage.CreateAndGet(ctx, docID, profileBytes)
if err != nil {
    log.Fatalf("문서 생성 실패: %v", err)
}

// 문서 수정 (낙관적 동시성 제어)
editOptions := nodestorage.DefaultEditOptions()
updatedBytes, diff, err := storage.Edit(ctx, docID, func(docBytes []byte) ([]byte, error) {
    var profile UserProfile
    if err := json.Unmarshal(docBytes, &profile); err != nil {
        return nil, err
    }

    // 프로필 수정
    profile.Username = "johndoe2"
    profile.LastSeen = time.Now()

    // 수정된 프로필 직렬화
    return json.Marshal(profile)
}, editOptions)

if err != nil {
    log.Fatalf("문서 수정 실패: %v", err)
}

// 변경 사항 확인
fmt.Printf("JSON Patch: %v\n", diff.JSONPatch)
fmt.Printf("Merge Patch: %s\n", string(diff.MergePatch))
```

### 변경 감시

```go
// 변경 감시 시작
watchChan, err := storage.Watch(ctx)
if err != nil {
    log.Fatalf("감시 시작 실패: %v", err)
}

// 변경 이벤트 처리
go func() {
    for event := range watchChan {
        fmt.Printf("이벤트: %s %s\n", event.Operation, event.ID)
        if event.Diff != nil {
            fmt.Printf("  JSON Patch: %v\n", event.Diff.JSONPatch)
            fmt.Printf("  Merge Patch: %s\n", string(event.Diff.MergePatch))
        }
    }
}()
```

## 아키텍처

NodeStorage는 다음과 같은 계층 구조로 설계되었습니다:

1. **Storage 계층**: 주요 인터페이스 및 구현체
2. **Cache 계층**: BadgerDB 기반 캐시 저장소
3. **Database 계층**: MongoDB 기반 영구 저장소
4. **PubSub 계층**: Redis 기반 발행-구독 시스템

## 동시성 제어

NodeStorage는 버전 기반 낙관적 동시성 제어를 사용합니다:

1. 문서를 읽고 현재 버전을 확인합니다.
2. 문서를 수정합니다.
3. 버전을 증가시킵니다.
4. 원래 버전과 함께 업데이트를 시도합니다.
5. 버전 충돌이 발생하면 재시도합니다.

## 성능 고려 사항

- **캐시 크기**: `MaxCacheSize` 옵션으로 캐시 크기를 조정할 수 있습니다.
- **TTL**: `CacheTTL` 옵션으로 캐시 항목의 수명을 설정할 수 있습니다.
- **재시도 횟수**: `MaxRetries` 옵션으로 최대 재시도 횟수를 설정할 수 있습니다.
- **재시도 지연**: `RetryDelay` 및 `MaxRetryDelay` 옵션으로 재시도 지연을 조정할 수 있습니다.

## 라이선스

MIT
