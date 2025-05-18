package infrastructure

import (
	"context"
	"fmt"
	"sync"
	"tictactoe/transport/cqrs/domain"
)

// CommandHandler는 커맨드를 처리하는 핸들러 인터페이스입니다.
type CommandHandler interface {
	// HandleCommand는 커맨드를 처리합니다.
	HandleCommand(ctx context.Context, command domain.Command) error
}

// CommandBus는 커맨드를 전달하는 인터페이스입니다.
type CommandBus interface {
	// Dispatch는 커맨드를 전달합니다.
	Dispatch(ctx context.Context, command domain.Command) error

	// RegisterHandler는 커맨드 타입에 대한 핸들러를 등록합니다.
	RegisterHandler(commandType string, handler CommandHandler) error
}

// InMemoryCommandBus는 메모리 내에서 동작하는 CommandBus 구현체입니다.
type InMemoryCommandBus struct {
	handlers map[string]CommandHandler
	mu       sync.RWMutex
}

// NewInMemoryCommandBus는 새로운 InMemoryCommandBus를 생성합니다.
func NewInMemoryCommandBus() *InMemoryCommandBus {
	return &InMemoryCommandBus{
		handlers: make(map[string]CommandHandler),
	}
}

// Dispatch는 커맨드를 전달합니다.
func (b *InMemoryCommandBus) Dispatch(ctx context.Context, command domain.Command) error {
	b.mu.RLock()
	handler, ok := b.handlers[command.CommandType()]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no handler registered for command type: %s", command.CommandType())
	}

	return handler.HandleCommand(ctx, command)
}

// RegisterHandler는 커맨드 타입에 대한 핸들러를 등록합니다.
func (b *InMemoryCommandBus) RegisterHandler(commandType string, handler CommandHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.handlers[commandType]; ok {
		return fmt.Errorf("handler already registered for command type: %s", commandType)
	}

	b.handlers[commandType] = handler
	return nil
}
