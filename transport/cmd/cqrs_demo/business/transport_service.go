package business

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"tictactoe/transport/cqrs/application"
	"tictactoe/transport/cqrs/domain"
	"tictactoe/transport/cqrs/infrastructure"
)

// TransportService는 이송 관련 비즈니스 로직을 처리하는 서비스입니다.
type TransportService struct {
	commandBus    infrastructure.CommandBus
	queryStore    *mongo.Collection
	guildService  *GuildService
	ticketService *TicketService
}

// NewTransportService는 새로운 TransportService를 생성합니다.
func NewTransportService(
	commandBus infrastructure.CommandBus,
	queryStore *mongo.Collection,
	guildService *GuildService,
	ticketService *TicketService,
) *TransportService {
	return &TransportService{
		commandBus:    commandBus,
		queryStore:    queryStore,
		guildService:  guildService,
		ticketService: ticketService,
	}
}

// CreateTransportParams는 이송 생성 파라미터입니다.
type CreateTransportParams struct {
	AllianceID      string `json:"alliance_id"`
	PlayerID        string `json:"player_id"`
	PlayerName      string `json:"player_name"`
	MineID          string `json:"mine_id"`
	MineName        string `json:"mine_name"`
	MineLevel       int    `json:"mine_level"`
	GeneralID       string `json:"general_id"`
	GoldAmount      int    `json:"gold_amount"`
	MaxParticipants int    `json:"max_participants"`
	PrepTime        int    `json:"prep_time"`
	TransportTime   int    `json:"transport_time"`
}

// CreateTransportResult는 이송 생성 결과입니다.
type CreateTransportResult struct {
	ID string `json:"id"`
}

// CreateTransport는 새로운 이송을 생성합니다.
func (s *TransportService) CreateTransport(ctx context.Context, params CreateTransportParams) (*CreateTransportResult, error) {
	// 입력 유효성 검증
	if params.AllianceID == "" {
		return nil, NewValidationError("alliance_id", "must not be empty")
	}
	if params.PlayerID == "" {
		return nil, NewValidationError("player_id", "must not be empty")
	}
	if params.MineID == "" {
		return nil, NewValidationError("mine_id", "must not be empty")
	}
	if params.GoldAmount <= 0 {
		return nil, NewValidationError("gold_amount", "must be positive")
	}
	if params.MaxParticipants <= 0 {
		return nil, NewValidationError("max_participants", "must be positive")
	}
	if params.PrepTime <= 0 {
		return nil, NewValidationError("prep_time", "must be positive")
	}
	if params.TransportTime <= 0 {
		return nil, NewValidationError("transport_time", "must be positive")
	}

	// 비즈니스 규칙 검증
	// 1. 길드 가입 여부 확인
	isMember, err := s.guildService.IsGuildMember(ctx, params.PlayerID, params.AllianceID)
	if err != nil {
		return nil, NewServerError("failed to check guild membership", err)
	}
	if !isMember {
		return nil, NewAuthorizationError("player is not a member of the alliance")
	}

	// 2. 티켓 보유 여부 확인
	hasTicket, err := s.ticketService.HasAvailableTicket(ctx, params.PlayerID)
	if err != nil {
		return nil, NewServerError("failed to check ticket availability", err)
	}
	if !hasTicket {
		return nil, NewValidationError("player", "does not have available transport tickets")
	}

	// 3. 대기 중인 이송 확인
	hasPendingTransport, err := s.hasPendingTransport(ctx, params.PlayerID)
	if err != nil {
		return nil, NewServerError("failed to check pending transports", err)
	}
	if hasPendingTransport {
		return nil, NewConflictError("transport", "player already has a pending transport")
	}

	// 4. 티켓 사용
	if err := s.ticketService.UseTicket(ctx, params.PlayerID); err != nil {
		return nil, NewServerError("failed to use transport ticket", err)
	}

	// 5. 새 ID 생성
	id := generateID()

	// 6. 커맨드 생성
	cmd := domain.NewCreateTransportCommand(
		id,
		params.AllianceID,
		params.PlayerID,
		params.PlayerName,
		params.MineID,
		params.MineName,
		params.MineLevel,
		params.GeneralID,
		params.GoldAmount,
		params.MaxParticipants,
		params.PrepTime,
		params.TransportTime,
	)

	// 7. 커맨드 전송
	if err := s.commandBus.Dispatch(ctx, cmd); err != nil {
		// 실패 시 티켓 환불 (보상 트랜잭션)
		if refundErr := s.ticketService.RefundTicket(ctx, params.PlayerID); refundErr != nil {
			// 환불 실패 로깅
			fmt.Printf("Failed to refund ticket: %v\n", refundErr)
		}
		return nil, NewServerError("failed to create transport", err)
	}

	return &CreateTransportResult{ID: id}, nil
}

// JoinTransportParams는 이송 참가 파라미터입니다.
type JoinTransportParams struct {
	TransportID string `json:"transport_id"`
	PlayerID    string `json:"player_id"`
	PlayerName  string `json:"player_name"`
	GoldAmount  int    `json:"gold_amount"`
}

// JoinTransportResult는 이송 참가 결과입니다.
type JoinTransportResult struct {
	Status string `json:"status"`
	ID     string `json:"id"`
}

// JoinTransport는 이송에 참가합니다.
func (s *TransportService) JoinTransport(ctx context.Context, params JoinTransportParams) (*JoinTransportResult, error) {
	// 입력 유효성 검증
	if params.TransportID == "" {
		return nil, NewValidationError("transport_id", "must not be empty")
	}
	if params.PlayerID == "" {
		return nil, NewValidationError("player_id", "must not be empty")
	}
	if params.GoldAmount <= 0 {
		return nil, NewValidationError("gold_amount", "must be positive")
	}

	// 이송 정보 조회
	var transport application.TransportReadModel
	if err := s.queryStore.FindOne(ctx, bson.M{"_id": params.TransportID}).Decode(&transport); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, NewNotFoundError("transport", params.TransportID)
		}
		return nil, NewServerError("failed to fetch transport", err)
	}

	// 이송 상태 확인
	if transport.Status != "PREPARING" {
		return nil, NewConflictError("transport", "transport is not in preparation phase")
	}

	// 참가자 수 확인
	if len(transport.Participants) >= transport.MaxParticipants {
		return nil, NewConflictError("transport", "transport is full")
	}

	// 이미 참가 중인지 확인
	for _, p := range transport.Participants {
		if p.PlayerID == params.PlayerID {
			return nil, NewConflictError("transport", "player is already participating in this transport")
		}
	}

	// 티켓 보유 여부 확인
	hasTicket, err := s.ticketService.HasAvailableTicket(ctx, params.PlayerID)
	if err != nil {
		return nil, NewServerError("failed to check ticket availability", err)
	}
	if !hasTicket {
		return nil, NewValidationError("player", "does not have available transport tickets")
	}

	// 티켓 사용
	if err := s.ticketService.UseTicket(ctx, params.PlayerID); err != nil {
		return nil, NewServerError("failed to use transport ticket", err)
	}

	// 커맨드 생성
	cmd := domain.NewAddParticipantCommand(
		params.TransportID,
		params.PlayerID,
		params.PlayerName,
		params.GoldAmount,
		time.Now(),
	)

	// 커맨드 전송
	if err := s.commandBus.Dispatch(ctx, cmd); err != nil {
		// 실패 시 티켓 환불 (보상 트랜잭션)
		if refundErr := s.ticketService.RefundTicket(ctx, params.PlayerID); refundErr != nil {
			// 환불 실패 로깅
			fmt.Printf("Failed to refund ticket: %v\n", refundErr)
		}
		return nil, NewServerError("failed to join transport", err)
	}

	// 참가 후 이송 정보 다시 조회
	if err := s.queryStore.FindOne(ctx, bson.M{"_id": params.TransportID}).Decode(&transport); err != nil {
		// 이미 커맨드는 성공했으므로 에러만 로깅
		fmt.Printf("Failed to fetch updated transport: %v\n", err)
	} else {
		// 참가자 수가 최대에 도달했는지 확인
		if len(transport.Participants) >= transport.MaxParticipants {
			// 이송 시작 커맨드 생성
			now := time.Now()
			estimatedArrivalTime := now.Add(time.Duration(60) * time.Minute) // 기본값 60분
			startCmd := domain.NewStartTransportCommand(
				params.TransportID,
				now,
				estimatedArrivalTime,
			)

			// 커맨드 전송 (비동기로 처리)
			go func() {
				ctx := context.Background()
				if err := s.commandBus.Dispatch(ctx, startCmd); err != nil {
					fmt.Printf("Failed to start transport automatically: %v\n", err)
				} else {
					fmt.Printf("Transport %s started automatically after reaching max participants\n", params.TransportID)
				}
			}()
		}
	}

	return &JoinTransportResult{
		Status: "joined",
		ID:     params.TransportID,
	}, nil
}

// StartTransportParams는 이송 시작 파라미터입니다.
type StartTransportParams struct {
	TransportID string `json:"transport_id"`
}

// StartTransportResult는 이송 시작 결과입니다.
type StartTransportResult struct {
	Status string `json:"status"`
}

// StartTransport는 이송을 시작합니다.
func (s *TransportService) StartTransport(ctx context.Context, params StartTransportParams) (*StartTransportResult, error) {
	// 입력 유효성 검증
	if params.TransportID == "" {
		return nil, NewValidationError("transport_id", "must not be empty")
	}

	// 이송 정보 조회
	var transport application.TransportReadModel
	if err := s.queryStore.FindOne(ctx, bson.M{"_id": params.TransportID}).Decode(&transport); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, NewNotFoundError("transport", params.TransportID)
		}
		return nil, NewServerError("failed to fetch transport", err)
	}

	// 이송 상태 확인
	if transport.Status != "PREPARING" {
		return nil, NewConflictError("transport", "transport is not in preparation phase")
	}

	// 현재 시간
	now := time.Now()

	// 예상 도착 시간 계산 (기본값 60분)
	estimatedArrivalTime := now.Add(time.Duration(60) * time.Minute)

	// 커맨드 생성
	cmd := domain.NewStartTransportCommand(
		params.TransportID,
		now,
		estimatedArrivalTime,
	)

	// 커맨드 전송
	if err := s.commandBus.Dispatch(ctx, cmd); err != nil {
		return nil, NewServerError("failed to start transport", err)
	}

	return &StartTransportResult{Status: "started"}, nil
}

// GetTransportParams는 이송 조회 파라미터입니다.
type GetTransportParams struct {
	TransportID string `json:"transport_id"`
}

// GetTransport는 이송 정보를 조회합니다.
func (s *TransportService) GetTransport(ctx context.Context, params GetTransportParams) (*application.TransportReadModel, error) {
	// 입력 유효성 검증
	if params.TransportID == "" {
		return nil, NewValidationError("transport_id", "must not be empty")
	}

	// 이송 정보 조회
	var transport application.TransportReadModel
	if err := s.queryStore.FindOne(ctx, bson.M{"_id": params.TransportID}).Decode(&transport); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, NewNotFoundError("transport", params.TransportID)
		}
		return nil, NewServerError("failed to fetch transport", err)
	}

	return &transport, nil
}

// GetActiveTransportsParams는 활성 이송 목록 조회 파라미터입니다.
type GetActiveTransportsParams struct {
	AllianceID string `json:"alliance_id"`
}

// GetActiveTransports는 활성 이송 목록을 조회합니다.
func (s *TransportService) GetActiveTransports(ctx context.Context, params GetActiveTransportsParams) ([]application.TransportReadModel, error) {
	// 입력 유효성 검증
	if params.AllianceID == "" {
		return nil, NewValidationError("alliance_id", "must not be empty")
	}

	// 활성 상태 이송 조회
	cursor, err := s.queryStore.Find(ctx, bson.M{
		"alliance_id": params.AllianceID,
		"status": bson.M{
			"$in": []string{"PREPARING", "IN_TRANSIT"},
		},
	})
	if err != nil {
		return nil, NewServerError("failed to fetch active transports", err)
	}
	defer cursor.Close(ctx)

	// 결과 변환
	var transports []application.TransportReadModel
	if err := cursor.All(ctx, &transports); err != nil {
		return nil, NewServerError("failed to decode transports", err)
	}

	return transports, nil
}

// RaidTransportParams는 이송 약탈 파라미터입니다.
type RaidTransportParams struct {
	TransportID string `json:"transport_id"`
	RaiderID    string `json:"raider_id"`
	RaiderName  string `json:"raider_name"`
}

// RaidTransportResult는 이송 약탈 결과입니다.
type RaidTransportResult struct {
	Status string `json:"status"`
	RaidID string `json:"raid_id"`
}

// RaidTransport는 이송을 약탈합니다.
func (s *TransportService) RaidTransport(ctx context.Context, params RaidTransportParams) (*RaidTransportResult, error) {
	// 입력 유효성 검증
	if params.TransportID == "" {
		return nil, NewValidationError("transport_id", "must not be empty")
	}
	if params.RaiderID == "" {
		return nil, NewValidationError("raider_id", "must not be empty")
	}

	// 이송 정보 조회
	var transport application.TransportReadModel
	if err := s.queryStore.FindOne(ctx, bson.M{"_id": params.TransportID}).Decode(&transport); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, NewNotFoundError("transport", params.TransportID)
		}
		return nil, NewServerError("failed to fetch transport", err)
	}

	// 이송 상태 확인
	if transport.Status != "IN_TRANSIT" {
		return nil, NewConflictError("transport", "transport cannot be raided in its current state")
	}

	// 이미 약탈 중인지 확인
	if transport.RaidInfo != nil {
		return nil, NewConflictError("transport", "transport is already being raided")
	}

	// 새 약탈 ID 생성
	raidID := generateID()

	// 현재 시간
	now := time.Now()

	// 방어 종료 시간 (30분 후)
	defenseEndTime := now.Add(30 * time.Minute)

	// 커맨드 생성
	cmd := domain.NewRaidTransportCommand(
		params.TransportID,
		raidID,
		params.RaiderID,
		params.RaiderName,
		now,
		defenseEndTime,
	)

	// 커맨드 전송
	if err := s.commandBus.Dispatch(ctx, cmd); err != nil {
		return nil, NewServerError("failed to raid transport", err)
	}

	return &RaidTransportResult{
		Status: "raiding",
		RaidID: raidID,
	}, nil
}

// DefendTransportParams는 이송 방어 파라미터입니다.
type DefendTransportParams struct {
	TransportID  string `json:"transport_id"`
	DefenderID   string `json:"defender_id"`
	DefenderName string `json:"defender_name"`
	Successful   bool   `json:"successful"`
}

// DefendTransportResult는 이송 방어 결과입니다.
type DefendTransportResult struct {
	Status string `json:"status"`
}

// DefendTransport는 이송을 방어합니다.
func (s *TransportService) DefendTransport(ctx context.Context, params DefendTransportParams) (*DefendTransportResult, error) {
	// 입력 유효성 검증
	if params.TransportID == "" {
		return nil, NewValidationError("transport_id", "must not be empty")
	}
	if params.DefenderID == "" {
		return nil, NewValidationError("defender_id", "must not be empty")
	}

	// 이송 정보 조회
	var transport application.TransportReadModel
	if err := s.queryStore.FindOne(ctx, bson.M{"_id": params.TransportID}).Decode(&transport); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, NewNotFoundError("transport", params.TransportID)
		}
		return nil, NewServerError("failed to fetch transport", err)
	}

	// 약탈 중인지 확인
	if transport.RaidInfo == nil {
		return nil, NewConflictError("transport", "transport is not being raided")
	}

	// 이미 방어되었는지 확인
	if transport.RaidInfo.IsDefended {
		return nil, NewConflictError("transport", "raid has already been defended")
	}

	// 방어 시간이 만료되었는지 확인
	now := time.Now()
	if now.After(transport.RaidInfo.DefenseEndTime) {
		return nil, NewConflictError("transport", "defense window has expired")
	}

	// 골드 손실량 계산 (방어 실패 시 30%)
	goldAmountLost := 0
	if !params.Successful {
		goldAmountLost = int(float64(transport.GoldAmount) * 0.3)
	}

	// 커맨드 생성
	cmd := domain.NewDefendTransportCommand(
		params.TransportID,
		params.DefenderID,
		params.DefenderName,
		params.Successful,
		now,
		goldAmountLost,
	)

	// 커맨드 전송
	if err := s.commandBus.Dispatch(ctx, cmd); err != nil {
		return nil, NewServerError("failed to defend transport", err)
	}

	return &DefendTransportResult{Status: "defended"}, nil
}

// hasPendingTransport는 플레이어가 대기 중인 이송이 있는지 확인합니다.
func (s *TransportService) hasPendingTransport(ctx context.Context, playerID string) (bool, error) {
	// 대기 중인 이송 조회
	count, err := s.queryStore.CountDocuments(ctx, bson.M{
		"player_id": playerID,
		"status":    "PREPARING",
	})
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// generateID는 고유 ID를 생성합니다.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
