package memory

import (
	"errors"
	"sync"
	"tictactoe/internal/domain"
)

// GameRepository is an in-memory implementation of domain.GameRepository
type GameRepository struct {
	games map[string]*domain.Game
	mu    sync.RWMutex
}

// NewGameRepository creates a new in-memory game repository
func NewGameRepository() *GameRepository {
	return &GameRepository{
		games: make(map[string]*domain.Game),
	}
}

// Create creates a new game
func (r *GameRepository) Create(game *domain.Game) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.games[game.ID]; exists {
		return errors.New("game already exists")
	}

	r.games[game.ID] = game
	return nil
}

// Get retrieves a game by ID
func (r *GameRepository) Get(id string) (*domain.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	game, exists := r.games[id]
	if !exists {
		return nil, errors.New("game not found")
	}

	return game, nil
}

// Update updates a game
func (r *GameRepository) Update(game *domain.Game) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.games[game.ID]; !exists {
		return errors.New("game not found")
	}

	r.games[game.ID] = game
	return nil
}

// Delete deletes a game
func (r *GameRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.games[id]; !exists {
		return errors.New("game not found")
	}

	delete(r.games, id)
	return nil
}

// List returns all games
func (r *GameRepository) List() ([]*domain.Game, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	games := make([]*domain.Game, 0, len(r.games))
	for _, game := range r.games {
		games = append(games, game)
	}

	return games, nil
}
