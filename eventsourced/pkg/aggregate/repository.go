package aggregate

import (
	"context"
	"errors"
	"fmt"

	"github.com/yourusername/eventsourced/pkg/event"
	"github.com/yourusername/eventsourced/pkg/storage"
)

// Repository는 애그리게이트 리포지토리 인터페이스입니다.
type Repository interface {
	// Load는 지정된 ID와 타입의 애그리게이트를 로드합니다.
	Load(ctx context.Context, id string, aggregateType string) (Aggregate, error)

	// Save는 애그리게이트를 저장합니다.
	Save(ctx context.Context, aggregate Aggregate) error
}

// EventSourcedRepository는 이벤트 소싱 기반 리포지토리 구현입니다.
type EventSourcedRepository struct {
	storage         *storage.EventSourcedStorage
	aggregateFactory AggregateFactory
	eventBus        event.EventBus
}

// NewRepository는 새로운 EventSourcedRepository를 생성합니다.
func NewRepository(
	storage *storage.EventSourcedStorage,
	aggregateFactory AggregateFactory,
) *EventSourcedRepository {
	return &EventSourcedRepository{
		storage:         storage,
		aggregateFactory: aggregateFactory,
	}
}

// Load는 지정된 ID와 타입의 애그리게이트를 로드합니다.
func (r *EventSourcedRepository) Load(ctx context.Context, id string, aggregateType string) (Aggregate, error) {
	// 애그리게이트 생성
	aggregate, err := r.aggregateFactory.CreateAggregate(aggregateType, id)
	if err != nil {
		return nil, fmt.Errorf("failed to create aggregate: %w", err)
	}

	// 이벤트 스트림 로드
	events, err := r.loadEvents(ctx, id, aggregateType)
	if err != nil {
		return nil, fmt.Errorf("failed to load events: %w", err)
	}

	// 이벤트 적용
	for _, e := range events {
		if err := aggregate.ApplyEvent(e); err != nil {
			return nil, fmt.Errorf("failed to apply event: %w", err)
		}
	}

	return aggregate, nil
}

// Save는 애그리게이트를 저장합니다.
func (r *EventSourcedRepository) Save(ctx context.Context, aggregate Aggregate) error {
	if aggregate == nil {
		return errors.New("aggregate cannot be nil")
	}

	// 커밋되지 않은 이벤트 가져오기
	events := aggregate.UncommittedEvents()
	if len(events) == 0 {
		return nil // 변경 사항 없음
	}

	// 이벤트 저장
	if err := r.saveEvents(ctx, aggregate.ID(), aggregate.Type(), events); err != nil {
		return fmt.Errorf("failed to save events: %w", err)
	}

	// 커밋되지 않은 이벤트 초기화
	aggregate.ClearUncommittedEvents()

	return nil
}

// loadEvents는 애그리게이트의 이벤트 스트림을 로드합니다.
func (r *EventSourcedRepository) loadEvents(ctx context.Context, id string, aggregateType string) ([]event.Event, error) {
	// 이벤트 스트림 로드 로직 구현
	// 실제 구현에서는 storage를 사용하여 이벤트를 로드합니다.
	return nil, nil
}

// saveEvents는 애그리게이트의 이벤트를 저장합니다.
func (r *EventSourcedRepository) saveEvents(ctx context.Context, id string, aggregateType string, events []event.Event) error {
	// 이벤트 저장 로직 구현
	// 실제 구현에서는 storage를 사용하여 이벤트를 저장합니다.
	return nil
}
