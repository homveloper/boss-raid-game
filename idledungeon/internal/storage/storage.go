package storage

import (
	"context"
	"fmt"
	"idledungeon/internal/model"
	"time"

	"nodestorage/v2"
	"nodestorage/v2/cache"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// GameStorage handles game data storage using nodestorage
type GameStorage struct {
	storage nodestorage.Storage[*model.Game]
	logger  *zap.Logger
}

// GetStorage returns the underlying nodestorage.Storage
func (s *GameStorage) GetStorage() nodestorage.Storage[*model.Game] {
	return s.storage
}

// NewGameStorage creates a new game storage
func NewGameStorage(ctx context.Context, client *mongo.Client, dbName string, logger *zap.Logger) (*GameStorage, error) {
	// Get collection
	collection := client.Database(dbName).Collection("games")

	// Create memory cache
	memCache := cache.NewMemoryCache[*model.Game](nil)

	// Create storage options
	storageOptions := &nodestorage.Options{
		VersionField:      "VectorClock",
		CacheTTL:          time.Minute * 10,
		WatchEnabled:      true,
		WatchFullDocument: "updateLookup",
		WatchMaxAwaitTime: time.Second * 1,
		WatchBatchSize:    100,
	}

	// Create storage
	storage, err := nodestorage.NewStorage[*model.Game](ctx, collection, memCache, storageOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	return &GameStorage{
		storage: storage,
		logger:  logger,
	}, nil
}

// CreateGame creates a new game
func (s *GameStorage) CreateGame(ctx context.Context, game *model.Game) (*model.Game, error) {
	// Create game in storage
	createdGame, err := s.storage.FindOneAndUpsert(ctx, game)
	if err != nil {
		return nil, fmt.Errorf("failed to create game: %w", err)
	}

	return createdGame, nil
}

// GetGame gets a game by ID
func (s *GameStorage) GetGame(ctx context.Context, id primitive.ObjectID) (*model.Game, error) {
	// Get game from storage
	game, err := s.storage.FindOne(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	return game, nil
}

// GetGameByName gets a game by name
func (s *GameStorage) GetGameByName(ctx context.Context, name string) (*model.Game, error) {
	// Create query
	query := bson.M{"name": name}

	// Get games from storage
	cursor, err := s.storage.Collection().Find(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get game by name: %w", err)
	}
	defer cursor.Close(ctx)

	// Decode games
	var games []*model.Game
	if err := cursor.All(ctx, &games); err != nil {
		return nil, fmt.Errorf("failed to decode games: %w", err)
	}

	// Check if any games were found
	if len(games) == 0 {
		return nil, fmt.Errorf("game not found")
	}

	return games[0], nil
}

// UpdateGame updates a game
func (s *GameStorage) UpdateGame(ctx context.Context, id primitive.ObjectID, updateFn func(*model.Game) (*model.Game, error)) (*model.Game, *nodestorage.Diff, error) {
	// Edit game in storage
	updatedGame, diff, err := s.storage.FindOneAndUpdate(ctx, id, func(game *model.Game) (*model.Game, error) {
		return updateFn(game)
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to update game: %w", err)
	}

	return updatedGame, diff, nil
}

// DeleteGame deletes a game
func (s *GameStorage) DeleteGame(ctx context.Context, id primitive.ObjectID) error {
	// Delete game from storage
	_, err := s.storage.Collection().DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete game: %w", err)
	}

	return nil
}

// WatchGames watches for changes to games
func (s *GameStorage) WatchGames(ctx context.Context) (<-chan nodestorage.WatchEvent[*model.Game], error) {
	// Create pipeline for all operations
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "operationType", Value: bson.D{{Key: "$in", Value: bson.A{"insert", "update", "delete"}}}}}}},
	}

	// Watch for changes
	watchCh, err := s.storage.Watch(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to watch games: %w", err)
	}

	return watchCh, nil
}

// GetAllGames gets all games
func (s *GameStorage) GetAllGames(ctx context.Context) ([]*model.Game, error) {
	// Get all games from storage
	cursor, err := s.storage.Collection().Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to get all games: %w", err)
	}
	defer cursor.Close(ctx)

	// Decode games
	var games []*model.Game
	if err := cursor.All(ctx, &games); err != nil {
		return nil, fmt.Errorf("failed to decode games: %w", err)
	}

	return games, nil
}

// Close closes the storage
func (s *GameStorage) Close() error {
	return s.storage.Close()
}
