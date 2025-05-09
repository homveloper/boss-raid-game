package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"eventsync"
)

func main() {
	// 명령행 인자 파싱
	mongoURI := flag.String("mongo", "mongodb://localhost:27017", "MongoDB 연결 URI")
	dbName := flag.String("db", "eventsync_example", "데이터베이스 이름")
	compactionInterval := flag.Duration("compaction-interval", 1*time.Hour, "이벤트 압축 주기")
	snapshotInterval := flag.Duration("snapshot-interval", 6*time.Hour, "스냅샷 생성 주기")
	flag.Parse()

	// 로거 설정
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("로거 생성 실패: %v", err)
	}
	defer logger.Sync()

	// MongoDB 연결
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(*mongoURI))
	if err != nil {
		logger.Fatal("MongoDB 연결 실패", zap.Error(err))
	}
	defer client.Disconnect(context.Background())

	// 컬렉션 설정
	eventCollection := client.Database(*dbName).Collection("events")
	snapshotCollection := client.Database(*dbName).Collection("snapshots")

	// 이벤트 저장소 생성
	eventStore, err := eventsync.NewMongoEventStore(ctx, client, *dbName, "events", logger)
	if err != nil {
		logger.Fatal("이벤트 저장소 생성 실패", zap.Error(err))
	}

	// 스냅샷 저장소 생성
	snapshotStore, err := eventsync.NewMongoSnapshotStore(ctx, client, *dbName, "snapshots", eventStore, logger)
	if err != nil {
		logger.Fatal("스냅샷 저장소 생성 실패", zap.Error(err))
	}

	// 이벤트 압축기 생성
	compactionOptions := eventsync.DefaultCompactionOptions()
	compactionOptions.MaxAge = 24 * time.Hour * 7    // 1주일
	compactionOptions.KeepLatest = 100               // 최신 100개 이벤트 유지
	compactionOptions.BatchSize = 500                // 한 번에 500개씩 처리
	compactor := eventsync.NewMongoEventCompactor(eventStore, snapshotStore, compactionOptions, logger)

	// 주기적인 압축 시작
	if err := compactor.ScheduleCompaction(*compactionInterval); err != nil {
		logger.Fatal("압축 스케줄링 실패", zap.Error(err))
	}

	// 주기적인 스냅샷 생성 시작
	stopSnapshotCh := make(chan struct{})
	go scheduleSnapshots(ctx, eventStore, snapshotStore, *snapshotInterval, stopSnapshotCh, logger)

	// 종료 신호 처리
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("서비스 종료 중...")
	compactor.StopCompaction()
	close(stopSnapshotCh)
	logger.Info("서비스 종료 완료")
}

// scheduleSnapshots는 주기적으로 스냅샷을 생성합니다.
func scheduleSnapshots(ctx context.Context, eventStore eventsync.EventStore, snapshotStore eventsync.SnapshotStore, interval time.Duration, stopCh <-chan struct{}, logger *zap.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			createSnapshots(ctx, eventStore, snapshotStore, logger)
		case <-stopCh:
			logger.Info("스냅샷 스케줄링 중지")
			return
		}
	}
}

// createSnapshots는 모든 문서에 대해 스냅샷을 생성합니다.
func createSnapshots(ctx context.Context, eventStore eventsync.EventStore, snapshotStore eventsync.SnapshotStore, logger *zap.Logger) {
	// 모든 문서 ID 조회
	mongoEventStore, ok := eventStore.(*eventsync.MongoEventStore)
	if !ok {
		logger.Error("이벤트 저장소 타입 변환 실패")
		return
	}

	// 모든 문서 ID 조회
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$document_id"}}}},
	}

	cursor, err := mongoEventStore.Collection().Aggregate(ctx, pipeline)
	if err != nil {
		logger.Error("문서 ID 조회 실패", zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	var snapshotCount int
	for cursor.Next(ctx) {
		var result struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cursor.Decode(&result); err != nil {
			logger.Warn("문서 ID 디코딩 실패", zap.Error(err))
			continue
		}

		// 문서의 최신 상태 조회
		snapshot, events, err := eventsync.GetEventsWithSnapshot(ctx, result.ID, snapshotStore, eventStore)
		if err != nil {
			logger.Warn("이벤트 조회 실패",
				zap.String("document_id", result.ID.Hex()),
				zap.Error(err))
			continue
		}

		// 이벤트가 없으면 스냅샷 생성 불필요
		if len(events) == 0 && snapshot != nil {
			continue
		}

		// 문서 상태 재구성
		var state map[string]interface{}
		var version int64

		if snapshot != nil {
			state = snapshot.State
			version = snapshot.Version
		} else {
			state = make(map[string]interface{})
			version = 0
		}

		// 이벤트 적용
		for _, event := range events {
			if event.Diff != nil && event.Diff.Changes != nil {
				// 이벤트의 변경 사항 적용
				for field, value := range event.Diff.Changes {
					if value == nil {
						delete(state, field)
					} else {
						state[field] = value
					}
				}
			}
			version++
		}

		// 스냅샷 생성
		_, err = snapshotStore.CreateSnapshot(ctx, result.ID, state, version)
		if err != nil {
			logger.Warn("스냅샷 생성 실패",
				zap.String("document_id", result.ID.Hex()),
				zap.Error(err))
			continue
		}

		snapshotCount++
	}

	if err := cursor.Err(); err != nil {
		logger.Error("커서 오류", zap.Error(err))
		return
	}

	logger.Info("스냅샷 생성 완료", zap.Int("snapshot_count", snapshotCount))
}
