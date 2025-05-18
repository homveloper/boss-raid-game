package command

import (
	"context"
	"fmt"

	"github.com/yourusername/eventsourced/pkg/aggregate"
)

// CommandHandler는 커맨드 핸들러 인터페이스입니다.
type CommandHandler interface {
	// Handle은 커맨드를 처리합니다.
	Handle(ctx context.Context, command Command) error
}

// BaseCommandHandler는 기본 커맨드 핸들러 구현입니다.
type BaseCommandHandler struct {
	repository aggregate.Repository
	handlers   map[string]CommandHandlerFunc
}

// CommandHandlerFunc는 커맨드 핸들러 함수 타입입니다.
type CommandHandlerFunc func(ctx context.Context, command Command) error

// NewCommandHandler는 새로운 BaseCommandHandler를 생성합니다.
func NewCommandHandler(repository aggregate.Repository) *BaseCommandHandler {
	return &BaseCommandHandler{
		repository: repository,
		handlers:   make(map[string]CommandHandlerFunc),
	}
}

// RegisterHandler는 커맨드 타입에 대한 핸들러 함수를 등록합니다.
func (h *BaseCommandHandler) RegisterHandler(commandType string, handler CommandHandlerFunc) {
	h.handlers[commandType] = handler
}

// Handle은 커맨드를 처리합니다.
func (h *BaseCommandHandler) Handle(ctx context.Context, command Command) error {
	if command == nil {
		return fmt.Errorf("command cannot be nil")
	}

	// 커맨드 타입에 대한 핸들러 찾기
	handler, ok := h.handlers[command.CommandType()]
	if !ok {
		return fmt.Errorf("no handler registered for command type %s", command.CommandType())
	}

	// 핸들러 실행
	return handler(ctx, command)
}

// LoadAggregate는 애그리게이트를 로드합니다.
func (h *BaseCommandHandler) LoadAggregate(ctx context.Context, id string, aggregateType string) (aggregate.Aggregate, error) {
	return h.repository.Load(ctx, id, aggregateType)
}

// SaveAggregate는 애그리게이트를 저장합니다.
func (h *BaseCommandHandler) SaveAggregate(ctx context.Context, aggregate aggregate.Aggregate) error {
	return h.repository.Save(ctx, aggregate)
}
