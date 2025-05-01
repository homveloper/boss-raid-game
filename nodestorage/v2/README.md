# nodestorage/v2

`nodestorage/v2`는 MongoDB를 기반으로 한 데이터 저장소 패키지로, 분산 환경에서의 낙관적 동시성 제어를 중심으로 설계되었습니다. 이 패키지는 MongoDB의 유연성과 확장성을 최대한 유지하면서 안정적인 데이터 동기화 기능을 제공합니다.

## 주요 기능

- **낙관적 동시성 제어**: 버전 필드를 통한 동시성 충돌 감지 및 자동 재시도
- **섹션 기반 동시성 제어**: 문서 내 특정 섹션에 대한 독립적인 버전 관리
- **MongoDB 네이티브 기능 활용**: 업데이트 연산자와 집계 파이프라인 직접 사용
- **트랜잭션 지원**: 여러 작업을 원자적으로 실행
- **변경 감시**: 실시간 변경 감지 및 알림
- **다양한 캐시 구현체**: 메모리, BadgerDB, Redis 캐시 지원

## 설치

```bash
go get github.com/yourusername/nodestorage/v2
```

## 사용 예시

### 기본 사용법

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/yourusername/nodestorage/v2"
    "github.com/yourusername/nodestorage/v2/cache"
    "go.mongodb.org/mongo-driver/bson/primitive"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

// GameState is a sample document type
type GameState struct {
    ID          primitive.ObjectID `bson:"_id"`
    PlayerName  string             `bson:"player_name"`
    Score       int                `bson:"score"`
    Level       int                `bson:"level"`
    Inventory   []string           `bson:"inventory"`
    VectorClock int64              `bson:"vector_clock"`
}

// Copy creates a deep copy of the document
func (g *GameState) Copy() *GameState {
    if g == nil {
        return nil
    }
    
    inventoryCopy := make([]string, len(g.Inventory))
    copy(inventoryCopy, g.Inventory)
    
    return &GameState{
        ID:          g.ID,
        PlayerName:  g.PlayerName,
        Score:       g.Score,
        Level:       g.Level,
        Inventory:   inventoryCopy,
        VectorClock: g.VectorClock,
    }
}

func main() {
    // Connect to MongoDB
    ctx := context.Background()
    client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
    if err != nil {
        log.Fatalf("Failed to connect to MongoDB: %v", err)
    }
    defer client.Disconnect(ctx)

    // Create a collection
    collection := client.Database("game_db").Collection("game_states")

    // Create a memory cache
    memCache := cache.NewMemoryCache[*GameState](nil)
    defer memCache.Close()

    // Create storage options
    options := &nodestorage.Options{
        VersionField: "vector_clock",
        CacheTTL:     time.Hour,
    }

    // Create storage
    storage, err := nodestorage.NewStorage[*GameState](ctx, client, collection, memCache, options)
    if err != nil {
        log.Fatalf("Failed to create storage: %v", err)
    }
    defer storage.Close()

    // Create a new game state
    gameState := &GameState{
        ID:         primitive.NewObjectID(),
        PlayerName: "Player1",
        Score:      0,
        Level:      1,
        Inventory:  []string{"Sword", "Shield"},
    }

    // Save the game state
    savedState, err := storage.FindOneAndUpsert(ctx, gameState)
    if err != nil {
        log.Fatalf("Failed to save game state: %v", err)
    }
    log.Printf("Saved game state: %+v", savedState)

    // Update the game state
    updatedState, _, err := storage.FindOneAndUpdate(ctx, savedState.ID, func(state *GameState) (*GameState, error) {
        state.Score += 100
        state.Level++
        state.Inventory = append(state.Inventory, "Potion")
        return state, nil
    })
    if err != nil {
        log.Fatalf("Failed to update game state: %v", err)
    }
    log.Printf("Updated game state: %+v", updatedState)

    // Get the game state
    retrievedState, err := storage.FindOne(ctx, savedState.ID)
    if err != nil {
        log.Fatalf("Failed to get game state: %v", err)
    }
    log.Printf("Retrieved game state: %+v", retrievedState)

    // Delete the game state
    err = storage.DeleteOne(ctx, savedState.ID)
    if err != nil {
        log.Fatalf("Failed to delete game state: %v", err)
    }
    log.Println("Game state deleted")
}
```

### 고급 기능 사용법

```go
// MongoDB 업데이트 연산자 직접 사용
updatedState, err := storage.UpdateOne(ctx, gameState.ID, bson.M{
    "$inc": bson.M{
        "score": 50,
        "level": 1,
    },
    "$push": bson.M{
        "inventory": "Gold Coin",
    },
})

// MongoDB 집계 파이프라인 사용
pipeline := mongo.Pipeline{
    bson.D{{Key: "$set", Value: bson.D{
        {Key: "score", Value: bson.D{{Key: "$add", Value: bson.A{"$score", 200}}}},
    }}},
}
updatedState, err := storage.UpdateOneWithPipeline(ctx, gameState.ID, pipeline)

// 섹션 기반 동시성 제어
updatedState, err := storage.UpdateSection(ctx, gameState.ID, "inventory", func(inv interface{}) (interface{}, error) {
    inventory := inv.(bson.M)
    items := inventory["items"].(primitive.A)
    items = append(items, "Diamond")
    inventory["items"] = items
    return inventory, nil
})

// 트랜잭션 사용
err := storage.WithTransaction(ctx, func(sessCtx mongo.SessionContext) error {
    // 플레이어 인벤토리에서 아이템 제거
    _, err := storage.UpdateOne(sessCtx, player1ID, bson.M{
        "$pull": bson.M{"inventory": "Gold Coin"},
    })
    if err != nil {
        return err
    }
    
    // 다른 플레이어 인벤토리에 아이템 추가
    _, err = storage.UpdateOne(sessCtx, player2ID, bson.M{
        "$push": bson.M{"inventory": "Gold Coin"},
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

## 테스트 실행

### 필요 조건

- Go 1.18 이상
- MongoDB 서버 (테스트용)
- Redis 서버 (Redis 캐시 테스트용, 선택 사항)

### 테스트 실행 방법

Windows:
```
run_tests.bat
```

Linux/macOS:
```
chmod +x run_tests.sh
./run_tests.sh
```

또는 직접 Go 테스트 명령어 실행:
```
go test -v ./...
```

Redis 테스트를 위한 환경 변수 설정:
```
REDIS_ADDR=localhost:6379 go test -v ./cache
```

## 라이센스

MIT
