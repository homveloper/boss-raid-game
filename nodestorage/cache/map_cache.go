package cache

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MapCache is an in-memory cache implementation using a map
type MapCache[T any] struct {
	data      map[string]cacheEntry[T]
	mu        sync.RWMutex
	options   *MapCacheOptions
	closeChan chan struct{}
	closed    bool
}

// cacheEntry represents a cached item with expiration
type cacheEntry[T any] struct {
	Value      T
	Expiration time.Time
}

// NewMapCache creates a new in-memory map cache with the provided options
func NewMapCache[T any](opts ...MapCacheOption) *MapCache[T] {
	// Start with default options
	options := DefaultMapCacheOptions()

	// Apply provided options
	for _, opt := range opts {
		opt(options)
	}

	cache := &MapCache[T]{
		data:      make(map[string]cacheEntry[T]),
		options:   options,
		closeChan: make(chan struct{}),
	}

	// Start eviction goroutine if eviction interval is set
	if options.EvictionInterval > 0 {
		go cache.evictionLoop()
	}

	return cache
}

// Get retrieves a document from the cache
func (c *MapCache[T]) Get(ctx context.Context, id interface{}) (T, error) {
	var empty T

	// Convert ID to string for map key
	key := fmt.Sprintf("%v", id)

	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return empty, fmt.Errorf("cache is closed")
	}

	entry, ok := c.data[key]
	if !ok {
		return empty, ErrCacheMiss
	}

	// Check if entry has expired
	if !entry.Expiration.IsZero() && time.Now().After(entry.Expiration) {
		// Entry has expired, remove it
		delete(c.data, key)
		return empty, ErrCacheMiss
	}

	return entry.Value, nil
}

// Set stores a document in the cache with optional TTL
func (c *MapCache[T]) Set(ctx context.Context, id interface{}, data T, ttl time.Duration) error {
	// Convert ID to string for map key
	key := fmt.Sprintf("%v", id)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("cache is closed")
	}

	// Calculate expiration time
	var expiration time.Time
	if ttl > 0 {
		expiration = time.Now().Add(ttl)
	} else if c.options.DefaultTTL > 0 {
		expiration = time.Now().Add(c.options.DefaultTTL)
	}

	// Store in cache
	c.data[key] = cacheEntry[T]{
		Value:      data,
		Expiration: expiration,
	}

	return nil
}

// Delete removes a document from the cache
func (c *MapCache[T]) Delete(ctx context.Context, id interface{}) error {
	// Convert ID to string for map key
	key := fmt.Sprintf("%v", id)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("cache is closed")
	}

	delete(c.data, key)
	return nil
}

// Clear removes all documents from the cache
func (c *MapCache[T]) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("cache is closed")
	}

	c.data = make(map[string]cacheEntry[T])
	return nil
}

// Close closes the cache
func (c *MapCache[T]) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.closeChan)
	return nil
}

// evictionLoop periodically removes expired entries
func (c *MapCache[T]) evictionLoop() {
	ticker := time.NewTicker(c.options.EvictionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.evictExpired()
		case <-c.closeChan:
			return
		}
	}
}

// evictExpired removes all expired entries
func (c *MapCache[T]) evictExpired() {
	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.data {
		if !entry.Expiration.IsZero() && now.After(entry.Expiration) {
			delete(c.data, key)
		}
	}
}
