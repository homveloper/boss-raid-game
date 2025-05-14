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
)

// GameState는 게임 상태를 나타내는 구조체입니다.
type GameState struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string             `bson:"name" json:"name"`
	Score     int                `bson:"score" json:"score"`
	Level     int                `bson:"level" json:"level"`
	Version   int64              `bson:"Version" json:"version"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updatedAt"`
}

// Clone은 GameState의 복제본을 반환합니다.
func (g GameState) Clone() GameState {
	return GameState{
		ID:        g.ID,
		Name:      g.Name,
		Score:     g.Score,
		Level:     g.Level,
		Version:   g.Version,
		UpdatedAt: g.UpdatedAt,
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// MongoDB 클라이언트 생성
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		logger.Fatal("MongoDB 연결 실패", zap.Error(err))
	}
	defer client.Disconnect(ctx)

	// MongoDB 연결 확인
	err = client.Ping(ctx, nil)
	if err != nil {
		logger.Fatal("MongoDB 핑 실패", zap.Error(err))
	}
	logger.Info("MongoDB 연결 성공")

	// 데이터베이스 및 컬렉션 설정
	dbName := "eventsync_example"
	collectionName := "game_states"

	// 클라이언트 ID 설정 (실제 환경에서는 사용자 또는 세션 ID 사용)
	clientIDs := []string{
		"game-client-1",
		"admin-panel",
		"analytics-service",
	}

	// MongoDB 저장소 제공자 생성
	provider := eventsync.NewMongoDBStorageProvider(client, dbName, logger)

	// EventSourcedStorage 생성 (옵션 패턴 사용)
	eventSourcedStorage, err := eventsync.NewEventSourcedStorageWithOptions[GameState](
		ctx,
		provider,
		collectionName,
		eventsync.WithEventCollectionName("game_events"),
		eventsync.WithEnableSnapshot(true),
		eventsync.WithSnapshotCollectionName("game_snapshots"),
		eventsync.WithFactoryAutoSnapshot(true),
		eventsync.WithFactorySnapshotInterval(5), // 5개 이벤트마다 스냅샷 생성
		eventsync.WithFactoryLogger(logger),
	)
	if err != nil {
		logger.Fatal("EventSourcedStorage 생성 실패", zap.Error(err))
	}
	defer eventSourcedStorage.Close()

	// 게임 상태 생성 (game-client-1 클라이언트로)
	gameState := GameState{
		ID:        primitive.NewObjectID(),
		Name:      "플레이어 1의 게임",
		Score:     0,
		Level:     1,
		UpdatedAt: time.Now(),
	}

	// 게임 상태 저장
	createdGame, err := eventSourcedStorage.FindOneAndUpsert(ctx, gameState, clientIDs[0])
	if err != nil {
		logger.Fatal("게임 상태 저장 실패", zap.Error(err))
	}

	gameID := createdGame.ID
	logger.Info("게임 상태 생성됨",
		zap.String("id", gameID.Hex()),
		zap.String("name", createdGame.Name),
		zap.Int("score", createdGame.Score),
		zap.Int("level", createdGame.Level))

	// 게임 상태 업데이트 (점수 증가)
	updatedGame, diff, err := eventSourcedStorage.FindOneAndUpdate(ctx, gameID, func(game GameState) (GameState, error) {
		game.Score += 100
		game.UpdatedAt = time.Now()
		return game, nil
	}, clientIDs[0])
	if err != nil {
		logger.Fatal("게임 상태 업데이트 실패", zap.Error(err))
	}

	logger.Info("게임 상태 업데이트됨 (점수 증가)",
		zap.String("id", gameID.Hex()),
		zap.Int("score", updatedGame.Score),
		zap.Int64("version", diff.Version))

	// 게임 상태 업데이트 (레벨 증가)
	updatedGame, diff, err = eventSourcedStorage.FindOneAndUpdate(ctx, gameID, func(game GameState) (GameState, error) {
		game.Level += 1
		game.UpdatedAt = time.Now()
		return game, nil
	}, clientIDs[0])
	if err != nil {
		logger.Fatal("게임 상태 업데이트 실패", zap.Error(err))
	}

	logger.Info("게임 상태 업데이트됨 (레벨 증가)",
		zap.String("id", gameID.Hex()),
		zap.Int("level", updatedGame.Level),
		zap.Int64("version", diff.Version))

	// 게임 상태 업데이트 (관리자 패널에서 점수 조정)
	updatedGame, diff, err = eventSourcedStorage.FindOneAndUpdate(ctx, gameID, func(game GameState) (GameState, error) {
		game.Score += 500 // 관리자 보너스
		game.UpdatedAt = time.Now()
		return game, nil
	}, clientIDs[1])
	if err != nil {
		logger.Fatal("게임 상태 업데이트 실패", zap.Error(err))
	}

	logger.Info("게임 상태 업데이트됨 (관리자 보너스)",
		zap.String("id", gameID.Hex()),
		zap.Int("score", updatedGame.Score),
		zap.Int64("version", diff.Version))

	// 게임 상태 업데이트 (분석 서비스에서 메타데이터 추가)
	updatedGame, diff, err = eventSourcedStorage.FindOneAndUpdate(ctx, gameID, func(game GameState) (GameState, error) {
		game.Name = fmt.Sprintf("%s [VIP]", game.Name) // VIP 태그 추가
		game.UpdatedAt = time.Now()
		return game, nil
	}, clientIDs[2])
	if err != nil {
		logger.Fatal("게임 상태 업데이트 실패", zap.Error(err))
	}

	logger.Info("게임 상태 업데이트됨 (VIP 태그 추가)",
		zap.String("id", gameID.Hex()),
		zap.String("name", updatedGame.Name),
		zap.Int64("version", diff.Version))

	// 게임 상태 업데이트 (게임 클라이언트에서 점수 및 레벨 증가)
	updatedGame, diff, err = eventSourcedStorage.FindOneAndUpdate(ctx, gameID, func(game GameState) (GameState, error) {
		game.Score += 200
		game.Level += 1
		game.UpdatedAt = time.Now()
		return game, nil
	}, clientIDs[0])
	if err != nil {
		logger.Fatal("게임 상태 업데이트 실패", zap.Error(err))
	}

	logger.Info("게임 상태 업데이트됨 (점수 및 레벨 증가)",
		zap.String("id", gameID.Hex()),
		zap.Int("score", updatedGame.Score),
		zap.Int("level", updatedGame.Level),
		zap.Int64("version", diff.Version))

	// 이벤트 조회
	events, err := eventSourcedStorage.GetEvents(ctx, gameID, 0)
	if err != nil {
		logger.Fatal("이벤트 조회 실패", zap.Error(err))
	}

	logger.Info("이벤트 조회됨", zap.Int("event_count", len(events)))
	for i, event := range events {
		logger.Info(fmt.Sprintf("이벤트 #%d", i+1),
			zap.String("operation", event.Operation),
			zap.String("client_id", event.ClientID),
			zap.Int64("server_seq", event.ServerSeq),
			zap.Time("timestamp", event.Timestamp))
	}

	// 스냅샷과 이벤트 함께 조회
	latestSnapshot, eventsAfterSnapshot, err := eventSourcedStorage.GetEventsWithSnapshot(ctx, gameID)
	if err != nil {
		logger.Fatal("스냅샷과 이벤트 조회 실패", zap.Error(err))
	}

	if latestSnapshot != nil {
		logger.Info("최신 스냅샷 조회됨",
			zap.String("document_id", gameID.Hex()),
			zap.Int64("version", latestSnapshot.Version),
			zap.Int64("server_seq", latestSnapshot.ServerSeq),
			zap.Time("created_at", latestSnapshot.CreatedAt))
	} else {
		logger.Info("스냅샷이 없습니다")
	}

	logger.Info("스냅샷 이후 이벤트 조회됨", zap.Int("event_count", len(eventsAfterSnapshot)))

	logger.Info("예제 실행 완료")
}
