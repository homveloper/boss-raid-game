package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// BadgerCache is a cache implementation using BadgerDB
type BadgerCache[T any] struct {
	db      *badger.DB
	options *BadgerCacheOptions
}

// NewBadgerCache creates a new BadgerDB cache with the provided options
func NewBadgerCache[T any](opts ...BadgerCacheOption) (*BadgerCache[T], error) {
	// Start with default options
	options := DefaultBadgerCacheOptions()

	// Apply provided options
	for _, opt := range opts {
		opt(options)
	}

	// Configure BadgerDB options
	badgerOpts := badger.DefaultOptions(options.Path)
	badgerOpts.Logger = nil // Disable logging

	if options.InMemory {
		badgerOpts = badgerOpts.WithInMemory(true)
	}

	// Open BadgerDB
	db, err := badger.Open(badgerOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB: %w", err)
	}

	cache := &BadgerCache[T]{
		db:      db,
		options: options,
	}

	// Start garbage collection
	if options.GCInterval > 0 {
		go cache.runGC()
	}

	return cache, nil
}

// Get retrieves a document from the cache
func (c *BadgerCache[T]) Get(ctx context.Context, id interface{}) (T, error) {
	var empty T

	// Convert ID to string for key
	key := fmt.Sprintf("%v", id)

	var value []byte
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		value, err = item.ValueCopy(nil)
		return err
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			return empty, ErrCacheMiss
		}
		return empty, fmt.Errorf("failed to get from cache: %w", err)
	}

	// Deserialize the value
	var result T
	if err := json.Unmarshal(value, &result); err != nil {
		return empty, fmt.Errorf("failed to deserialize cached value: %w", err)
	}

	return result, nil
}

// Set stores a document in the cache with optional TTL
func (c *BadgerCache[T]) Set(ctx context.Context, id interface{}, data T, ttl time.Duration) error {
	// Convert ID to string for key
	key := fmt.Sprintf("%v", id)

	// Serialize the data
	value, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to serialize data: %w", err)
	}

	// Set TTL if not provided
	if ttl <= 0 && c.options.DefaultTTL > 0 {
		ttl = c.options.DefaultTTL
	}

	// Store in BadgerDB
	err = c.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry([]byte(key), value)
		if ttl > 0 {
			entry = entry.WithTTL(ttl)
		}
		return txn.SetEntry(entry)
	})

	if err != nil {
		return fmt.Errorf("failed to set in cache: %w", err)
	}

	return nil
}

// Delete removes a document from the cache
func (c *BadgerCache[T]) Delete(ctx context.Context, id interface{}) error {
	// Convert ID to string for key
	key := fmt.Sprintf("%v", id)

	err := c.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})

	if err != nil {
		return fmt.Errorf("failed to delete from cache: %w", err)
	}

	return nil
}

// Clear removes all documents from the cache
func (c *BadgerCache[T]) Clear(ctx context.Context) error {
	// Drop all data in BadgerDB
	err := c.db.DropAll()
	if err != nil {
		return fmt.Errorf("failed to clear cache: %w", err)
	}

	return nil
}

// Close closes the cache
func (c *BadgerCache[T]) Close() error {
	if c.db != nil {
		if err := c.db.Close(); err != nil {
			return fmt.Errorf("failed to close BadgerDB: %w", err)
		}
	}
	return nil
}

// runGC runs BadgerDB garbage collection periodically
func (c *BadgerCache[T]) runGC() {
	ticker := time.NewTicker(c.options.GCInterval)
	defer ticker.Stop()

	for range ticker.C {
		err := c.db.RunValueLogGC(0.5)
		if err != nil && err != badger.ErrNoRewrite {
			// Log error but continue
			fmt.Printf("BadgerDB GC error: %v\n", err)
		}
	}
}
