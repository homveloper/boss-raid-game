package helper

import (
	"context"

	"eventsourced/pkg/aggregate"
	"eventsourced/pkg/command"
	"eventsourced/pkg/event"
	"eventsourced/pkg/storage"

	"go.mongodb.org/mongo-driver/mongo"
)

// SimpleCQRS는 CQRS 초보자를 위한 간단한 헬퍼 구조체입니다.
type SimpleCQRS struct {
	client           *mongo.Client
	eventBus         event.EventBus
	storage          *storage.EventSourcedStorage
	aggregateFactory *aggregate.DefaultAggregateFactory
	repository       *aggregate.EventSourcedRepository
	commandHandler   *command.BaseCommandHandler
	dispatcher       *command.DefaultDispatcher
}

// NewSimpleCQRS는 새로운 SimpleCQRS 인스턴스를 생성합니다.
func NewSimpleCQRS(ctx context.Context, client *mongo.Client, dbName string) (*SimpleCQRS, error) {
	// 이벤트 버스 생성
	eventBus := event.NewInMemoryEventBus()

	// 이벤트 매퍼 생성
	eventMapper := event.NewDefaultEventMapper()

	// EventSourcedStorage 생성
	storageOpts := &storage.EventSourcedStorageOptions{
		StorageOptions: &storage.StorageOptions{
			VersionField: "version",
		},
		EventBus:    eventBus,
		EventMapper: eventMapper,
	}

	eventSourcedStorage, err := storage.NewEventSourcedStorage(ctx, client, dbName, storageOpts)
	if err != nil {
		return nil, err
	}

	// 애그리게이트 팩토리 생성
	aggregateFactory := aggregate.NewAggregateFactory()

	// 리포지토리 생성
	repository := aggregate.NewRepository(eventSourcedStorage, aggregateFactory)

	// 커맨드 핸들러 생성
	commandHandler := command.NewCommandHandler(repository)

	// 커맨드 디스패처 생성
	dispatcher := command.NewDispatcher()

	return &SimpleCQRS{
		client:           client,
		eventBus:         eventBus,
		storage:          eventSourcedStorage,
		aggregateFactory: aggregateFactory,
		repository:       repository,
		commandHandler:   commandHandler,
		dispatcher:       dispatcher,
	}, nil
}

// RegisterAggregate는 애그리게이트 타입을 등록합니다.
func (s *SimpleCQRS) RegisterAggregate(aggregateType string, creator aggregate.AggregateCreator) {
	s.aggregateFactory.RegisterAggregate(aggregateType, creator)
}

// RegisterCommandHandler는 커맨드 핸들러를 등록합니다.
func (s *SimpleCQRS) RegisterCommandHandler(commandType string, handler command.CommandHandlerFunc) {
	s.commandHandler.RegisterHandler(commandType, handler)
	s.dispatcher.RegisterHandler(commandType, s.commandHandler)
}

// RegisterEventHandler는 이벤트 핸들러를 등록합니다.
func (s *SimpleCQRS) RegisterEventHandler(eventType string, handler event.EventHandler) {
	s.eventBus.Subscribe(eventType, handler)
}

// RegisterEventHandlerFunc는 이벤트 핸들러 함수를 등록합니다.
func (s *SimpleCQRS) RegisterEventHandlerFunc(eventType string, handler func(ctx context.Context, e event.Event) error) {
	s.eventBus.Subscribe(eventType, event.EventHandlerFunc(handler))
}

// ExecuteCommand는 커맨드를 실행합니다.
func (s *SimpleCQRS) ExecuteCommand(ctx context.Context, commandType string, aggregateID string, payload interface{}) error {
	cmd := command.NewCommand(commandType, aggregateID, payload)
	return s.dispatcher.Dispatch(ctx, cmd)
}

// ExecuteCommandWithType은 애그리게이트 타입이 지정된 커맨드를 실행합니다.
func (s *SimpleCQRS) ExecuteCommandWithType(ctx context.Context, commandType string, aggregateID string, aggregateType string, payload interface{}) error {
	cmd := command.NewCommandWithType(commandType, aggregateID, aggregateType, payload)
	return s.dispatcher.Dispatch(ctx, cmd)
}

// GetRepository는 리포지토리를 반환합니다.
func (s *SimpleCQRS) GetRepository() aggregate.Repository {
	return s.repository
}

// GetEventBus는 이벤트 버스를 반환합니다.
func (s *SimpleCQRS) GetEventBus() event.EventBus {
	return s.eventBus
}

// GetStorage는 저장소를 반환합니다.
func (s *SimpleCQRS) GetStorage() *storage.EventSourcedStorage {
	return s.storage
}

// Close는 리소스를 정리합니다.
func (s *SimpleCQRS) Close(ctx context.Context) error {
	return nil
}
