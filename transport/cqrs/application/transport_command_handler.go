package application

import (
	"context"
	"fmt"
	"tictactoe/transport/cqrs/domain"
	"tictactoe/transport/cqrs/infrastructure"
)

// TransportCommandHandler는 이송 관련 커맨드를 처리하는 핸들러입니다.
type TransportCommandHandler struct {
	repository infrastructure.Repository
}

// NewTransportCommandHandler는 새로운 TransportCommandHandler를 생성합니다.
func NewTransportCommandHandler(repository infrastructure.Repository) *TransportCommandHandler {
	return &TransportCommandHandler{
		repository: repository,
	}
}

// HandleCommand는 커맨드를 처리합니다.
func (h *TransportCommandHandler) HandleCommand(ctx context.Context, command domain.Command) error {
	switch cmd := command.(type) {
	case *domain.CreateTransportCommand:
		return h.handleCreateTransport(ctx, cmd)
	case *domain.StartTransportCommand:
		return h.handleStartTransport(ctx, cmd)
	case *domain.CompleteTransportCommand:
		return h.handleCompleteTransport(ctx, cmd)
	case *domain.RaidTransportCommand:
		return h.handleRaidTransport(ctx, cmd)
	case *domain.DefendTransportCommand:
		return h.handleDefendTransport(ctx, cmd)
	case *domain.AddParticipantCommand:
		return h.handleAddParticipant(ctx, cmd)
	default:
		return fmt.Errorf("unknown command type: %s", command.CommandType())
	}
}

// handleCreateTransport는 이송 생성 커맨드를 처리합니다.
func (h *TransportCommandHandler) handleCreateTransport(
	ctx context.Context,
	cmd *domain.CreateTransportCommand,
) error {
	// 새 애그리게이트 생성
	aggregate := domain.NewTransportAggregate(cmd.AggregateID())

	// 커맨드 처리
	if err := aggregate.Create(cmd); err != nil {
		return fmt.Errorf("failed to create transport: %w", err)
	}

	// 애그리게이트 저장
	if err := h.repository.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save transport: %w", err)
	}

	return nil
}

// handleStartTransport는 이송 시작 커맨드를 처리합니다.
func (h *TransportCommandHandler) handleStartTransport(
	ctx context.Context,
	cmd *domain.StartTransportCommand,
) error {
	// 애그리게이트 로드
	aggregate, err := h.repository.Load(ctx, cmd.AggregateID(), "Transport")
	if err != nil {
		return fmt.Errorf("failed to load transport: %w", err)
	}

	// 애그리게이트 타입 변환
	transport, ok := aggregate.(*domain.TransportAggregate)
	if !ok {
		return fmt.Errorf("invalid aggregate type: %T", aggregate)
	}

	// 커맨드 처리
	if err := transport.Start(cmd); err != nil {
		return fmt.Errorf("failed to start transport: %w", err)
	}

	// 애그리게이트 저장
	if err := h.repository.Save(ctx, transport); err != nil {
		return fmt.Errorf("failed to save transport: %w", err)
	}

	return nil
}

// handleCompleteTransport는 이송 완료 커맨드를 처리합니다.
func (h *TransportCommandHandler) handleCompleteTransport(
	ctx context.Context,
	cmd *domain.CompleteTransportCommand,
) error {
	// 애그리게이트 로드
	aggregate, err := h.repository.Load(ctx, cmd.AggregateID(), "Transport")
	if err != nil {
		return fmt.Errorf("failed to load transport: %w", err)
	}

	// 애그리게이트 타입 변환
	transport, ok := aggregate.(*domain.TransportAggregate)
	if !ok {
		return fmt.Errorf("invalid aggregate type: %T", aggregate)
	}

	// 커맨드 처리
	if err := transport.Complete(cmd); err != nil {
		return fmt.Errorf("failed to complete transport: %w", err)
	}

	// 애그리게이트 저장
	if err := h.repository.Save(ctx, transport); err != nil {
		return fmt.Errorf("failed to save transport: %w", err)
	}

	return nil
}

// handleRaidTransport는 이송 약탈 커맨드를 처리합니다.
func (h *TransportCommandHandler) handleRaidTransport(
	ctx context.Context,
	cmd *domain.RaidTransportCommand,
) error {
	// 애그리게이트 로드
	aggregate, err := h.repository.Load(ctx, cmd.AggregateID(), "Transport")
	if err != nil {
		return fmt.Errorf("failed to load transport: %w", err)
	}

	// 애그리게이트 타입 변환
	transport, ok := aggregate.(*domain.TransportAggregate)
	if !ok {
		return fmt.Errorf("invalid aggregate type: %T", aggregate)
	}

	// 커맨드 처리
	if err := transport.Raid(cmd); err != nil {
		return fmt.Errorf("failed to raid transport: %w", err)
	}

	// 애그리게이트 저장
	if err := h.repository.Save(ctx, transport); err != nil {
		return fmt.Errorf("failed to save transport: %w", err)
	}

	return nil
}

// handleDefendTransport는 이송 방어 커맨드를 처리합니다.
func (h *TransportCommandHandler) handleDefendTransport(
	ctx context.Context,
	cmd *domain.DefendTransportCommand,
) error {
	// 애그리게이트 로드
	aggregate, err := h.repository.Load(ctx, cmd.AggregateID(), "Transport")
	if err != nil {
		return fmt.Errorf("failed to load transport: %w", err)
	}

	// 애그리게이트 타입 변환
	transport, ok := aggregate.(*domain.TransportAggregate)
	if !ok {
		return fmt.Errorf("invalid aggregate type: %T", aggregate)
	}

	// 커맨드 처리
	if err := transport.Defend(cmd); err != nil {
		return fmt.Errorf("failed to defend transport: %w", err)
	}

	// 애그리게이트 저장
	if err := h.repository.Save(ctx, transport); err != nil {
		return fmt.Errorf("failed to save transport: %w", err)
	}

	return nil
}

// handleAddParticipant는 이송 참여자 추가 커맨드를 처리합니다.
func (h *TransportCommandHandler) handleAddParticipant(
	ctx context.Context,
	cmd *domain.AddParticipantCommand,
) error {
	// 애그리게이트 로드
	aggregate, err := h.repository.Load(ctx, cmd.AggregateID(), "Transport")
	if err != nil {
		return fmt.Errorf("failed to load transport: %w", err)
	}

	// 애그리게이트 타입 변환
	transport, ok := aggregate.(*domain.TransportAggregate)
	if !ok {
		return fmt.Errorf("invalid aggregate type: %T", aggregate)
	}

	// 참여자 추가 로직 구현 필요
	// TODO: 참여자 추가 메서드 구현

	// 애그리게이트 저장
	if err := h.repository.Save(ctx, transport); err != nil {
		return fmt.Errorf("failed to save transport: %w", err)
	}

	return nil
}
