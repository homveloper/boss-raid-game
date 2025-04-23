package cache

import (
	"context"
	"time"
)

// Cache defines the generic interface for cache operations
type Cache[T any] interface {
	// Get retrieves a document from the cache
	Get(ctx context.Context, id interface{}) (T, error)

	// Set stores a document in the cache with optional TTL
	Set(ctx context.Context, id interface{}, data T, ttl time.Duration) error

	// Delete removes a document from the cache
	Delete(ctx context.Context, id interface{}) error

	// Clear removes all documents from the cache
	Clear(ctx context.Context) error

	// Close closes the cache
	Close() error
}
