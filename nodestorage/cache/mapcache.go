package cache

// // MapCache implements the Cache interface using an in-memory map
// type MapCache struct {
// 	data      map[string]cacheEntry
// 	mu        sync.RWMutex
// 	options   *Options
// 	itemCount int64
// 	closed    bool
// }

// // cacheEntry represents a cached item with expiration
// type cacheEntry struct {
// 	data       []byte
// 	expiration time.Time
// }

// // NewMapCache creates a new in-memory map cache
// func NewMapCache(options *Options) *MapCache {
// 	if options == nil {
// 		options = DefaultOptions()
// 	}

// 	cache := &MapCache{
// 		data:    make(map[string]cacheEntry),
// 		options: options,
// 	}

// 	// Start background eviction process
// 	go cache.startEvictionProcess()

// 	return cache
// }

// // Get retrieves a document from the cache
// func (c *MapCache) Get(ctx context.Context, id interface{}) ([]byte, error) {
// 	if c.closed {
// 		return nil, errors.New("cache is closed")
// 	}

// 	// Convert id to string for map key
// 	key := fmt.Sprintf("%v", id)

// 	c.mu.RLock()
// 	entry, ok := c.data[key]
// 	c.mu.RUnlock()

// 	if !ok {
// 		return nil, errors.New("document not found in cache")
// 	}

// 	// Check if expired
// 	if time.Now().After(entry.expiration) {
// 		c.mu.Lock()
// 		delete(c.data, key)
// 		c.mu.Unlock()
// 		return nil, errors.New("document expired in cache")
// 	}

// 	return entry.data, nil
// }

// // Set stores a document in the cache with optional TTL
// func (c *MapCache) Set(ctx context.Context, id interface{}, data []byte, ttl time.Duration) error {
// 	if c.closed {
// 		return errors.New("cache is closed")
// 	}

// 	// Convert id to string for map key
// 	key := fmt.Sprintf("%v", id)

// 	// Use default TTL if not specified
// 	if ttl == 0 {
// 		ttl = c.options.DefaultTTL
// 	}

// 	// Calculate expiration time
// 	expiration := time.Now().Add(ttl)

// 	c.mu.Lock()
// 	defer c.mu.Unlock()

// 	// Store in cache
// 	c.data[key] = cacheEntry{
// 		data:       data,
// 		expiration: expiration,
// 	}

// 	return nil
// }

// // Delete removes a document from the cache
// func (c *MapCache) Delete(ctx context.Context, id interface{}) error {
// 	if c.closed {
// 		return errors.New("cache is closed")
// 	}

// 	// Convert id to string for map key
// 	key := fmt.Sprintf("%v", id)

// 	c.mu.Lock()
// 	defer c.mu.Unlock()

// 	delete(c.data, key)
// 	return nil
// }

// // Clear removes all documents from the cache
// func (c *MapCache) Clear(ctx context.Context) error {
// 	if c.closed {
// 		return errors.New("cache is closed")
// 	}

// 	c.mu.Lock()
// 	defer c.mu.Unlock()

// 	c.data = make(map[string]cacheEntry)
// 	return nil
// }

// // Close closes the cache
// func (c *MapCache) Close() error {
// 	if c.closed {
// 		return nil
// 	}

// 	c.closed = true
// 	return nil
// }

// // startEvictionProcess starts the background eviction process
// func (c *MapCache) startEvictionProcess() {
// 	ticker := time.NewTicker(c.options.EvictionInterval)
// 	defer ticker.Stop()

// 	for {
// 		if c.closed {
// 			return
// 		}

// 		select {
// 		case <-ticker.C:
// 			c.evictExpired()

// 			// Check if we need to enforce size limits
// 			if c.options.MaxSize > 0 {
// 				c.enforceMaxSize()
// 			}
// 		}
// 	}
// }

// // evictExpired removes expired entries from the cache
// func (c *MapCache) evictExpired() {
// 	now := time.Now()

// 	c.mu.Lock()
// 	defer c.mu.Unlock()

// 	for key, entry := range c.data {
// 		if now.After(entry.expiration) {
// 			delete(c.data, key)
// 		}
// 	}

// 	c.itemCount = int64(len(c.data))
// }

// // enforceMaxSize ensures the cache doesn't exceed the maximum size
// func (c *MapCache) enforceMaxSize() {
// 	c.mu.Lock()
// 	defer c.mu.Unlock()

// 	count := int64(len(c.data))
// 	c.itemCount = count

// 	// If we're under the limit, nothing to do
// 	if count <= c.options.MaxSize {
// 		return
// 	}

// 	// We need to remove some items
// 	toRemove := count - c.options.MaxSize
// 	removed := int64(0)

// 	// Simple LRU-like eviction: remove oldest items first
// 	// In a real implementation, you might want to track access times
// 	// and remove least recently used items
// 	for key := range c.data {
// 		if removed >= toRemove {
// 			break
// 		}

// 		delete(c.data, key)
// 		removed++
// 	}
// }
