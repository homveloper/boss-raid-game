package business

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

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
		return NewValidationError("player", "does not have available transport tickets")
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

// CreateTicket은 플레이어에게 새 티켓을 생성합니다.
func (s *TicketService) CreateTicket(ctx context.Context, playerID, allianceID string, maxTickets int) error {
	// 이미 존재하는지 확인
	count, err := s.collection.CountDocuments(ctx, bson.M{"player_id": playerID})
	if err != nil {
		return err
	}

	if count > 0 {
		return NewConflictError("ticket", "player already has tickets")
	}

	// 새 티켓 생성
	now := time.Now()
	resetTime := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())

	_, err = s.collection.InsertOne(ctx, bson.M{
		"player_id":        playerID,
		"alliance_id":      allianceID,
		"current_tickets":  maxTickets,
		"max_tickets":      maxTickets,
		"last_refill_time": now,
		"purchase_count":   0,
		"reset_time":       resetTime,
		"created_at":       now,
		"updated_at":       now,
	})

	return err
}

// RefillTickets는 플레이어의 티켓을 리필합니다.
func (s *TicketService) RefillTickets(ctx context.Context, playerID string) error {
	// 티켓 정보 조회
	var ticket struct {
		MaxTickets int `bson:"max_tickets"`
	}

	err := s.collection.FindOne(ctx, bson.M{
		"player_id": playerID,
	}).Decode(&ticket)

	if err == mongo.ErrNoDocuments {
		return NewNotFoundError("ticket", playerID)
	}
	if err != nil {
		return err
	}

	// 티켓 리필
	now := time.Now()
	_, err = s.collection.UpdateOne(
		ctx,
		bson.M{"player_id": playerID},
		bson.M{
			"$set": bson.M{
				"current_tickets":  ticket.MaxTickets,
				"last_refill_time": now,
				"updated_at":       now,
			},
		},
	)

	return err
}

// ResetPurchaseCount는 플레이어의 구매 횟수를 리셋합니다.
func (s *TicketService) ResetPurchaseCount(ctx context.Context) error {
	// 현재 날짜
	now := time.Now()
	resetTime := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())

	// 모든 플레이어의 구매 횟수 리셋
	_, err := s.collection.UpdateMany(
		ctx,
		bson.M{},
		bson.M{
			"$set": bson.M{
				"purchase_count": 0,
				"reset_time":     resetTime,
				"updated_at":     now,
			},
		},
	)

	return err
}
