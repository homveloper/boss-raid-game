package event

import (
	"context"
	"eventsourced/pkg/common"
	"time"
)

// Event 인터페이스는 common 패키지로 이동했습니다.
// 하위 호환성을 위해 타입 별칭 제공
type Event = common.Event

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

// EventHandler 인터페이스는 common 패키지로 이동했습니다.
// 하위 호환성을 위해 타입 별칭 제공
type EventHandler = common.EventHandler

// EventHandlerFunc는 함수를 EventHandler로 변환합니다.
type EventHandlerFunc func(ctx context.Context, event Event) error

// HandleEvent는 EventHandler 인터페이스를 구현합니다.
func (f EventHandlerFunc) HandleEvent(ctx context.Context, event Event) error {
	return f(ctx, event)
}

// EventBus 인터페이스는 common 패키지로 이동했습니다.
// 하위 호환성을 위해 타입 별칭 제공
type EventBus = common.EventBus

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
