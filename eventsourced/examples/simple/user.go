package main

import (
	"errors"

	"github.com/yourusername/eventsourced/pkg/aggregate"
	"github.com/yourusername/eventsourced/pkg/event"
)

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
	if name == "" {
		return errors.New("name cannot be empty")
	}
	if email == "" {
		return errors.New("email cannot be empty")
	}

	return a.ApplyChange("UserCreated", map[string]interface{}{
		"name":  name,
		"email": email,
	})
}

// UpdateEmail은 사용자 이메일을 업데이트합니다.
func (a *UserAggregate) UpdateEmail(email string) error {
	if email == "" {
		return errors.New("email cannot be empty")
	}
	if email == a.Email {
		return errors.New("new email is the same as current email")
	}

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
		return errors.New("invalid event data")
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
		return errors.New("invalid event data")
	}

	email, _ := data["new_email"].(string)
	a.Email = email

	return nil
}
