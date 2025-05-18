package main

import (
	"context"
	"log"

	"github.com/yourusername/eventsourced/pkg/aggregate"
	"github.com/yourusername/eventsourced/pkg/command"
	"github.com/yourusername/eventsourced/pkg/event"
	"github.com/yourusername/eventsourced/pkg/helper"
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

	// SimpleCQRS 생성
	simpleCQRS, err := helper.NewSimpleCQRS(ctx, client, "mydb")
	if err != nil {
		log.Fatalf("Failed to create SimpleCQRS: %v", err)
	}

	// 1. 애그리게이트 등록
	simpleCQRS.RegisterAggregate("User", func(id string) aggregate.Aggregate {
		return NewUserAggregate(id)
	})

	// 2. 커맨드 핸들러 등록
	simpleCQRS.RegisterCommandHandler("CreateUser", handleCreateUser)
	simpleCQRS.RegisterCommandHandler("UpdateUser", handleUpdateUser)

	// 3. 이벤트 핸들러 등록 (함수형 방식)
	simpleCQRS.RegisterEventHandlerFunc("UserCreated", func(ctx context.Context, e event.Event) error {
		log.Printf("이벤트 알림: 새 사용자가 생성되었습니다! ID: %s", e.AggregateID())
		return nil
	})

	simpleCQRS.RegisterEventHandlerFunc("UserEmailChanged", func(ctx context.Context, e event.Event) error {
		data, ok := e.Data().(map[string]interface{})
		if !ok {
			return nil
		}
		newEmail, _ := data["new_email"].(string)
		log.Printf("이벤트 알림: 사용자 %s의 이메일이 %s로 변경되었습니다", e.AggregateID(), newEmail)
		return nil
	})

	// 4. 커맨드 실행
	log.Println("사용자 생성 중...")
	err = simpleCQRS.ExecuteCommandWithType(ctx, "CreateUser", "user123", "User", map[string]interface{}{
		"name":  "홍길동",
		"email": "hong@example.com",
	})
	if err != nil {
		log.Fatalf("사용자 생성 실패: %v", err)
	}

	log.Println("사용자 이메일 업데이트 중...")
	err = simpleCQRS.ExecuteCommandWithType(ctx, "UpdateUser", "user123", "User", map[string]interface{}{
		"email": "hong.gildong@example.com",
	})
	if err != nil {
		log.Fatalf("사용자 업데이트 실패: %v", err)
	}

	log.Println("모든 작업이 성공적으로 완료되었습니다!")
}

// UserAggregate는 사용자 애그리게이트입니다.
type UserAggregate struct {
	*aggregate.BaseAggregate
	Name  string
	Email string
}

// NewUserAggregate는 새로운 UserAggregate를 생성합니다.
func NewUserAggregate(id string) *UserAggregate {
	return &UserAggregate{
		BaseAggregate: aggregate.NewBaseAggregate(id, "User"),
	}
}

// Create는 사용자를 생성합니다.
func (a *UserAggregate) Create(name, email string) error {
	return a.ApplyChange("UserCreated", map[string]interface{}{
		"name":  name,
		"email": email,
	})
}

// UpdateEmail은 사용자 이메일을 업데이트합니다.
func (a *UserAggregate) UpdateEmail(email string) error {
	return a.ApplyChange("UserEmailChanged", map[string]interface{}{
		"old_email": a.Email,
		"new_email": email,
	})
}

// callEventHandler는 이벤트 타입에 해당하는 핸들러 메서드를 호출합니다.
func (a *UserAggregate) callEventHandler(e event.Event) error {
	switch e.EventType() {
	case "UserCreated":
		return a.applyUserCreated(e)
	case "UserEmailChanged":
		return a.applyUserEmailChanged(e)
	default:
		return nil
	}
}

// applyUserCreated는 UserCreated 이벤트를 적용합니다.
func (a *UserAggregate) applyUserCreated(e event.Event) error {
	data, ok := e.Data().(map[string]interface{})
	if !ok {
		return nil
	}

	name, _ := data["name"].(string)
	email, _ := data["email"].(string)

	a.Name = name
	a.Email = email

	return nil
}

// applyUserEmailChanged는 UserEmailChanged 이벤트를 적용합니다.
func (a *UserAggregate) applyUserEmailChanged(e event.Event) error {
	data, ok := e.Data().(map[string]interface{})
	if !ok {
		return nil
	}

	email, _ := data["new_email"].(string)
	a.Email = email

	return nil
}

// handleCreateUser는 CreateUser 커맨드를 처리합니다.
func handleCreateUser(ctx context.Context, cmd command.Command) error {
	log.Printf("CreateUser 커맨드 처리 중: %s", cmd.AggregateID())

	// 애그리게이트 생성
	userAggregate := NewUserAggregate(cmd.AggregateID())

	// 페이로드 추출
	payload, ok := cmd.Payload().(map[string]interface{})
	if !ok {
		return nil
	}

	// 사용자 생성
	name, _ := payload["name"].(string)
	email, _ := payload["email"].(string)

	if err := userAggregate.Create(name, email); err != nil {
		return err
	}

	// 애그리게이트 저장
	// 실제 구현에서는 repository.Save(ctx, userAggregate) 호출
	log.Printf("사용자 생성됨: %s, %s", name, email)

	return nil
}

// handleUpdateUser는 UpdateUser 커맨드를 처리합니다.
func handleUpdateUser(ctx context.Context, cmd command.Command) error {
	log.Printf("UpdateUser 커맨드 처리 중: %s", cmd.AggregateID())

	// 애그리게이트 로드
	// 실제 구현에서는 repository.Load(ctx, cmd.AggregateID(), cmd.AggregateType()) 호출
	userAggregate := NewUserAggregate(cmd.AggregateID())

	// 페이로드 추출
	payload, ok := cmd.Payload().(map[string]interface{})
	if !ok {
		return nil
	}

	// 사용자 업데이트
	if email, ok := payload["email"].(string); ok {
		if err := userAggregate.UpdateEmail(email); err != nil {
			return err
		}
	}

	// 애그리게이트 저장
	// 실제 구현에서는 repository.Save(ctx, userAggregate) 호출
	log.Printf("사용자 업데이트됨: %s", cmd.AggregateID())

	return nil
}
