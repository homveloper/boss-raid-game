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

	// 3. 벡터 시계 관리자 생성 (MongoDB 기반 또는 기본 구현 사용)
	// MongoDB 기반 벡터 시계 관리자 생성
	vectorClockManager, err := eventsync.NewMongoVectorClockManager(ctx, client, dbName, "vector_clocks", logger)
	if err != nil {
		logger.Fatal("벡터 시계 관리자 생성 실패", zap.Error(err))
	}

	// 기본 구현을 사용하려면 아래 코드 사용
	// vectorClockManager := eventsync.NewDefaultVectorClockManager()

	// 4. EventSyncStorage 생성 (옵션 설정)
	eventSyncStorage := eventsync.NewEventSyncStorage(
		storage,
		eventStore,
		logger,
		eventsync.WithClientID("game-server"),
		eventsync.WithVectorClockManager(vectorClockManager),
	)

	// 4. 상태 벡터 관리자 설정
	stateVectorManager, err := eventsync.NewMongoStateVectorManager(ctx, client, dbName, "state_vectors", eventStore, logger)
	if err != nil {
		logger.Fatal("StateVectorManager 생성 실패", zap.Error(err))
	}

	// 5. 동기화 서비스 설정
	syncService := eventsync.NewSyncService(eventStore, stateVectorManager, logger)

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

	// 게임 상태 생성
	createdGame, err := eventSyncStorage.FindOneAndUpsert(ctx, game)
	if err != nil {
		logger.Fatal("게임 상태 생성 실패", zap.Error(err))
	}
	logger.Info("게임 상태 생성됨", zap.String("id", createdGame.ID.Hex()))

	// 게임 상태 업데이트
	updateFn := func(g *GameState) (*GameState, error) {
		g.Gold += 50
		g.Experience += 100
		g.LastUpdated = time.Now()
		g.Version++
		return g, nil
	}

	updatedGame, diff, err := eventSyncStorage.FindOneAndUpdate(ctx, gameID, updateFn)
	if err != nil {
		logger.Fatal("게임 상태 업데이트 실패", zap.Error(err))
	}
	logger.Info("게임 상태 업데이트됨",
		zap.String("id", updatedGame.ID.Hex()),
		zap.Int("gold", updatedGame.Gold),
		zap.Int("experience", updatedGame.Experience),
		zap.Bool("has_changes", diff.HasChanges))

	// 레벨 업 조건 확인 및 처리
	if updatedGame.Experience >= 100 && updatedGame.Level == 1 {
		levelUpFn := func(g *GameState) (*GameState, error) {
			g.Level++
			g.Experience -= 100
			g.LastUpdated = time.Now()
			g.Version++
			return g, nil
		}

		leveledUpGame, levelDiff, err := eventSyncStorage.FindOneAndUpdate(ctx, gameID, levelUpFn)
		if err != nil {
			logger.Fatal("레벨 업 실패", zap.Error(err))
		}
		logger.Info("레벨 업 성공",
			zap.String("id", leveledUpGame.ID.Hex()),
			zap.Int("level", leveledUpGame.Level),
			zap.Int("experience", leveledUpGame.Experience),
			zap.Bool("has_changes", levelDiff.HasChanges))
	}

	// 누락된 이벤트 조회 예시
	clientID := "client1"
	vectorClock := map[string]int64{"server": 0}

	// 클라이언트 등록
	err = syncService.RegisterClient(ctx, clientID)
	if err != nil {
		logger.Fatal("클라이언트 등록 실패", zap.Error(err))
	}

	// 누락된 이벤트 조회
	events, err := syncService.GetMissingEvents(ctx, clientID, gameID, vectorClock)
	if err != nil {
		logger.Fatal("누락된 이벤트 조회 실패", zap.Error(err))
	}

	logger.Info("누락된 이벤트 조회됨", zap.Int("event_count", len(events)))
	for i, event := range events {
		logger.Info(fmt.Sprintf("이벤트 #%d", i+1),
			zap.String("operation", event.Operation),
			zap.Int64("sequence_num", event.SequenceNum),
			zap.Time("timestamp", event.Timestamp))
	}

	// 벡터 시계 업데이트
	if len(events) > 0 {
		lastEvent := events[len(events)-1]
		vectorClock["server"] = lastEvent.SequenceNum

		err = syncService.UpdateVectorClock(ctx, clientID, gameID, vectorClock)
		if err != nil {
			logger.Fatal("벡터 시계 업데이트 실패", zap.Error(err))
		}
		logger.Info("벡터 시계 업데이트됨", zap.Any("vector_clock", vectorClock))
	}

	// 게임 상태 삭제
	// err = eventSyncStorage.DeleteOne(ctx, gameID)
	// if err != nil {
	// 	logger.Fatal("게임 상태 삭제 실패", zap.Error(err))
	// }
	// logger.Info("게임 상태 삭제됨", zap.String("id", gameID.Hex()))

	logger.Info("예제 실행 완료")
}
