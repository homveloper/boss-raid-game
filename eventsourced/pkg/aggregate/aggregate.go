package aggregate

import (
	"errors"
	"fmt"

	"eventsourced/pkg/event"
)

// Aggregate는 애그리게이트 인터페이스입니다.
type Aggregate interface {
	// ID는 애그리게이트의 고유 식별자를 반환합니다.
	ID() string

	// Type은 애그리게이트의 타입을 반환합니다.
	Type() string

	// Version은 애그리게이트의 현재 버전을 반환합니다.
	Version() int

	// SetVersion은 애그리게이트의 버전을 설정합니다.
	SetVersion(version int)

	// UncommittedEvents는 아직 커밋되지 않은 이벤트 목록을 반환합니다.
	UncommittedEvents() []event.Event

	// ClearUncommittedEvents는 커밋되지 않은 이벤트 목록을 초기화합니다.
	ClearUncommittedEvents()

	// ApplyEvent는 이벤트를 애그리게이트에 적용합니다.
	ApplyEvent(event event.Event) error

	// ApplyChange는 새 이벤트를 생성하고 애그리게이트에 적용합니다.
	ApplyChange(eventType string, data interface{}) error
}

// BaseAggregate는 기본 애그리게이트 구현입니다.
type BaseAggregate struct {
	id                string
	aggregateType     string
	version           int
	uncommittedEvents []event.Event
}

// NewBaseAggregate는 새로운 BaseAggregate를 생성합니다.
func NewBaseAggregate(id string, aggregateType string) *BaseAggregate {
	return &BaseAggregate{
		id:                id,
		aggregateType:     aggregateType,
		version:           0,
		uncommittedEvents: make([]event.Event, 0),
	}
}

// ID는 애그리게이트의 고유 식별자를 반환합니다.
func (a *BaseAggregate) ID() string {
	return a.id
}

// Type은 애그리게이트의 타입을 반환합니다.
func (a *BaseAggregate) Type() string {
	return a.aggregateType
}

// Version은 애그리게이트의 현재 버전을 반환합니다.
func (a *BaseAggregate) Version() int {
	return a.version
}

// SetVersion은 애그리게이트의 버전을 설정합니다.
func (a *BaseAggregate) SetVersion(version int) {
	a.version = version
}

// UncommittedEvents는 아직 커밋되지 않은 이벤트 목록을 반환합니다.
func (a *BaseAggregate) UncommittedEvents() []event.Event {
	return a.uncommittedEvents
}

// ClearUncommittedEvents는 커밋되지 않은 이벤트 목록을 초기화합니다.
func (a *BaseAggregate) ClearUncommittedEvents() {
	a.uncommittedEvents = make([]event.Event, 0)
}

// ApplyEvent는 이벤트를 애그리게이트에 적용합니다.
func (a *BaseAggregate) ApplyEvent(e event.Event) error {
	if e == nil {
		return errors.New("event cannot be nil")
	}

	if e.AggregateID() != a.id {
		return fmt.Errorf("event aggregate ID %s does not match aggregate ID %s", e.AggregateID(), a.id)
	}

	if e.AggregateType() != a.aggregateType {
		return fmt.Errorf("event aggregate type %s does not match aggregate type %s", e.AggregateType(), a.aggregateType)
	}

	// 이벤트 핸들러 메서드 호출
	if err := a.callEventHandler(e); err != nil {
		return err
	}

	// 버전 업데이트
	a.version = e.Version()

	return nil
}

// ApplyChange는 새 이벤트를 생성하고 애그리게이트에 적용합니다.
func (a *BaseAggregate) ApplyChange(eventType string, data interface{}) error {
	// 새 이벤트 생성
	e := event.NewDefaultEvent(
		eventType,
		a.id,
		a.aggregateType,
		a.version+1,
		event.Now(),
		data,
	)

	// 이벤트 핸들러 메서드 호출
	if err := a.callEventHandler(e); err != nil {
		return err
	}

	// 버전 업데이트
	a.version = e.Version()

	// 커밋되지 않은 이벤트 목록에 추가
	a.uncommittedEvents = append(a.uncommittedEvents, e)

	return nil
}

// callEventHandler는 이벤트 타입에 해당하는 핸들러 메서드를 호출합니다.
func (a *BaseAggregate) callEventHandler(e event.Event) error {
	// 이 메서드는 구체적인 애그리게이트 구현에서 오버라이드해야 합니다.
	// 기본 구현은 아무 작업도 수행하지 않습니다.
	return nil
}
