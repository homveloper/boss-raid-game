package command

import (
	"context"
	"fmt"
)

// Dispatcher는 커맨드 디스패처 인터페이스입니다.
type Dispatcher interface {
	// RegisterHandler는 커맨드 타입에 대한 핸들러를 등록합니다.
	RegisterHandler(commandType string, handler CommandHandler)

	// Dispatch는 커맨드를 디스패치합니다.
	Dispatch(ctx context.Context, command Command) error
}

// DefaultDispatcher는 기본 디스패처 구현입니다.
type DefaultDispatcher struct {
	handlers map[string]CommandHandler
}

// NewDispatcher는 새로운 DefaultDispatcher를 생성합니다.
func NewDispatcher() *DefaultDispatcher {
	return &DefaultDispatcher{
		handlers: make(map[string]CommandHandler),
	}
}

// RegisterHandler는 커맨드 타입에 대한 핸들러를 등록합니다.
func (d *DefaultDispatcher) RegisterHandler(commandType string, handler CommandHandler) {
	d.handlers[commandType] = handler
}

// Dispatch는 커맨드를 디스패치합니다.
func (d *DefaultDispatcher) Dispatch(ctx context.Context, command Command) error {
	if command == nil {
		return fmt.Errorf("command cannot be nil")
	}

	// 커맨드 타입에 대한 핸들러 찾기
	handler, ok := d.handlers[command.CommandType()]
	if !ok {
		return fmt.Errorf("no handler registered for command type %s", command.CommandType())
	}

	// 핸들러 실행
	return handler.Handle(ctx, command)
}
