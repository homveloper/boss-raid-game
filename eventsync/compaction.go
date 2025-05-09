package eventsync

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// CompactionOptions 구조체는 이벤트 압축 옵션을 정의합니다.
type CompactionOptions struct {
	// MaxAge는 압축 대상 이벤트의 최대 나이입니다.
	MaxAge time.Duration

	// MaxEvents는 압축 대상 이벤트의 최대 개수입니다.
	MaxEvents int64

	// KeepLatest는 각 문서별로 유지할 최신 이벤트 수입니다.
	KeepLatest int64

	// BatchSize는 한 번에 처리할 이벤트 수입니다.
	BatchSize int64
}

// DefaultCompactionOptions는 기본 압축 옵션을 반환합니다.
func DefaultCompactionOptions() *CompactionOptions {
	return &CompactionOptions{
		MaxAge:     24 * time.Hour * 7, // 1주일
		MaxEvents:  1000,
		KeepLatest: 100,
		BatchSize:  100,
	}
}

// EventCompactor 인터페이스는 이벤트 압축기의 기능을 정의합니다.
type EventCompactor interface {
	// CompactEvents는 이벤트를 압축합니다.
	CompactEvents(ctx context.Context, documentID primitive.ObjectID) (int64, error)

	// CompactAllEvents는 모든 문서의 이벤트를 압축합니다.
	CompactAllEvents(ctx context.Context) (int64, error)

	// ScheduleCompaction은 주기적인 압축을 예약합니다.
	ScheduleCompaction(interval time.Duration) error

	// StopCompaction은 주기적인 압축을 중지합니다.
	StopCompaction() error
}

// MongoEventCompactor는 MongoDB 기반 이벤트 압축기 구현체입니다.
type MongoEventCompactor struct {
	eventStore    EventStore
	snapshotStore SnapshotStore
	options       *CompactionOptions
	logger        *zap.Logger
	stopCh        chan struct{}
}

// NewMongoEventCompactor는 새로운 MongoDB 이벤트 압축기를 생성합니다.
func NewMongoEventCompactor(eventStore EventStore, snapshotStore SnapshotStore, options *CompactionOptions, logger *zap.Logger) *MongoEventCompactor {
	if options == nil {
		options = DefaultCompactionOptions()
	}

	return &MongoEventCompactor{
		eventStore:    eventStore,
		snapshotStore: snapshotStore,
		options:       options,
		logger:        logger,
		stopCh:        make(chan struct{}),
	}
}

// CompactEvents는 특정 문서의 이벤트를 압축합니다.
func (c *MongoEventCompactor) CompactEvents(ctx context.Context, documentID primitive.ObjectID) (int64, error) {
	// 최신 스냅샷 조회
	snapshot, err := c.snapshotStore.GetLatestSnapshot(ctx, documentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest snapshot: %w", err)
	}

	// 스냅샷이 없으면 압축할 수 없음
	if snapshot == nil {
		c.logger.Debug("No snapshot found for compaction",
			zap.String("document_id", documentID.Hex()))
		return 0, nil
	}

	// 압축 기준 시간 계산
	cutoffTime := time.Now().Add(-c.options.MaxAge)

	// 압축 대상 이벤트 조회
	// 스냅샷 이전의 이벤트 중 오래된 것들만 삭제
	filter := bson.M{
		"document_id":  documentID,
		"sequence_num": bson.M{"$lt": snapshot.SequenceNum},
		"timestamp":    bson.M{"$lt": cutoffTime},
	}

	// 최신 이벤트는 유지
	if c.options.KeepLatest > 0 {
		// 최신 이벤트의 시퀀스 번호 조회
		latestSeq, err := c.eventStore.GetLatestSequence(ctx, documentID)
		if err != nil {
			return 0, fmt.Errorf("failed to get latest sequence: %w", err)
		}

		// 유지할 이벤트의 최소 시퀀스 번호 계산
		keepSeq := latestSeq - c.options.KeepLatest
		if keepSeq > 0 {
			filter["sequence_num"] = bson.M{
				"$lt": snapshot.SequenceNum,
				"$lt": keepSeq,
			}
		}
	}

	// 이벤트 삭제
	result, err := c.eventStore.(*MongoEventStore).collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to delete events: %w", err)
	}

	c.logger.Info("Events compacted",
		zap.String("document_id", documentID.Hex()),
		zap.Int64("deleted_count", result.DeletedCount),
		zap.Int64("snapshot_sequence", snapshot.SequenceNum))

	return result.DeletedCount, nil
}

// CompactAllEvents는 모든 문서의 이벤트를 압축합니다.
func (c *MongoEventCompactor) CompactAllEvents(ctx context.Context) (int64, error) {
	// 모든 문서 ID 조회
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$document_id"}}}},
	}

	cursor, err := c.eventStore.(*MongoEventStore).collection.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, fmt.Errorf("failed to aggregate document IDs: %w", err)
	}
	defer cursor.Close(ctx)

	var totalCompacted int64
	for cursor.Next(ctx) {
		var result struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cursor.Decode(&result); err != nil {
			c.logger.Warn("Failed to decode document ID",
				zap.Error(err))
			continue
		}

		// 각 문서별로 압축 수행
		compacted, err := c.CompactEvents(ctx, result.ID)
		if err != nil {
			c.logger.Warn("Failed to compact events",
				zap.String("document_id", result.ID.Hex()),
				zap.Error(err))
			continue
		}

		totalCompacted += compacted
	}

	if err := cursor.Err(); err != nil {
		return totalCompacted, fmt.Errorf("cursor error: %w", err)
	}

	return totalCompacted, nil
}

// ScheduleCompaction은 주기적인 압축을 예약합니다.
func (c *MongoEventCompactor) ScheduleCompaction(interval time.Duration) error {
	ticker := time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), interval/2)
				compacted, err := c.CompactAllEvents(ctx)
				if err != nil {
					c.logger.Error("Scheduled compaction failed",
						zap.Error(err))
				} else {
					c.logger.Info("Scheduled compaction completed",
						zap.Int64("compacted_events", compacted))
				}
				cancel()
			case <-c.stopCh:
				ticker.Stop()
				return
			}
		}
	}()

	c.logger.Info("Scheduled compaction started",
		zap.Duration("interval", interval))

	return nil
}

// StopCompaction은 주기적인 압축을 중지합니다.
func (c *MongoEventCompactor) StopCompaction() error {
	close(c.stopCh)
	c.logger.Info("Scheduled compaction stopped")
	return nil
}
