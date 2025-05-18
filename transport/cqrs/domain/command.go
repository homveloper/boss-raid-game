package domain

// Command는 CQRS 패턴의 커맨드 인터페이스입니다.
// 모든 커맨드는 이 인터페이스를 구현해야 합니다.
type Command interface {
	// AggregateID는 커맨드가 대상으로 하는 애그리게이트의 ID를 반환합니다.
	AggregateID() string

	// AggregateType은 커맨드가 대상으로 하는 애그리게이트의 타입을 반환합니다.
	AggregateType() string

	// CommandType은 커맨드의 타입을 반환합니다.
	CommandType() string
}

// BaseCommand는 Command 인터페이스의 기본 구현입니다.
// 모든 커맨드는 이 구조체를 임베딩하여 사용할 수 있습니다.
type BaseCommand struct {
	ID             string `json:"id" bson:"id"`
	Type           string `json:"type" bson:"type"`
	AggregateId    string `json:"aggregate_id" bson:"aggregate_id"`
	AggregateTyped string `json:"aggregate_type" bson:"aggregate_type"`
}

// AggregateID는 커맨드가 대상으로 하는 애그리게이트의 ID를 반환합니다.
func (c *BaseCommand) AggregateID() string {
	return c.AggregateId
}

// AggregateType은 커맨드가 대상으로 하는 애그리게이트의 타입을 반환합니다.
func (c *BaseCommand) AggregateType() string {
	return c.AggregateTyped
}

// CommandType은 커맨드의 타입을 반환합니다.
func (c *BaseCommand) CommandType() string {
	return c.Type
}

// NewBaseCommand는 새로운 BaseCommand를 생성합니다.
func NewBaseCommand(id, commandType, aggregateID, aggregateType string) BaseCommand {
	return BaseCommand{
		ID:             id,
		Type:           commandType,
		AggregateId:    aggregateID,
		AggregateTyped: aggregateType,
	}
}
