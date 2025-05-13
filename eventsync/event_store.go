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

	"nodestorage/v2"
)

// Event 구조체는 문서 변경 이벤트를 나타냅니다.
type Event struct {
	ID          primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	DocumentID  primitive.ObjectID     `bson:"document_id" json:"documentId"`
	Timestamp   time.Time              `bson:"timestamp" json:"timestamp"`
	SequenceNum int64                  `bson:"sequence_num" json:"sequenceNum"`
	Operation   string                 `bson:"operation" json:"operation"`
	Diff        *nodestorage.Diff      `bson:"diff" json:"diff"`
	VectorClock map[string]int64       `bson:"vector_clock" json:"vectorClock"`
	ClientID    string                 `bson:"client_id" json:"clientId"`
	ServerSeq   int64                  `bson:"server_seq" json:"serverSeq"`
	Metadata    map[string]interface{} `bson:"metadata,omitempty" json:"metadata,omitempty"`
}

// EventStore 인터페이스는 이벤트 저장소의 기능을 정의합니다.
type EventStore interface {
	// StoreEvent는 이벤트를 저장합니다.
	StoreEvent(ctx context.Context, event *Event) error

	// GetEvents는 문서의 이벤트를 조회합니다.
	GetEvents(ctx context.Context, documentID primitive.ObjectID, afterSequence int64) ([]*Event, error)

	// GetLatestSequence는 문서의 최신 시퀀스 번호를 조회합니다.
	GetLatestSequence(ctx context.Context, documentID primitive.ObjectID) (int64, error)

	// GetEventsByVectorClock는 상태 벡터를 기준으로 누락된 이벤트를 조회합니다.
	GetEventsByVectorClock(ctx context.Context, documentID primitive.ObjectID, vectorClock map[string]int64) ([]*Event, error)

	// GetEventsAfterVersion은 지정된 버전 이후의 이벤트를 조회합니다.
	GetEventsAfterVersion(ctx context.Context, documentID primitive.ObjectID, afterVersion int64) ([]*Event, error)

	// GetLatestVersion은 문서의 최신 버전을 조회합니다.
	GetLatestVersion(ctx context.Context, documentID primitive.ObjectID) (int64, error)

	// Close는 이벤트 저장소를 닫습니다.
	Close() error
}

// MongoEventStore는 MongoDB 기반 이벤트 저장소 구현체입니다.
type MongoEventStore struct {
	collection *mongo.Collection
	seqMutex   sync.Mutex
	logger     *zap.Logger
}

// NewMongoEventStore는 새로운 MongoDB 이벤트 저장소를 생성합니다.
func NewMongoEventStore(ctx context.Context, client *mongo.Client, database, collection string, logger *zap.Logger) (*MongoEventStore, error) {
	// 컬렉션 가져오기
	coll := client.Database(database).Collection(collection)

	// 인덱스 생성
	indexModels := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "document_id", Value: 1},
				{Key: "sequence_num", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "document_id", Value: 1},
				{Key: "timestamp", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "client_id", Value: 1},
			},
		},
	}

	_, err := coll.Indexes().CreateMany(ctx, indexModels)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	return &MongoEventStore{
		collection: coll,
		logger:     logger,
	}, nil
}

// StoreEvent는 이벤트를 저장합니다.
func (s *MongoEventStore) StoreEvent(ctx context.Context, event *Event) error {
	// 시퀀스 번호 할당
	if event.SequenceNum == 0 {
		seq, err := s.getNextSequence(ctx, event.DocumentID)
		if err != nil {
			return fmt.Errorf("failed to get next sequence: %w", err)
		}
		event.SequenceNum = seq
	}

	// 타임스탬프 설정
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// ID 생성
	if event.ID.IsZero() {
		event.ID = primitive.NewObjectID()
	}

	// 이벤트 저장
	_, err := s.collection.InsertOne(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	s.logger.Debug("Event stored",
		zap.String("event_id", event.ID.Hex()),
		zap.String("document_id", event.DocumentID.Hex()),
		zap.Int64("sequence_num", event.SequenceNum),
		zap.String("operation", event.Operation))

	return nil
}

// GetEvents는 문서의 이벤트를 조회합니다.
func (s *MongoEventStore) GetEvents(ctx context.Context, documentID primitive.ObjectID, afterSequence int64) ([]*Event, error) {
	filter := bson.M{
		"document_id":  documentID,
		"sequence_num": bson.M{"$gt": afterSequence},
	}

	opts := options.Find().SetSort(bson.D{{Key: "sequence_num", Value: 1}})

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find events: %w", err)
	}
	defer cursor.Close(ctx)

	var events []*Event
	if err := cursor.All(ctx, &events); err != nil {
		return nil, fmt.Errorf("failed to decode events: %w", err)
	}

	return events, nil
}

// GetLatestSequence는 문서의 최신 시퀀스 번호를 조회합니다.
func (s *MongoEventStore) GetLatestSequence(ctx context.Context, documentID primitive.ObjectID) (int64, error) {
	filter := bson.M{"document_id": documentID}
	opts := options.FindOne().SetSort(bson.D{{Key: "sequence_num", Value: -1}})

	var event Event
	err := s.collection.FindOne(ctx, filter, opts).Decode(&event)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to find latest event: %w", err)
	}

	return event.SequenceNum, nil
}

// GetEventsByVectorClock는 상태 벡터를 기준으로 누락된 이벤트를 조회합니다.
func (s *MongoEventStore) GetEventsByVectorClock(ctx context.Context, documentID primitive.ObjectID, vectorClock map[string]int64) ([]*Event, error) {
	// 기본 필터: 문서 ID로 필터링
	filter := bson.M{"document_id": documentID}

	// 벡터 시계가 비어 있지 않은 경우에만 $or 조건 추가
	if len(vectorClock) > 0 {
		// 각 클라이언트별로 벡터 시계 이후의 이벤트 조회
		var orConditions []bson.M
		for clientID, seq := range vectorClock {
			orConditions = append(orConditions, bson.M{
				"client_id":    clientID,
				"sequence_num": bson.M{"$gt": seq},
			})
		}

		// 알려지지 않은 클라이언트의 이벤트도 포함
		var knownClients []string
		for clientID := range vectorClock {
			knownClients = append(knownClients, clientID)
		}

		orConditions = append(orConditions, bson.M{
			"client_id": bson.M{"$nin": knownClients},
		})

		// $or 조건 추가
		filter["$or"] = orConditions
	}

	opts := options.Find().SetSort(bson.D{{Key: "sequence_num", Value: 1}})

	s.logger.Debug("Finding events by vector clock",
		zap.String("document_id", documentID.Hex()),
		zap.Any("vector_clock", vectorClock),
		zap.Any("filter", filter))

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find events by vector clock: %w", err)
	}
	defer cursor.Close(ctx)

	var events []*Event
	if err := cursor.All(ctx, &events); err != nil {
		return nil, fmt.Errorf("failed to decode events: %w", err)
	}

	s.logger.Debug("Found events by vector clock",
		zap.String("document_id", documentID.Hex()),
		zap.Int("event_count", len(events)))

	return events, nil
}

// GetEventsAfterVersion은 지정된 버전 이후의 이벤트를 조회합니다.
func (s *MongoEventStore) GetEventsAfterVersion(ctx context.Context, documentID primitive.ObjectID, afterVersion int64) ([]*Event, error) {
	filter := bson.M{
		"document_id": documentID,
		"server_seq":  bson.M{"$gt": afterVersion},
	}

	opts := options.Find().SetSort(bson.D{{Key: "server_seq", Value: 1}})

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find events after version: %w", err)
	}
	defer cursor.Close(ctx)

	var events []*Event
	if err := cursor.All(ctx, &events); err != nil {
		return nil, fmt.Errorf("failed to decode events: %w", err)
	}

	return events, nil
}

// GetLatestVersion은 문서의 최신 버전을 조회합니다.
func (s *MongoEventStore) GetLatestVersion(ctx context.Context, documentID primitive.ObjectID) (int64, error) {
	filter := bson.M{"document_id": documentID}
	opts := options.FindOne().SetSort(bson.D{{Key: "server_seq", Value: -1}})

	var event Event
	err := s.collection.FindOne(ctx, filter, opts).Decode(&event)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, nil // 이벤트가 없으면 버전 0 반환
		}
		return 0, fmt.Errorf("failed to find latest event: %w", err)
	}

	return event.ServerSeq, nil
}

// Close는 이벤트 저장소를 닫습니다.
func (s *MongoEventStore) Close() error {
	// MongoDB 클라이언트는 외부에서 관리하므로 여기서는 특별한 작업이 필요 없음
	return nil
}

// getNextSequence는 문서의 다음 시퀀스 번호를 가져옵니다.
func (s *MongoEventStore) getNextSequence(ctx context.Context, documentID primitive.ObjectID) (int64, error) {
	s.seqMutex.Lock()
	defer s.seqMutex.Unlock()

	// 현재 최대 시퀀스 번호 조회
	currentSeq, err := s.GetLatestSequence(ctx, documentID)
	if err != nil {
		return 0, err
	}

	// 다음 시퀀스 번호 반환
	return currentSeq + 1, nil
}
