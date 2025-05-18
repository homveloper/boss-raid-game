package common

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

// EventBus는 이벤트 버스 인터페이스입니다.
type EventBus interface {
	// PublishEvent는 이벤트를 발행합니다.
	PublishEvent(ctx context.Context, event Event) error

	// Subscribe는 특정 이벤트 타입에 핸들러를 등록합니다.
	Subscribe(eventType string, handler EventHandler)

	// SubscribeAll은 모든 이벤트에 핸들러를 등록합니다.
	SubscribeAll(handler EventHandler)
}

// EventHandler는 이벤트 핸들러 인터페이스입니다.
type EventHandler interface {
	// HandleEvent는 이벤트를 처리합니다.
	HandleEvent(ctx context.Context, event Event) error
}

// EventMapper는 문서 변경을 이벤트로 매핑하는 인터페이스입니다.
type EventMapper interface {
	// MapToEvents는 문서 변경을 이벤트로 매핑합니다.
	MapToEvents(collection string, id string, diff *Diff) []Event
}
