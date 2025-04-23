package cache

// // BadgerCache implements the Cache interface using BadgerDB
// type BadgerCache struct {
// 	db        *badger.DB
// 	options   *Options
// 	itemCount int64
// 	closed    bool
// }

// // NewBadgerCache creates a new BadgerDB cache
// func NewBadgerCache(options *Options) (*BadgerCache, error) {
// 	if options == nil {
// 		options = DefaultOptions()
// 	}

// 	// Configure BadgerDB options
// 	opts := badger.DefaultOptions(options.Path)
// 	opts.Logger = nil // Disable logging

// 	if options.InMemory {
// 		opts = opts.WithInMemory(true)
// 	}

// 	// Open BadgerDB
// 	db, err := badger.Open(opts)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to open BadgerDB: %w", err)
// 	}

// 	cache := &BadgerCache{
// 		db:      db,
// 		options: options,
// 	}

// 	// Start background eviction process
// 	go cache.startEvictionProcess()

// 	return cache, nil
// }

// // Get retrieves a document from the cache
// func (c *BadgerCache) Get(ctx context.Context, id interface{}) ([]byte, error) {
// 	if c.closed {
// 		return nil, errors.New("cache is closed")
// 	}

// 	// Convert id to string for BadgerDB key
// 	key := fmt.Sprintf("%v", id)

// 	var data []byte
// 	err := c.db.View(func(txn *badger.Txn) error {
// 		item, err := txn.Get([]byte(key))
// 		if err != nil {
// 			if err == badger.ErrKeyNotFound {
// 				return fmt.Errorf("document not found in cache: %w", err)
// 			}
// 			return err
// 		}

// 		// Copy value
// 		data, err = item.ValueCopy(nil)
// 		return err
// 	})

// 	if err != nil {
// 		return nil, err
// 	}

// 	return data, nil
// }

// // Set stores a document in the cache with optional TTL
// func (c *BadgerCache) Set(ctx context.Context, id interface{}, data []byte, ttl time.Duration) error {
// 	if c.closed {
// 		return errors.New("cache is closed")
// 	}

// 	// Convert id to string for BadgerDB key
// 	key := fmt.Sprintf("%v", id)

// 	// Use default TTL if not specified
// 	if ttl == 0 {
// 		ttl = c.options.DefaultTTL
// 	}

// 	err := c.db.Update(func(txn *badger.Txn) error {
// 		entry := badger.NewEntry([]byte(key), data).WithTTL(ttl)
// 		return txn.SetEntry(entry)
// 	})

// 	if err != nil {
// 		return fmt.Errorf("failed to set cache entry: %w", err)
// 	}

// 	return nil
// }

// // Delete removes a document from the cache
// func (c *BadgerCache) Delete(ctx context.Context, id interface{}) error {
// 	if c.closed {
// 		return errors.New("cache is closed")
// 	}

// 	// Convert id to string for BadgerDB key
// 	key := fmt.Sprintf("%v", id)

// 	err := c.db.Update(func(txn *badger.Txn) error {
// 		return txn.Delete([]byte(key))
// 	})

// 	if err != nil {
// 		return fmt.Errorf("failed to delete cache entry: %w", err)
// 	}

// 	return nil
// }

// // Clear removes all documents from the cache
// func (c *BadgerCache) Clear(ctx context.Context) error {
// 	if c.closed {
// 		return errors.New("cache is closed")
// 	}

// 	// Drop all data
// 	return c.db.DropAll()
// }

// // Close closes the cache
// func (c *BadgerCache) Close() error {
// 	if c.closed {
// 		return nil
// 	}

// 	c.closed = true
// 	return c.db.Close()
// }

// // startEvictionProcess starts the background eviction process
// func (c *BadgerCache) startEvictionProcess() {
// 	ticker := time.NewTicker(c.options.EvictionInterval)
// 	defer ticker.Stop()

// 	for {
// 		if c.closed {
// 			return
// 		}

// 		select {
// 		case <-ticker.C:
// 			// Run garbage collection
// 			err := c.db.RunValueLogGC(0.5)
// 			if err != nil && err != badger.ErrNoRewrite {
// 				// Log error but continue
// 				fmt.Printf("Error during BadgerDB GC: %v\n", err)
// 			}

// 			// Check if we need to enforce size limits
// 			if c.options.MaxSize > 0 {
// 				c.enforceMaxSize()
// 			}
// 		}
// 	}
// }

// // enforceMaxSize ensures the cache doesn't exceed the maximum size
// func (c *BadgerCache) enforceMaxSize() {
// 	// Count items
// 	var count int64
// 	err := c.db.View(func(txn *badger.Txn) error {
// 		opts := badger.DefaultIteratorOptions
// 		opts.PrefetchValues = false
// 		it := txn.NewIterator(opts)
// 		defer it.Close()

// 		for it.Rewind(); it.Valid(); it.Next() {
// 			count++
// 		}
// 		return nil
// 	})

// 	if err != nil {
// 		fmt.Printf("Error counting cache items: %v\n", err)
// 		return
// 	}

// 	c.itemCount = count

// 	// If we're under the limit, nothing to do
// 	if count <= c.options.MaxSize {
// 		return
// 	}

// 	// We need to remove some items
// 	toRemove := count - c.options.MaxSize
// 	removed := int64(0)

// 	// Simple LRU-like eviction: remove oldest items first
// 	// In a real implementation, you might want to use a more sophisticated approach
// 	err = c.db.Update(func(txn *badger.Txn) error {
// 		opts := badger.DefaultIteratorOptions
// 		opts.PrefetchValues = false
// 		it := txn.NewIterator(opts)
// 		defer it.Close()

// 		for it.Rewind(); it.Valid() && removed < toRemove; it.Next() {
// 			key := it.Item().Key()
// 			if err := txn.Delete(key); err != nil {
// 				return err
// 			}
// 			removed++
// 		}
// 		return nil
// 	})

// 	if err != nil {
// 		fmt.Printf("Error during cache eviction: %v\n", err)
// 	}
// }
