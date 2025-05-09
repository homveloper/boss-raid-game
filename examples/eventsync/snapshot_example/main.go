package main

import (
	"context"
	"encoding/json"
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
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Gold        int                `bson:"gold" json:"gold"`
	Level       int                `bson:"level" json:"level"`
	Experience  int                `bson:"experience" json:"experience"`
	LastUpdated time.Time          `bson:"last_updated" json:"lastUpdated"`
	Version     int64              `bson:"version" json:"version"`
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

	// 컬렉션 설정
	dbName := "eventsync_example"
	eventCollection := client.Database(dbName).Collection("events")
	snapshotCollection := client.Database(dbName).Collection("snapshots")

	// 이벤트 저장소 생성
	eventStore, err := eventsync.NewMongoEventStore(ctx, client, dbName, "events", logger)
	if err != nil {
		logger.Fatal("이벤트 저장소 생성 실패", zap.Error(err))
	}

	// 스냅샷 저장소 생성
	snapshotStore, err := eventsync.NewMongoSnapshotStore(ctx, client, dbName, "snapshots", eventStore, logger)
	if err != nil {
		logger.Fatal("스냅샷 저장소 생성 실패", zap.Error(err))
	}

	// 예제 실행
	documentID := primitive.NewObjectID()
	fmt.Printf("문서 ID: %s\n", documentID.Hex())

	// 1. 초기 상태 생성 및 이벤트 저장
	initialState := &GameState{
		ID:          documentID,
		Name:        "Player1",
		Gold:        100,
		Level:       1,
		Experience:  0,
		LastUpdated: time.Now(),
		Version:     1,
	}

	// 초기 상태를 이벤트로 저장
	initialEvent := &eventsync.Event{
		ID:          primitive.NewObjectID(),
		DocumentID:  documentID,
		Timestamp:   time.Now(),
		SequenceNum: 1,
		Operation:   "create",
		Diff: &nodestorage.Diff{
			Changes: map[string]interface{}{
				"name":        initialState.Name,
				"gold":        initialState.Gold,
				"level":       initialState.Level,
				"experience":  initialState.Experience,
				"last_updated": initialState.LastUpdated,
				"version":     initialState.Version,
			},
		},
		ClientID: "server",
	}

	err = eventStore.StoreEvent(ctx, initialEvent)
	if err != nil {
		logger.Fatal("초기 이벤트 저장 실패", zap.Error(err))
	}
	fmt.Println("초기 이벤트 저장 완료")

	// 2. 여러 이벤트 생성
	for i := 0; i < 5; i++ {
		// 상태 업데이트
		initialState.Gold += 50
		initialState.Experience += 100
		if initialState.Experience >= 1000 {
			initialState.Level++
			initialState.Experience -= 1000
		}
		initialState.LastUpdated = time.Now()
		initialState.Version++

		// 이벤트 생성
		event := &eventsync.Event{
			ID:          primitive.NewObjectID(),
			DocumentID:  documentID,
			Timestamp:   time.Now(),
			SequenceNum: int64(i + 2), // 초기 이벤트가 1이므로 2부터 시작
			Operation:   "update",
			Diff: &nodestorage.Diff{
				Changes: map[string]interface{}{
					"gold":        initialState.Gold,
					"level":       initialState.Level,
					"experience":  initialState.Experience,
					"last_updated": initialState.LastUpdated,
					"version":     initialState.Version,
				},
			},
			ClientID: "server",
		}

		// 이벤트 저장
		err = eventStore.StoreEvent(ctx, event)
		if err != nil {
			logger.Fatal("이벤트 저장 실패", zap.Error(err))
		}
		fmt.Printf("이벤트 %d 저장 완료\n", i+2)

		// 잠시 대기
		time.Sleep(100 * time.Millisecond)
	}

	// 3. 스냅샷 생성
	snapshotState := map[string]interface{}{
		"name":        initialState.Name,
		"gold":        initialState.Gold,
		"level":       initialState.Level,
		"experience":  initialState.Experience,
		"last_updated": initialState.LastUpdated,
		"version":     initialState.Version,
	}

	snapshot, err := snapshotStore.CreateSnapshot(ctx, documentID, snapshotState, initialState.Version)
	if err != nil {
		logger.Fatal("스냅샷 생성 실패", zap.Error(err))
	}
	fmt.Printf("스냅샷 생성 완료 (시퀀스: %d, 버전: %d)\n", snapshot.SequenceNum, snapshot.Version)

	// 4. 추가 이벤트 생성
	for i := 0; i < 3; i++ {
		// 상태 업데이트
		initialState.Gold += 100
		initialState.Experience += 200
		if initialState.Experience >= 1000 {
			initialState.Level++
			initialState.Experience -= 1000
		}
		initialState.LastUpdated = time.Now()
		initialState.Version++

		// 이벤트 생성
		event := &eventsync.Event{
			ID:          primitive.NewObjectID(),
			DocumentID:  documentID,
			Timestamp:   time.Now(),
			SequenceNum: int64(i + 7), // 이전 이벤트가 6까지 있으므로 7부터 시작
			Operation:   "update",
			Diff: &nodestorage.Diff{
				Changes: map[string]interface{}{
					"gold":        initialState.Gold,
					"level":       initialState.Level,
					"experience":  initialState.Experience,
					"last_updated": initialState.LastUpdated,
					"version":     initialState.Version,
				},
			},
			ClientID: "server",
		}

		// 이벤트 저장
		err = eventStore.StoreEvent(ctx, event)
		if err != nil {
			logger.Fatal("이벤트 저장 실패", zap.Error(err))
		}
		fmt.Printf("이벤트 %d 저장 완료\n", i+7)

		// 잠시 대기
		time.Sleep(100 * time.Millisecond)
	}

	// 5. 스냅샷과 이벤트를 사용하여 최신 상태 조회
	latestSnapshot, events, err := eventsync.GetEventsWithSnapshot(ctx, documentID, snapshotStore, eventStore)
	if err != nil {
		logger.Fatal("상태 조회 실패", zap.Error(err))
	}

	fmt.Printf("\n=== 상태 조회 결과 ===\n")
	fmt.Printf("스냅샷: 시퀀스 %d, 버전 %d\n", latestSnapshot.SequenceNum, latestSnapshot.Version)
	fmt.Printf("추가 이벤트 수: %d\n", len(events))

	// 스냅샷 상태 출력
	snapshotJSON, _ := json.MarshalIndent(latestSnapshot.State, "", "  ")
	fmt.Printf("\n스냅샷 상태:\n%s\n", string(snapshotJSON))

	// 이벤트 적용하여 최신 상태 구성
	currentState := make(map[string]interface{})
	for k, v := range latestSnapshot.State {
		currentState[k] = v
	}

	for _, event := range events {
		fmt.Printf("\n이벤트 적용: 시퀀스 %d\n", event.SequenceNum)
		if event.Diff != nil && event.Diff.Changes != nil {
			for field, value := range event.Diff.Changes {
				if value == nil {
					delete(currentState, field)
				} else {
					currentState[field] = value
				}
			}
		}
	}

	// 최종 상태 출력
	finalJSON, _ := json.MarshalIndent(currentState, "", "  ")
	fmt.Printf("\n최종 상태:\n%s\n", string(finalJSON))
}
