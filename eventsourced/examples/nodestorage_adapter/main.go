package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"eventsourced/pkg/event"
	"eventsourced/pkg/storage"

	"github.com/yourusername/nodestorage/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// User는 사용자 모델입니다.
type User struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	Email     string    `bson:"email"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
	Version   int       `bson:"version"`
}

// UserCreatedHandler는 UserCreated 이벤트 핸들러입니다.
type UserCreatedHandler struct{}

func (h *UserCreatedHandler) HandleEvent(ctx context.Context, e event.Event) error {
	fmt.Printf("User created event: %s\n", e.AggregateID())
	return nil
}

// UserUpdatedHandler는 UserUpdated 이벤트 핸들러입니다.
type UserUpdatedHandler struct{}

func (h *UserUpdatedHandler) HandleEvent(ctx context.Context, e event.Event) error {
	fmt.Printf("User updated event: %s\n", e.AggregateID())
	return nil
}

func main() {
	// MongoDB 연결
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	// nodestorage 생성
	nodeStorage, err := nodestorage.NewStorage(ctx, client, "mydb", &nodestorage.StorageOptions{
		VersionField: "version",
	})
	if err != nil {
		log.Fatalf("Failed to create nodestorage: %v", err)
	}

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

	// NodeStorageAdapter 생성
	adapter, err := storage.NewNodeStorageAdapter(nodeStorage, &storage.NodeStorageAdapterOptions{
		EventBus:    eventBus,
		EventMapper: eventMapper,
	})
	if err != nil {
		log.Fatalf("Failed to create NodeStorageAdapter: %v", err)
	}

	// 제네릭 어댑터 생성
	genericAdapter, err := storage.NewNodeStorageGenericAdapter[User](nodeStorage, &storage.NodeStorageAdapterOptions{
		EventBus:    eventBus,
		EventMapper: eventMapper,
	})
	if err != nil {
		log.Fatalf("Failed to create NodeStorageGenericAdapter: %v", err)
	}

	// 사용자 ID
	userID := "user123"

	// 1. 비제네릭 어댑터를 사용한 사용자 생성
	fmt.Println("=== 비제네릭 어댑터를 사용한 사용자 생성 ===")
	_, err = adapter.Update(ctx, "users", userID, func(doc interface{}) (interface{}, error) {
		// 새 사용자 생성
		now := time.Now()
		return &User{
			ID:        userID,
			Name:      "John Doe",
			Email:     "john@example.com",
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
		}, nil
	})
	if err != nil {
		log.Fatalf("Failed to create user with non-generic adapter: %v", err)
	}

	// 2. 제네릭 어댑터를 사용한 사용자 업데이트
	fmt.Println("=== 제네릭 어댑터를 사용한 사용자 업데이트 ===")
	_, err = genericAdapter.Update(ctx, "users", userID, func(user User) (User, error) {
		// 사용자 업데이트
		user.Email = "john.doe@example.com"
		user.UpdatedAt = time.Now()
		user.Version++
		return user, nil
	})
	if err != nil {
		log.Fatalf("Failed to update user with generic adapter: %v", err)
	}

	// 3. 사용자 조회
	fmt.Println("=== 사용자 조회 ===")
	user, err := genericAdapter.GetDocument(ctx, "users", userID)
	if err != nil {
		log.Fatalf("Failed to get user: %v", err)
	}
	fmt.Printf("User: %+v\n", user)

	// 4. 제네릭 어댑터를 사용한 새 사용자 생성
	fmt.Println("=== 제네릭 어댑터를 사용한 새 사용자 생성 ===")
	newUser := User{
		Name:      "Jane Smith",
		Email:     "jane@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err = genericAdapter.CreateDocument(ctx, "users", "user456", newUser)
	if err != nil {
		log.Fatalf("Failed to create new user with generic adapter: %v", err)
	}

	// 5. 비제네릭 어댑터를 사용한 문서 조회 및 업데이트
	fmt.Println("=== 비제네릭 어댑터를 사용한 문서 조회 및 업데이트 ===")
	var result bson.M
	err = adapter.FindOne(ctx, "users", bson.M{"_id": "user456"}).Decode(&result)
	if err != nil {
		log.Fatalf("Failed to find user: %v", err)
	}
	fmt.Printf("Found user: %+v\n", result)

	// 6. FindOneAndUpdate 사용
	fmt.Println("=== FindOneAndUpdate 사용 ===")
	updateResult := adapter.FindOneAndUpdate(
		ctx,
		"users",
		bson.M{"_id": "user456"},
		bson.M{"$set": bson.M{"email": "jane.smith@example.com", "updated_at": time.Now()}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var updatedUser bson.M
	if err := updateResult.Decode(&updatedUser); err != nil {
		log.Fatalf("Failed to update user: %v", err)
	}
	fmt.Printf("Updated user: %+v\n", updatedUser)

	fmt.Println("모든 작업이 성공적으로 완료되었습니다!")
}
