package domain

// Event는 CQRS 패턴의 이벤트 인터페이스입니다.
// 모든 이벤트는 이 인터페이스를 구현해야 합니다.
type Event interface {
	// AggregateID는 이벤트가 속한 애그리게이트의 ID를 반환합니다.
	AggregateID() string

	// AggregateType은 이벤트가 속한 애그리게이트의 타입을 반환합니다.
	AggregateType() string

	// EventType은 이벤트의 타입을 반환합니다.
	EventType() string

	// Version은 이벤트의 버전을 반환합니다.
	Version() int

	// SetVersion은 이벤트의 버전을 설정합니다.
	SetVersion(version int)
}

// BaseEvent는 Event 인터페이스의 기본 구현입니다.
// 모든 이벤트는 이 구조체를 임베딩하여 사용할 수 있습니다.
type BaseEvent struct {
	ID             string `json:"id" bson:"id"`
	Type           string `json:"type" bson:"type"`
	AggregateId    string `json:"aggregate_id" bson:"aggregate_id"`
	AggregateTyped string `json:"aggregate_type" bson:"aggregate_type"`
	EventVer       int    `json:"version" bson:"version"`
}

// AggregateID는 이벤트가 속한 애그리게이트의 ID를 반환합니다.
func (e *BaseEvent) AggregateID() string {
	return e.AggregateId
}

// AggregateType은 이벤트가 속한 애그리게이트의 타입을 반환합니다.
func (e *BaseEvent) AggregateType() string {
	return e.AggregateTyped
}

// EventType은 이벤트의 타입을 반환합니다.
func (e *BaseEvent) EventType() string {
	return e.Type
}

// Version은 이벤트의 버전을 반환합니다.
func (e *BaseEvent) Version() int {
	return e.EventVer
}

// SetVersion은 이벤트의 버전을 설정합니다.
func (e *BaseEvent) SetVersion(version int) {
	e.EventVer = version
}

// NewBaseEvent는 새로운 BaseEvent를 생성합니다.
func NewBaseEvent(id, eventType, aggregateID, aggregateType string) BaseEvent {
	return BaseEvent{
		ID:             id,
		Type:           eventType,
		AggregateId:    aggregateID,
		AggregateTyped: aggregateType,
		EventVer:       0,
	}
}
