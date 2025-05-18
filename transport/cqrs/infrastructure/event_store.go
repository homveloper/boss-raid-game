package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"tictactoe/transport/cqrs/domain"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EventStore는 이벤트를 저장하고 로드하는 인터페이스입니다.
type EventStore interface {
	// SaveEvent는 이벤트를 저장합니다.
	SaveEvent(ctx context.Context, event domain.Event) error

	// GetEvents는 애그리게이트의 모든 이벤트를 가져옵니다.
	GetEvents(ctx context.Context, aggregateID string, aggregateType string) ([]domain.Event, error)

	// GetEventsUpToVersion은 특정 버전까지의 이벤트를 가져옵니다.
	GetEventsUpToVersion(ctx context.Context, aggregateID string, aggregateType string, version int) ([]domain.Event, error)

	// GetEventsByType은 특정 타입의 이벤트를 가져옵니다.
	GetEventsByType(ctx context.Context, eventType string) ([]domain.Event, error)
}

// MongoEventStore는 MongoDB를 사용하는 EventStore 구현체입니다.
type MongoEventStore struct {
	client     *mongo.Client
	database   string
	collection string
	eventTypes map[string]reflect.Type
}

// EventDocument는 MongoDB에 저장되는 이벤트 문서입니다.
type EventDocument struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	EventID       string             `bson:"event_id"`
	EventType     string             `bson:"event_type"`
	AggregateID   string             `bson:"aggregate_id"`
	AggregateType string             `bson:"aggregate_type"`
	Version       int                `bson:"version"`
	Timestamp     time.Time          `bson:"timestamp"`
	Payload       []byte             `bson:"payload"`
}

// NewMongoEventStore는 새로운 MongoEventStore를 생성합니다.
func NewMongoEventStore(
	client *mongo.Client,
	database string,
	collection string,
) *MongoEventStore {
	store := &MongoEventStore{
		client:     client,
		database:   database,
		collection: collection,
		eventTypes: make(map[string]reflect.Type),
	}

	// 이벤트 타입 등록
	store.RegisterEventType("TransportCreated", reflect.TypeOf(domain.TransportCreatedEvent{}))
	store.RegisterEventType("TransportStarted", reflect.TypeOf(domain.TransportStartedEvent{}))
	store.RegisterEventType("TransportCompleted", reflect.TypeOf(domain.TransportCompletedEvent{}))
	store.RegisterEventType("TransportRaided", reflect.TypeOf(domain.TransportRaidedEvent{}))
	store.RegisterEventType("TransportDefended", reflect.TypeOf(domain.TransportDefendedEvent{}))
	store.RegisterEventType("TransportParticipantAdded", reflect.TypeOf(domain.TransportParticipantAddedEvent{}))
	store.RegisterEventType("TransportRaidCompleted", reflect.TypeOf(domain.TransportRaidCompletedEvent{}))

	store.RegisterEventType("RaidCreated", reflect.TypeOf(domain.RaidCreatedEvent{}))
	store.RegisterEventType("RaidStarted", reflect.TypeOf(domain.RaidStartedEvent{}))
	store.RegisterEventType("RaidSucceeded", reflect.TypeOf(domain.RaidSucceededEvent{}))
	store.RegisterEventType("RaidFailed", reflect.TypeOf(domain.RaidFailedEvent{}))
	store.RegisterEventType("RaidCanceled", reflect.TypeOf(domain.RaidCanceledEvent{}))

	return store
}

// RegisterEventType은 이벤트 타입을 등록합니다.
func (s *MongoEventStore) RegisterEventType(eventType string, eventTypeObj reflect.Type) {
	s.eventTypes[eventType] = eventTypeObj
}

// SaveEvent는 이벤트를 저장합니다.
func (s *MongoEventStore) SaveEvent(ctx context.Context, event domain.Event) error {
	// 이벤트 직렬화
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// 이벤트 문서 생성
	doc := EventDocument{
		EventID:       event.AggregateID() + "-" + fmt.Sprintf("%d", event.Version()),
		EventType:     event.EventType(),
		AggregateID:   event.AggregateID(),
		AggregateType: event.AggregateType(),
		Version:       event.Version(),
		Timestamp:     time.Now(),
		Payload:       payload,
	}

	// 이벤트 저장
	collection := s.client.Database(s.database).Collection(s.collection)
	_, err = collection.InsertOne(ctx, doc)
	if err != nil {
		return fmt.Errorf("failed to save event: %w", err)
	}

	return nil
}

// GetEvents는 애그리게이트의 모든 이벤트를 가져옵니다.
func (s *MongoEventStore) GetEvents(
	ctx context.Context,
	aggregateID string,
	aggregateType string,
) ([]domain.Event, error) {
	// 이벤트 조회
	collection := s.client.Database(s.database).Collection(s.collection)
	filter := bson.M{
		"aggregate_id":   aggregateID,
		"aggregate_type": aggregateType,
	}
	opts := options.Find().SetSort(bson.M{"version": 1})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find events: %w", err)
	}
	defer cursor.Close(ctx)

	// 이벤트 역직렬화
	var events []domain.Event
	for cursor.Next(ctx) {
		var doc EventDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("failed to decode event document: %w", err)
		}

		// 이벤트 타입 가져오기
		eventType, ok := s.eventTypes[doc.EventType]
		if !ok {
			return nil, fmt.Errorf("unknown event type: %s", doc.EventType)
		}

		// 이벤트 생성
		event := reflect.New(eventType).Interface().(domain.Event)

		// 이벤트 역직렬화
		if err := json.Unmarshal(doc.Payload, &event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event: %w", err)
		}

		events = append(events, event)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return events, nil
}

// GetEventsUpToVersion은 특정 버전까지의 이벤트를 가져옵니다.
func (s *MongoEventStore) GetEventsUpToVersion(
	ctx context.Context,
	aggregateID string,
	aggregateType string,
	version int,
) ([]domain.Event, error) {
	// 이벤트 조회
	collection := s.client.Database(s.database).Collection(s.collection)
	filter := bson.M{
		"aggregate_id":   aggregateID,
		"aggregate_type": aggregateType,
		"version":        bson.M{"$lte": version},
	}
	opts := options.Find().SetSort(bson.M{"version": 1})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find events: %w", err)
	}
	defer cursor.Close(ctx)

	// 이벤트 역직렬화
	var events []domain.Event
	for cursor.Next(ctx) {
		var doc EventDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("failed to decode event document: %w", err)
		}

		// 이벤트 타입 가져오기
		eventType, ok := s.eventTypes[doc.EventType]
		if !ok {
			return nil, fmt.Errorf("unknown event type: %s", doc.EventType)
		}

		// 이벤트 생성
		event := reflect.New(eventType).Interface().(domain.Event)

		// 이벤트 역직렬화
		if err := json.Unmarshal(doc.Payload, &event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event: %w", err)
		}

		events = append(events, event)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return events, nil
}

// GetEventsByType은 특정 타입의 이벤트를 가져옵니다.
func (s *MongoEventStore) GetEventsByType(
	ctx context.Context,
	eventType string,
) ([]domain.Event, error) {
	// 이벤트 조회
	collection := s.client.Database(s.database).Collection(s.collection)
	filter := bson.M{"event_type": eventType}
	opts := options.Find().SetSort(bson.M{"timestamp": 1})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to find events: %w", err)
	}
	defer cursor.Close(ctx)

	// 이벤트 역직렬화
	var events []domain.Event
	for cursor.Next(ctx) {
		var doc EventDocument
		if err := cursor.Decode(&doc); err != nil {
			return nil, fmt.Errorf("failed to decode event document: %w", err)
		}

		// 이벤트 타입 가져오기
		eventTypeObj, ok := s.eventTypes[doc.EventType]
		if !ok {
			return nil, fmt.Errorf("unknown event type: %s", doc.EventType)
		}

		// 이벤트 생성
		event := reflect.New(eventTypeObj).Interface().(domain.Event)

		// 이벤트 역직렬화
		if err := json.Unmarshal(doc.Payload, &event); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event: %w", err)
		}

		events = append(events, event)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return events, nil
}
