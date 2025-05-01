// Package cache provides caching interfaces and implementations for nodestorage/v2.
//
// This package defines a generic Cache interface and provides multiple implementations:
//   - MemoryCache: An in-memory cache implementation using a map
//   - BadgerCache: A persistent cache implementation using BadgerDB
//   - RedisCache: A distributed cache implementation using Redis
//
// Each implementation has its own strengths and trade-offs:
//   - MemoryCache is the fastest but limited by available memory and not shared between processes
//   - BadgerCache provides persistence and larger capacity but is still local to a single machine
//   - RedisCache enables sharing cache data between multiple processes or machines
//
// Basic usage example:
//
//	// Create a memory cache for Document type
//	memCache := cache.NewMemoryCache[*Document](nil)
//	defer memCache.Close()
//
//	// Store a document with 1-hour TTL
//	err := memCache.Set(ctx, doc.ID.Hex(), doc, time.Hour)
//
//	// Retrieve the document
//	cachedDoc, err := memCache.Get(ctx, doc.ID.Hex())
//	if err == cache.ErrCacheMiss {
//	    // Document not in cache
//	}
package cache

import (
	"context"
	"errors"
	"time"
)

// Cache errors define the standard error types returned by cache implementations.
// These errors provide a consistent way to handle cache-related issues across
// different implementations.
var (
	// ErrCacheMiss is returned when a document is not found in the cache.
	// This is the most common error and indicates that the requested document
	// either was never cached, expired, or was evicted.
	ErrCacheMiss = errors.New("cache miss")

	// ErrCacheFull is returned when the cache is full and cannot store more items.
	// This typically occurs with memory caches that have a MaxItems limit.
	// The cache implementation may evict older items to make room for new ones.
	ErrCacheFull = errors.New("cache is full")

	// ErrCacheClosed is returned when attempting to operate on a closed cache.
	// Once a cache is closed, it cannot be used for any operations.
	// Always check for this error when using a cache that might be closed.
	ErrCacheClosed = errors.New("cache is closed")

	// ErrInvalidKey is returned when an invalid key is provided to a cache operation.
	// This might occur if the key is nil or otherwise invalid for the specific cache implementation.
	ErrInvalidKey = errors.New("invalid cache key")

	// ErrInvalidValue is returned when an invalid value is provided to a cache operation.
	// This might occur if the value is nil or otherwise invalid for the specific cache implementation.
	ErrInvalidValue = errors.New("invalid cache value")

	// ErrSerializationFailed is returned when the cache fails to serialize a value for storage.
	// This typically occurs with persistent caches that need to convert objects to bytes.
	ErrSerializationFailed = errors.New("failed to serialize cache value")

	// ErrDeserializationFailed is returned when the cache fails to deserialize a stored value.
	// This typically occurs with persistent caches when reading corrupted or incompatible data.
	ErrDeserializationFailed = errors.New("failed to deserialize cache value")
)

// Cache is the interface for caching documents of type T.
// It provides a generic interface that can be implemented by different cache backends,
// such as in-memory maps, persistent storage, or distributed caches.
//
// The interface is designed to be simple and consistent across implementations,
// focusing on the core operations needed for document caching.
type Cache[T any] interface {
	// Get retrieves a document from the cache by its ID.
	//
	// Parameters:
	//   - ctx: The context for the operation
	//   - key: The unique identifier of the document to retrieve
	//
	// Returns:
	//   - The cached document if found
	//   - ErrCacheMiss if the document is not in the cache
	//   - ErrCacheClosed if the cache is closed
	//   - Other implementation-specific errors
	Get(ctx context.Context, key string) (T, error)

	// Set stores a document in the cache with an optional TTL (time-to-live).
	//
	// Parameters:
	//   - ctx: The context for the operation
	//   - key: The unique identifier of the document
	//   - data: The document to store
	//   - ttl: The time-to-live for the document (0 for default TTL)
	//
	// Returns:
	//   - nil if the document was successfully stored
	//   - ErrCacheFull if the cache is full and cannot store more items
	//   - ErrCacheClosed if the cache is closed
	//   - ErrInvalidKey if the ID is invalid
	//   - ErrInvalidValue if the data is invalid
	//   - ErrSerializationFailed if the data could not be serialized
	//   - Other implementation-specific errors
	Set(ctx context.Context, key string, data T, ttl time.Duration) error

	// Delete removes a document from the cache by its ID.
	//
	// Parameters:
	//   - ctx: The context for the operation
	//   - key: The unique identifier of the document to remove
	//
	// Returns:
	//   - nil if the document was successfully removed or did not exist
	//   - ErrCacheClosed if the cache is closed
	//   - ErrInvalidKey if the ID is invalid
	//   - Other implementation-specific errors
	Delete(ctx context.Context, key string) error

	// Clear removes all documents from the cache.
	//
	// Parameters:
	//   - ctx: The context for the operation
	//
	// Returns:
	//   - nil if the cache was successfully cleared
	//   - ErrCacheClosed if the cache is closed
	//   - Other implementation-specific errors
	Clear(ctx context.Context) error

	// Close closes the cache and releases any resources it holds.
	// After calling Close, the cache cannot be used for any operations.
	//
	// Returns:
	//   - nil if the cache was successfully closed
	//   - An error if the cache could not be closed properly
	Close() error
}

// CacheOptions represents configuration options for cache implementations.
// These options provide a common set of configuration parameters that can be
// used across different cache implementations.
//
// Each cache implementation may have additional options specific to that implementation.
type CacheOptions struct {
	// DefaultTTL is the default time-to-live for cached items.
	// This is used when a TTL of 0 is specified in the Set method.
	// A value of 0 means no expiration (items remain in cache until evicted).
	DefaultTTL time.Duration

	// MaxItems is the maximum number of items that can be stored in the cache.
	// This is primarily used by memory-based caches to limit memory usage.
	// When the cache reaches this limit, it may evict older items to make room for new ones.
	// A value of 0 means no limit (bounded only by available memory).
	MaxItems int

	// LogEnabled determines whether cache operations should be logged.
	// When enabled, cache implementations may log information about cache hits, misses,
	// evictions, and other operations for debugging and monitoring purposes.
	LogEnabled bool
}

// DefaultCacheOptions returns the default cache options.
// These defaults provide a good starting point for most applications,
// but can be customized as needed.
//
// The default options are:
// - DefaultTTL: 24 hours
// - MaxItems: 10,000
// - LogEnabled: true
func DefaultCacheOptions() *CacheOptions {
	return &CacheOptions{
		DefaultTTL: time.Hour * 24,
		MaxItems:   10000,
		LogEnabled: true,
	}
}
