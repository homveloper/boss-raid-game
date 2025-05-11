package storage

import (
	"context"
	"fmt"
	"idledungeon/internal/model"
	"time"

	"eventsync"
	"nodestorage/v2"
	"nodestorage/v2/cache"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// GameStorage handles game state storage and synchronization
type GameStorage struct {
	storage            nodestorage.Storage[*model.GameState]
	eventStore         eventsync.EventStore
	stateVectorManager eventsync.StateVectorManager
	syncService        eventsync.SyncService
	storageListener    *eventsync.StorageListener
	logger             *zap.Logger
}

// NewGameStorage creates a new game storage instance
func NewGameStorage(ctx context.Context, mongoURI, dbName string, logger *zap.Logger) (*GameStorage, error) {
	// Connect to MongoDB
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping the database
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	// Create collections
	gameCollection := client.Database(dbName).Collection("games")

	// Create memory cache
	memCache := cache.NewMemoryCache[*model.GameState](nil)

	// Create storage options
	storageOptions := &nodestorage.Options{
		VersionField:      "Version",
		CacheTTL:          time.Minute * 10,
		WatchEnabled:      true,
		WatchFullDocument: "updateLookup",
	}

	// Create storage
	storage, err := nodestorage.NewStorage[*model.GameState](ctx, gameCollection, memCache, storageOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// Create event store
	eventStore, err := eventsync.NewMongoEventStore(ctx, client, dbName, "events", logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create event store: %w", err)
	}

	// Create state vector manager
	stateVectorManager, err := eventsync.NewMongoStateVectorManager(ctx, client, dbName, "state_vectors", eventStore, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create state vector manager: %w", err)
	}

	// Create sync service
	syncService := eventsync.NewSyncService(eventStore, stateVectorManager, logger)

	// Create storage adapter
	storageAdapter := eventsync.NewStorageAdapter[*model.GameState](storage, logger)

	// Create storage listener
	storageListener := eventsync.NewStorageListener(storageAdapter, syncService, logger)

	// Start storage listener
	if err := storageListener.Start(); err != nil {
		return nil, fmt.Errorf("failed to start storage listener: %w", err)
	}

	return &GameStorage{
		storage:            storage,
		eventStore:         eventStore,
		stateVectorManager: stateVectorManager,
		syncService:        syncService,
		storageListener:    storageListener,
		logger:             logger,
	}, nil
}

// Close closes the game storage
func (s *GameStorage) Close() error {
	s.storageListener.Stop()
	return s.storage.Close()
}

// CreateGame creates a new game
func (s *GameStorage) CreateGame(ctx context.Context, name string, worldConfig model.WorldConfig) (*model.GameState, error) {
	// Initialize game state
	game := &model.GameState{
		ID:          primitive.NewObjectID(),
		Name:        name,
		Units:       model.InitializeWorld(worldConfig),
		LastUpdated: time.Now(),
		Version:     1,
	}

	// Create game in storage
	createdGame, err := s.storage.FindOneAndUpsert(ctx, game)
	if err != nil {
		return nil, fmt.Errorf("failed to create game: %w", err)
	}

	return createdGame, nil
}

// GetGame gets a game by ID
func (s *GameStorage) GetGame(ctx context.Context, id primitive.ObjectID) (*model.GameState, error) {
	game, err := s.storage.FindOne(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	return game, nil
}

// UpdateGame updates a game
func (s *GameStorage) UpdateGame(ctx context.Context, id primitive.ObjectID, updateFn func(*model.GameState) (*model.GameState, error)) (*model.GameState, error) {
	game, _, err := s.storage.FindOneAndUpdate(ctx, id, updateFn)
	if err != nil {
		return nil, fmt.Errorf("failed to update game: %w", err)
	}

	return game, nil
}

// GetSyncService returns the sync service
func (s *GameStorage) GetSyncService() eventsync.SyncService {
	return s.syncService
}
