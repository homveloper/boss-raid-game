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

// Snapshot 구조체는 특정 시점의 문서 상태를 나타냅니다.
type Snapshot struct {
	ID          primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	DocumentID  primitive.ObjectID     `bson:"document_id" json:"documentId"`
	State       map[string]interface{} `bson:"state" json:"state"`
	Version     int64                  `bson:"version" json:"version"`
	SequenceNum int64                  `bson:"sequence_num" json:"sequenceNum"`
	CreatedAt   time.Time              `bson:"created_at" json:"createdAt"`
}

// SnapshotStore 인터페이스는 스냅샷 저장소의 기능을 정의합니다.
type SnapshotStore interface {
	// CreateSnapshot은 새로운 스냅샷을 생성합니다.
	CreateSnapshot(ctx context.Context, documentID primitive.ObjectID, state map[string]interface{}, version int64) (*Snapshot, error)

	// GetLatestSnapshot은 문서의 최신 스냅샷을 조회합니다.
	GetLatestSnapshot(ctx context.Context, documentID primitive.ObjectID) (*Snapshot, error)

	// GetSnapshotBySequence는 특정 시퀀스 번호 이전의 가장 최근 스냅샷을 조회합니다.
	GetSnapshotBySequence(ctx context.Context, documentID primitive.ObjectID, maxSequence int64) (*Snapshot, error)

	// DeleteSnapshots는 특정 시퀀스 번호 이전의 모든 스냅샷을 삭제합니다.
	DeleteSnapshots(ctx context.Context, documentID primitive.ObjectID, maxSequence int64) (int64, error)

	// Close는 스냅샷 저장소를 닫습니다.
	Close() error
}

// MongoSnapshotStore는 MongoDB 기반 스냅샷 저장소 구현체입니다.
type MongoSnapshotStore struct {
	collection *mongo.Collection
	eventStore EventStore
	logger     *zap.Logger
}

// NewMongoSnapshotStore는 새로운 MongoDB 스냅샷 저장소를 생성합니다.
func NewMongoSnapshotStore(ctx context.Context, client *mongo.Client, database, collection string, eventStore EventStore, logger *zap.Logger) (*MongoSnapshotStore, error) {
	// 컬렉션 가져오기
	coll := client.Database(database).Collection(collection)

	// 인덱스 생성
	indexModels := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "document_id", Value: 1},
				{Key: "sequence_num", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "created_at", Value: 1},
			},
		},
	}

	_, err := coll.Indexes().CreateMany(ctx, indexModels)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	return &MongoSnapshotStore{
		collection: coll,
		eventStore: eventStore,
		logger:     logger,
	}, nil
}

// CreateSnapshot은 새로운 스냅샷을 생성합니다.
func (s *MongoSnapshotStore) CreateSnapshot(ctx context.Context, documentID primitive.ObjectID, state map[string]interface{}, version int64) (*Snapshot, error) {
	// 현재 시퀀스 번호 조회
	seqNum, err := s.eventStore.GetLatestSequence(ctx, documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest sequence: %w", err)
	}

	// 스냅샷 생성
	snapshot := &Snapshot{
		ID:          primitive.NewObjectID(),
		DocumentID:  documentID,
		State:       state,
		Version:     version,
		SequenceNum: seqNum,
		CreatedAt:   time.Now(),
	}

	// 스냅샷 저장
	_, err = s.collection.InsertOne(ctx, snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to insert snapshot: %w", err)
	}

	s.logger.Info("Snapshot created",
		zap.String("document_id", documentID.Hex()),
		zap.Int64("sequence_num", seqNum),
		zap.Int64("version", version))

	return snapshot, nil
}

// GetLatestSnapshot은 문서의 최신 스냅샷을 조회합니다.
// 버전이 가장 높은 스냅샷을 반환합니다.
func (s *MongoSnapshotStore) GetLatestSnapshot(ctx context.Context, documentID primitive.ObjectID) (*Snapshot, error) {
	// 버전 기준으로 내림차순 정렬하여 최신 스냅샷 조회
	opts := options.FindOne().SetSort(bson.D{
		{Key: "version", Value: -1},
		{Key: "created_at", Value: -1}, // 동일 버전이 있을 경우 최근에 생성된 것
	})

	var snapshot Snapshot
	err := s.collection.FindOne(ctx, bson.M{"document_id": documentID}, opts).Decode(&snapshot)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // 스냅샷이 없는 경우
		}
		return nil, fmt.Errorf("failed to find snapshot: %w", err)
	}

	s.logger.Debug("Found latest snapshot",
		zap.String("document_id", documentID.Hex()),
		zap.String("snapshot_id", snapshot.ID.Hex()),
		zap.Int64("version", snapshot.Version),
		zap.Int64("sequence_num", snapshot.SequenceNum))

	return &snapshot, nil
}

// GetSnapshotBySequence는 특정 시퀀스 번호 이전의 가장 최근 스냅샷을 조회합니다.
func (s *MongoSnapshotStore) GetSnapshotBySequence(ctx context.Context, documentID primitive.ObjectID, maxSequence int64) (*Snapshot, error) {
	filter := bson.M{
		"document_id":  documentID,
		"sequence_num": bson.M{"$lte": maxSequence},
	}
	opts := options.FindOne().SetSort(bson.D{{Key: "sequence_num", Value: -1}})

	var snapshot Snapshot
	err := s.collection.FindOne(ctx, filter, opts).Decode(&snapshot)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // 스냅샷이 없는 경우
		}
		return nil, fmt.Errorf("failed to find snapshot: %w", err)
	}

	return &snapshot, nil
}

// DeleteSnapshots는 특정 시퀀스 번호 이전의 모든 스냅샷을 삭제합니다.
func (s *MongoSnapshotStore) DeleteSnapshots(ctx context.Context, documentID primitive.ObjectID, maxSequence int64) (int64, error) {
	// 최신 스냅샷은 유지하기 위해 해당 시퀀스 번호 이전의 스냅샷 중 가장 최근 것을 찾음
	latestSnapshot, err := s.GetSnapshotBySequence(ctx, documentID, maxSequence)
	if err != nil {
		return 0, err
	}

	// 스냅샷이 없으면 삭제할 것도 없음
	if latestSnapshot == nil {
		return 0, nil
	}

	// 최신 스냅샷보다 오래된 스냅샷만 삭제
	filter := bson.M{
		"document_id":  documentID,
		"sequence_num": bson.M{"$lt": latestSnapshot.SequenceNum},
	}

	result, err := s.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to delete snapshots: %w", err)
	}

	s.logger.Info("Snapshots deleted",
		zap.String("document_id", documentID.Hex()),
		zap.Int64("deleted_count", result.DeletedCount),
		zap.Int64("kept_sequence", latestSnapshot.SequenceNum))

	return result.DeletedCount, nil
}

// Close는 스냅샷 저장소를 닫습니다.
func (s *MongoSnapshotStore) Close() error {
	// MongoDB 클라이언트는 외부에서 관리하므로 여기서는 특별한 작업이 필요 없음
	return nil
}

// GetEventsWithSnapshot은 스냅샷과 그 이후의 이벤트를 함께 조회합니다.
func GetEventsWithSnapshot(ctx context.Context, documentID primitive.ObjectID, snapshotStore SnapshotStore, eventStore EventStore) (*Snapshot, []*Event, error) {
	// 최신 스냅샷 조회
	snapshot, err := snapshotStore.GetLatestSnapshot(ctx, documentID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get latest snapshot: %w", err)
	}

	var events []*Event
	if snapshot == nil {
		// 스냅샷이 없으면 모든 이벤트 조회
		events, err = eventStore.GetEvents(ctx, documentID, 0)
	} else {
		// 스냅샷 이후의 이벤트만 조회
		events, err = eventStore.GetEvents(ctx, documentID, snapshot.SequenceNum)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to get events: %w", err)
	}

	return snapshot, events, nil
}
