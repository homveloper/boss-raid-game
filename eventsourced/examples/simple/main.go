package main

import (
	"context"
	"log"

	"github.com/yourusername/eventsourced/pkg/aggregate"
	"github.com/yourusername/eventsourced/pkg/command"
	"github.com/yourusername/eventsourced/pkg/event"
	"github.com/yourusername/eventsourced/pkg/storage"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// MongoDB 연결
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	// 이벤트 버스 생성
	eventBus := event.NewInMemoryEventBus()

	// 이벤트 핸들러 등록
	eventBus.Subscribe("UserCreated", &UserCreatedHandler{})
	eventBus.Subscribe("UserUpdated", &UserUpdatedHandler{})

	// 이벤트 매퍼 생성
	eventMapper := event.NewDefaultEventMapper()
	eventMapper.RegisterCollectionEventTypes("users", event.CollectionEventTypes{
		Created: "UserCreated",
		Updated: "UserUpdated",
		Deleted: "UserDeleted",
	})

	// EventSourcedStorage 생성
	storageOpts := &storage.EventSourcedStorageOptions{
		StorageOptions: &storage.StorageOptions{
			VersionField: "version",
		},
		EventBus:    eventBus,
		EventMapper: eventMapper,
	}

	eventSourcedStorage, err := storage.NewEventSourcedStorage(ctx, client, "mydb", storageOpts)
	if err != nil {
		log.Fatalf("Failed to create EventSourcedStorage: %v", err)
	}

	// 애그리게이트 팩토리 생성
	aggregateFactory := aggregate.NewAggregateFactory()
	aggregateFactory.RegisterAggregate("User", func(id string) aggregate.Aggregate {
		return NewUserAggregate(id)
	})

	// 리포지토리 생성
	repository := aggregate.NewRepository(eventSourcedStorage, aggregateFactory)

	// 커맨드 핸들러 생성
	commandHandler := command.NewCommandHandler(repository)
	commandHandler.RegisterHandler("CreateUser", handleCreateUser)
	commandHandler.RegisterHandler("UpdateUser", handleUpdateUser)

	// 커맨드 디스패처 생성
	dispatcher := command.NewDispatcher()
	dispatcher.RegisterHandler("CreateUser", commandHandler)
	dispatcher.RegisterHandler("UpdateUser", commandHandler)

	// 사용자 생성 커맨드 실행
	createUserCmd := command.NewCommandWithType("CreateUser", "user123", "User", map[string]interface{}{
		"name":  "John Doe",
		"email": "john@example.com",
	})

	if err := dispatcher.Dispatch(ctx, createUserCmd); err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}

	log.Println("User created successfully")

	// 사용자 업데이트 커맨드 실행
	updateUserCmd := command.NewCommandWithType("UpdateUser", "user123", "User", map[string]interface{}{
		"email": "john.doe@example.com",
	})

	if err := dispatcher.Dispatch(ctx, updateUserCmd); err != nil {
		log.Fatalf("Failed to update user: %v", err)
	}

	log.Println("User updated successfully")
}

// handleCreateUser는 CreateUser 커맨드를 처리합니다.
func handleCreateUser(ctx context.Context, cmd command.Command) error {
	log.Printf("Handling CreateUser command for aggregate %s", cmd.AggregateID())

	// 애그리게이트 생성
	userAggregate := NewUserAggregate(cmd.AggregateID())

	// 페이로드 추출
	payload, ok := cmd.Payload().(map[string]interface{})
	if !ok {
		return ErrInvalidPayload
	}

	// 사용자 생성
	name, _ := payload["name"].(string)
	email, _ := payload["email"].(string)

	if err := userAggregate.Create(name, email); err != nil {
		return err
	}

	// 애그리게이트 저장
	// 실제 구현에서는 repository.Save(ctx, userAggregate) 호출
	log.Printf("User created: %s, %s", name, email)

	return nil
}

// handleUpdateUser는 UpdateUser 커맨드를 처리합니다.
func handleUpdateUser(ctx context.Context, cmd command.Command) error {
	log.Printf("Handling UpdateUser command for aggregate %s", cmd.AggregateID())

	// 애그리게이트 로드
	// 실제 구현에서는 repository.Load(ctx, cmd.AggregateID(), cmd.AggregateType()) 호출
	userAggregate := NewUserAggregate(cmd.AggregateID())

	// 페이로드 추출
	payload, ok := cmd.Payload().(map[string]interface{})
	if !ok {
		return ErrInvalidPayload
	}

	// 사용자 업데이트
	if email, ok := payload["email"].(string); ok {
		if err := userAggregate.UpdateEmail(email); err != nil {
			return err
		}
	}

	// 애그리게이트 저장
	// 실제 구현에서는 repository.Save(ctx, userAggregate) 호출
	log.Printf("User updated: %s", cmd.AggregateID())

	return nil
}

// UserCreatedHandler는 UserCreated 이벤트 핸들러입니다.
type UserCreatedHandler struct{}

func (h *UserCreatedHandler) HandleEvent(ctx context.Context, e event.Event) error {
	log.Printf("User created event: %s", e.AggregateID())
	return nil
}

// UserUpdatedHandler는 UserUpdated 이벤트 핸들러입니다.
type UserUpdatedHandler struct{}

func (h *UserUpdatedHandler) HandleEvent(ctx context.Context, e event.Event) error {
	log.Printf("User updated event: %s", e.AggregateID())
	return nil
}

// ErrInvalidPayload는 잘못된 페이로드 오류입니다.
var ErrInvalidPayload = &customError{message: "invalid payload"}

// customError는 사용자 정의 오류입니다.
type customError struct {
	message string
}

func (e *customError) Error() string {
	return e.message
}
