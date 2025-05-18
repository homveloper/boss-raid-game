package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// GuildService는 길드 관련 서비스를 제공합니다.
type GuildService struct {
	collection *mongo.Collection
}

// NewGuildService는 새로운 GuildService를 생성합니다.
func NewGuildService(collection *mongo.Collection) *GuildService {
	return &GuildService{
		collection: collection,
	}
}

// IsGuildMember는 플레이어가 길드의 멤버인지 확인합니다.
func (s *GuildService) IsGuildMember(ctx context.Context, playerID, allianceID string) (bool, error) {
	// 길드 멤버십 조회
	count, err := s.collection.CountDocuments(ctx, bson.M{
		"alliance_id": allianceID,
		"members": bson.M{
			"$elemMatch": bson.M{
				"player_id": playerID,
				"status":    "ACTIVE", // 활성 상태인 멤버만 확인
			},
		},
	})
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// TicketService는 이송 티켓 관련 서비스를 제공합니다.
type TicketService struct {
	collection *mongo.Collection
}

// NewTicketService는 새로운 TicketService를 생성합니다.
func NewTicketService(collection *mongo.Collection) *TicketService {
	return &TicketService{
		collection: collection,
	}
}

// HasAvailableTicket은 플레이어가 사용 가능한 티켓을 보유하고 있는지 확인합니다.
func (s *TicketService) HasAvailableTicket(ctx context.Context, playerID string) (bool, error) {
	// 티켓 정보 조회
	var ticket struct {
		CurrentTickets int `bson:"current_tickets"`
	}

	err := s.collection.FindOne(ctx, bson.M{
		"player_id": playerID,
	}).Decode(&ticket)

	if err == mongo.ErrNoDocuments {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return ticket.CurrentTickets > 0, nil
}

// UseTicket은 플레이어의 티켓을 사용합니다.
func (s *TicketService) UseTicket(ctx context.Context, playerID string) error {
	// 티켓 사용
	result, err := s.collection.UpdateOne(
		ctx,
		bson.M{
			"player_id":       playerID,
			"current_tickets": bson.M{"$gt": 0},
		},
		bson.M{
			"$inc": bson.M{"current_tickets": -1},
			"$set": bson.M{"updated_at": time.Now()},
		},
	)

	if err != nil {
		return err
	}

	if result.ModifiedCount == 0 {
		return fmt.Errorf("no available tickets")
	}

	return nil
}

// RefundTicket은 플레이어에게 티켓을 환불합니다.
func (s *TicketService) RefundTicket(ctx context.Context, playerID string) error {
	// 티켓 환불
	_, err := s.collection.UpdateOne(
		ctx,
		bson.M{"player_id": playerID},
		bson.M{
			"$inc": bson.M{"current_tickets": 1},
			"$set": bson.M{"updated_at": time.Now()},
		},
	)

	return err
}

// BusinessRuleService는 비즈니스 규칙 검증 서비스를 제공합니다.
type BusinessRuleService struct {
	guildService  *GuildService
	ticketService *TicketService
	queryStore    *mongo.Collection
}

// NewBusinessRuleService는 새로운 BusinessRuleService를 생성합니다.
func NewBusinessRuleService(
	guildService *GuildService,
	ticketService *TicketService,
	queryStore *mongo.Collection,
) *BusinessRuleService {
	return &BusinessRuleService{
		guildService:  guildService,
		ticketService: ticketService,
		queryStore:    queryStore,
	}
}

// ValidateTransportCreation은 이송 생성 규칙을 검증합니다.
func (s *BusinessRuleService) ValidateTransportCreation(
	ctx context.Context,
	playerID, allianceID string,
) error {
	// 1. 길드 가입 여부 확인
	isMember, err := s.guildService.IsGuildMember(ctx, playerID, allianceID)
	if err != nil {
		return fmt.Errorf("failed to check guild membership: %w", err)
	}
	if !isMember {
		return fmt.Errorf("player is not a member of the alliance")
	}

	// 2. 티켓 보유 여부 확인
	hasTicket, err := s.ticketService.HasAvailableTicket(ctx, playerID)
	if err != nil {
		return fmt.Errorf("failed to check ticket availability: %w", err)
	}
	if !hasTicket {
		return fmt.Errorf("player does not have available transport tickets")
	}

	// 3. 대기 중인 이송 확인
	hasPendingTransport, err := s.hasPendingTransport(ctx, playerID)
	if err != nil {
		return fmt.Errorf("failed to check pending transports: %w", err)
	}
	if hasPendingTransport {
		return fmt.Errorf("player already has a pending transport")
	}

	return nil
}

// hasPendingTransport는 플레이어가 대기 중인 이송이 있는지 확인합니다.
func (s *BusinessRuleService) hasPendingTransport(ctx context.Context, playerID string) (bool, error) {
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

// EnhancedTransportService는 비즈니스 규칙이 적용된 이송 서비스입니다.
type EnhancedTransportService struct {
	*TransportService
	businessRuleService *BusinessRuleService
	ticketService       *TicketService
}

// NewEnhancedTransportService는 새로운 EnhancedTransportService를 생성합니다.
func NewEnhancedTransportService(
	transportService *TransportService,
	businessRuleService *BusinessRuleService,
	ticketService *TicketService,
) *EnhancedTransportService {
	return &EnhancedTransportService{
		TransportService:    transportService,
		businessRuleService: businessRuleService,
		ticketService:       ticketService,
	}
}

// CreateTransportHandler는 비즈니스 규칙이 적용된 이송 생성 핸들러입니다.
func (s *EnhancedTransportService) CreateTransportHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateTransportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 비즈니스 규칙 검증
	if err := s.businessRuleService.ValidateTransportCreation(
		r.Context(),
		req.PlayerID,
		req.AllianceID,
	); err != nil {
		// 에러 타입에 따른 HTTP 상태 코드 설정
		switch {
		case strings.Contains(err.Error(), "not a member"):
			http.Error(w, err.Error(), http.StatusForbidden)
		case strings.Contains(err.Error(), "does not have available"):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case strings.Contains(err.Error(), "already has a pending"):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// 티켓 사용
	if err := s.ticketService.UseTicket(r.Context(), req.PlayerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 이송 생성 (기존 핸들러 호출)
	s.TransportService.CreateTransportHandler(w, r)
}
