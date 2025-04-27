package transport

import (
	"context"
	"fmt"
	"time"

	v2 "nodestorage/v2"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// GeneralService handles operations related to generals
type GeneralService struct {
	storage v2.Storage[*General]
}

// NewGeneralService creates a new GeneralService
func NewGeneralService(storage v2.Storage[*General]) *GeneralService {
	return &GeneralService{
		storage: storage,
	}
}

// CreateGeneral creates a new general
func (s *GeneralService) CreateGeneral(
	ctx context.Context,
	playerID primitive.ObjectID,
	name string,
	level int,
	stars int,
	rarity GeneralRarity,
) (*General, error) {
	// Validate input
	if name == "" {
		return nil, fmt.Errorf("general name cannot be empty")
	}
	if level < 1 || level > 80 {
		return nil, fmt.Errorf("general level must be between 1 and 80")
	}
	if stars < 0 || stars > 15 {
		return nil, fmt.Errorf("general stars must be between 0 and 15")
	}

	// Create general
	now := time.Now()
	general := &General{
		ID:          primitive.NewObjectID(),
		PlayerID:    playerID,
		Name:        name,
		Level:       level,
		Stars:       stars,
		Rarity:      rarity,
		Status:      GeneralStatusIdle,
		AssignedTo:  nil,
		CreatedAt:   now,
		UpdatedAt:   now,
		VectorClock: 1, // Set initial version
	}

	// Save to database
	return s.storage.FindOneAndUpsert(ctx, general)
}

// GetGeneralByID gets a general by ID
func (s *GeneralService) GetGeneralByID(ctx context.Context, generalID primitive.ObjectID) (*General, error) {
	return s.storage.FindOne(ctx, generalID)
}

// GetPlayerGenerals gets all generals for a player
func (s *GeneralService) GetPlayerGenerals(ctx context.Context, playerID primitive.ObjectID) ([]*General, error) {
	return s.storage.FindMany(ctx, bson.M{"player_id": playerID})
}

// GetAvailableGenerals gets all available (idle) generals for a player
func (s *GeneralService) GetAvailableGenerals(ctx context.Context, playerID primitive.ObjectID) ([]*General, error) {
	return s.storage.FindMany(ctx, bson.M{
		"player_id": playerID,
		"status":    GeneralStatusIdle,
	})
}

// AssignGeneral assigns a general to a target
func (s *GeneralService) AssignGeneral(
	ctx context.Context,
	generalID primitive.ObjectID,
	assignmentType string,
	targetID primitive.ObjectID,
	targetName string,
) (*General, error) {
	// Find the general
	general, err := s.GetGeneralByID(ctx, generalID)
	if err != nil {
		return nil, fmt.Errorf("failed to find general: %w", err)
	}

	// Check if general is already assigned
	if general.Status == GeneralStatusAssigned {
		return nil, fmt.Errorf("general is already assigned")
	}

	// Update general status
	general, _, err = s.storage.FindOneAndUpdate(ctx, generalID, func(g *General) (*General, error) {
		g.Status = GeneralStatusAssigned
		g.AssignedTo = &AssignmentInfo{
			Type:       assignmentType,
			TargetID:   targetID,
			TargetName: targetName,
			AssignedAt: time.Now(),
		}
		g.UpdatedAt = time.Now()
		return g, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update general: %w", err)
	}

	return general, nil
}

// UnassignGeneral unassigns a general from its current assignment
func (s *GeneralService) UnassignGeneral(ctx context.Context, generalID primitive.ObjectID) (*General, error) {
	// Find the general
	general, err := s.GetGeneralByID(ctx, generalID)
	if err != nil {
		return nil, fmt.Errorf("failed to find general: %w", err)
	}

	// Check if general is assigned
	if general.Status != GeneralStatusAssigned {
		return nil, fmt.Errorf("general is not assigned")
	}

	// Update general status
	general, _, err = s.storage.FindOneAndUpdate(ctx, generalID, func(g *General) (*General, error) {
		g.Status = GeneralStatusIdle
		g.AssignedTo = nil
		g.UpdatedAt = time.Now()
		return g, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update general: %w", err)
	}

	return general, nil
}

// CalculateContributionRate calculates the contribution rate of a general for mine development
// Returns the points contributed per hour
func (s *GeneralService) CalculateContributionRate(general *General) float64 {
	if general == nil {
		return 0
	}

	// Base contribution rate: 10/60 points per hour
	baseRate := 10.0 / 60.0

	// Star bonus: 2% per star
	starBonus := float64(general.Stars) * 0.02

	// Level bonus: 0.12658% per level above 1
	levelBonus := float64(general.Level-1) * 0.0012658

	// Rarity multiplier
	rarityMultiplier := 0.2 // Default for common
	switch general.Rarity {
	case GeneralRarityCommon:
		rarityMultiplier = 0.2
	case GeneralRarityUncommon:
		rarityMultiplier = 0.4
	case GeneralRarityRare, GeneralRaritySoldier:
		rarityMultiplier = 0.6
	case GeneralRarityEpic:
		rarityMultiplier = 0.8
	case GeneralRarityLegendary:
		rarityMultiplier = 1.0
	}

	// Calculate total contribution rate
	// Formula: baseRate * (1 + starBonus + levelBonus) * rarityMultiplier
	contributionRate := baseRate * (1 + starBonus + levelBonus) * rarityMultiplier

	return contributionRate
}

// UpdateGeneralWithFunction updates a general using the provided update function (for demo purposes)
func (s *GeneralService) UpdateGeneralWithFunction(ctx context.Context, generalID primitive.ObjectID, updateFn func(*General) (*General, error)) (*General, error) {
	general, _, err := s.storage.FindOneAndUpdate(ctx, generalID, updateFn)
	return general, err
}
