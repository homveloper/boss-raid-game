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

// VectorClockDocument는 MongoDB에 저장되는 벡터 시계 문서 구조입니다.
type VectorClockDocument struct {
	ID          primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	DocumentID  primitive.ObjectID     `bson:"document_id" json:"documentId"`
	VectorClock map[string]int64       `bson:"vector_clock" json:"vectorClock"`
	UpdatedAt   time.Time              `bson:"updated_at" json:"updatedAt"`
}

// MongoVectorClockManager는 MongoDB 기반 벡터 시계 관리자 구현체입니다.
type MongoVectorClockManager struct {
	collection *mongo.Collection
	logger     *zap.Logger
}

// NewMongoVectorClockManager는 새로운 MongoVectorClockManager를 생성합니다.
func NewMongoVectorClockManager(ctx context.Context, client *mongo.Client, database, collection string, logger *zap.Logger) (*MongoVectorClockManager, error) {
	// 컬렉션 가져오기
	coll := client.Database(database).Collection(collection)

	// 인덱스 생성
	indexModels := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "document_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "updated_at", Value: 1},
			},
		},
	}

	_, err := coll.Indexes().CreateMany(ctx, indexModels)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	return &MongoVectorClockManager{
		collection: coll,
		logger:     logger,
	}, nil
}

// GetVectorClock은 지정된 문서 ID에 대한 현재 벡터 시계를 반환합니다.
func (m *MongoVectorClockManager) GetVectorClock(ctx context.Context, documentID primitive.ObjectID) (map[string]int64, error) {
	filter := bson.M{"document_id": documentID}

	var doc VectorClockDocument
	err := m.collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// 문서가 없으면 빈 벡터 시계 반환
			return make(map[string]int64), nil
		}
		return nil, fmt.Errorf("failed to get vector clock: %w", err)
	}

	// 맵 복사본 반환
	result := make(map[string]int64)
	for k, v := range doc.VectorClock {
		result[k] = v
	}

	return result, nil
}

// UpdateVectorClock은 지정된 문서 ID에 대한 벡터 시계를 업데이트합니다.
func (m *MongoVectorClockManager) UpdateVectorClock(ctx context.Context, documentID primitive.ObjectID, clientID string, sequenceNum int64) error {
	filter := bson.M{"document_id": documentID}

	// 현재 벡터 시계 조회
	var doc VectorClockDocument
	err := m.collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil && err != mongo.ErrNoDocuments {
		return fmt.Errorf("failed to find vector clock document: %w", err)
	}

	// 문서가 없으면 새로 생성
	if err == mongo.ErrNoDocuments {
		doc = VectorClockDocument{
			ID:          primitive.NewObjectID(),
			DocumentID:  documentID,
			VectorClock: make(map[string]int64),
			UpdatedAt:   time.Now(),
		}
	}

	// 현재 값보다 큰 경우에만 업데이트
	currentSeq := doc.VectorClock[clientID]
	if sequenceNum <= currentSeq {
		return nil
	}

	// 벡터 시계 업데이트
	if doc.VectorClock == nil {
		doc.VectorClock = make(map[string]int64)
	}
	doc.VectorClock[clientID] = sequenceNum
	doc.UpdatedAt = time.Now()

	// 업데이트 또는 삽입
	opts := options.Update().SetUpsert(true)
	update := bson.M{
		"$set": bson.M{
			"vector_clock": doc.VectorClock,
			"updated_at":   doc.UpdatedAt,
		},
	}

	if err == mongo.ErrNoDocuments {
		// 새 문서 삽입
		_, err = m.collection.UpdateOne(ctx, filter, update, opts)
	} else {
		// 기존 문서 업데이트
		_, err = m.collection.UpdateOne(ctx, filter, update)
	}

	if err != nil {
		return fmt.Errorf("failed to update vector clock: %w", err)
	}

	return nil
}

// CleanupOldVectorClocks는 오래된 벡터 시계 문서를 정리합니다.
func (m *MongoVectorClockManager) CleanupOldVectorClocks(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-olderThan)
	filter := bson.M{"updated_at": bson.M{"$lt": cutoffTime}}

	result, err := m.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old vector clocks: %w", err)
	}

	return result.DeletedCount, nil
}
