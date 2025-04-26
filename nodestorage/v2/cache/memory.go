package cache

import (
	"context"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CacheItem represents an item in the memory cache
type CacheItem[T any] struct {
	Data      T
	ExpiresAt time.Time
}

// MemoryCache implements the Cache interface using in-memory storage
type MemoryCache[T any] struct {
	items   map[string]CacheItem[T]
	mu      sync.RWMutex
	options *CacheOptions
}

// NewMemoryCache creates a new MemoryCache instance
func NewMemoryCache[T any](options *CacheOptions) *MemoryCache[T] {
	if options == nil {
		options = DefaultCacheOptions()
	}

	cache := &MemoryCache[T]{
		items:   make(map[string]CacheItem[T]),
		options: options,
	}

	// Start cleanup goroutine if MaxItems is set
	if options.MaxItems > 0 {
		go cache.cleanup()
	}

	return cache
}

// Get retrieves a document from the cache
func (c *MemoryCache[T]) Get(ctx context.Context, id primitive.ObjectID) (T, error) {
	var empty T

	c.mu.RLock()
	item, ok := c.items[id.Hex()]
	c.mu.RUnlock()

	if !ok {
		return empty, ErrCacheMiss
	}

	// Check if item has expired
	if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
		// Remove expired item
		c.mu.Lock()
		delete(c.items, id.Hex())
		c.mu.Unlock()
		return empty, ErrCacheMiss
	}

	return item.Data, nil
}

// Set stores a document in the cache with an optional TTL
func (c *MemoryCache[T]) Set(ctx context.Context, id primitive.ObjectID, data T, ttl time.Duration) error {
	// Use default TTL if not provided
	if ttl <= 0 {
		ttl = c.options.DefaultTTL
	}

	// Calculate expiration time
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	// Create cache item
	item := CacheItem[T]{
		Data:      data,
		ExpiresAt: expiresAt,
	}

	// Store in cache
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to enforce MaxItems limit
	if c.options.MaxItems > 0 && len(c.items) >= c.options.MaxItems {
		// Check if this is a new item (not an update)
		_, exists := c.items[id.Hex()]
		if !exists {
			// Remove oldest item (simple implementation - in a real system, use LRU)
			var oldestKey string
			var oldestTime time.Time

			for key, item := range c.items {
				if oldestKey == "" || (item.ExpiresAt.After(time.Time{}) && item.ExpiresAt.Before(oldestTime)) ||
					(oldestTime.IsZero() && !item.ExpiresAt.IsZero()) {
					oldestKey = key
					oldestTime = item.ExpiresAt
				}
			}

			// If no item with expiration found, just take the first one
			if oldestKey == "" {
				for key := range c.items {
					oldestKey = key
					break
				}
			}

			if oldestKey != "" {
				delete(c.items, oldestKey)
			}
		}
	}

	c.items[id.Hex()] = item
	return nil
}

// Delete removes a document from the cache
func (c *MemoryCache[T]) Delete(ctx context.Context, id primitive.ObjectID) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, id.Hex())
	return nil
}

// Clear removes all documents from the cache
func (c *MemoryCache[T]) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]CacheItem[T])
	return nil
}

// Close closes the cache
func (c *MemoryCache[T]) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = nil
	return nil
}

// cleanup periodically removes expired items from the cache
func (c *MemoryCache[T]) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()

		for key, item := range c.items {
			if !item.ExpiresAt.IsZero() && now.After(item.ExpiresAt) {
				delete(c.items, key)
			}
		}

		c.mu.Unlock()
	}
}

// MemoryCacheOptions represents additional options for MemoryCache
type MemoryCacheOptions struct {
	// Base cache options
	CacheOptions

	// MemoryCache specific options
	CleanupInterval time.Duration
}

// DefaultMemoryCacheOptions returns the default MemoryCache options
func DefaultMemoryCacheOptions() *MemoryCacheOptions {
	return &MemoryCacheOptions{
		CacheOptions:    *DefaultCacheOptions(),
		CleanupInterval: time.Minute,
	}
}

// NewMemoryCacheWithOptions creates a new MemoryCache with custom options
func NewMemoryCacheWithOptions[T any](options *MemoryCacheOptions) *MemoryCache[T] {
	if options == nil {
		options = DefaultMemoryCacheOptions()
	}

	cache := &MemoryCache[T]{
		items:   make(map[string]CacheItem[T]),
		options: &options.CacheOptions,
	}

	// Start cleanup goroutine
	go func() {
		ticker := time.NewTicker(options.CleanupInterval)
		defer ticker.Stop()

		for range ticker.C {
			cache.mu.Lock()
			now := time.Now()

			for key, item := range cache.items {
				if !item.ExpiresAt.IsZero() && now.After(item.ExpiresAt) {
					delete(cache.items, key)
				}
			}

			cache.mu.Unlock()
		}
	}()

	return cache
}
