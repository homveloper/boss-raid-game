package command

import (
	"fmt"
)

// Command는 커맨드 인터페이스입니다.
type Command interface {
	// CommandType은 커맨드 타입을 반환합니다.
	CommandType() string

	// AggregateID는 커맨드가 대상으로 하는 애그리게이트 ID를 반환합니다.
	AggregateID() string

	// AggregateType은 커맨드가 대상으로 하는 애그리게이트 타입을 반환합니다.
	AggregateType() string

	// Payload는 커맨드 페이로드를 반환합니다.
	Payload() interface{}
}

// BaseCommand는 기본 커맨드 구현입니다.
type BaseCommand struct {
	Type          string
	ID            string
	AggrType      string
	PayloadData   interface{}
}

// NewCommand는 새로운 BaseCommand를 생성합니다.
func NewCommand(commandType string, aggregateID string, payload interface{}) *BaseCommand {
	return &BaseCommand{
		Type:        commandType,
		ID:          aggregateID,
		PayloadData: payload,
	}
}

// NewCommandWithType은 애그리게이트 타입이 지정된 새로운 BaseCommand를 생성합니다.
func NewCommandWithType(commandType string, aggregateID string, aggregateType string, payload interface{}) *BaseCommand {
	return &BaseCommand{
		Type:        commandType,
		ID:          aggregateID,
		AggrType:    aggregateType,
		PayloadData: payload,
	}
}

// CommandType은 커맨드 타입을 반환합니다.
func (c *BaseCommand) CommandType() string {
	return c.Type
}

// AggregateID는 커맨드가 대상으로 하는 애그리게이트 ID를 반환합니다.
func (c *BaseCommand) AggregateID() string {
	return c.ID
}

// AggregateType은 커맨드가 대상으로 하는 애그리게이트 타입을 반환합니다.
func (c *BaseCommand) AggregateType() string {
	return c.AggrType
}

// Payload는 커맨드 페이로드를 반환합니다.
func (c *BaseCommand) Payload() interface{} {
	return c.PayloadData
}

// String은 커맨드의 문자열 표현을 반환합니다.
func (c *BaseCommand) String() string {
	return fmt.Sprintf("Command[Type=%s, AggregateID=%s, AggregateType=%s]", c.Type, c.ID, c.AggrType)
}
