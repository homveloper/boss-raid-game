package domain

import (
	"fmt"
	"reflect"
)

// AggregateRoot는 CQRS 패턴의 애그리게이트 루트 인터페이스입니다.
// 모든 애그리게이트는 이 인터페이스를 구현해야 합니다.
type AggregateRoot interface {
	// AggregateID는 애그리게이트의 고유 ID를 반환합니다.
	AggregateID() string

	// AggregateType은 애그리게이트의 타입을 반환합니다.
	AggregateType() string

	// Version은 애그리게이트의 현재 버전을 반환합니다.
	Version() int

	// IncrementVersion은 애그리게이트의 버전을 증가시킵니다.
	IncrementVersion()

	// ApplyChange는 이벤트를 애그리게이트에 적용합니다.
	ApplyChange(event Event)

	// GetUncommittedChanges는 아직 커밋되지 않은 변경 사항을 반환합니다.
	GetUncommittedChanges() []Event

	// ClearUncommittedChanges는 커밋되지 않은 변경 사항을 지웁니다.
	ClearUncommittedChanges()
}

// BaseAggregateRoot는 AggregateRoot 인터페이스의 기본 구현입니다.
// 모든 애그리게이트는 이 구조체를 임베딩하여 사용할 수 있습니다.
type BaseAggregateRoot struct {
	ID      string
	Type    string
	version int
	changes []Event
}

// AggregateID는 애그리게이트의 고유 ID를 반환합니다.
func (a *BaseAggregateRoot) AggregateID() string {
	return a.ID
}

// AggregateType은 애그리게이트의 타입을 반환합니다.
func (a *BaseAggregateRoot) AggregateType() string {
	return a.Type
}

// Version은 애그리게이트의 현재 버전을 반환합니다.
func (a *BaseAggregateRoot) Version() int {
	return a.version
}

// IncrementVersion은 애그리게이트의 버전을 증가시킵니다.
func (a *BaseAggregateRoot) IncrementVersion() {
	a.version++
}

// ApplyChange는 이벤트를 애그리게이트에 적용합니다.
func (a *BaseAggregateRoot) ApplyChange(event Event) {
	a.ApplyChangeHelper(a, event, false)
}

// ApplyChangeHelper는 이벤트를 애그리게이트에 적용하는 헬퍼 함수입니다.
func (a *BaseAggregateRoot) ApplyChangeHelper(aggregate AggregateRoot, event Event, isHistory bool) {
	// Apply 메서드 호출
	handler := a.getApplyMethod(aggregate, event)
	if handler.IsValid() {
		handler.Call([]reflect.Value{reflect.ValueOf(event)})
	}

	// 이벤트가 히스토리가 아닌 경우 변경 사항에 추가
	if !isHistory {
		a.changes = append(a.changes, event)
	}
}

// getApplyMethod는 이벤트 타입에 맞는 Apply 메서드를 찾습니다.
func (a *BaseAggregateRoot) getApplyMethod(aggregate AggregateRoot, event Event) reflect.Value {
	aggregateType := reflect.TypeOf(aggregate)
	methodName := fmt.Sprintf("Apply%s", event.EventType())

	// 메서드 찾기
	method, ok := aggregateType.MethodByName(methodName)
	if !ok {
		return reflect.Value{}
	}

	return method.Func
}

// GetUncommittedChanges는 아직 커밋되지 않은 변경 사항을 반환합니다.
func (a *BaseAggregateRoot) GetUncommittedChanges() []Event {
	return a.changes
}

// ClearUncommittedChanges는 커밋되지 않은 변경 사항을 지웁니다.
func (a *BaseAggregateRoot) ClearUncommittedChanges() {
	a.changes = []Event{}
}

// LoadFromHistory는 이벤트 히스토리를 기반으로 애그리게이트 상태를 재구성합니다.
func (a *BaseAggregateRoot) LoadFromHistory(aggregate AggregateRoot, events []Event) {
	for _, event := range events {
		a.version = event.Version()
		a.ApplyChangeHelper(aggregate, event, true)
	}
}
