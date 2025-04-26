package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// BadgerCache implements the Cache interface using BadgerDB
type BadgerCache[T any] struct {
	db      *badger.DB
	options *CacheOptions
}

// NewBadgerCache creates a new BadgerCache instance
func NewBadgerCache[T any](dbPath string, options *CacheOptions) (*BadgerCache[T], error) {
	if options == nil {
		options = DefaultCacheOptions()
	}

	// Configure BadgerDB options
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil // Disable default logger

	// Open BadgerDB
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB: %w", err)
	}

	// Start garbage collection in background
	go runBadgerGC(db)

	return &BadgerCache[T]{
		db:      db,
		options: options,
	}, nil
}

// Get retrieves a document from the cache
func (c *BadgerCache[T]) Get(ctx context.Context, id primitive.ObjectID) (T, error) {
	var result T

	// Create key from ID
	key := getKey(id)

	// Read from BadgerDB
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			// Unmarshal the value
			return bson.Unmarshal(val, &result)
		})
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			return result, ErrCacheMiss
		}
		return result, fmt.Errorf("failed to get from cache: %w", err)
	}

	return result, nil
}

// Set stores a document in the cache with an optional TTL
func (c *BadgerCache[T]) Set(ctx context.Context, id primitive.ObjectID, data T, ttl time.Duration) error {
	// Create key from ID
	key := getKey(id)

	// Marshal the data
	value, err := bson.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Use default TTL if not provided
	if ttl <= 0 {
		ttl = c.options.DefaultTTL
	}

	// Write to BadgerDB
	err = c.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry(key, value).WithTTL(ttl)
		return txn.SetEntry(entry)
	})

	if err != nil {
		return fmt.Errorf("failed to set in cache: %w", err)
	}

	return nil
}

// Delete removes a document from the cache
func (c *BadgerCache[T]) Delete(ctx context.Context, id primitive.ObjectID) error {
	// Create key from ID
	key := getKey(id)

	// Delete from BadgerDB
	err := c.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(key)
	})

	if err != nil {
		return fmt.Errorf("failed to delete from cache: %w", err)
	}

	return nil
}

// Clear removes all documents from the cache
func (c *BadgerCache[T]) Clear(ctx context.Context) error {
	// Drop all data in BadgerDB
	return c.db.DropAll()
}

// Close closes the cache
func (c *BadgerCache[T]) Close() error {
	return c.db.Close()
}

// Helper function to create a key from an ObjectID
func getKey(id primitive.ObjectID) []byte {
	return []byte(id.Hex())
}

// Helper function to run BadgerDB garbage collection in background
func runBadgerGC(db *badger.DB) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
	again:
		err := db.RunValueLogGC(0.5) // Run GC if 50% or more space can be reclaimed
		if err == nil {
			// If GC was successful, run it again until it returns an error
			goto again
		}
	}
}

// BadgerCacheOptions represents additional options for BadgerCache
type BadgerCacheOptions struct {
	// Base cache options
	CacheOptions

	// BadgerDB specific options
	ValueLogFileSize        int64
	MemTableSize            int64
	NumMemtables            int
	NumLevelZeroTables      int
	NumLevelZeroTablesStall int
	ValueThreshold          int64
	SyncWrites              bool
	NumCompactors           int
}

// DefaultBadgerCacheOptions returns the default BadgerCache options
func DefaultBadgerCacheOptions() *BadgerCacheOptions {
	return &BadgerCacheOptions{
		CacheOptions: *DefaultCacheOptions(),

		// BadgerDB specific defaults
		ValueLogFileSize:        1 << 28, // 256 MB
		MemTableSize:            1 << 26, // 64 MB
		NumMemtables:            5,
		NumLevelZeroTables:      5,
		NumLevelZeroTablesStall: 10,
		ValueThreshold:          1 << 10, // 1 KB
		SyncWrites:              false,
		NumCompactors:           2,
	}
}

// NewBadgerCacheWithOptions creates a new BadgerCache with custom options
func NewBadgerCacheWithOptions[T any](dbPath string, options *BadgerCacheOptions) (*BadgerCache[T], error) {
	if options == nil {
		options = DefaultBadgerCacheOptions()
	}

	// Configure BadgerDB options
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil // Disable default logger

	// Apply custom options
	opts.ValueLogFileSize = options.ValueLogFileSize
	opts.MemTableSize = options.MemTableSize
	opts.NumMemtables = options.NumMemtables
	opts.NumLevelZeroTables = options.NumLevelZeroTables
	opts.NumLevelZeroTablesStall = options.NumLevelZeroTablesStall
	opts.ValueThreshold = options.ValueThreshold
	opts.SyncWrites = options.SyncWrites
	opts.NumCompactors = options.NumCompactors

	// Open BadgerDB
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB: %w", err)
	}

	// Start garbage collection in background
	go runBadgerGC(db)

	return &BadgerCache[T]{
		db:      db,
		options: &options.CacheOptions,
	}, nil
}
