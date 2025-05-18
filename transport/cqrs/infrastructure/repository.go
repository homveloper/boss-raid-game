package infrastructure

import (
	"context"
	"fmt"
	"tictactoe/transport/cqrs/domain"

	"go.mongodb.org/mongo-driver/mongo"
)

// Repository는 애그리게이트를 저장하고 로드하는 인터페이스입니다.
type Repository interface {
	// Save는 애그리게이트를 저장합니다.
	Save(ctx context.Context, aggregate domain.AggregateRoot) error

	// Load는 애그리게이트를 로드합니다.
	Load(ctx context.Context, aggregateID string, aggregateType string) (domain.AggregateRoot, error)
}

// MongoRepository는 MongoDB를 사용하는 Repository 구현체입니다.
type MongoRepository struct {
	client         *mongo.Client
	database       string
	eventStore     EventStore
	eventBus       EventBus
	aggregateStore map[string]func(string) domain.AggregateRoot
}

// NewMongoRepository는 새로운 MongoRepository를 생성합니다.
func NewMongoRepository(
	client *mongo.Client,
	database string,
	eventStore EventStore,
	eventBus EventBus,
) *MongoRepository {
	repo := &MongoRepository{
		client:         client,
		database:       database,
		eventStore:     eventStore,
		eventBus:       eventBus,
		aggregateStore: make(map[string]func(string) domain.AggregateRoot),
	}

	// 애그리게이트 팩토리 등록
	repo.RegisterAggregate("Transport", func(id string) domain.AggregateRoot {
		return domain.NewTransportAggregate(id)
	})

	repo.RegisterAggregate("Raid", func(id string) domain.AggregateRoot {
		return domain.NewRaidAggregate(id)
	})

	return repo
}

// RegisterAggregate는 애그리게이트 팩토리를 등록합니다.
func (r *MongoRepository) RegisterAggregate(
	aggregateType string,
	factory func(string) domain.AggregateRoot,
) {
	r.aggregateStore[aggregateType] = factory
}

// Save는 애그리게이트를 저장합니다.
func (r *MongoRepository) Save(ctx context.Context, aggregate domain.AggregateRoot) error {
	// 커밋되지 않은 이벤트 가져오기
	events := aggregate.GetUncommittedChanges()
	if len(events) == 0 {
		return nil
	}

	// 이벤트 저장
	for _, event := range events {
		// 이벤트 버전 설정
		event.SetVersion(aggregate.Version())

		// 이벤트 저장
		if err := r.eventStore.SaveEvent(ctx, event); err != nil {
			return fmt.Errorf("failed to save event: %w", err)
		}

		// 이벤트 발행
		if err := r.eventBus.PublishEvent(ctx, event); err != nil {
			return fmt.Errorf("failed to publish event: %w", err)
		}
	}

	// 커밋되지 않은 이벤트 지우기
	aggregate.ClearUncommittedChanges()

	return nil
}

// Load는 애그리게이트를 로드합니다.
func (r *MongoRepository) Load(
	ctx context.Context,
	aggregateID string,
	aggregateType string,
) (domain.AggregateRoot, error) {
	// 애그리게이트 팩토리 가져오기
	factory, ok := r.aggregateStore[aggregateType]
	if !ok {
		return nil, fmt.Errorf("unknown aggregate type: %s", aggregateType)
	}

	// 애그리게이트 생성
	aggregate := factory(aggregateID)

	// 이벤트 로드
	events, err := r.eventStore.GetEvents(ctx, aggregateID, aggregateType)
	if err != nil {
		return nil, fmt.Errorf("failed to load events: %w", err)
	}

	// 이벤트가 없으면 새 애그리게이트 반환
	if len(events) == 0 {
		return aggregate, nil
	}

	// 이벤트 적용
	baseAggregate, ok := aggregate.(*domain.BaseAggregateRoot)
	if !ok {
		return nil, fmt.Errorf("aggregate does not implement BaseAggregateRoot")
	}

	baseAggregate.LoadFromHistory(aggregate, events)

	return aggregate, nil
}

// LoadWithVersion은 특정 버전까지의 애그리게이트를 로드합니다.
func (r *MongoRepository) LoadWithVersion(
	ctx context.Context,
	aggregateID string,
	aggregateType string,
	version int,
) (domain.AggregateRoot, error) {
	// 애그리게이트 팩토리 가져오기
	factory, ok := r.aggregateStore[aggregateType]
	if !ok {
		return nil, fmt.Errorf("unknown aggregate type: %s", aggregateType)
	}

	// 애그리게이트 생성
	aggregate := factory(aggregateID)

	// 이벤트 로드
	events, err := r.eventStore.GetEventsUpToVersion(ctx, aggregateID, aggregateType, version)
	if err != nil {
		return nil, fmt.Errorf("failed to load events: %w", err)
	}

	// 이벤트가 없으면 새 애그리게이트 반환
	if len(events) == 0 {
		return aggregate, nil
	}

	// 이벤트 적용
	baseAggregate, ok := aggregate.(*domain.BaseAggregateRoot)
	if !ok {
		return nil, fmt.Errorf("aggregate does not implement BaseAggregateRoot")
	}

	baseAggregate.LoadFromHistory(aggregate, events)

	return aggregate, nil
}
