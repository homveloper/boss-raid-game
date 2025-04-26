package guild_territory

import (
	"context"
	"testing"
	"time"

	v2 "nodestorage/v2"
	"nodestorage/v2/cache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// setupTestDB sets up a test database and returns a cleanup function
func setupTestDB(t *testing.T) (*mongo.Client, *mongo.Collection, func()) {
	// Connect to MongoDB
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	require.NoError(t, err, "Failed to connect to MongoDB")

	// Create a unique collection name for this test
	collectionName := "test_territories_" + primitive.NewObjectID().Hex()
	collection := client.Database("test_db").Collection(collectionName)

	// Return cleanup function
	cleanup := func() {
		collection.Drop(ctx)
		client.Disconnect(ctx)
	}

	return client, collection, cleanup
}

// TestTerritoryService_CreateTerritory tests the CreateTerritory method
func TestTerritoryService_CreateTerritory(t *testing.T) {
	// Set up test database
	client, collection, cleanup := setupTestDB(t)
	defer cleanup()

	// Create memory cache
	memCache := cache.NewMemoryCache[*Territory](nil)
	defer memCache.Close()

	// Create storage with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	storageOptions := &v2.Options{
		VersionField: "VectorClock", // Must match the struct field name, not the bson tag
		CacheTTL:     time.Hour,
	}
	storage, err := v2.NewStorage[*Territory](ctx, client, collection, memCache, storageOptions)
	require.NoError(t, err, "Failed to create storage")
	defer storage.Close()

	// Create territory service
	service := NewTerritoryService(storage)

	// Create a guild ID
	guildID := primitive.NewObjectID()

	// Test creating a territory
	territory, err := service.CreateTerritory(ctx, guildID, "Test Territory")
	require.NoError(t, err, "Failed to create territory")
	assert.Equal(t, "Test Territory", territory.Name, "Territory name should match")
	assert.Equal(t, guildID, territory.GuildID, "Territory guild ID should match")
	assert.Equal(t, 1, territory.Level, "Territory level should be 1")
	assert.Equal(t, 10, territory.Size, "Territory size should be 10")
	assert.Empty(t, territory.Buildings, "Territory should have no buildings")
	assert.Equal(t, int64(1), territory.VectorClock, "Territory vector clock should be 1")
}

// TestTerritoryService_PlanBuilding tests the PlanBuilding method
func TestTerritoryService_PlanBuilding(t *testing.T) {
	// Set up test database
	client, collection, cleanup := setupTestDB(t)
	defer cleanup()

	// Create memory cache
	memCache := cache.NewMemoryCache[*Territory](nil)
	defer memCache.Close()

	// Create storage with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	storageOptions := &v2.Options{
		VersionField: "VectorClock", // Must match the struct field name, not the bson tag
		CacheTTL:     time.Hour,
	}
	storage, err := v2.NewStorage[*Territory](ctx, client, collection, memCache, storageOptions)
	require.NoError(t, err, "Failed to create storage")
	defer storage.Close()

	// Create territory service
	service := NewTerritoryService(storage)

	// Create a territory
	guildID := primitive.NewObjectID()
	territory, err := service.CreateTerritory(ctx, guildID, "Test Territory")
	require.NoError(t, err, "Failed to create territory")

	// Test planning a building
	updatedTerritory, err := service.PlanBuilding(ctx, territory.ID, BuildingTypeHeadquarters, Position{X: 5, Y: 5})
	require.NoError(t, err, "Failed to plan building")
	assert.Len(t, updatedTerritory.Buildings, 1, "Territory should have one building")
	assert.Equal(t, BuildingTypeHeadquarters, updatedTerritory.Buildings[0].Type, "Building type should be headquarters")
	assert.Equal(t, BuildingStatusPlanned, updatedTerritory.Buildings[0].Status, "Building status should be planned")
	assert.Equal(t, Position{X: 5, Y: 5}, updatedTerritory.Buildings[0].Position, "Building position should match")
	assert.Equal(t, 0.0, updatedTerritory.Buildings[0].ConstructionProgress, "Building progress should be 0")
	assert.NotEmpty(t, updatedTerritory.Buildings[0].RequiredResources, "Building should have required resources")

	// Test planning a building at the same position (should fail)
	_, err = service.PlanBuilding(ctx, territory.ID, BuildingTypeBarracks, Position{X: 5, Y: 5})
	assert.Error(t, err, "Planning a building at an occupied position should fail")
	assert.Contains(t, err.Error(), "position already occupied", "Error message should mention occupied position")
}

// TestTerritoryService_ContributeToBuilding tests the ContributeToBuilding method
func TestTerritoryService_ContributeToBuilding(t *testing.T) {
	// Set up test database
	client, collection, cleanup := setupTestDB(t)
	defer cleanup()

	// Create memory cache
	memCache := cache.NewMemoryCache[*Territory](nil)
	defer memCache.Close()

	// Create storage with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	storageOptions := &v2.Options{
		VersionField: "VectorClock", // Must match the struct field name, not the bson tag
		CacheTTL:     time.Hour,
	}
	storage, err := v2.NewStorage[*Territory](ctx, client, collection, memCache, storageOptions)
	require.NoError(t, err, "Failed to create storage")
	defer storage.Close()

	// Create territory service
	service := NewTerritoryService(storage)

	// Create a territory
	guildID := primitive.NewObjectID()
	territory, err := service.CreateTerritory(ctx, guildID, "Test Territory")
	require.NoError(t, err, "Failed to create territory")

	// Plan a building
	territory, err = service.PlanBuilding(ctx, territory.ID, BuildingTypeHeadquarters, Position{X: 5, Y: 5})
	require.NoError(t, err, "Failed to plan building")
	buildingID := territory.Buildings[0].ID

	// Test contributing to the building
	memberID := primitive.NewObjectID()
	resources := Resources{
		Gold:  500,
		Wood:  250,
		Stone: 250,
		Iron:  100,
		Food:  0,
	}
	updatedTerritory, err := service.ContributeToBuilding(ctx, territory.ID, buildingID, memberID, "TestMember", resources)
	require.NoError(t, err, "Failed to contribute to building")

	// Find the building
	var building Building
	for _, b := range updatedTerritory.Buildings {
		if b.ID == buildingID {
			building = b
			break
		}
	}

	assert.Equal(t, BuildingStatusUnderConstruction, building.Status, "Building status should be under construction")
	assert.Len(t, building.Contributors, 1, "Building should have one contributor")
	assert.Equal(t, memberID, building.Contributors[0].MemberID, "Contributor ID should match")
	assert.Equal(t, "TestMember", building.Contributors[0].MemberName, "Contributor name should match")
	assert.Equal(t, resources, building.Contributors[0].Resources, "Contributed resources should match")
	assert.True(t, building.ConstructionProgress > 0, "Construction progress should be greater than 0")
	assert.True(t, building.ConstructionProgress < 1, "Construction progress should be less than 1")
	assert.Equal(t, int64(2), building.VectorClock, "Building vector clock should be incremented")

	// Test contributing enough resources to complete the building
	// First, get the remaining required resources
	requiredResources := building.RequiredResources
	totalContributed := calculateTotalContributions(building.Contributors)
	remainingGold := requiredResources.Gold - totalContributed.Gold
	remainingWood := requiredResources.Wood - totalContributed.Wood
	remainingStone := requiredResources.Stone - totalContributed.Stone
	remainingIron := requiredResources.Iron - totalContributed.Iron

	// Contribute the remaining resources
	memberID2 := primitive.NewObjectID()
	resources2 := Resources{
		Gold:  remainingGold + 100, // Extra to ensure completion
		Wood:  remainingWood + 100,
		Stone: remainingStone + 100,
		Iron:  remainingIron + 100,
		Food:  0,
	}

	updatedTerritory, err = service.ContributeToBuilding(ctx, territory.ID, buildingID, memberID2, "TestMember2", resources2)
	require.NoError(t, err, "Failed to contribute to building")

	// Find the building again
	for _, b := range updatedTerritory.Buildings {
		if b.ID == buildingID {
			building = b
			break
		}
	}

	assert.Equal(t, BuildingStatusCompleted, building.Status, "Building status should be completed")
	assert.Len(t, building.Contributors, 2, "Building should have two contributors")
	assert.Equal(t, 1.0, building.ConstructionProgress, "Construction progress should be 1.0")
	assert.NotNil(t, building.CompletedAt, "CompletedAt should not be nil")
	assert.Equal(t, int64(3), building.VectorClock, "Building vector clock should be incremented again")
}

// TestTerritoryService_UpgradeBuilding tests the UpgradeBuilding method
func TestTerritoryService_UpgradeBuilding(t *testing.T) {
	// Set up test database
	client, collection, cleanup := setupTestDB(t)
	defer cleanup()

	// Create memory cache
	memCache := cache.NewMemoryCache[*Territory](nil)
	defer memCache.Close()

	// Create storage with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	storageOptions := &v2.Options{
		VersionField: "VectorClock", // Must match the struct field name, not the bson tag
		CacheTTL:     time.Hour,
	}
	storage, err := v2.NewStorage[*Territory](ctx, client, collection, memCache, storageOptions)
	require.NoError(t, err, "Failed to create storage")
	defer storage.Close()

	// Create territory service
	service := NewTerritoryService(storage)

	// Create a territory
	guildID := primitive.NewObjectID()
	territory, err := service.CreateTerritory(ctx, guildID, "Test Territory")
	require.NoError(t, err, "Failed to create territory")

	// Plan a building
	territory, err = service.PlanBuilding(ctx, territory.ID, BuildingTypeHeadquarters, Position{X: 5, Y: 5})
	require.NoError(t, err, "Failed to plan building")
	buildingID := territory.Buildings[0].ID

	// Complete the building by directly updating it in the database
	now := time.Now()
	_, err = collection.UpdateOne(
		ctx,
		map[string]interface{}{"_id": territory.ID, "buildings._id": buildingID},
		map[string]interface{}{
			"$set": map[string]interface{}{
				"buildings.$.status":                BuildingStatusCompleted,
				"buildings.$.construction_progress": 1.0,
				"buildings.$.completed_at":          now,
				"buildings.$.vector_clock":          2,
			},
		},
	)
	require.NoError(t, err, "Failed to complete building")

	// Clear the cache to ensure we get the latest data
	err = memCache.Clear(ctx)
	require.NoError(t, err, "Failed to clear cache")

	// Get the updated territory to verify the building is completed
	territory, err = service.GetTerritory(ctx, territory.ID)
	require.NoError(t, err, "Failed to get updated territory")

	// Verify the building is completed
	var completedBuilding Building
	for _, b := range territory.Buildings {
		if b.ID == buildingID {
			completedBuilding = b
			break
		}
	}
	require.Equal(t, BuildingStatusCompleted, completedBuilding.Status, "Building status should be completed")

	// Test upgrading the building
	updatedTerritory, err := service.UpgradeBuilding(ctx, territory.ID, buildingID)
	require.NoError(t, err, "Failed to upgrade building")

	// Find the building
	var building Building
	for _, b := range updatedTerritory.Buildings {
		if b.ID == buildingID {
			building = b
			break
		}
	}

	assert.Equal(t, 2, building.Level, "Building level should be 2")
	assert.Equal(t, BuildingStatusPlanned, building.Status, "Building status should be planned")
	assert.Equal(t, 0.0, building.ConstructionProgress, "Construction progress should be reset to 0")
	assert.Empty(t, building.Contributors, "Contributors should be reset")
	assert.Nil(t, building.CompletedAt, "CompletedAt should be nil")
	assert.Equal(t, int64(3), building.VectorClock, "Building vector clock should be incremented")

	// Verify that required resources for level 2 are higher than for level 1
	level1Resources := getBuildingRequiredResources(BuildingTypeHeadquarters, 1)
	assert.True(t, building.RequiredResources.Gold > level1Resources.Gold, "Level 2 should require more gold")
	assert.True(t, building.RequiredResources.Wood > level1Resources.Wood, "Level 2 should require more wood")
	assert.True(t, building.RequiredResources.Stone > level1Resources.Stone, "Level 2 should require more stone")
}

// TestTerritoryService_AddResources tests the AddResources method
func TestTerritoryService_AddResources(t *testing.T) {
	// Set up test database
	client, collection, cleanup := setupTestDB(t)
	defer cleanup()

	// Create memory cache
	memCache := cache.NewMemoryCache[*Territory](nil)
	defer memCache.Close()

	// Create storage with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	storageOptions := &v2.Options{
		VersionField: "VectorClock", // Must match the struct field name, not the bson tag
		CacheTTL:     time.Hour,
	}
	storage, err := v2.NewStorage[*Territory](ctx, client, collection, memCache, storageOptions)
	require.NoError(t, err, "Failed to create storage")
	defer storage.Close()

	// Create territory service
	service := NewTerritoryService(storage)

	// Create a territory
	guildID := primitive.NewObjectID()
	territory, err := service.CreateTerritory(ctx, guildID, "Test Territory")
	require.NoError(t, err, "Failed to create territory")

	// Test adding resources
	resources := Resources{
		Gold:  1000,
		Wood:  500,
		Stone: 500,
		Iron:  200,
		Food:  100,
	}
	updatedTerritory, err := service.AddResources(ctx, territory.ID, resources)
	require.NoError(t, err, "Failed to add resources")

	assert.Equal(t, resources.Gold, updatedTerritory.Resources.Gold, "Gold should match")
	assert.Equal(t, resources.Wood, updatedTerritory.Resources.Wood, "Wood should match")
	assert.Equal(t, resources.Stone, updatedTerritory.Resources.Stone, "Stone should match")
	assert.Equal(t, resources.Iron, updatedTerritory.Resources.Iron, "Iron should match")
	assert.Equal(t, resources.Food, updatedTerritory.Resources.Food, "Food should match")

	// Test adding more resources
	resources2 := Resources{
		Gold:  500,
		Wood:  300,
		Stone: 200,
		Iron:  100,
		Food:  50,
	}
	updatedTerritory, err = service.AddResources(ctx, territory.ID, resources2)
	require.NoError(t, err, "Failed to add more resources")

	assert.Equal(t, resources.Gold+resources2.Gold, updatedTerritory.Resources.Gold, "Gold should be summed")
	assert.Equal(t, resources.Wood+resources2.Wood, updatedTerritory.Resources.Wood, "Wood should be summed")
	assert.Equal(t, resources.Stone+resources2.Stone, updatedTerritory.Resources.Stone, "Stone should be summed")
	assert.Equal(t, resources.Iron+resources2.Iron, updatedTerritory.Resources.Iron, "Iron should be summed")
	assert.Equal(t, resources.Food+resources2.Food, updatedTerritory.Resources.Food, "Food should be summed")
}
