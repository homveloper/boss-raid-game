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

	// RegisterClient는 새 클라이언트를 등록합니다.
	RegisterClient(ctx context.Context, clientID string) error

	// UnregisterClient는 클라이언트를 등록 해제합니다.
	UnregisterClient(ctx context.Context, clientID string) error

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
// 상태 벡터가 없으면 새로 생성합니다. FindOneAndUpdate를 사용하여 원자적으로 처리합니다.
func (m *MongoStateVectorManager) GetStateVector(ctx context.Context, clientID string, documentID primitive.ObjectID) (*StateVector, error) {
	filter := bson.M{
		"client_id":   clientID,
		"document_id": documentID,
	}

	// 새 상태 벡터 생성 (없는 경우 사용)
	now := time.Now()
	newStateVector := StateVector{
		ID:          primitive.NewObjectID(),
		ClientID:    clientID,
		DocumentID:  documentID,
		VectorClock: make(map[string]int64),
		LastUpdated: now,
	}

	// 업데이트 옵션 설정
	opts := options.FindOneAndUpdate().
		SetUpsert(true).                 // 문서가 없으면 새로 생성
		SetReturnDocument(options.After) // 업데이트 후 문서 반환

	// 업데이트 내용 정의
	update := bson.M{
		"$setOnInsert": bson.M{
			"_id":          newStateVector.ID,
			"client_id":    clientID,
			"document_id":  documentID,
			"vector_clock": newStateVector.VectorClock,
			"last_updated": now,
		},
	}

	// FindOneAndUpdate 실행
	var stateVector StateVector
	err := m.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&stateVector)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create state vector: %w", err)
	}

	// 새로 생성된 경우 로그 출력
	if stateVector.LastUpdated.Equal(now) {
		m.logger.Debug("New state vector created",
			zap.String("client_id", clientID),
			zap.String("document_id", documentID.Hex()))
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

	// _id 필드를 제외한 필드만 업데이트
	update := bson.M{
		"$set": bson.M{
			"vector_clock": stateVector.VectorClock,
			"last_updated": stateVector.LastUpdated,
		},
	}

	// 새로운 문서 삽입 시 사용할 데이터
	if opts.Upsert != nil && *opts.Upsert {
		update["$setOnInsert"] = bson.M{
			"_id": stateVector.ID,
		}
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
	// 현재 상태 벡터 조회 (뮤텍스 없이)
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

	// 상태 벡터 업데이트 (UpdateStateVector 내부에서 뮤텍스 잠금)
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

// RegisterClient는 새 클라이언트를 등록합니다.
func (m *MongoStateVectorManager) RegisterClient(ctx context.Context, clientID string) error {
	// 클라이언트 등록 시 특별한 작업은 필요 없음
	// 클라이언트가 문서에 접근할 때 자동으로 상태 벡터가 생성됨
	m.logger.Info("Client registered", zap.String("client_id", clientID))
	return nil
}

// UnregisterClient는 클라이언트를 등록 해제합니다.
func (m *MongoStateVectorManager) UnregisterClient(ctx context.Context, clientID string) error {
	// 클라이언트의 모든 상태 벡터 삭제
	filter := bson.M{"client_id": clientID}

	result, err := m.collection.DeleteMany(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to remove client state vectors: %w", err)
	}

	m.logger.Info("Client unregistered",
		zap.String("client_id", clientID),
		zap.Int64("removed_state_vectors", result.DeletedCount))

	return nil
}

// Close는 상태 벡터 관리자를 닫습니다.
func (m *MongoStateVectorManager) Close() error {
	// MongoDB 클라이언트는 외부에서 관리하므로 여기서는 특별한 작업이 필요 없음
	return nil
}
