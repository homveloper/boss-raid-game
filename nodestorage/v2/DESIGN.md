# nodestorage/v2 패키지 설계 문서

## 개요

`nodestorage/v2`는 MongoDB를 기반으로 한 데이터 저장소 패키지로, 분산 환경에서의 낙관적 동시성 제어를 중심으로 설계되었습니다. 이 패키지는 MongoDB의 유연성과 확장성을 최대한 유지하면서 안정적인 데이터 동기화 기능을 제공합니다.

## 설계 원칙

1. **MongoDB 네이티브 기능 활용**: MongoDB의 강력한 쿼리 및 업데이트 기능을 직접 활용할 수 있도록 합니다.
2. **확장성 유지**: 사용자가 필요에 따라 기능을 확장할 수 있도록 유연한 인터페이스를 제공합니다.
3. **낙관적 동시성 제어 강화**: 분산 환경에서의 데이터 일관성을 보장하는 메커니즘을 중심으로 설계합니다.
4. **최소한의 추상화**: 필요한 추상화만 제공하여 MongoDB의 기능을 직접 활용할 수 있도록 합니다.

## 패키지 구조

```
nodestorage/v2/
├── cache/              # 캐싱 관련 인터페이스 및 구현체
│   ├── cache.go        # 캐시 인터페이스 정의
│   ├── memory.go       # 메모리 캐시 구현
│   └── redis.go        # Redis 캐시 구현
│
├── core/               # 핵심 유틸리티 및 로깅
│   ├── log.go          # 로깅 유틸리티
│   └── util.go         # 공통 유틸리티 함수
│
├── storage.go          # 메인 Storage 인터페이스 및 구현체
├── options.go          # 옵션 관련 구조체 및 함수
├── errors.go           # 에러 정의
├── transaction.go      # 트랜잭션 관련 기능
├── section.go          # 섹션 기반 동시성 제어
├── watch.go            # 변경 감시 관련 기능
└── diff.go             # 문서 차이 계산 기능
```

## 주요 인터페이스

### Storage 인터페이스

```go
// Storage는 MongoDB 기반 저장소의 주요 인터페이스입니다.
type Storage[T Cachable[T]] interface {
    // 기본 CRUD 작업
    Get(ctx context.Context, id primitive.ObjectID, opts ...*options.FindOneOptions) (T, error)
    GetByQuery(ctx context.Context, query interface{}, opts ...*options.FindOptions) ([]T, error)
    CreateAndGet(ctx context.Context, data T) (T, error)
    Edit(ctx context.Context, id primitive.ObjectID, editFn EditFunc[T], opts ...EditOption) (T, *Diff, error)
    Delete(ctx context.Context, id primitive.ObjectID) error
    
    // MongoDB 네이티브 기능 활용
    EditWithUpdate(ctx context.Context, id primitive.ObjectID, update bson.M, opts ...EditOption) (T, error)
    EditWithPipeline(ctx context.Context, id primitive.ObjectID, pipeline mongo.Pipeline, opts ...EditOption) (T, error)
    
    // 섹션 기반 동시성 제어
    EditSection(ctx context.Context, id primitive.ObjectID, sectionPath string, editFn func(interface{}) (interface{}, error), opts ...EditOption) (T, error)
    
    // 트랜잭션 지원
    WithTransaction(ctx context.Context, fn func(sessCtx mongo.SessionContext) error) error
    
    // 변경 감시
    Watch(ctx context.Context, pipeline mongo.Pipeline, opts ...*options.ChangeStreamOptions) (<-chan WatchEvent[T], error)
    
    // 기타 유틸리티
    Collection() *mongo.Collection
    Close() error
}
```

### Cachable 인터페이스

```go
// Cachable은 캐싱 및 버전 관리가 가능한 객체의 인터페이스입니다.
// T는 반드시 포인터 타입이어야 합니다.
type Cachable[T any] interface {
    Copy() T                // 객체의 깊은 복사본 생성
    Version(...int64) int64 // 버전 조회 또는 설정 (인자가 제공된 경우)
}
```

## 주요 기능

### 1. 낙관적 동시성 제어

모든 문서는 버전 필드를 가지며, 업데이트 시 버전 확인을 통해 동시성 충돌을 감지합니다. 충돌 발생 시 자동 재시도 메커니즘을 제공합니다.

### 2. 섹션 기반 동시성 제어

문서 내 특정 섹션에 대한 독립적인 버전 관리를 지원하여, 문서의 다른 부분에 대한 동시 업데이트가 가능합니다.

### 3. MongoDB 네이티브 기능 활용

MongoDB의 업데이트 연산자와 집계 파이프라인을 직접 사용할 수 있는 인터페이스를 제공합니다.

### 4. 트랜잭션 지원

여러 작업을 원자적으로 실행할 수 있는 트랜잭션 기능을 제공합니다.

### 5. 변경 감시

MongoDB 변경 스트림을 활용한 실시간 변경 감지 및 알림 기능을 제공합니다.

### 6. 차이 계산

문서 변경 시 이전 버전과의 차이를 계산하여 JSON Patch 형태로 제공합니다.

## 사용 예시

```go
// 저장소 생성
storage, err := nodestorage.NewStorage[*GameState](
    ctx,
    mongoClient,
    collection,
    cacheImpl,
    &nodestorage.Options{
        VersionField: "version",
        CacheTTL:     time.Hour,
    },
)

// 문서 조회
gameState, err := storage.Get(ctx, id)

// 문서 편집 (낙관적 동시성 제어)
updatedState, diff, err := storage.Edit(ctx, id, func(state *GameState) (*GameState, error) {
    state.Score += 100
    state.Level++
    return state, nil
})

// MongoDB 업데이트 연산자 직접 사용
updatedState, err := storage.EditWithUpdate(ctx, id, bson.M{
    "$inc": bson.M{
        "score": 100,
        "level": 1,
    },
    "$push": bson.M{
        "achievements": "LEVEL_UP",
    },
})

// 섹션 기반 편집
updatedState, err := storage.EditSection(ctx, id, "inventory", func(inv interface{}) (interface{}, error) {
    inventory := inv.(bson.M)
    items := inventory["items"].(primitive.A)
    items = append(items, bson.M{"id": "sword", "level": 1})
    inventory["items"] = items
    return inventory, nil
})

// 트랜잭션 사용
err := storage.WithTransaction(ctx, func(sessCtx mongo.SessionContext) error {
    // 플레이어 인벤토리에서 아이템 제거
    _, err := storage.EditWithUpdate(sessCtx, playerID, bson.M{
        "$pull": bson.M{"inventory.items": bson.M{"id": "gold_coin"}},
    })
    if err != nil {
        return err
    }
    
    // 상점 인벤토리에 아이템 추가
    _, err = storage.EditWithUpdate(sessCtx, shopID, bson.M{
        "$inc": bson.M{"gold": 1},
    })
    return err
})

// 변경 감시
events, err := storage.Watch(ctx, mongo.Pipeline{
    bson.D{{Key: "$match", Value: bson.D{{Key: "operationType", Value: "update"}}}},
})
for event := range events {
    fmt.Printf("Document %s was %s\n", event.ID, event.Operation)
    if event.Diff != nil {
        fmt.Printf("Changes: %v\n", event.Diff.JSONPatch)
    }
}
```

## v1과의 차이점

1. MongoDB 네이티브 기능에 대한 직접 접근 제공
2. 섹션 기반 동시성 제어 지원
3. 트랜잭션 지원 추가
4. MongoDB 옵션 직접 전달 지원
5. 더 유연한 변경 감시 기능
