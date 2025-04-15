package memory

import (
	"errors"
	"sync"
	"tictactoe/internal/domain"
)

// CharacterRepository is an in-memory implementation of domain.CharacterRepository
type CharacterRepository struct {
	characters map[string]*domain.Character
	mu         sync.RWMutex
}

// NewCharacterRepository creates a new in-memory character repository
func NewCharacterRepository() *CharacterRepository {
	return &CharacterRepository{
		characters: make(map[string]*domain.Character),
	}
}

// Create creates a new character
func (r *CharacterRepository) Create(character *domain.Character) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.characters[character.ID]; exists {
		return errors.New("character already exists")
	}

	r.characters[character.ID] = character
	return nil
}

// GetByID retrieves a character by ID
func (r *CharacterRepository) GetByID(id string) (*domain.Character, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	character, exists := r.characters[id]
	if !exists {
		return nil, errors.New("character not found")
	}

	return character, nil
}

// Update updates a character
func (r *CharacterRepository) Update(character *domain.Character) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.characters[character.ID]; !exists {
		return errors.New("character not found")
	}

	r.characters[character.ID] = character
	return nil
}

// Delete deletes a character
func (r *CharacterRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.characters[id]; !exists {
		return errors.New("character not found")
	}

	delete(r.characters, id)
	return nil
}
