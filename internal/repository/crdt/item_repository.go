package crdt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"tictactoe/internal/domain"

	ds "github.com/ipfs/go-datastore"
	dsquery "github.com/ipfs/go-datastore/query"
)

// ItemRepository is a CRDT-based implementation of domain.ItemRepository
type ItemRepository struct {
	crdtStore ds.Datastore
	prefix    string
	mu        sync.RWMutex
	ctx       context.Context
}

// NewItemRepository creates a new CRDT-based item repository
func NewItemRepository(ctx context.Context, crdtStore ds.Datastore, prefix string) *ItemRepository {
	return &ItemRepository{
		crdtStore: crdtStore,
		prefix:    prefix,
		ctx:       ctx,
	}
}

// GetByID retrieves an item by ID
func (r *ItemRepository) GetByID(id string) (*domain.Item, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get item from CRDT datastore
	key := ds.NewKey(r.prefix + "/items/" + id)
	data, err := r.crdtStore.Get(r.ctx, key)
	if err != nil {
		if err == ds.ErrNotFound {
			return nil, errors.New("item not found")
		}
		return nil, fmt.Errorf("failed to get item: %w", err)
	}

	// Deserialize item from JSON
	var item domain.Item
	err = json.Unmarshal(data, &item)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal item: %w", err)
	}

	return &item, nil
}

// GetAllWeapons retrieves all weapons
func (r *ItemRepository) GetAllWeapons() ([]*domain.Item, error) {
	return r.GetByType(domain.ItemTypeWeapon)
}

// GetAllArmors retrieves all armors
func (r *ItemRepository) GetAllArmors() ([]*domain.Item, error) {
	return r.GetByType(domain.ItemTypeArmor)
}

// GetByType retrieves all items of a specific type
func (r *ItemRepository) GetByType(itemType domain.ItemType) ([]*domain.Item, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Query all items from CRDT datastore
	query := dsquery.Query{Prefix: r.prefix + "/items/"}
	results, err := r.crdtStore.Query(r.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query items: %w", err)
	}
	defer results.Close()

	// Deserialize items from JSON and filter by type
	var items []*domain.Item
	for {
		result, ok := results.NextSync()
		if !ok {
			break
		}

		var item domain.Item
		err = json.Unmarshal(result.Value, &item)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal item: %w", err)
		}

		if item.Type == itemType {
			items = append(items, &item)
		}
	}

	return items, nil
}

// Create creates a new item
func (r *ItemRepository) Create(item *domain.Item) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if item already exists
	key := ds.NewKey(r.prefix + "/items/" + item.ID)
	exists, err := r.crdtStore.Has(r.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check if item exists: %w", err)
	}
	if exists {
		return errors.New("item already exists")
	}

	// Serialize item to JSON
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	// Store item in CRDT datastore
	err = r.crdtStore.Put(r.ctx, key, data)
	if err != nil {
		return fmt.Errorf("failed to store item: %w", err)
	}

	return nil
}

// Update updates an item
func (r *ItemRepository) Update(item *domain.Item) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if item exists
	key := ds.NewKey(r.prefix + "/items/" + item.ID)
	exists, err := r.crdtStore.Has(r.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check if item exists: %w", err)
	}
	if !exists {
		return errors.New("item not found")
	}

	// Serialize item to JSON
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	// Store item in CRDT datastore
	err = r.crdtStore.Put(r.ctx, key, data)
	if err != nil {
		return fmt.Errorf("failed to store item: %w", err)
	}

	return nil
}

// Delete deletes an item
func (r *ItemRepository) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if item exists
	key := ds.NewKey(r.prefix + "/items/" + id)
	exists, err := r.crdtStore.Has(r.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check if item exists: %w", err)
	}
	if !exists {
		return errors.New("item not found")
	}

	// Delete item from CRDT datastore
	err = r.crdtStore.Delete(r.ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}

	return nil
}
