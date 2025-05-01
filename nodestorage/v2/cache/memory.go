package cache

import (
	"context"
	"sync"
	"time"
)

// CacheItem represents an item in the memory cache
type CacheItem[T any] struct {
	Data       T
	ExpiresAt  time.Time
	LastAccess time.Time // 마지막 접근 시간 추가
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
func (c *MemoryCache[T]) Get(ctx context.Context, key string) (T, error) {
	var empty T

	c.mu.RLock()
	item, ok := c.items[key]
	c.mu.RUnlock()

	if !ok {
		return empty, ErrCacheMiss
	}

	// Check if item has expired
	if !item.ExpiresAt.IsZero() && time.Now().After(item.ExpiresAt) {
		// Remove expired item
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return empty, ErrCacheMiss
	}

	// Update last access time
	c.mu.Lock()
	item.LastAccess = time.Now()
	c.items[key] = item
	c.mu.Unlock()

	return item.Data, nil
}

// Set stores a document in the cache with an optional TTL
func (c *MemoryCache[T]) Set(ctx context.Context, key string, data T, ttl time.Duration) error {
	// Use default TTL if not provided
	if ttl <= 0 {
		ttl = c.options.DefaultTTL
	}

	// Calculate expiration time
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	now := time.Now()

	// Create cache item
	item := CacheItem[T]{
		Data:       data,
		ExpiresAt:  expiresAt,
		LastAccess: now,
	}

	// Store in cache
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we need to enforce MaxItems limit
	if c.options.MaxItems > 0 && len(c.items) >= c.options.MaxItems {
		// Check if this is a new item (not an update)
		_, exists := c.items[key]
		if !exists {
			// 가장 오래된 키를 찾기 위한 맵
			keyInsertOrder := make(map[string]time.Time)

			// 모든 항목의 키와 마지막 접근 시간을 맵에 저장
			for k, v := range c.items {
				keyInsertOrder[k] = v.LastAccess
			}

			// 가장 오래된 키 찾기
			var oldestKey string
			var oldestTime time.Time
			first := true

			for k, t := range keyInsertOrder {
				if first || t.Before(oldestTime) {
					oldestKey = k
					oldestTime = t
					first = false
				}
			}

			if oldestKey != "" {
				delete(c.items, oldestKey)
			}
		}
	}

	c.items[key] = item
	return nil
}

// Delete removes a document from the cache
func (c *MemoryCache[T]) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
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
