package crdt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"tictactoe/internal/domain"

	ds "github.com/ipfs/go-datastore"
)

// CharacterRepository is a CRDT-based implementation of domain.CharacterRepository
type CharacterRepository struct {
	crdtStore ds.Datastore
	prefix    string
	mu        sync.RWMutex
	ctx       context.Context
}

// NewCharacterRepository creates a new CRDT-based character repository
func NewCharacterRepository(ctx context.Context, crdtStore ds.Datastore, prefix string) *CharacterRepository {
	return &CharacterRepository{
		crdtStore: crdtStore,
		prefix:    prefix,
		ctx:       ctx,
	}
}

// Create creates a new character
func (r *CharacterRepository) Create(character *domain.Character) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if character already exists
	key := ds.NewKey(r.prefix + "/characters/" + character.ID)
	exists, err := r.crdtStore.Has(r.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check if character exists: %w", err)
	}
	if exists {
		return errors.New("character already exists")
	}

	// Serialize character to JSON
	data, err := json.Marshal(character)
	if err != nil {
		return fmt.Errorf("failed to marshal character: %w", err)
	}

	// Store character in CRDT datastore
	err = r.crdtStore.Put(r.ctx, key, data)
	if err != nil {
		return fmt.Errorf("failed to store character: %w", err)
	}

	return nil
}

// GetByID retrieves a character by ID
func (r *CharacterRepository) GetByID(id string) (*domain.Character, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get character from CRDT datastore
	key := ds.NewKey(r.prefix + "/characters/" + id)
	data, err := r.crdtStore.Get(r.ctx, key)
	if err != nil {
		if err == ds.ErrNotFound {
			return nil, errors.New("character not found")
		}
		return nil, fmt.Errorf("failed to get character: %w", err)
	}

	// Deserialize character from JSON
	var character domain.Character
	err = json.Unmarshal(data, &character)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal character: %w", err)
	}

	return &character, nil
}

// Update updates a character
func (r *CharacterRepository) Update(character *domain.Character) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if character exists
	key := ds.NewKey(r.prefix + "/characters/" + character.ID)
	exists, err := r.crdtStore.Has(r.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check if character exists: %w", err)
	}
	if !exists {
		return errors.New("character not found")
	}

	// Serialize character to JSON
	data, err := json.Marshal(character)
	if err != nil {
		return fmt.Errorf("failed to marshal character: %w", err)
	}

	// Store character in CRDT datastore
	err = r.crdtStore.Put(r.ctx, key, data)
	if err != nil {
		return fmt.Errorf("failed to store character: %w", err)
	}

	return nil
}

// Delete deletes a character
func (r *CharacterRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if character exists
	key := ds.NewKey(r.prefix + "/characters/" + id)
	exists, err := r.crdtStore.Has(r.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check if character exists: %w", err)
	}
	if !exists {
		return errors.New("character not found")
	}

	// Delete character from CRDT datastore
	err = r.crdtStore.Delete(r.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete character: %w", err)
	}

	return nil
}
