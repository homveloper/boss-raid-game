package aggregate

import (
	"fmt"
)

// AggregateFactory는 애그리게이트 팩토리 인터페이스입니다.
type AggregateFactory interface {
	// CreateAggregate는 지정된 타입과 ID로 새 애그리게이트를 생성합니다.
	CreateAggregate(aggregateType string, id string) (Aggregate, error)
}

// AggregateCreator는 애그리게이트 생성 함수 타입입니다.
type AggregateCreator func(id string) Aggregate

// DefaultAggregateFactory는 기본 애그리게이트 팩토리 구현입니다.
type DefaultAggregateFactory struct {
	creators map[string]AggregateCreator
}

// NewAggregateFactory는 새로운 DefaultAggregateFactory를 생성합니다.
func NewAggregateFactory() *DefaultAggregateFactory {
	return &DefaultAggregateFactory{
		creators: make(map[string]AggregateCreator),
	}
}

// RegisterAggregate는 애그리게이트 타입과 생성자를 등록합니다.
func (f *DefaultAggregateFactory) RegisterAggregate(aggregateType string, creator AggregateCreator) {
	f.creators[aggregateType] = creator
}

// CreateAggregate는 지정된 타입과 ID로 새 애그리게이트를 생성합니다.
func (f *DefaultAggregateFactory) CreateAggregate(aggregateType string, id string) (Aggregate, error) {
	creator, ok := f.creators[aggregateType]
	if !ok {
		return nil, fmt.Errorf("no creator registered for aggregate type %s", aggregateType)
	}

	return creator(id), nil
}
