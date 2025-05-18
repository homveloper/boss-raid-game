package event

import (
	"context"
	"time"
)

// Event는 도메인 이벤트 인터페이스입니다.
type Event interface {
	// EventType은 이벤트 타입을 반환합니다.
	EventType() string

	// AggregateID는 이벤트가 속한 애그리게이트 ID를 반환합니다.
	AggregateID() string

	// AggregateType은 이벤트가 속한 애그리게이트 타입을 반환합니다.
	AggregateType() string

	// Version은 이벤트 버전을 반환합니다.
	Version() int

	// Timestamp는 이벤트 발생 시간을 반환합니다.
	Timestamp() time.Time

	// Data는 이벤트 데이터를 반환합니다.
	Data() interface{}
}

// DefaultEvent는 기본 이벤트 구현입니다.
type DefaultEvent struct {
	Type           string      `json:"type" bson:"type"`
	AggrID         string      `json:"aggregate_id" bson:"aggregate_id"`
	AggrType       string      `json:"aggregate_type" bson:"aggregate_type"`
	VersionValue   int         `json:"version" bson:"version"`
	TimestampValue time.Time   `json:"timestamp" bson:"timestamp"`
	DataValue      interface{} `json:"data" bson:"data"`
}

// NewDefaultEvent는 새로운 DefaultEvent를 생성합니다.
func NewDefaultEvent(
	eventType string,
	aggregateID string,
	aggregateType string,
	version int,
	timestamp time.Time,
	data interface{},
) *DefaultEvent {
	return &DefaultEvent{
		Type:           eventType,
		AggrID:         aggregateID,
		AggrType:       aggregateType,
		VersionValue:   version,
		TimestampValue: timestamp,
		DataValue:      data,
	}
}

// EventType은 이벤트 타입을 반환합니다.
func (e *DefaultEvent) EventType() string {
	return e.Type
}

// AggregateID는 이벤트가 속한 애그리게이트 ID를 반환합니다.
func (e *DefaultEvent) AggregateID() string {
	return e.AggrID
}

// AggregateType은 이벤트가 속한 애그리게이트 타입을 반환합니다.
func (e *DefaultEvent) AggregateType() string {
	return e.AggrType
}

// Version은 이벤트 버전을 반환합니다.
func (e *DefaultEvent) Version() int {
	return e.VersionValue
}

// Timestamp는 이벤트 발생 시간을 반환합니다.
func (e *DefaultEvent) Timestamp() time.Time {
	return e.TimestampValue
}

// Data는 이벤트 데이터를 반환합니다.
func (e *DefaultEvent) Data() interface{} {
	return e.DataValue
}

// Now는 현재 시간을 반환합니다.
func Now() time.Time {
	return time.Now().UTC()
}

// EventHandler는 이벤트 핸들러 인터페이스입니다.
type EventHandler interface {
	// HandleEvent는 이벤트를 처리합니다.
	HandleEvent(ctx context.Context, event Event) error
}

// EventHandlerFunc는 함수를 EventHandler로 변환합니다.
type EventHandlerFunc func(ctx context.Context, event Event) error

// HandleEvent는 EventHandler 인터페이스를 구현합니다.
func (f EventHandlerFunc) HandleEvent(ctx context.Context, event Event) error {
	return f(ctx, event)
}

// EventBus는 이벤트 버스 인터페이스입니다.
type EventBus interface {
	// PublishEvent는 이벤트를 발행합니다.
	PublishEvent(ctx context.Context, event Event) error

	// Subscribe는 특정 이벤트 타입에 핸들러를 등록합니다.
	Subscribe(eventType string, handler EventHandler)

	// SubscribeAll은 모든 이벤트에 핸들러를 등록합니다.
	SubscribeAll(handler EventHandler)
}

// EventSerializer는 이벤트 직렬화 인터페이스입니다.
type EventSerializer interface {
	// Serialize는 이벤트를 바이트 배열로 직렬화합니다.
	Serialize(event Event) ([]byte, error)
}

// EventDeserializer는 이벤트 역직렬화 인터페이스입니다.
type EventDeserializer interface {
	// Deserialize는 바이트 배열을 이벤트로 역직렬화합니다.
	Deserialize(data []byte) (Event, error)
}
