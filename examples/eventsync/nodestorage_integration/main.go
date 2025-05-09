package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"nodestorage/v2"
	"nodestorage/v2/cache"
	"eventsync"
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

// StorageEventHandler는 nodestorage 이벤트를 처리하는 핸들러입니다.
type StorageEventHandler struct {
	eventStore eventsync.EventStore
	logger     *zap.Logger
}

// NewStorageEventHandler는 새로운 StorageEventHandler를 생성합니다.
func NewStorageEventHandler(eventStore eventsync.EventStore, logger *zap.Logger) *StorageEventHandler {
	return &StorageEventHandler{
		eventStore: eventStore,
		logger:     logger,
	}
}

// HandleEvent는 nodestorage 이벤트를 처리합니다.
func (h *StorageEventHandler) HandleEvent(ctx context.Context, event nodestorage.WatchEvent[*GameState]) error {
	// nodestorage 이벤트를 eventsync 이벤트로 변환
	syncEvent := &eventsync.Event{
		ID:          primitive.NewObjectID(),
		DocumentID:  event.ID,
		Timestamp:   time.Now(),
		Operation:   event.Operation,
		Diff:        event.Diff,
		ClientID:    "server",
		VectorClock: make(map[string]int64),
	}

	// 시퀀스 번호는 이벤트 저장소에서 자동 할당됨

	// 이벤트 저장
	err := h.eventStore.StoreEvent(ctx, syncEvent)
	if err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}

	h.logger.Info("Storage event converted and stored",
		zap.String("document_id", event.ID.Hex()),
		zap.String("operation", event.Operation))

	return nil
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
	dbName := "nodestorage_eventsync_example"
	gameCollection := client.Database(dbName).Collection("games")
	eventCollection := client.Database(dbName).Collection("events")

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

	// 2. eventsync 설정
	eventStore, err := eventsync.NewMongoEventStore(ctx, client, dbName, "events", logger)
	if err != nil {
		logger.Fatal("이벤트 저장소 생성 실패", zap.Error(err))
	}

	// 3. nodestorage 이벤트 핸들러 설정
	eventHandler := NewStorageEventHandler(eventStore, logger)

	// 4. nodestorage 이벤트 구독
	watchCh, err := storage.Watch(ctx, mongo.Pipeline{}, nil)
	if err != nil {
		logger.Fatal("Watch 설정 실패", zap.Error(err))
	}

	// 이벤트 처리 고루틴 시작
	go func() {
		for event := range watchCh {
			if err := eventHandler.HandleEvent(ctx, event); err != nil {
				logger.Error("이벤트 처리 실패", zap.Error(err))
			}
		}
	}()

	logger.Info("이벤트 처리 시작")

	// 5. 테스트 데이터 생성
	gameID := primitive.NewObjectID()
	game := &GameState{
		ID:          gameID,
		Name:        "TestPlayer",
		Gold:        100,
		Level:       1,
		Experience:  0,
		LastUpdated: time.Now(),
		Version:     1,
	}

	// 게임 생성
	_, err = storage.CreateAndGet(ctx, game)
	if err != nil {
		logger.Fatal("게임 생성 실패", zap.Error(err))
	}
	logger.Info("게임 생성 완료", zap.String("id", gameID.Hex()))

	// 잠시 대기
	time.Sleep(500 * time.Millisecond)

	// 6. 게임 상태 업데이트
	for i := 0; i < 5; i++ {
		_, _, err = storage.FindOneAndUpdate(ctx, gameID, func(g *GameState) (*GameState, error) {
			g.Gold += 50
			g.Experience += 100
			if g.Experience >= 1000 {
				g.Level++
				g.Experience -= 1000
			}
			g.LastUpdated = time.Now()
			g.Version++
			return g, nil
		})

		if err != nil {
			logger.Error("게임 업데이트 실패", zap.Error(err))
			continue
		}

		logger.Info("게임 업데이트 완료", zap.Int("update", i+1))
		time.Sleep(500 * time.Millisecond)
	}

	// 7. 저장된 이벤트 조회
	events, err := eventStore.GetEvents(ctx, gameID, 0)
	if err != nil {
		logger.Fatal("이벤트 조회 실패", zap.Error(err))
	}

	logger.Info("저장된 이벤트", zap.Int("count", len(events)))
	for i, event := range events {
		logger.Info(fmt.Sprintf("이벤트 %d", i+1),
			zap.Int64("sequence", event.SequenceNum),
			zap.String("operation", event.Operation),
			zap.Time("timestamp", event.Timestamp))
	}

	// 8. 최종 게임 상태 조회
	finalGame, err := storage.FindOne(ctx, gameID)
	if err != nil {
		logger.Fatal("게임 조회 실패", zap.Error(err))
	}

	logger.Info("최종 게임 상태",
		zap.String("name", finalGame.Name),
		zap.Int("gold", finalGame.Gold),
		zap.Int("level", finalGame.Level),
		zap.Int("experience", finalGame.Experience),
		zap.Time("lastUpdated", finalGame.LastUpdated),
		zap.Int64("version", finalGame.Version))

	logger.Info("프로그램 종료")
}
