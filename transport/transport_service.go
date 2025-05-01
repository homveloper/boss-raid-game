package transport

import (
	"context"
	"fmt"
	"time"

	"nodestorage/v2"
	v2 "nodestorage/v2"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// TransportService provides operations for managing transports
type TransportService struct {
	storage       nodestorage.Storage[*Transport]
	mineService   *MineService
	ticketService *TicketService
}

// NewTransportService creates a new TransportService
func NewTransportService(
	storage nodestorage.Storage[*Transport],
	mineService *MineService,
	ticketService *TicketService,
) *TransportService {
	return &TransportService{
		storage:       storage,
		mineService:   mineService,
		ticketService: ticketService,
	}
}

// StartTransport starts a new transport from a mine
func (s *TransportService) StartTransport(
	ctx context.Context,
	playerID primitive.ObjectID,
	playerName string,
	mineID primitive.ObjectID,
	goldOreAmount int,
) (*Transport, error) {
	// Get the mine
	mine, err := s.mineService.GetMine(ctx, mineID)
	if err != nil {
		return nil, fmt.Errorf("failed to get mine: %w", err)
	}

	// Get mine configuration
	mineConfig, err := s.mineService.GetMineConfig(ctx, mine.Level)
	if err != nil {
		return nil, fmt.Errorf("failed to get mine configuration: %w", err)
	}

	// Validate gold ore amount
	if goldOreAmount < mineConfig.MinTransportAmount {
		return nil, fmt.Errorf("gold ore amount is below minimum (%d)", mineConfig.MinTransportAmount)
	}
	if goldOreAmount > mineConfig.MaxTransportAmount {
		return nil, fmt.Errorf("gold ore amount exceeds maximum (%d)", mineConfig.MaxTransportAmount)
	}

	// Check if there's enough gold ore in the mine
	actualAmount := goldOreAmount
	if mine.GoldOre < goldOreAmount {
		// If not enough gold ore, use minimum amount
		if mine.GoldOre < mineConfig.MinTransportAmount {
			actualAmount = mineConfig.MinTransportAmount
		} else {
			actualAmount = mine.GoldOre
		}
	}

	// Use a transport ticket
	_, err = s.ticketService.UseTicket(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to use transport ticket: %w", err)
	}

	// Remove gold ore from mine if there's enough
	if mine.GoldOre >= actualAmount {
		_, err = s.mineService.RemoveGoldOre(ctx, mineID, actualAmount)
		if err != nil {
			// Refund the ticket if we can't remove gold ore
			// This is a simplification - in a real system, you'd use transactions
			return nil, fmt.Errorf("failed to remove gold ore from mine: %w", err)
		}
	}

	// Create transport
	now := time.Now()
	prepEndTime := now.Add(30 * time.Minute)
	transportTime := time.Duration(mineConfig.TransportTime) * time.Minute

	transport := &Transport{
		ID:              primitive.NewObjectID(),
		AllianceID:      mine.AllianceID,
		MineID:          mineID,
		MineName:        mine.Name,
		MineLevel:       mine.Level,
		Status:          TransportStatusPreparing,
		GoldOreAmount:   actualAmount,
		MaxParticipants: mineConfig.MaxParticipants,
		Participants: []TransportMember{
			{
				PlayerID:      playerID,
				PlayerName:    playerName,
				GoldOreAmount: actualAmount,
				JoinedAt:      now,
			},
		},
		PrepStartTime: now,
		PrepEndTime:   prepEndTime,
		TransportTime: transportTime,
		CreatedAt:     now,
		UpdatedAt:     now,
		VectorClock:   1, // Set initial version
	}

	// Save the transport
	transport, err = s.storage.FindOneAndUpsert(ctx, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	// Schedule transport start after prep time
	go s.scheduleTransportStart(context.Background(), transport.ID, prepEndTime)

	return transport, nil
}

// JoinTransport joins an existing transport
func (s *TransportService) JoinTransport(
	ctx context.Context,
	transportID primitive.ObjectID,
	playerID primitive.ObjectID,
	playerName string,
	goldOreAmount int,
) (*Transport, error) {
	// Use a transport ticket
	_, err := s.ticketService.UseTicket(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("failed to use transport ticket: %w", err)
	}

	// Join the transport
	transport, _, err := s.storage.FindOneAndUpdate(ctx, transportID, func(t *Transport) (*Transport, error) {
		// Check if transport is still in preparation phase
		if t.Status != TransportStatusPreparing {
			return nil, fmt.Errorf("transport is no longer accepting participants")
		}

		// Check if player is already participating
		for _, p := range t.Participants {
			if p.PlayerID == playerID {
				return nil, fmt.Errorf("player is already participating in this transport")
			}
		}

		// Check if transport is full
		if len(t.Participants) >= t.MaxParticipants {
			return nil, fmt.Errorf("transport is full")
		}

		// Get mine configuration
		mineConfig, err := s.mineService.GetMineConfig(ctx, t.MineLevel)
		if err != nil {
			return nil, fmt.Errorf("failed to get mine configuration: %w", err)
		}

		// Validate gold ore amount
		if goldOreAmount < mineConfig.MinTransportAmount {
			return nil, fmt.Errorf("gold ore amount is below minimum (%d)", mineConfig.MinTransportAmount)
		}
		if goldOreAmount > mineConfig.MaxTransportAmount {
			return nil, fmt.Errorf("gold ore amount exceeds maximum (%d)", mineConfig.MaxTransportAmount)
		}

		// Get the mine
		mine, err := s.mineService.GetMine(ctx, t.MineID)
		if err != nil {
			return nil, fmt.Errorf("failed to get mine: %w", err)
		}

		// Check if there's enough gold ore in the mine
		actualAmount := goldOreAmount
		if mine.GoldOre < goldOreAmount {
			// If not enough gold ore, use minimum amount
			if mine.GoldOre < mineConfig.MinTransportAmount {
				actualAmount = mineConfig.MinTransportAmount
			} else {
				actualAmount = mine.GoldOre
			}
		}

		// Remove gold ore from mine if there's enough
		if mine.GoldOre >= actualAmount {
			_, err = s.mineService.RemoveGoldOre(ctx, t.MineID, actualAmount)
			if err != nil {
				return nil, fmt.Errorf("failed to remove gold ore from mine: %w", err)
			}
		}

		// Add player to participants
		t.Participants = append(t.Participants, TransportMember{
			PlayerID:      playerID,
			PlayerName:    playerName,
			GoldOreAmount: actualAmount,
			JoinedAt:      time.Now(),
		})

		// Update total gold ore amount
		t.GoldOreAmount += actualAmount

		// Check if transport is now full
		if len(t.Participants) >= t.MaxParticipants {
			// Start transport immediately if full
			now := time.Now()
			endTime := now.Add(t.TransportTime)
			t.Status = TransportStatusInProgress
			t.StartTime = &now
			t.EndTime = &endTime

			// Schedule transport completion
			go s.scheduleTransportCompletion(context.Background(), t.ID, endTime)
		}

		t.UpdatedAt = time.Now()
		return t, nil
	})

	if err != nil {
		// Refund the ticket if joining fails
		// This is a simplification - in a real system, you'd use transactions
		return nil, fmt.Errorf("failed to join transport: %w", err)
	}

	return transport, nil
}

// GetTransport retrieves a transport by ID
func (s *TransportService) GetTransport(ctx context.Context, transportID primitive.ObjectID) (*Transport, error) {
	return s.storage.FindOne(ctx, transportID)
}

// GetActiveTransports retrieves all active transports for an alliance
func (s *TransportService) GetActiveTransports(ctx context.Context, allianceID primitive.ObjectID) ([]*Transport, error) {
	return s.storage.FindMany(ctx, bson.M{
		"alliance_id": allianceID,
		"status": bson.M{
			"$in": []TransportStatus{
				TransportStatusPreparing,
				TransportStatusInProgress,
			},
		},
	})
}

// GetPlayerTransports retrieves all transports for a player
func (s *TransportService) GetPlayerTransports(ctx context.Context, playerID primitive.ObjectID) ([]*Transport, error) {
	return s.storage.FindMany(ctx, bson.M{
		"participants.player_id": playerID,
	})
}

// RaidTransport initiates a raid on a transport
func (s *TransportService) RaidTransport(
	ctx context.Context,
	transportID primitive.ObjectID,
	raiderID primitive.ObjectID,
	raiderName string,
) (*Transport, error) {
	transport, _, err := s.storage.FindOneAndUpdate(ctx, transportID, func(t *Transport) (*Transport, error) {
		// Check if transport is in progress
		if t.Status != TransportStatusInProgress {
			return nil, fmt.Errorf("transport cannot be raided in its current state")
		}

		// Check if transport is already being raided
		if t.RaidStatus != nil {
			return nil, fmt.Errorf("transport is already being raided")
		}

		// Create raid status
		now := time.Now()
		defenseEndTime := now.Add(30 * time.Minute)
		t.RaidStatus = &RaidStatus{
			RaiderID:       raiderID,
			RaiderName:     raiderName,
			RaidStartTime:  now,
			DefenseEndTime: defenseEndTime,
			IsDefended:     false,
			DefenseResult:  nil,
		}

		t.UpdatedAt = now
		return t, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to raid transport: %w", err)
	}

	// Schedule raid completion if not defended
	go s.scheduleRaidCompletion(context.Background(), transportID, transport.RaidStatus.DefenseEndTime)

	return transport, nil
}

// DefendTransport defends a transport from a raid
func (s *TransportService) DefendTransport(
	ctx context.Context,
	transportID primitive.ObjectID,
	defenderID primitive.ObjectID,
	defenderName string,
	successful bool,
) (*Transport, error) {
	transport, _, err := s.storage.FindOneAndUpdate(ctx, transportID, func(t *Transport) (*Transport, error) {
		// Check if transport is being raided
		if t.RaidStatus == nil {
			return nil, fmt.Errorf("transport is not being raided")
		}

		// Check if raid has already been defended
		if t.RaidStatus.IsDefended {
			return nil, fmt.Errorf("raid has already been defended")
		}

		// Check if defense window has expired
		now := time.Now()
		if now.After(t.RaidStatus.DefenseEndTime) {
			return nil, fmt.Errorf("defense window has expired")
		}

		// Mark as defended
		t.RaidStatus.IsDefended = true

		// Calculate gold ore lost if defense failed
		goldOreLost := 0
		if !successful {
			// If defense failed, lose 30% of gold ore
			goldOreLost = int(float64(t.GoldOreAmount) * 0.3)
			t.GoldOreAmount -= goldOreLost

			// If all gold ore is lost, mark transport as raided
			if t.GoldOreAmount <= 0 {
				t.Status = TransportStatusRaided
				t.GoldOreAmount = 0
			}
		}

		// Record defense result
		t.RaidStatus.DefenseResult = &DefenseResult{
			Successful:   successful,
			DefenderID:   defenderID,
			DefenderName: defenderName,
			CompletedAt:  now,
			GoldOreLost:  goldOreLost,
		}

		t.UpdatedAt = now
		return t, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to defend transport: %w", err)
	}

	return transport, nil
}

// WatchTransport watches for changes to a transport
func (s *TransportService) WatchTransport(ctx context.Context, transportID primitive.ObjectID) (<-chan v2.WatchEvent[*Transport], error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "documentKey._id", Value: transportID},
		}}},
	}

	return s.storage.Watch(ctx, pipeline)
}

// WatchAllTransports watches for changes to all transports for an alliance
func (s *TransportService) WatchAllTransports(ctx context.Context, allianceID primitive.ObjectID) (<-chan v2.WatchEvent[*Transport], error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "fullDocument.alliance_id", Value: allianceID},
		}}},
	}

	return s.storage.Watch(ctx, pipeline)
}

// UpdateTransportStatus updates the status of a transport
func (s *TransportService) UpdateTransportStatus(ctx context.Context, transportID primitive.ObjectID, status TransportStatus) (*Transport, error) {
	transport, _, err := s.storage.FindOneAndUpdate(ctx, transportID, func(t *Transport) (*Transport, error) {
		t.Status = status

		// If status is in progress, set start time
		if status == TransportStatusInProgress {
			now := time.Now()
			t.StartTime = &now

			// Set end time based on transport time
			endTime := now.Add(t.TransportTime)
			t.EndTime = &endTime
		}

		t.UpdatedAt = time.Now()
		return t, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update transport status: %w", err)
	}

	return transport, nil
}

// scheduleTransportStart schedules the start of a transport after preparation time
func (s *TransportService) scheduleTransportStart(ctx context.Context, transportID primitive.ObjectID, startTime time.Time) {
	// Wait until start time
	waitTime := time.Until(startTime)
	if waitTime > 0 {
		time.Sleep(waitTime)
	}

	// Start the transport
	transport, _, err := s.storage.FindOneAndUpdate(ctx, transportID, func(t *Transport) (*Transport, error) {
		// Only start if still in preparation phase
		if t.Status != TransportStatusPreparing {
			return t, nil
		}

		now := time.Now()
		endTime := now.Add(t.TransportTime)
		t.Status = TransportStatusInProgress
		t.StartTime = &now
		t.EndTime = &endTime
		t.UpdatedAt = now
		return t, nil
	})

	if err != nil {
		// Log error in real implementation
		return
	}

	// Schedule transport completion
	if transport.Status == TransportStatusInProgress && transport.EndTime != nil {
		go s.scheduleTransportCompletion(ctx, transportID, *transport.EndTime)
	}
}

// scheduleTransportCompletion schedules the completion of a transport
func (s *TransportService) scheduleTransportCompletion(ctx context.Context, transportID primitive.ObjectID, endTime time.Time) {
	// Wait until end time
	waitTime := time.Until(endTime)
	if waitTime > 0 {
		time.Sleep(waitTime)
	}

	// Complete the transport
	_, _, err := s.storage.FindOneAndUpdate(ctx, transportID, func(t *Transport) (*Transport, error) {
		// Only complete if in progress and not raided
		if t.Status != TransportStatusInProgress {
			return t, nil
		}

		// If being raided, don't complete yet
		if t.RaidStatus != nil && !t.RaidStatus.IsDefended {
			return t, nil
		}

		now := time.Now()
		t.Status = TransportStatusCompleted
		t.UpdatedAt = now
		return t, nil
	})

	if err != nil {
		// Log error in real implementation
		return
	}
}

// scheduleRaidCompletion schedules the completion of a raid if not defended
func (s *TransportService) scheduleRaidCompletion(ctx context.Context, transportID primitive.ObjectID, defenseEndTime time.Time) {
	// Wait until defense end time
	waitTime := time.Until(defenseEndTime)
	if waitTime > 0 {
		time.Sleep(waitTime)
	}

	// Complete the raid if not defended
	_, _, err := s.storage.FindOneAndUpdate(ctx, transportID, func(t *Transport) (*Transport, error) {
		// Check if raid exists and hasn't been defended
		if t.RaidStatus == nil || t.RaidStatus.IsDefended {
			return t, nil
		}

		now := time.Now()

		// Raid succeeded - lose 50% of gold ore
		goldOreLost := int(float64(t.GoldOreAmount) * 0.5)
		t.GoldOreAmount -= goldOreLost

		// If all gold ore is lost, mark transport as raided
		if t.GoldOreAmount <= 0 {
			t.Status = TransportStatusRaided
			t.GoldOreAmount = 0
		}

		// Record defense result (automatic failure)
		t.RaidStatus.IsDefended = true
		t.RaidStatus.DefenseResult = &DefenseResult{
			Successful:   false,
			DefenderID:   primitive.NilObjectID, // No defender
			DefenderName: "",
			CompletedAt:  now,
			GoldOreLost:  goldOreLost,
		}

		t.UpdatedAt = now
		return t, nil
	})

	if err != nil {
		// Log error in real implementation
		return
	}
}
