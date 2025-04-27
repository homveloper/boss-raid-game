package transport

import (
	"context"
	"fmt"
	"time"

	v2 "nodestorage/v2"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TicketService provides operations for managing transport tickets
type TicketService struct {
	storage v2.Storage[*TransportTicket]
}

// NewTicketService creates a new TicketService
func NewTicketService(storage v2.Storage[*TransportTicket]) *TicketService {
	return &TicketService{
		storage: storage,
	}
}

// GetOrCreateTickets gets or creates transport tickets for a player
func (s *TicketService) GetOrCreateTickets(
	ctx context.Context,
	playerID primitive.ObjectID,
	allianceID primitive.ObjectID,
	maxTickets int,
) (*TransportTicket, error) {
	// Try to find existing tickets
	tickets, err := s.storage.FindMany(ctx, bson.M{"player_id": playerID})
	if err != nil {
		return nil, err
	}

	// If tickets exist, return them
	if len(tickets) > 0 {
		// Check if tickets need to be refilled
		ticket := tickets[0]
		return s.checkAndRefillTickets(ctx, ticket)
	}

	// Create new tickets
	now := time.Now()
	ticket := &TransportTicket{
		ID:             primitive.NewObjectID(),
		PlayerID:       playerID,
		AllianceID:     allianceID,
		CurrentTickets: maxTickets, // Start with max tickets
		MaxTickets:     maxTickets,
		LastRefillTime: now,
		PurchaseCount:  0,
		LastPurchaseAt: nil,
		ResetTime:      getNextResetTime(now),
		CreatedAt:      now,
		UpdatedAt:      now,
		VectorClock:    1, // Set initial version
	}

	return s.storage.FindOneAndUpsert(ctx, ticket)
}

// UseTicket uses a transport ticket
func (s *TicketService) UseTicket(ctx context.Context, playerID primitive.ObjectID) (*TransportTicket, error) {
	// Get player's tickets
	tickets, err := s.storage.FindMany(ctx, bson.M{"player_id": playerID})
	if err != nil {
		return nil, err
	}

	if len(tickets) == 0 {
		return nil, fmt.Errorf("player has no transport tickets")
	}

	// Check and refill tickets if needed
	ticket, err := s.checkAndRefillTickets(ctx, tickets[0])
	if err != nil {
		return nil, err
	}

	// Use a ticket
	if ticket.CurrentTickets <= 0 {
		return nil, fmt.Errorf("no transport tickets available")
	}

	ticket, _, err = s.storage.FindOneAndUpdate(ctx, ticket.ID, func(t *TransportTicket) (*TransportTicket, error) {
		t.CurrentTickets--
		t.UpdatedAt = time.Now()
		return t, nil
	})

	return ticket, err
}

// PurchaseTicket purchases a transport ticket
func (s *TicketService) PurchaseTicket(ctx context.Context, playerID primitive.ObjectID) (*TransportTicket, int, error) {
	// Get player's tickets
	tickets, err := s.storage.FindMany(ctx, bson.M{"player_id": playerID})
	if err != nil {
		return nil, 0, err
	}

	if len(tickets) == 0 {
		return nil, 0, fmt.Errorf("player has no transport tickets")
	}

	// Check and refill tickets if needed
	ticket, err := s.checkAndRefillTickets(ctx, tickets[0])
	if err != nil {
		return nil, 0, err
	}

	// Check if purchase count needs to be reset
	now := time.Now()
	if now.After(ticket.ResetTime) {
		ticket, _, err = s.storage.FindOneAndUpdate(ctx, ticket.ID, func(t *TransportTicket) (*TransportTicket, error) {
			t.PurchaseCount = 0
			t.ResetTime = getNextResetTime(now)
			t.UpdatedAt = now
			return t, nil
		})
		if err != nil {
			return nil, 0, err
		}
	}

	// Calculate purchase price
	price := calculatePurchasePrice(ticket.PurchaseCount)

	// Purchase a ticket
	ticket, _, err = s.storage.FindOneAndUpdate(ctx, ticket.ID, func(t *TransportTicket) (*TransportTicket, error) {
		t.CurrentTickets++
		t.PurchaseCount++
		t.LastPurchaseAt = &now
		t.UpdatedAt = now
		return t, nil
	})

	return ticket, price, err
}

// checkAndRefillTickets checks if tickets need to be refilled and does so if necessary
func (s *TicketService) checkAndRefillTickets(ctx context.Context, ticket *TransportTicket) (*TransportTicket, error) {
	now := time.Now()

	// Check if it's a new day (UTC+0 00:00)
	if isNewDay(ticket.LastRefillTime, now) {
		ticket, _, err := s.storage.FindOneAndUpdate(ctx, ticket.ID, func(t *TransportTicket) (*TransportTicket, error) {
			t.CurrentTickets = t.MaxTickets
			t.LastRefillTime = now
			t.PurchaseCount = 0
			t.ResetTime = getNextResetTime(now)
			t.UpdatedAt = now
			return t, nil
		})
		return ticket, err
	}

	return ticket, nil
}

// isNewDay checks if the current time is a new day (UTC+0 00:00) compared to the last refill time
func isNewDay(lastRefill, now time.Time) bool {
	lastRefillUTC := lastRefill.UTC()
	nowUTC := now.UTC()

	// Check if the date has changed
	return lastRefillUTC.Year() != nowUTC.Year() ||
		lastRefillUTC.Month() != nowUTC.Month() ||
		lastRefillUTC.Day() != nowUTC.Day()
}

// getNextResetTime returns the next reset time (UTC+0 00:00)
func getNextResetTime(now time.Time) time.Time {
	nowUTC := now.UTC()
	tomorrow := time.Date(nowUTC.Year(), nowUTC.Month(), nowUTC.Day()+1, 0, 0, 0, 0, time.UTC)
	return tomorrow
}

// calculatePurchasePrice calculates the price for purchasing a ticket
func calculatePurchasePrice(purchaseCount int) int {
	basePrice := 300
	priceIncrease := 100

	return basePrice + (purchaseCount * priceIncrease)
}

// GetTicketsByAlliance gets all transport tickets for an alliance
func (s *TicketService) GetTicketsByAlliance(ctx context.Context, allianceID primitive.ObjectID) ([]*TransportTicket, error) {
	return s.storage.FindMany(ctx, bson.M{"alliance_id": allianceID})
}

// UpdateMaxTickets updates the maximum number of tickets for a player
func (s *TicketService) UpdateMaxTickets(ctx context.Context, playerID primitive.ObjectID, maxTickets int) (*TransportTicket, error) {
	// Get player's tickets
	tickets, err := s.storage.FindMany(ctx, bson.M{"player_id": playerID})
	if err != nil {
		return nil, err
	}

	if len(tickets) == 0 {
		return nil, fmt.Errorf("player has no transport tickets")
	}

	ticket := tickets[0]

	// If new max is less than current max, don't update
	if maxTickets <= ticket.MaxTickets {
		return ticket, nil
	}

	// Update max tickets
	ticket, _, err = s.storage.FindOneAndUpdate(ctx, ticket.ID, func(t *TransportTicket) (*TransportTicket, error) {
		// Calculate how many tickets to add
		ticketsToAdd := maxTickets - t.MaxTickets

		// Update max tickets
		t.MaxTickets = maxTickets

		// Add the same number of current tickets (up to the new max)
		t.CurrentTickets = min(t.CurrentTickets+ticketsToAdd, maxTickets)

		t.UpdatedAt = time.Now()
		return t, nil
	})

	return ticket, err
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
