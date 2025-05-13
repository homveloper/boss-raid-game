package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"eventsync"
	"nodestorage/v2"
	"nodestorage/v2/cache"
)

// GameState는 게임 상태를 나타내는 구조체입니다.
type GameState struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Gold        int                `bson:"gold" json:"gold"`
	Level       int                `bson:"level" json:"level"`
	Experience  int                `bson:"experience" json:"experience"`
	LastUpdated time.Time          `bson:"last_updated" json:"lastUpdated"`
	Version     int64              `bson:"version" json:"version"`
}

// Copy는 GameState의 복사본을 생성합니다.
func (g *GameState) Copy() *GameState {
	if g == nil {
		return nil
	}
	return &GameState{
		ID:          g.ID,
		Name:        g.Name,
		Gold:        g.Gold,
		Level:       g.Level,
		Experience:  g.Experience,
		LastUpdated: g.LastUpdated,
		Version:     g.Version,
	}
}

func main() {
	// 로거 설정
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("로거 생성 실패: %v", err)
	}
	defer logger.Sync()

	// MongoDB 연결
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		logger.Fatal("MongoDB 연결 실패", zap.Error(err))
	}
	defer client.Disconnect(ctx)

	// 데이터베이스 및 컬렉션 설정
	dbName := "eventsync_example"
	gameCollection := client.Database(dbName).Collection("games")

	// 1. nodestorage 설정
	memCache := cache.NewMemoryCache[*GameState](nil)
	storageOptions := &nodestorage.Options{
		VersionField:      "version",
		CacheTTL:          time.Minute * 10,
		WatchEnabled:      true,
		WatchFullDocument: "updateLookup",
	}

	storage, err := nodestorage.NewStorage[*GameState](ctx, gameCollection, memCache, storageOptions)
	if err != nil {
		logger.Fatal("Storage 생성 실패", zap.Error(err))
	}
	defer storage.Close()

	// 2. 이벤트 저장소 설정
	eventStore, err := eventsync.NewMongoEventStore(ctx, client, dbName, "events", logger)
	if err != nil {
		logger.Fatal("EventStore 생성 실패", zap.Error(err))
	}

	// 3. EventSourcedStorage 생성
	eventSourcedStorage := eventsync.NewEventSourcedStorage(
		storage,
		eventStore,
		logger,
	)

	// 게임 상태 생성 또는 업데이트
	gameID := primitive.NewObjectID()
	game := &GameState{
		ID:          gameID,
		Name:        "Player 1",
		Gold:        100,
		Level:       1,
		Experience:  0,
		LastUpdated: time.Now(),
		Version:     1,
	}

	// 다양한 클라이언트 ID 시뮬레이션
	clientIDs := []string{"game-server", "admin-panel", "mobile-client"}

	// 게임 상태 생성 (game-server 클라이언트로)
	createdGame, err := eventSourcedStorage.FindOneAndUpsert(ctx, game, clientIDs[0])
	if err != nil {
		logger.Fatal("게임 상태 생성 실패", zap.Error(err))
	}
	logger.Info("게임 상태 생성됨",
		zap.String("id", createdGame.ID.Hex()),
		zap.String("client", clientIDs[0]))

	// 게임 상태 업데이트 (admin-panel 클라이언트로)
	updateFn := func(g *GameState) (*GameState, error) {
		g.Gold += 50
		g.Experience += 100
		g.LastUpdated = time.Now()
		g.Version++
		return g, nil
	}

	updatedGame, diff, err := eventSourcedStorage.FindOneAndUpdate(ctx, gameID, updateFn, clientIDs[1])
	if err != nil {
		logger.Fatal("게임 상태 업데이트 실패", zap.Error(err))
	}
	logger.Info("게임 상태 업데이트됨",
		zap.String("id", updatedGame.ID.Hex()),
		zap.Int("gold", updatedGame.Gold),
		zap.Int("experience", updatedGame.Experience),
		zap.Bool("has_changes", diff.HasChanges),
		zap.String("client", clientIDs[1]))

	// 레벨 업 조건 확인 및 처리 (mobile-client 클라이언트로)
	if updatedGame.Experience >= 100 && updatedGame.Level == 1 {
		levelUpFn := func(g *GameState) (*GameState, error) {
			g.Level++
			g.Experience -= 100
			g.LastUpdated = time.Now()
			g.Version++
			return g, nil
		}

		leveledUpGame, levelDiff, err := eventSourcedStorage.FindOneAndUpdate(ctx, gameID, levelUpFn, clientIDs[2])
		if err != nil {
			logger.Fatal("레벨 업 실패", zap.Error(err))
		}
		logger.Info("레벨 업 성공",
			zap.String("id", leveledUpGame.ID.Hex()),
			zap.Int("level", leveledUpGame.Level),
			zap.Int("experience", leveledUpGame.Experience),
			zap.Bool("has_changes", levelDiff.HasChanges),
			zap.String("client", clientIDs[2]))
	}

	// 모바일 클라이언트의 마지막 버전 시뮬레이션
	lastReceivedVersion := int64(1) // 버전 1까지 받음

	// 누락된 이벤트 조회
	events, err := eventSourcedStorage.GetMissingEvents(ctx, gameID, lastReceivedVersion)
	if err != nil {
		logger.Fatal("누락된 이벤트 조회 실패", zap.Error(err))
	}

	logger.Info("누락된 이벤트 조회됨", zap.Int("event_count", len(events)))
	for i, event := range events {
		logger.Info(fmt.Sprintf("이벤트 #%d", i+1),
			zap.String("operation", event.Operation),
			zap.String("client_id", event.ClientID),
			zap.Int64("server_seq", event.ServerSeq),
			zap.Time("timestamp", event.Timestamp))
	}

	// 게임 상태 삭제 (admin-panel 클라이언트로)
	// err = eventSourcedStorage.DeleteOne(ctx, gameID, clientIDs[1])
	// if err != nil {
	// 	logger.Fatal("게임 상태 삭제 실패", zap.Error(err))
	// }
	// logger.Info("게임 상태 삭제됨",
	// 	zap.String("id", gameID.Hex()),
	// 	zap.String("client", clientIDs[1]))

	logger.Info("예제 실행 완료")
}
