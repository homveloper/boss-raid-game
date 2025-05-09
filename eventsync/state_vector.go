package eventsync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// StateVector 구조체는 클라이언트의 상태 벡터를 나타냅니다.
type StateVector struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ClientID    string             `bson:"client_id" json:"clientId"`
	DocumentID  primitive.ObjectID `bson:"document_id" json:"documentId"`
	VectorClock map[string]int64   `bson:"vector_clock" json:"vectorClock"`
	LastUpdated time.Time          `bson:"last_updated" json:"lastUpdated"`
}

// StateVectorManager 인터페이스는 상태 벡터 관리자의 기능을 정의합니다.
type StateVectorManager interface {
	// GetStateVector는 클라이언트의 상태 벡터를 조회합니다.
	GetStateVector(ctx context.Context, clientID string, documentID primitive.ObjectID) (*StateVector, error)

	// UpdateStateVector는 클라이언트의 상태 벡터를 업데이트합니다.
	UpdateStateVector(ctx context.Context, stateVector *StateVector) error

	// UpdateVectorClock는 클라이언트의 벡터 시계를 업데이트합니다.
	UpdateVectorClock(ctx context.Context, clientID string, documentID primitive.ObjectID, vectorClock map[string]int64) error

	// GetMissingEvents는 클라이언트가 놓친 이벤트를 조회합니다.
	GetMissingEvents(ctx context.Context, clientID string, documentID primitive.ObjectID, vectorClock map[string]int64) ([]*Event, error)

	// Close는 상태 벡터 관리자를 닫습니다.
	Close() error
}

// MongoStateVectorManager는 MongoDB 기반 상태 벡터 관리자 구현체입니다.
type MongoStateVectorManager struct {
	collection *mongo.Collection
	eventStore EventStore
	mutex      sync.RWMutex
	logger     *zap.Logger
}

// NewMongoStateVectorManager는 새로운 MongoDB 상태 벡터 관리자를 생성합니다.
func NewMongoStateVectorManager(ctx context.Context, client *mongo.Client, database, collection string, eventStore EventStore, logger *zap.Logger) (*MongoStateVectorManager, error) {
	// 컬렉션 가져오기
	coll := client.Database(database).Collection(collection)

	// 인덱스 생성
	indexModels := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "client_id", Value: 1},
				{Key: "document_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "last_updated", Value: 1},
			},
		},
	}

	_, err := coll.Indexes().CreateMany(ctx, indexModels)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	return &MongoStateVectorManager{
		collection: coll,
		eventStore: eventStore,
		logger:     logger,
	}, nil
}

// GetStateVector는 클라이언트의 상태 벡터를 조회합니다.
func (m *MongoStateVectorManager) GetStateVector(ctx context.Context, clientID string, documentID primitive.ObjectID) (*StateVector, error) {
	filter := bson.M{
		"client_id":   clientID,
		"document_id": documentID,
	}

	var stateVector StateVector
	err := m.collection.FindOne(ctx, filter).Decode(&stateVector)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// 상태 벡터가 없으면 새로 생성
			stateVector = StateVector{
				ClientID:    clientID,
				DocumentID:  documentID,
				VectorClock: make(map[string]int64),
				LastUpdated: time.Now(),
			}
			return &stateVector, nil
		}
		return nil, fmt.Errorf("failed to find state vector: %w", err)
	}

	return &stateVector, nil
}

// UpdateStateVector는 클라이언트의 상태 벡터를 업데이트합니다.
func (m *MongoStateVectorManager) UpdateStateVector(ctx context.Context, stateVector *StateVector) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 타임스탬프 업데이트
	stateVector.LastUpdated = time.Now()

	// ID가 없으면 새로 생성
	if stateVector.ID.IsZero() {
		stateVector.ID = primitive.NewObjectID()
	}

	// 업서트 옵션
	opts := options.Update().SetUpsert(true)

	filter := bson.M{
		"client_id":   stateVector.ClientID,
		"document_id": stateVector.DocumentID,
	}

	update := bson.M{
		"$set": stateVector,
	}

	_, err := m.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to update state vector: %w", err)
	}

	m.logger.Debug("State vector updated",
		zap.String("client_id", stateVector.ClientID),
		zap.String("document_id", stateVector.DocumentID.Hex()))

	return nil
}

// UpdateVectorClock는 클라이언트의 벡터 시계를 업데이트합니다.
func (m *MongoStateVectorManager) UpdateVectorClock(ctx context.Context, clientID string, documentID primitive.ObjectID, vectorClock map[string]int64) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// 현재 상태 벡터 조회
	stateVector, err := m.GetStateVector(ctx, clientID, documentID)
	if err != nil {
		return err
	}

	// 벡터 시계 병합
	if stateVector.VectorClock == nil {
		stateVector.VectorClock = make(map[string]int64)
	}

	for id, seq := range vectorClock {
		if currentSeq, ok := stateVector.VectorClock[id]; !ok || seq > currentSeq {
			stateVector.VectorClock[id] = seq
		}
	}

	// 상태 벡터 업데이트
	return m.UpdateStateVector(ctx, stateVector)
}

// GetMissingEvents는 클라이언트가 놓친 이벤트를 조회합니다.
func (m *MongoStateVectorManager) GetMissingEvents(ctx context.Context, clientID string, documentID primitive.ObjectID, vectorClock map[string]int64) ([]*Event, error) {
	// 상태 벡터 조회
	stateVector, err := m.GetStateVector(ctx, clientID, documentID)
	if err != nil {
		return nil, err
	}

	// 클라이언트가 제공한 벡터 시계가 없으면 현재 상태 벡터 사용
	if vectorClock == nil || len(vectorClock) == 0 {
		vectorClock = stateVector.VectorClock
	}

	// 누락된 이벤트 조회
	events, err := m.eventStore.GetEventsByVectorClock(ctx, documentID, vectorClock)
	if err != nil {
		return nil, fmt.Errorf("failed to get missing events: %w", err)
	}

	// 이벤트 로깅
	m.logger.Debug("Missing events retrieved",
		zap.String("client_id", clientID),
		zap.String("document_id", documentID.Hex()),
		zap.Int("event_count", len(events)))

	return events, nil
}

// Close는 상태 벡터 관리자를 닫습니다.
func (m *MongoStateVectorManager) Close() error {
	// MongoDB 클라이언트는 외부에서 관리하므로 여기서는 특별한 작업이 필요 없음
	return nil
}
