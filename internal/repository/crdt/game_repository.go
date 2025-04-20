package crdt

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"tictactoe/internal/domain"
	"time"

	"github.com/pkg/errors"

	ds "github.com/ipfs/go-datastore"
	dsquery "github.com/ipfs/go-datastore/query"
)

// GameRepository is a CRDT-based implementation of domain.GameRepository
type GameRepository struct {
	crdtStore ds.Datastore
	prefix    string
	mu        sync.RWMutex
	ctx       context.Context
}

// NewGameRepository creates a new CRDT-based game repository
func NewGameRepository(ctx context.Context, crdtStore ds.Datastore, prefix string) *GameRepository {
	return &GameRepository{
		crdtStore: crdtStore,
		prefix:    prefix,
		ctx:       ctx,
	}
}

// Create creates a new game
func (r *GameRepository) Create(game *domain.Game) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 각 작업에 대해 타임아웃이 있는 컨텍스트 생성 (10초)
	ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
	defer cancel()

	// Check if game already exists
	key := ds.NewKey(r.prefix + "/games/" + game.ID)
	exists, err := r.crdtStore.Has(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check if game exists: %w", err)
	}
	if exists {
		return errors.New("game already exists")
	}

	// Serialize game to JSON
	data, err := json.Marshal(game)
	if err != nil {
		return fmt.Errorf("failed to marshal game: %w", err)
	}

	// Store game in CRDT datastore
	fmt.Printf("Storing game in CRDT datastore: %s\n", key)
	err = r.crdtStore.Put(ctx, key, data)
	if err != nil {
		fmt.Printf("Error storing game: %v\n", err)
		return fmt.Errorf("failed to store game: %w", err)
	}

	fmt.Printf("Game created successfully: %s\n", game.ID)
	return nil
}

// Get retrieves a game by ID
func (r *GameRepository) Get(id string) (*domain.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get game from CRDT datastore
	key := ds.NewKey(r.prefix + "/games/" + id)
	data, err := r.crdtStore.Get(r.ctx, key)
	if err != nil {
		if err == ds.ErrNotFound {
			return nil, errors.New("game not found")
		}
		return nil, fmt.Errorf("failed to get game: %w", err)
	}

	// Deserialize game from JSON
	var game domain.Game
	err = json.Unmarshal(data, &game)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal game: %w", err)
	}

	return &game, nil
}

// Update updates a game
func (r *GameRepository) Update(game *domain.Game) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if game exists
	key := ds.NewKey(r.prefix + "/games/" + game.ID)
	exists, err := r.crdtStore.Has(r.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check if game exists: %w", err)
	}
	if !exists {
		return errors.New("game not found")
	}

	// Serialize game to JSON
	data, err := json.Marshal(game)
	if err != nil {
		return fmt.Errorf("failed to marshal game: %w", err)
	}

	// Store game in CRDT datastore
	err = r.crdtStore.Put(r.ctx, key, data)
	if err != nil {
		return fmt.Errorf("failed to store game: %w", err)
	}

	return nil
}

// Delete deletes a game
func (r *GameRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if game exists
	key := ds.NewKey(r.prefix + "/games/" + id)
	exists, err := r.crdtStore.Has(r.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check if game exists: %w", err)
	}
	if !exists {
		return errors.New("game not found")
	}

	// Delete game from CRDT datastore
	err = r.crdtStore.Delete(r.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete game: %w", err)
	}

	return nil
}

// List returns all games
func (r *GameRepository) List() ([]*domain.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Query all games from CRDT datastore
	query := dsquery.Query{Prefix: r.prefix + "/games/"}
	results, err := r.crdtStore.Query(r.ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query games")
	}
	defer results.Close()

	// Deserialize games from JSON
	var games []*domain.Game
	for {
		result, ok := results.NextSync()
		if !ok {
			break
		}

		if len(result.Value) == 0 {
			continue
		}

		var game domain.Game
		err = json.Unmarshal(result.Value, &game)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal game key : %s, value: %s", result.Key, string(result.Value))
		}

		games = append(games, &game)
	}

	return games, nil
}
