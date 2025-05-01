package guild_territory

import (
	"context"
	"fmt"
	"time"

	"nodestorage/v2"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// TerritoryService provides operations for managing guild territories
type TerritoryService struct {
	storage nodestorage.Storage[*Territory]
}

// NewTerritoryService creates a new TerritoryService
func NewTerritoryService(storage nodestorage.Storage[*Territory]) *TerritoryService {
	return &TerritoryService{
		storage: storage,
	}
}

// CreateTerritory creates a new territory for a guild
func (s *TerritoryService) CreateTerritory(ctx context.Context, guildID primitive.ObjectID, name string) (*Territory, error) {
	territory := &Territory{
		ID:          primitive.NewObjectID(),
		GuildID:     guildID,
		Name:        name,
		Level:       1,
		Size:        10, // Initial size
		Buildings:   []Building{},
		Resources:   Resources{},
		UpdatedAt:   time.Now(),
		VectorClock: 1, // Set initial version
	}

	// Explicitly set the vector clock to ensure it's properly initialized
	territory.SetVectorClock(1)

	return s.storage.FindOneAndUpsert(ctx, territory)
}

// GetTerritory retrieves a territory by ID
func (s *TerritoryService) GetTerritory(ctx context.Context, territoryID primitive.ObjectID) (*Territory, error) {
	return s.storage.FindOne(ctx, territoryID)
}

// GetTerritoryByGuild retrieves a territory by guild ID
func (s *TerritoryService) GetTerritoryByGuild(ctx context.Context, guildID primitive.ObjectID) (*Territory, error) {
	territories, err := s.storage.FindMany(ctx, bson.M{"guild_id": guildID})
	if err != nil {
		return nil, err
	}

	if len(territories) == 0 {
		return nil, nodestorage.ErrNotFound
	}

	return territories[0], nil
}

// PlanBuilding plans a new building in the territory
func (s *TerritoryService) PlanBuilding(ctx context.Context, territoryID primitive.ObjectID, buildingType BuildingType, position Position) (*Territory, error) {
	updated, _, err := s.storage.FindOneAndUpdate(ctx, territoryID, func(territory *Territory) (*Territory, error) {
		// Check if position is valid and not occupied
		for _, building := range territory.Buildings {
			if building.Position.X == position.X && building.Position.Y == position.Y {
				return nil, fmt.Errorf("position already occupied")
			}
		}

		// Check if territory has enough space
		if len(territory.Buildings) >= territory.Size {
			return nil, fmt.Errorf("territory is full")
		}

		// Create new building
		building := Building{
			ID:                   primitive.NewObjectID(),
			Type:                 buildingType,
			Level:                1,
			Status:               BuildingStatusPlanned,
			Position:             position,
			ConstructionProgress: 0.0,
			RequiredResources:    getBuildingRequiredResources(buildingType, 1),
			Contributors:         []Contribution{},
			StartedAt:            time.Now(),
			VectorClock:          1,
		}

		// Add building to territory
		territory.Buildings = append(territory.Buildings, building)
		territory.UpdatedAt = time.Now()

		return territory, nil
	})

	return updated, err
}

// ContributeToBuilding contributes resources to a building under construction
func (s *TerritoryService) ContributeToBuilding(
	ctx context.Context,
	territoryID primitive.ObjectID,
	buildingID primitive.ObjectID,
	memberID primitive.ObjectID,
	memberName string,
	resources Resources,
) (*Territory, error) {
	// Instead of using UpdateSection, use FindOneAndUpdate for the entire territory
	territory, _, err := s.storage.FindOneAndUpdate(ctx, territoryID, func(territory *Territory) (*Territory, error) {
		// Find the building
		buildingIndex := -1
		for i, b := range territory.Buildings {
			if b.ID == buildingID {
				buildingIndex = i
				break
			}
		}

		if buildingIndex == -1 {
			return nil, fmt.Errorf("building not found")
		}

		// Get a reference to the building
		building := &territory.Buildings[buildingIndex]

		// Check building status
		if building.Status != BuildingStatusPlanned && building.Status != BuildingStatusUnderConstruction {
			return nil, fmt.Errorf("building is not under construction")
		}

		// Update building status if it's the first contribution
		if building.Status == BuildingStatusPlanned {
			building.Status = BuildingStatusUnderConstruction
		}

		// Add contribution
		contribution := Contribution{
			MemberID:   memberID,
			MemberName: memberName,
			Resources:  resources,
			Timestamp:  time.Now(),
		}
		building.Contributors = append(building.Contributors, contribution)

		// Update construction progress
		totalContributed := calculateTotalContributions(building.Contributors)
		building.ConstructionProgress = calculateConstructionProgress(totalContributed, building.RequiredResources)

		// Check if construction is complete
		if building.ConstructionProgress >= 1.0 {
			building.Status = BuildingStatusCompleted
			now := time.Now()
			building.CompletedAt = &now
			building.ConstructionProgress = 1.0
		}

		// Increment the building's vector clock
		building.VectorClock++

		return territory, nil
	})

	return territory, err
}

// UpgradeBuilding upgrades an existing building
func (s *TerritoryService) UpgradeBuilding(ctx context.Context, territoryID primitive.ObjectID, buildingID primitive.ObjectID) (*Territory, error) {
	// Instead of using UpdateSection, use FindOneAndUpdate for the entire territory
	territory, _, err := s.storage.FindOneAndUpdate(ctx, territoryID, func(territory *Territory) (*Territory, error) {
		// Find the building
		buildingIndex := -1
		for i, b := range territory.Buildings {
			if b.ID == buildingID {
				buildingIndex = i
				break
			}
		}

		if buildingIndex == -1 {
			return nil, fmt.Errorf("building not found")
		}

		// Get a reference to the building
		building := &territory.Buildings[buildingIndex]

		// Check building status
		if building.Status != BuildingStatusCompleted {
			return nil, fmt.Errorf("building is not completed")
		}

		// Reset construction state for upgrade
		building.Level++
		building.Status = BuildingStatusPlanned
		building.ConstructionProgress = 0.0
		building.RequiredResources = getBuildingRequiredResources(building.Type, building.Level)
		building.Contributors = []Contribution{}
		building.StartedAt = time.Now()
		building.CompletedAt = nil

		// Increment the building's vector clock
		building.VectorClock++

		return territory, nil
	})

	return territory, err
}

// AddResources adds resources to the territory
func (s *TerritoryService) AddResources(ctx context.Context, territoryID primitive.ObjectID, resources Resources) (*Territory, error) {
	// Instead of using UpdateSection, use FindOneAndUpdate for the resources field
	territory, _, err := s.storage.FindOneAndUpdate(ctx, territoryID, func(territory *Territory) (*Territory, error) {
		// Add the new resources to the territory's resources
		territory.Resources.Add(resources)
		return territory, nil
	})
	return territory, err
}

// WatchTerritory watches for changes to a territory
func (s *TerritoryService) WatchTerritory(ctx context.Context, territoryID primitive.ObjectID) (<-chan nodestorage.WatchEvent[*Territory], error) {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "documentKey._id", Value: territoryID},
		}}},
	}

	return s.storage.Watch(ctx, pipeline)
}

// Helper functions

// getBuildingRequiredResources returns the resources required to build a building of the given type and level
func getBuildingRequiredResources(buildingType BuildingType, level int) Resources {
	// Base costs for level 1
	baseCosts := map[BuildingType]Resources{
		BuildingTypeHeadquarters: {Gold: 1000, Wood: 500, Stone: 500, Iron: 200, Food: 0},
		BuildingTypeBarracks:     {Gold: 500, Wood: 300, Stone: 200, Iron: 100, Food: 0},
		BuildingTypeMine:         {Gold: 300, Wood: 100, Stone: 200, Iron: 50, Food: 0},
		BuildingTypeLumberMill:   {Gold: 300, Wood: 200, Stone: 100, Iron: 50, Food: 0},
		BuildingTypeStoneQuarry:  {Gold: 300, Wood: 100, Stone: 300, Iron: 50, Food: 0},
		BuildingTypeFarm:         {Gold: 200, Wood: 150, Stone: 100, Iron: 0, Food: 0},
	}

	// Get base cost for the building type
	baseCost, ok := baseCosts[buildingType]
	if !ok {
		// Default cost if building type is not found
		baseCost = Resources{Gold: 500, Wood: 200, Stone: 200, Iron: 100, Food: 0}
	}

	// Scale costs based on level (each level costs 50% more than the previous)
	levelMultiplier := 1.0
	for i := 1; i < level; i++ {
		levelMultiplier *= 1.5
	}

	return Resources{
		Gold:  int(float64(baseCost.Gold) * levelMultiplier),
		Wood:  int(float64(baseCost.Wood) * levelMultiplier),
		Stone: int(float64(baseCost.Stone) * levelMultiplier),
		Iron:  int(float64(baseCost.Iron) * levelMultiplier),
		Food:  int(float64(baseCost.Food) * levelMultiplier),
	}
}

// calculateTotalContributions calculates the total resources contributed to a building
func calculateTotalContributions(contributions []Contribution) Resources {
	total := Resources{}
	for _, contribution := range contributions {
		total.Add(contribution.Resources)
	}
	return total
}

// calculateConstructionProgress calculates the construction progress based on contributed and required resources
func calculateConstructionProgress(contributed, required Resources) float64 {
	// Calculate the percentage of each resource type
	goldPercent := float64(contributed.Gold) / float64(required.Gold)
	woodPercent := float64(contributed.Wood) / float64(required.Wood)
	stonePercent := float64(contributed.Stone) / float64(required.Stone)

	// Calculate iron percentage (avoid division by zero)
	ironPercent := 1.0
	if required.Iron > 0 {
		ironPercent = float64(contributed.Iron) / float64(required.Iron)
	}

	// Calculate food percentage (avoid division by zero)
	foodPercent := 1.0
	if required.Food > 0 {
		foodPercent = float64(contributed.Food) / float64(required.Food)
	}

	// Take the minimum percentage as the overall progress
	progress := goldPercent
	if woodPercent < progress {
		progress = woodPercent
	}
	if stonePercent < progress {
		progress = stonePercent
	}
	if ironPercent < progress {
		progress = ironPercent
	}
	if foodPercent < progress {
		progress = foodPercent
	}

	// Ensure progress is between 0 and 1
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	return progress
}
