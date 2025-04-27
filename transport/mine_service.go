package transport

import (
	"context"
	"fmt"
	"time"

	nodestorage "nodestorage/v2"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// MineService provides operations for managing mines
type MineService struct {
	storage        nodestorage.Storage[*Mine]
	configStorage  nodestorage.Storage[*MineConfig]
	generalService *GeneralService
	ticketService  *TicketService
}

// NewMineService creates a new MineService
func NewMineService(
	storage nodestorage.Storage[*Mine],
	configStorage nodestorage.Storage[*MineConfig],
	generalService *GeneralService,
	ticketService *TicketService,
) *MineService {
	return &MineService{
		storage:        storage,
		configStorage:  configStorage,
		generalService: generalService,
		ticketService:  ticketService,
	}
}

// CreateMine creates a new mine for an alliance
func (s *MineService) CreateMine(ctx context.Context, allianceID primitive.ObjectID, name string, level MineLevel) (*Mine, error) {
	// Get mine config for this level
	config, err := s.GetMineConfig(ctx, level)
	if err != nil {
		// If no config exists, create a default one based on level
		requiredPoints := float64(10000)
		transportTicketMax := 5

		switch level {
		case MineLevel1:
			requiredPoints = 10000 // 약 1일
			transportTicketMax = 5
		case MineLevel2:
			requiredPoints = 30000 // 약 3일
			transportTicketMax = 10
		case MineLevel3:
			requiredPoints = 70000 // 약 7일
			transportTicketMax = 15
		default:
			requiredPoints = float64(10000 * int(level))
			transportTicketMax = 5 * int(level)
		}

		config, err = s.CreateOrUpdateMineConfigWithDevelopment(
			ctx,
			level,
			100*int(level),     // minTransportAmount
			500*int(level),     // maxTransportAmount
			30+(10*int(level)), // transportTime
			3+int(level),       // maxParticipants
			requiredPoints,     // requiredPoints
			transportTicketMax, // transportTicketMax
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create mine config: %w", err)
		}
	}

	now := time.Now()
	mine := &Mine{
		ID:                primitive.NewObjectID(),
		AllianceID:        allianceID,
		Name:              name,
		Level:             level,
		GoldOre:           0,
		Status:            MineStatusUndeveloped,
		DevelopmentPoints: 0,
		RequiredPoints:    config.RequiredPoints,
		AssignedGenerals:  []AssignedGeneral{},
		LastUpdatedAt:     now,
		CreatedAt:         now,
		UpdatedAt:         now,
		VectorClock:       1, // Set initial version
	}

	return s.storage.FindOneAndUpsert(ctx, mine)
}

// GetMine retrieves a mine by ID
func (s *MineService) GetMine(ctx context.Context, mineID primitive.ObjectID) (*Mine, error) {
	return s.storage.FindOne(ctx, mineID)
}

// GetMinesByAlliance retrieves all mines for an alliance
func (s *MineService) GetMinesByAlliance(ctx context.Context, allianceID primitive.ObjectID) ([]*Mine, error) {
	return s.storage.FindMany(ctx, bson.M{"alliance_id": allianceID})
}

// AddGoldOre adds gold ore to a mine
func (s *MineService) AddGoldOre(ctx context.Context, mineID primitive.ObjectID, amount int) (*Mine, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	// Get the mine first to check its status
	mine, err := s.GetMine(ctx, mineID)
	if err != nil {
		return nil, fmt.Errorf("failed to get mine: %w", err)
	}

	// Check if mine is developed or active
	if mine.Status != MineStatusDeveloped && mine.Status != MineStatusActive {
		return nil, fmt.Errorf("cannot add gold ore to undeveloped mine (status: %s)", mine.Status)
	}

	mine, _, err = s.storage.FindOneAndUpdate(ctx, mineID, func(mine *Mine) (*Mine, error) {
		mine.GoldOre += amount
		mine.UpdatedAt = time.Now()
		return mine, nil
	})

	return mine, err
}

// RemoveGoldOre removes gold ore from a mine
func (s *MineService) RemoveGoldOre(ctx context.Context, mineID primitive.ObjectID, amount int) (*Mine, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	// Get the mine first to check its status
	mine, err := s.GetMine(ctx, mineID)
	if err != nil {
		return nil, fmt.Errorf("failed to get mine: %w", err)
	}

	// Check if mine is developed or active
	if mine.Status != MineStatusDeveloped && mine.Status != MineStatusActive {
		return nil, fmt.Errorf("cannot remove gold ore from undeveloped mine (status: %s)", mine.Status)
	}

	mine, _, err = s.storage.FindOneAndUpdate(ctx, mineID, func(mine *Mine) (*Mine, error) {
		if mine.GoldOre < amount {
			return nil, fmt.Errorf("not enough gold ore in mine")
		}
		mine.GoldOre -= amount
		mine.UpdatedAt = time.Now()
		return mine, nil
	})

	return mine, err
}

// GetMineConfig retrieves the configuration for a mine level
func (s *MineService) GetMineConfig(ctx context.Context, level MineLevel) (*MineConfig, error) {
	configs, err := s.configStorage.FindMany(ctx, bson.M{"level": level})
	if err != nil {
		return nil, err
	}

	if len(configs) == 0 {
		return nil, fmt.Errorf("no configuration found for mine level %d", level)
	}

	return configs[0], nil
}

// CreateOrUpdateMineConfig creates or updates a mine configuration
func (s *MineService) CreateOrUpdateMineConfig(
	ctx context.Context,
	level MineLevel,
	minTransportAmount int,
	maxTransportAmount int,
	transportTime int,
	maxParticipants int,
) (*MineConfig, error) {
	return s.CreateOrUpdateMineConfigWithDevelopment(
		ctx,
		level,
		minTransportAmount,
		maxTransportAmount,
		transportTime,
		maxParticipants,
		0, // Default requiredPoints
		5, // Default transportTicketMax
	)
}

// CreateOrUpdateMineConfigWithDevelopment creates or updates a mine configuration with development settings
func (s *MineService) CreateOrUpdateMineConfigWithDevelopment(
	ctx context.Context,
	level MineLevel,
	minTransportAmount int,
	maxTransportAmount int,
	transportTime int,
	maxParticipants int,
	requiredPoints float64,
	transportTicketMax int,
) (*MineConfig, error) {
	// Find existing config
	configs, err := s.configStorage.FindMany(ctx, bson.M{"level": level})
	if err != nil {
		return nil, err
	}

	var config *MineConfig
	if len(configs) > 0 {
		// Update existing config
		config = configs[0]
		config, _, err = s.configStorage.FindOneAndUpdate(ctx, config.ID, func(c *MineConfig) (*MineConfig, error) {
			c.MinTransportAmount = minTransportAmount
			c.MaxTransportAmount = maxTransportAmount
			c.TransportTime = transportTime
			c.MaxParticipants = maxParticipants
			c.RequiredPoints = requiredPoints
			c.TransportTicketMax = transportTicketMax
			c.UpdatedAt = time.Now()
			return c, nil
		})
		return config, err
	} else {
		// Create new config
		now := time.Now()
		config = &MineConfig{
			ID:                 primitive.NewObjectID(),
			Level:              level,
			MinTransportAmount: minTransportAmount,
			MaxTransportAmount: maxTransportAmount,
			TransportTime:      transportTime,
			MaxParticipants:    maxParticipants,
			RequiredPoints:     requiredPoints,
			TransportTicketMax: transportTicketMax,
			CreatedAt:          now,
			UpdatedAt:          now,
			VectorClock:        1, // Set initial version
		}
		return s.configStorage.FindOneAndUpsert(ctx, config)
	}
}

// WatchMine watches for changes to a mine
func (s *MineService) WatchMine(ctx context.Context, mineID primitive.ObjectID) (<-chan nodestorage.WatchEvent[*Mine], error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "documentKey._id", Value: mineID},
		}}},
	}

	return s.storage.Watch(ctx, pipeline)
}

// WatchAllMines watches for changes to all mines for an alliance
func (s *MineService) WatchAllMines(ctx context.Context, allianceID primitive.ObjectID) (<-chan nodestorage.WatchEvent[*Mine], error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "fullDocument.alliance_id", Value: allianceID},
		}}},
	}

	return s.storage.Watch(ctx, pipeline)
}

// AssignGeneralToMine assigns a general to a mine for development
func (s *MineService) AssignGeneralToMine(
	ctx context.Context,
	mineID primitive.ObjectID,
	playerID primitive.ObjectID,
	playerName string,
	generalID primitive.ObjectID,
) (*Mine, error) {
	// Get the mine
	mine, err := s.GetMine(ctx, mineID)
	if err != nil {
		return nil, fmt.Errorf("failed to get mine: %w", err)
	}

	// Check if mine is in a valid state for development
	if mine.Status != MineStatusUndeveloped && mine.Status != MineStatusDeveloping {
		return nil, fmt.Errorf("mine is not in a valid state for development")
	}

	// Check if player already has a general assigned to this mine
	for _, ag := range mine.AssignedGenerals {
		if ag.PlayerID == playerID {
			return nil, fmt.Errorf("player already has a general assigned to this mine")
		}
	}

	// Check if mine has reached maximum number of assigned generals (30)
	if len(mine.AssignedGenerals) >= 30 {
		return nil, fmt.Errorf("mine has reached maximum number of assigned generals")
	}

	// Get the general
	general, err := s.generalService.GetGeneralByID(ctx, generalID)
	if err != nil {
		return nil, fmt.Errorf("failed to get general: %w", err)
	}

	// Check if general belongs to the player
	if general.PlayerID != playerID {
		return nil, fmt.Errorf("general does not belong to the player")
	}

	// Check if general is already assigned
	if general.Status == GeneralStatusAssigned {
		return nil, fmt.Errorf("general is already assigned to another task")
	}

	// Calculate contribution rate
	contributionRate := s.generalService.CalculateContributionRate(general)

	// Assign general to mine
	_, err = s.generalService.AssignGeneral(ctx, generalID, "mine_development", mineID, mine.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to assign general: %w", err)
	}

	// Update mine with assigned general
	mine, _, err = s.storage.FindOneAndUpdate(ctx, mineID, func(m *Mine) (*Mine, error) {
		// Create assigned general record
		assignedGeneral := AssignedGeneral{
			PlayerID:         playerID,
			PlayerName:       playerName,
			GeneralID:        generalID,
			GeneralName:      general.Name,
			Level:            general.Level,
			Stars:            general.Stars,
			Rarity:           general.Rarity,
			AssignedAt:       time.Now(),
			ContributionRate: contributionRate,
		}

		// Add to assigned generals list
		m.AssignedGenerals = append(m.AssignedGenerals, assignedGeneral)

		// Update mine status if this is the first general
		if len(m.AssignedGenerals) == 1 {
			m.Status = MineStatusDeveloping
		}

		m.UpdatedAt = time.Now()
		return m, nil
	})

	if err != nil {
		// If mine update fails, unassign the general
		_, _ = s.generalService.UnassignGeneral(ctx, generalID)
		return nil, fmt.Errorf("failed to update mine: %w", err)
	}

	return mine, nil
}

// UnassignGeneralFromMine unassigns a general from a mine
func (s *MineService) UnassignGeneralFromMine(
	ctx context.Context,
	mineID primitive.ObjectID,
	playerID primitive.ObjectID,
	generalID primitive.ObjectID,
) (*Mine, error) {
	// Get the mine
	mine, err := s.GetMine(ctx, mineID)
	if err != nil {
		return nil, fmt.Errorf("failed to get mine: %w", err)
	}

	// Check if mine is in a valid state for development
	if mine.Status != MineStatusDeveloping {
		return nil, fmt.Errorf("mine is not in development")
	}

	// Find the assigned general
	var foundIndex = -1
	for i, ag := range mine.AssignedGenerals {
		if ag.GeneralID == generalID && ag.PlayerID == playerID {
			foundIndex = i
			break
		}
	}

	if foundIndex == -1 {
		return nil, fmt.Errorf("general is not assigned to this mine")
	}

	// Unassign general
	_, err = s.generalService.UnassignGeneral(ctx, generalID)
	if err != nil {
		return nil, fmt.Errorf("failed to unassign general: %w", err)
	}

	// Update mine
	mine, _, err = s.storage.FindOneAndUpdate(ctx, mineID, func(m *Mine) (*Mine, error) {
		// Remove from assigned generals list
		m.AssignedGenerals = append(m.AssignedGenerals[:foundIndex], m.AssignedGenerals[foundIndex+1:]...)

		// Update mine status if no generals left
		if len(m.AssignedGenerals) == 0 {
			m.Status = MineStatusUndeveloped
		}

		m.UpdatedAt = time.Now()
		return m, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update mine: %w", err)
	}

	return mine, nil
}

// UpdateMineDevelopment updates the development points of a mine
func (s *MineService) UpdateMineDevelopment(ctx context.Context, mineID primitive.ObjectID) (*Mine, error) {
	// Get the mine
	mine, err := s.GetMine(ctx, mineID)
	if err != nil {
		return nil, fmt.Errorf("failed to get mine: %w", err)
	}

	// Check if mine is in development
	if mine.Status != MineStatusDeveloping {
		return nil, fmt.Errorf("mine is not in development")
	}

	// Calculate time since last update
	now := time.Now()
	hoursSinceLastUpdate := now.Sub(mine.LastUpdatedAt).Hours()

	// If less than a minute has passed, don't update
	if hoursSinceLastUpdate < 1.0/60.0 {
		return mine, nil
	}

	// Calculate development points contributed by each general
	var totalPointsAdded float64 = 0
	for _, ag := range mine.AssignedGenerals {
		pointsFromGeneral := ag.ContributionRate * hoursSinceLastUpdate
		totalPointsAdded += pointsFromGeneral
	}

	// Update mine development points
	mine, _, err = s.storage.FindOneAndUpdate(ctx, mineID, func(m *Mine) (*Mine, error) {
		m.DevelopmentPoints += totalPointsAdded
		m.LastUpdatedAt = now

		// Check if development is complete
		if m.DevelopmentPoints >= m.RequiredPoints {
			m.DevelopmentPoints = m.RequiredPoints
			m.Status = MineStatusDeveloped

			// Unassign all generals
			for _, ag := range m.AssignedGenerals {
				_, _ = s.generalService.UnassignGeneral(ctx, ag.GeneralID)
			}

			// Clear assigned generals list
			m.AssignedGenerals = []AssignedGeneral{}
		}

		m.UpdatedAt = now
		return m, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update mine: %w", err)
	}

	// If mine development is complete, update transport tickets
	if mine.Status == MineStatusDeveloped {
		err = s.updateTransportTicketsForAlliance(ctx, mine.AllianceID, mine.Level)
		if err != nil {
			return mine, fmt.Errorf("mine development completed but failed to update transport tickets: %w", err)
		}
	}

	return mine, nil
}

// updateTransportTicketsForAlliance updates the max transport tickets for all players in an alliance
func (s *MineService) updateTransportTicketsForAlliance(ctx context.Context, allianceID primitive.ObjectID, mineLevel MineLevel) error {
	// Get mine config for this level
	config, err := s.GetMineConfig(ctx, mineLevel)
	if err != nil {
		return fmt.Errorf("failed to get mine config: %w", err)
	}

	// Get all transport tickets for this alliance
	tickets, err := s.ticketService.GetTicketsByAlliance(ctx, allianceID)
	if err != nil {
		return fmt.Errorf("failed to get transport tickets: %w", err)
	}

	// Update max tickets for each player
	for _, ticket := range tickets {
		_, err = s.ticketService.UpdateMaxTickets(ctx, ticket.PlayerID, config.TransportTicketMax)
		if err != nil {
			return fmt.Errorf("failed to update max tickets for player %s: %w", ticket.PlayerID.Hex(), err)
		}
	}

	return nil
}

// ActivateMine activates a developed mine
func (s *MineService) ActivateMine(ctx context.Context, mineID primitive.ObjectID) (*Mine, error) {
	// Get the mine
	mine, err := s.GetMine(ctx, mineID)
	if err != nil {
		return nil, fmt.Errorf("failed to get mine: %w", err)
	}

	// Check if mine is developed
	if mine.Status != MineStatusDeveloped {
		return nil, fmt.Errorf("mine is not developed")
	}

	// Update mine status
	mine, _, err = s.storage.FindOneAndUpdate(ctx, mineID, func(m *Mine) (*Mine, error) {
		m.Status = MineStatusActive
		m.UpdatedAt = time.Now()
		return m, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update mine: %w", err)
	}

	return mine, nil
}

// GetDevelopingMines gets all mines that are currently being developed for an alliance
func (s *MineService) GetDevelopingMines(ctx context.Context, allianceID primitive.ObjectID) ([]*Mine, error) {
	return s.storage.FindMany(ctx, bson.M{
		"alliance_id": allianceID,
		"status":      MineStatusDeveloping,
	})
}

// GetMinesByStatus gets all mines with a specific status for an alliance
func (s *MineService) GetMinesByStatus(ctx context.Context, allianceID primitive.ObjectID, status MineStatus) ([]*Mine, error) {
	return s.storage.FindMany(ctx, bson.M{
		"alliance_id": allianceID,
		"status":      status,
	})
}

// SimulateDevelopmentProgress simulates the development progress of a mine over a period of time
// timeInHours is the number of hours to simulate
func (s *MineService) SimulateDevelopmentProgress(ctx context.Context, mineID primitive.ObjectID, timeInHours float64) (*Mine, error) {
	// Get the mine
	mine, err := s.GetMine(ctx, mineID)
	if err != nil {
		return nil, fmt.Errorf("failed to get mine: %w", err)
	}

	// Check if mine is in development
	if mine.Status != MineStatusDeveloping {
		return nil, fmt.Errorf("mine is not in development")
	}

	// Calculate development points contributed by each general
	var totalPointsAdded float64 = 0
	for _, ag := range mine.AssignedGenerals {
		pointsFromGeneral := ag.ContributionRate * timeInHours
		totalPointsAdded += pointsFromGeneral
	}

	// Update mine development points
	mine, _, err = s.storage.FindOneAndUpdate(ctx, mineID, func(m *Mine) (*Mine, error) {
		m.DevelopmentPoints += totalPointsAdded
		m.LastUpdatedAt = time.Now()

		// Check if development is complete
		if m.DevelopmentPoints >= m.RequiredPoints {
			m.DevelopmentPoints = m.RequiredPoints
			m.Status = MineStatusDeveloped

			// Unassign all generals
			for _, ag := range m.AssignedGenerals {
				_, _ = s.generalService.UnassignGeneral(ctx, ag.GeneralID)
			}

			// Clear assigned generals list
			m.AssignedGenerals = []AssignedGeneral{}
		}

		m.UpdatedAt = time.Now()
		return m, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update mine: %w", err)
	}

	// If mine development is complete, update transport tickets
	if mine.Status == MineStatusDeveloped {
		err = s.updateTransportTicketsForAlliance(ctx, mine.AllianceID, mine.Level)
		if err != nil {
			return mine, fmt.Errorf("mine development completed but failed to update transport tickets: %w", err)
		}
	}

	return mine, nil
}

// CreateGeneralForDemo creates a general for demo purposes
func (s *MineService) CreateGeneralForDemo(
	ctx context.Context,
	playerID primitive.ObjectID,
	name string,
	level int,
	stars int,
	rarity GeneralRarity,
) (*General, error) {
	return s.generalService.CreateGeneral(ctx, playerID, name, level, stars, rarity)
}

// ForceCompleteDevelopment forces a mine's development to complete (for demo purposes)
func (s *MineService) ForceCompleteDevelopment(ctx context.Context, mineID primitive.ObjectID) (*Mine, error) {
	mine, err := s.GetMine(ctx, mineID)
	if err != nil {
		return nil, fmt.Errorf("failed to get mine: %w", err)
	}

	mine, _, err = s.storage.FindOneAndUpdate(ctx, mineID, func(m *Mine) (*Mine, error) {
		m.DevelopmentPoints = m.RequiredPoints
		m.Status = MineStatusDeveloped

		// Unassign all generals
		for _, ag := range m.AssignedGenerals {
			_, _ = s.generalService.UnassignGeneral(ctx, ag.GeneralID)
		}

		// Clear assigned generals list
		m.AssignedGenerals = []AssignedGeneral{}

		return m, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to update mine development points: %w", err)
	}

	// Update transport tickets for alliance
	err = s.updateTransportTicketsForAlliance(ctx, mine.AllianceID, mine.Level)
	if err != nil {
		return mine, fmt.Errorf("mine development completed but failed to update transport tickets: %w", err)
	}

	return mine, nil
}

// UpdateMineWithFunction updates a mine using the provided update function (for demo purposes)
func (s *MineService) UpdateMineWithFunction(ctx context.Context, mineID primitive.ObjectID, updateFn func(*Mine) (*Mine, error)) (*Mine, error) {
	mine, _, err := s.storage.FindOneAndUpdate(ctx, mineID, updateFn)
	return mine, err
}

// GetGeneralService returns the general service (for demo purposes)
func (s *MineService) GetGeneralService() *GeneralService {
	return s.generalService
}
