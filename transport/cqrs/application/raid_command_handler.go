package application

import (
	"context"
	"fmt"
	"tictactoe/transport/cqrs/domain"
	"tictactoe/transport/cqrs/infrastructure"
)

// RaidCommandHandler는 약탈 관련 커맨드를 처리하는 핸들러입니다.
type RaidCommandHandler struct {
	repository infrastructure.Repository
}

// NewRaidCommandHandler는 새로운 RaidCommandHandler를 생성합니다.
func NewRaidCommandHandler(repository infrastructure.Repository) *RaidCommandHandler {
	return &RaidCommandHandler{
		repository: repository,
	}
}

// HandleCommand는 커맨드를 처리합니다.
func (h *RaidCommandHandler) HandleCommand(ctx context.Context, command domain.Command) error {
	switch cmd := command.(type) {
	case *domain.CreateRaidCommand:
		return h.handleCreateRaid(ctx, cmd)
	case *domain.StartRaidCommand:
		return h.handleStartRaid(ctx, cmd)
	case *domain.RaidSucceedCommand:
		return h.handleRaidSucceed(ctx, cmd)
	case *domain.RaidFailCommand:
		return h.handleRaidFail(ctx, cmd)
	case *domain.CancelRaidCommand:
		return h.handleCancelRaid(ctx, cmd)
	default:
		return fmt.Errorf("unknown command type: %s", command.CommandType())
	}
}

// handleCreateRaid는 약탈 생성 커맨드를 처리합니다.
func (h *RaidCommandHandler) handleCreateRaid(
	ctx context.Context,
	cmd *domain.CreateRaidCommand,
) error {
	// 새 애그리게이트 생성
	aggregate := domain.NewRaidAggregate(cmd.AggregateID())

	// 커맨드 처리
	if err := aggregate.Create(cmd); err != nil {
		return fmt.Errorf("failed to create raid: %w", err)
	}

	// 애그리게이트 저장
	if err := h.repository.Save(ctx, aggregate); err != nil {
		return fmt.Errorf("failed to save raid: %w", err)
	}

	return nil
}

// handleStartRaid는 약탈 시작 커맨드를 처리합니다.
func (h *RaidCommandHandler) handleStartRaid(
	ctx context.Context,
	cmd *domain.StartRaidCommand,
) error {
	// 애그리게이트 로드
	aggregate, err := h.repository.Load(ctx, cmd.AggregateID(), "Raid")
	if err != nil {
		return fmt.Errorf("failed to load raid: %w", err)
	}

	// 애그리게이트 타입 변환
	raid, ok := aggregate.(*domain.RaidAggregate)
	if !ok {
		return fmt.Errorf("invalid aggregate type: %T", aggregate)
	}

	// 커맨드 처리
	if err := raid.Start(cmd); err != nil {
		return fmt.Errorf("failed to start raid: %w", err)
	}

	// 애그리게이트 저장
	if err := h.repository.Save(ctx, raid); err != nil {
		return fmt.Errorf("failed to save raid: %w", err)
	}

	return nil
}

// handleRaidSucceed는 약탈 성공 커맨드를 처리합니다.
func (h *RaidCommandHandler) handleRaidSucceed(
	ctx context.Context,
	cmd *domain.RaidSucceedCommand,
) error {
	// 애그리게이트 로드
	aggregate, err := h.repository.Load(ctx, cmd.AggregateID(), "Raid")
	if err != nil {
		return fmt.Errorf("failed to load raid: %w", err)
	}

	// 애그리게이트 타입 변환
	raid, ok := aggregate.(*domain.RaidAggregate)
	if !ok {
		return fmt.Errorf("invalid aggregate type: %T", aggregate)
	}

	// 커맨드 처리
	if err := raid.Succeed(cmd); err != nil {
		return fmt.Errorf("failed to succeed raid: %w", err)
	}

	// 애그리게이트 저장
	if err := h.repository.Save(ctx, raid); err != nil {
		return fmt.Errorf("failed to save raid: %w", err)
	}

	return nil
}

// handleRaidFail는 약탈 실패 커맨드를 처리합니다.
func (h *RaidCommandHandler) handleRaidFail(
	ctx context.Context,
	cmd *domain.RaidFailCommand,
) error {
	// 애그리게이트 로드
	aggregate, err := h.repository.Load(ctx, cmd.AggregateID(), "Raid")
	if err != nil {
		return fmt.Errorf("failed to load raid: %w", err)
	}

	// 애그리게이트 타입 변환
	raid, ok := aggregate.(*domain.RaidAggregate)
	if !ok {
		return fmt.Errorf("invalid aggregate type: %T", aggregate)
	}

	// 커맨드 처리
	if err := raid.Fail(cmd); err != nil {
		return fmt.Errorf("failed to fail raid: %w", err)
	}

	// 애그리게이트 저장
	if err := h.repository.Save(ctx, raid); err != nil {
		return fmt.Errorf("failed to save raid: %w", err)
	}

	return nil
}

// handleCancelRaid는 약탈 취소 커맨드를 처리합니다.
func (h *RaidCommandHandler) handleCancelRaid(
	ctx context.Context,
	cmd *domain.CancelRaidCommand,
) error {
	// 애그리게이트 로드
	aggregate, err := h.repository.Load(ctx, cmd.AggregateID(), "Raid")
	if err != nil {
		return fmt.Errorf("failed to load raid: %w", err)
	}

	// 애그리게이트 타입 변환
	raid, ok := aggregate.(*domain.RaidAggregate)
	if !ok {
		return fmt.Errorf("invalid aggregate type: %T", aggregate)
	}

	// 커맨드 처리
	if err := raid.Cancel(cmd); err != nil {
		return fmt.Errorf("failed to cancel raid: %w", err)
	}

	// 애그리게이트 저장
	if err := h.repository.Save(ctx, raid); err != nil {
		return fmt.Errorf("failed to save raid: %w", err)
	}

	return nil
}
