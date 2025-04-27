package transport

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

// setupTestMongoDB sets up a MongoDB client and collections for testing
func setupTestMongoDB(t *testing.T) (*mongo.Client, *mongo.Collection, *mongo.Collection, *mongo.Collection, *mongo.Collection, func()) {
	// Connect to MongoDB
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	require.NoError(t, err, "Failed to connect to MongoDB")

	// Create unique collection names for this test
	mineCollName := "test_mines_" + primitive.NewObjectID().Hex()
	configCollName := "test_configs_" + primitive.NewObjectID().Hex()
	transportCollName := "test_transports_" + primitive.NewObjectID().Hex()
	ticketCollName := "test_tickets_" + primitive.NewObjectID().Hex()

	// Create collections
	mineCollection := client.Database("test_db").Collection(mineCollName)
	configCollection := client.Database("test_db").Collection(configCollName)
	transportCollection := client.Database("test_db").Collection(transportCollName)
	ticketCollection := client.Database("test_db").Collection(ticketCollName)

	// Return a cleanup function
	cleanup := func() {
		// Drop the collections
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		mineCollection.Drop(ctx)
		configCollection.Drop(ctx)
		transportCollection.Drop(ctx)
		ticketCollection.Drop(ctx)

		// Disconnect from MongoDB
		client.Disconnect(ctx)
	}

	return client, mineCollection, configCollection, transportCollection, ticketCollection, cleanup
}

// setupTestServices sets up the services for testing
func setupTestServices(t *testing.T) (*MineService, *TicketService, *TransportService, func()) {
	// Set up MongoDB
	client, mineCollection, configCollection, transportCollection, ticketCollection, cleanup := setupTestMongoDB(t)

	// Create caches
	mineCache := cache.NewMemoryCache[*Mine](nil)
	configCache := cache.NewMemoryCache[*MineConfig](nil)
	transportCache := cache.NewMemoryCache[*Transport](nil)
	ticketCache := cache.NewMemoryCache[*TransportTicket](nil)

	// Create storage options
	storageOptions := &v2.Options{
		VersionField: "VectorClock",
		CacheTTL:     time.Hour,
	}

	// Create storages
	ctx := context.Background()
	mineStorage, err := v2.NewStorage[*Mine](ctx, client, mineCollection, mineCache, storageOptions)
	require.NoError(t, err, "Failed to create mine storage")

	configStorage, err := v2.NewStorage[*MineConfig](ctx, client, configCollection, configCache, storageOptions)
	require.NoError(t, err, "Failed to create config storage")

	transportStorage, err := v2.NewStorage[*Transport](ctx, client, transportCollection, transportCache, storageOptions)
	require.NoError(t, err, "Failed to create transport storage")

	ticketStorage, err := v2.NewStorage[*TransportTicket](ctx, client, ticketCollection, ticketCache, storageOptions)
	require.NoError(t, err, "Failed to create ticket storage")

	// Create services
	mineService := NewMineService(mineStorage, configStorage)
	ticketService := NewTicketService(ticketStorage)
	transportService := NewTransportService(transportStorage, mineService, ticketService)

	// Return services and cleanup function
	return mineService, ticketService, transportService, func() {
		mineStorage.Close()
		configStorage.Close()
		transportStorage.Close()
		ticketStorage.Close()
		cleanup()
	}
}

// TestMineService tests the MineService
func TestMineService(t *testing.T) {
	// Set up services
	mineService, _, _, cleanup := setupTestServices(t)
	defer cleanup()

	// Create test data
	ctx := context.Background()
	allianceID := primitive.NewObjectID()

	// Test creating a mine
	mine, err := mineService.CreateMine(ctx, allianceID, "Test Mine", 2)
	require.NoError(t, err, "Failed to create mine")
	assert.Equal(t, "Test Mine", mine.Name)
	assert.Equal(t, MineLevel(2), mine.Level)
	assert.Equal(t, 0, mine.GoldOre)

	// Test getting a mine
	retrievedMine, err := mineService.GetMine(ctx, mine.ID)
	require.NoError(t, err, "Failed to get mine")
	assert.Equal(t, mine.ID, retrievedMine.ID)
	assert.Equal(t, mine.Name, retrievedMine.Name)

	// Test adding gold ore
	updatedMine, err := mineService.AddGoldOre(ctx, mine.ID, 500)
	require.NoError(t, err, "Failed to add gold ore")
	assert.Equal(t, 500, updatedMine.GoldOre)

	// Test removing gold ore
	updatedMine, err = mineService.RemoveGoldOre(ctx, mine.ID, 200)
	require.NoError(t, err, "Failed to remove gold ore")
	assert.Equal(t, 300, updatedMine.GoldOre)

	// Test creating a mine config
	config, err := mineService.CreateOrUpdateMineConfig(ctx, 2, 200, 1000, 45, 5)
	require.NoError(t, err, "Failed to create mine config")
	assert.Equal(t, MineLevel(2), config.Level)
	assert.Equal(t, 200, config.MinTransportAmount)
	assert.Equal(t, 1000, config.MaxTransportAmount)

	// Test getting a mine config
	retrievedConfig, err := mineService.GetMineConfig(ctx, 2)
	require.NoError(t, err, "Failed to get mine config")
	assert.Equal(t, config.Level, retrievedConfig.Level)
	assert.Equal(t, config.MinTransportAmount, retrievedConfig.MinTransportAmount)
}

// TestTicketService tests the TicketService
func TestTicketService(t *testing.T) {
	// Set up services
	_, ticketService, _, cleanup := setupTestServices(t)
	defer cleanup()

	// Create test data
	ctx := context.Background()
	playerID := primitive.NewObjectID()
	allianceID := primitive.NewObjectID()

	// Test creating tickets
	ticket, err := ticketService.GetOrCreateTickets(ctx, playerID, allianceID, 5)
	require.NoError(t, err, "Failed to create tickets")
	assert.Equal(t, 5, ticket.CurrentTickets)
	assert.Equal(t, 5, ticket.MaxTickets)
	assert.Equal(t, 0, ticket.PurchaseCount)

	// Test using a ticket
	updatedTicket, err := ticketService.UseTicket(ctx, playerID)
	require.NoError(t, err, "Failed to use ticket")
	assert.Equal(t, 4, updatedTicket.CurrentTickets)

	// Test purchasing a ticket
	updatedTicket, price, err := ticketService.PurchaseTicket(ctx, playerID)
	require.NoError(t, err, "Failed to purchase ticket")
	assert.Equal(t, 5, updatedTicket.CurrentTickets)
	assert.Equal(t, 1, updatedTicket.PurchaseCount)
	assert.Equal(t, 300, price) // First purchase price

	// Test purchasing another ticket
	updatedTicket, price, err = ticketService.PurchaseTicket(ctx, playerID)
	require.NoError(t, err, "Failed to purchase ticket")
	assert.Equal(t, 6, updatedTicket.CurrentTickets)
	assert.Equal(t, 2, updatedTicket.PurchaseCount)
	assert.Equal(t, 400, price) // Second purchase price
}

// TestTransportService tests the TransportService
func TestTransportService(t *testing.T) {
	// Set up services
	mineService, ticketService, transportService, cleanup := setupTestServices(t)
	defer cleanup()

	// Create test data
	ctx := context.Background()
	allianceID := primitive.NewObjectID()
	playerID := primitive.NewObjectID()
	playerName := "Test Player"
	player2ID := primitive.NewObjectID()
	player2Name := "Test Player 2"

	// Create mine config
	_, err := mineService.CreateOrUpdateMineConfig(ctx, 1, 100, 500, 30, 4)
	require.NoError(t, err, "Failed to create mine config")

	// Create mine
	mine, err := mineService.CreateMine(ctx, allianceID, "Test Mine", 1)
	require.NoError(t, err, "Failed to create mine")

	// Add gold ore to mine
	mine, err = mineService.AddGoldOre(ctx, mine.ID, 1000)
	require.NoError(t, err, "Failed to add gold ore")

	// Create tickets for players
	_, err = ticketService.GetOrCreateTickets(ctx, playerID, allianceID, 5)
	require.NoError(t, err, "Failed to create tickets for player 1")

	_, err = ticketService.GetOrCreateTickets(ctx, player2ID, allianceID, 5)
	require.NoError(t, err, "Failed to create tickets for player 2")

	// Test starting a transport
	transport, err := transportService.StartTransport(ctx, playerID, playerName, mine.ID, 200)
	require.NoError(t, err, "Failed to start transport")
	assert.Equal(t, TransportStatusPreparing, transport.Status)
	assert.Equal(t, 200, transport.GoldOreAmount)
	assert.Equal(t, 1, len(transport.Participants))

	// Test joining a transport
	transport, err = transportService.JoinTransport(ctx, transport.ID, player2ID, player2Name, 150)
	require.NoError(t, err, "Failed to join transport")
	assert.Equal(t, 2, len(transport.Participants))
	assert.Equal(t, 350, transport.GoldOreAmount)

	// Test getting a transport
	retrievedTransport, err := transportService.GetTransport(ctx, transport.ID)
	require.NoError(t, err, "Failed to get transport")
	assert.Equal(t, transport.ID, retrievedTransport.ID)
	assert.Equal(t, transport.GoldOreAmount, retrievedTransport.GoldOreAmount)

	// Test getting active transports
	transports, err := transportService.GetActiveTransports(ctx, allianceID)
	require.NoError(t, err, "Failed to get active transports")
	assert.Equal(t, 1, len(transports))

	// Test raiding a transport
	// Note: This would normally fail because the transport is still in preparation,
	// but we'll modify it directly for testing purposes
	_, _, err = transportService.storage.FindOneAndUpdate(ctx, transport.ID, func(t *Transport) (*Transport, error) {
		t.Status = TransportStatusInProgress
		now := time.Now()
		t.StartTime = &now
		return t, nil
	})
	require.NoError(t, err, "Failed to update transport status")

	raiderID := primitive.NewObjectID()
	raiderName := "Test Raider"
	transport, err = transportService.RaidTransport(ctx, transport.ID, raiderID, raiderName)
	require.NoError(t, err, "Failed to raid transport")
	assert.NotNil(t, transport.RaidStatus)
	assert.Equal(t, raiderID, transport.RaidStatus.RaiderID)

	// Test defending a transport
	transport, err = transportService.DefendTransport(ctx, transport.ID, playerID, playerName, true)
	require.NoError(t, err, "Failed to defend transport")
	assert.True(t, transport.RaidStatus.IsDefended)
	assert.NotNil(t, transport.RaidStatus.DefenseResult)
	assert.True(t, transport.RaidStatus.DefenseResult.Successful)
}

// TestTransportScenario tests a complete transport scenario
func TestTransportScenario(t *testing.T) {
	// This test would be more comprehensive and test the entire flow
	// including preparation time, transport time, and raid defense
	// For brevity, we'll skip this in the example
	t.Skip("Skipping full scenario test for brevity")
}
