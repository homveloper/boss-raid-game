package nodestorage

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"boss-raid-game/nodestorage/cache"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Cachable is an interface for objects that can be cached and versioned
type Cachable[T any] interface {
	Copy() T                // Create a deep copy of the object
	Version(...int64) int64 // Get or set the version (if argument is provided)
}

// Storage is the main storage interface with generic support
type Storage[T Cachable[T]] interface {
	// Get retrieves a document by ID
	Get(ctx context.Context, id primitive.ObjectID) (T, error)

	// GetByQuery retrieves documents using a query
	GetByQuery(ctx context.Context, query interface{}) ([]T, error)

	// Create creates a new document
	Create(ctx context.Context, data T) (primitive.ObjectID, error)

	// Edit edits a document with optimistic concurrency control
	Edit(ctx context.Context, id primitive.ObjectID, editFn EditFunc[T], opts ...EditOption) (T, *Diff, error)

	// Delete deletes a document
	Delete(ctx context.Context, id primitive.ObjectID) error

	// Watch watches for changes to documents
	Watch(ctx context.Context) (<-chan WatchEvent[T], error)

	// Close closes the storage
	Close() error

	// GetCache returns the cache implementation
	GetCache() cache.Cache[T]
}

// EditFunc is a function that edits a document
type EditFunc[T Cachable[T]] func(doc T) (T, error)

// WatchEvent represents a document change event
type WatchEvent[T Cachable[T]] struct {
	ID        primitive.ObjectID
	Operation string // "create", "update", "delete"
	Data      T
	Diff      *Diff
}

// Diff represents the difference between two document versions
type Diff struct {
	Operations []Operation `json:"operations"`
}

// Operation represents a single change operation
type Operation struct {
	Type  string      `json:"type"`           // "add", "remove", "replace", "move", "copy", "test"
	Path  string      `json:"path"`           // JSON pointer path
	Value interface{} `json:"value"`          // New value for add/replace operations
	From  string      `json:"from,omitempty"` // Source path for move/copy operations
}

// Subscriber represents a watch subscriber
type Subscriber[T Cachable[T]] struct {
	ID     int
	Chan   chan WatchEvent[T]
	Ctx    context.Context
	Cancel context.CancelFunc
}

// StorageImpl implements the Storage interface
type StorageImpl[T Cachable[T]] struct {
	client      *mongo.Client
	collection  *mongo.Collection
	cache       cache.Cache[T]
	options     *Options
	ctx         context.Context
	cancel      context.CancelFunc
	closed      bool
	closeMu     sync.Mutex
	subscribers map[int]*Subscriber[T]
	subMu       sync.RWMutex
	nextSubID   int
}

// NewStorage creates a new storage instance
func NewStorage[T Cachable[T]](ctx context.Context,
	client *mongo.Client,
	collection *mongo.Collection,
	cacheImpl cache.Cache[T],
	options *Options) (*StorageImpl[T], error) {
	if options == nil {
		options = DefaultOptions()
	}

	// Validate required options
	if options.VersionField == "" {
		return nil, ErrMissingVersionField
	}

	// Validate cache dependency
	if cacheImpl == nil {
		return nil, fmt.Errorf("cache implementation is required")
	}

	// Create context with cancel
	storageCtx, cancel := context.WithCancel(ctx)

	storage := &StorageImpl[T]{
		client:      client,
		collection:  collection,
		cache:       cacheImpl,
		options:     options,
		ctx:         storageCtx,
		cancel:      cancel,
		subscribers: make(map[int]*Subscriber[T]),
		nextSubID:   1,
	}

	// Start watching for changes if enabled
	if options.WatchEnabled {
		if err := storage.startWatching(); err != nil {
			storage.Close()
			return nil, fmt.Errorf("failed to start watching: %w", err)
		}
	}

	return storage, nil
}

// Get retrieves a document by ID
func (s *StorageImpl[T]) Get(ctx context.Context, id primitive.ObjectID) (T, error) {
	var result T

	if s.closed {
		return result, ErrClosed
	}

	// Try to get from cache first
	doc, err := s.cache.Get(ctx, id)
	if err == nil {
		return doc, nil
	}

	// If not in cache, get from database
	var dbDoc bson.M
	err = s.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&dbDoc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return result, ErrNotFound
		}
		return result, fmt.Errorf("failed to get document: %w", err)
	}

	// Extract data field
	dataBytes, err := bson.Marshal(dbDoc)
	if err != nil {
		return result, fmt.Errorf("failed to marshal document: %w", err)
	}

	// Deserialize document
	if err := bson.Unmarshal(dataBytes, &result); err != nil {
		return result, fmt.Errorf("failed to unmarshal document: %w", err)
	}

	// Store in cache
	if err := s.cache.Set(ctx, id, result, s.options.CacheTTL); err != nil {
		// Log error but continue
		fmt.Printf("Failed to cache document: %v\n", err)
	}

	return result, nil
}

// GetByQuery retrieves documents using a query
func (s *StorageImpl[T]) GetByQuery(ctx context.Context, query interface{}) ([]T, error) {
	if s.closed {
		return nil, ErrClosed
	}

	// Query can only be executed against the database
	cursor, err := s.collection.Find(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer cursor.Close(ctx)

	var results []T
	for cursor.Next(ctx) {
		var dbDoc bson.M
		if err := cursor.Decode(&dbDoc); err != nil {
			return nil, fmt.Errorf("failed to decode document: %w", err)
		}

		// Extract data field
		dataBytes, err := bson.Marshal(dbDoc)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal document: %w", err)
		}

		// Deserialize document
		var doc T
		if err := bson.Unmarshal(dataBytes, &doc); err != nil {
			return nil, fmt.Errorf("failed to unmarshal document: %w", err)
		}

		results = append(results, doc)
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return results, nil
}

// Create creates a new document
func (s *StorageImpl[T]) Create(ctx context.Context, data T) (primitive.ObjectID, error) {
	if s.closed {
		return primitive.NilObjectID, ErrClosed
	}

	// Initialize version to 1
	data.Version(1)

	// Generate new ObjectID
	id := primitive.NewObjectID()

	// Insert into database
	_, err := s.collection.InsertOne(ctx, data)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return primitive.NilObjectID, fmt.Errorf("document already exists: %w", err)
		}
		return primitive.NilObjectID, fmt.Errorf("failed to insert document: %w", err)
	}

	// Store in cache
	if err := s.cache.Set(ctx, id, data, s.options.CacheTTL); err != nil {
		// Log error but continue
		fmt.Printf("Failed to cache document: %v\n", err)
	}

	return id, nil
}

// Edit edits a document with optimistic concurrency control
func (s *StorageImpl[T]) Edit(ctx context.Context, id primitive.ObjectID, editFn EditFunc[T], options ...EditOption) (T, *Diff, error) {
	var empty T

	if s.closed {
		return empty, nil, ErrClosed
	}

	// Create options with defaults and apply provided options
	opts := NewEditOptions(options...)

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	// Use the version field from storage options
	versionField := s.options.VersionField // This should never be empty due to validation in NewStorage

	var (
		retries    int
		retryDelay = opts.RetryDelay
		lastErr    error
	)

	for opts.MaxRetries == 0 || retries < opts.MaxRetries {
		// Get the document
		doc, err := s.Get(timeoutCtx, id)
		if err != nil {
			return empty, nil, err
		}

		// Get current version
		currentVersion := doc.Version()

		// Create a copy of the document for editing
		docCopy := doc.Copy()

		// Apply edit function to the document
		updatedDoc, err := editFn(docCopy)
		if err != nil {
			return empty, nil, fmt.Errorf("edit function failed: %w", err)
		}

		// Increment version
		updatedDoc.Version(currentVersion + 1)

		// Generate diff
		diff, err := generateDiff(doc, updatedDoc)
		if err != nil {
			// Log error but continue
			fmt.Printf("Failed to generate diff: %v\n", err)
		}

		// Update in database with version check
		result, err := s.collection.UpdateOne(
			timeoutCtx,
			bson.M{
				"_id":        id,
				versionField: currentVersion,
			},
			bson.M{
				"$set": updatedDoc,
			},
		)

		if err == nil && result.MatchedCount > 0 {
			// Update succeeded

			// Update cache
			if err := s.cache.Set(timeoutCtx, id, updatedDoc, s.options.CacheTTL); err != nil {
				// Log error but continue
				fmt.Printf("Failed to update cache: %v\n", err)
			}

			return updatedDoc, diff, nil
		}

		// Update failed, check if it's a version conflict
		if err == nil && result.MatchedCount == 0 {
			// Version conflict, retry
			lastErr = ErrVersionMismatch
			retries++

			// Add jitter to retry delay
			jitter := float64(retryDelay) * opts.RetryJitter * (rand.Float64()*2 - 1)
			delay := time.Duration(float64(retryDelay) + jitter)

			// Exponential backoff with cap
			retryDelay = time.Duration(math.Min(
				float64(opts.MaxRetryDelay),
				float64(retryDelay)*2,
			))

			// Wait before retrying
			select {
			case <-time.After(delay):
				// Continue with retry
			case <-timeoutCtx.Done():
				return empty, nil, fmt.Errorf("operation timed out: %w", timeoutCtx.Err())
			}

			// Invalidate cache to get fresh data on next retry
			if err := s.cache.Delete(timeoutCtx, id); err != nil {
				// Log error but continue
				fmt.Printf("Failed to invalidate cache: %v\n", err)
			}

			continue
		}

		// Other error, return immediately
		if err != nil {
			return empty, nil, fmt.Errorf("failed to update document: %w", err)
		}
	}

	return empty, nil, fmt.Errorf("maximum retries exceeded: %w", lastErr)
}

// Delete deletes a document
func (s *StorageImpl[T]) Delete(ctx context.Context, id primitive.ObjectID) error {
	if s.closed {
		return ErrClosed
	}

	// Delete from database
	_, err := s.collection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	// Delete from cache
	if err := s.cache.Delete(ctx, id); err != nil {
		// Log error but continue
		fmt.Printf("Failed to delete from cache: %v\n", err)
	}

	return nil
}

// Watch watches for changes to documents
func (s *StorageImpl[T]) Watch(ctx context.Context) (<-chan WatchEvent[T], error) {
	if s.closed {
		return nil, ErrClosed
	}

	// Create a new context with cancellation
	subCtx, subCancel := context.WithCancel(ctx)

	// Create a new channel for this subscriber
	ch := make(chan WatchEvent[T], 100)

	// Create a new subscriber
	s.subMu.Lock()
	subID := s.nextSubID
	s.nextSubID++
	sub := &Subscriber[T]{
		ID:     subID,
		Chan:   ch,
		Ctx:    subCtx,
		Cancel: subCancel,
	}
	s.subscribers[subID] = sub
	s.subMu.Unlock()

	// Start a goroutine to clean up the subscriber when the context is done
	go func() {
		<-ctx.Done()
		s.removeSubscriber(subID)
	}()

	return ch, nil
}

// removeSubscriber removes a subscriber by ID
func (s *StorageImpl[T]) removeSubscriber(id int) {
	s.subMu.Lock()
	defer s.subMu.Unlock()

	if sub, ok := s.subscribers[id]; ok {
		// Cancel the subscriber's context
		sub.Cancel()

		// Close the subscriber's channel
		close(sub.Chan)

		// Remove the subscriber from the map
		delete(s.subscribers, id)
	}
}

// broadcastEvent broadcasts an event to all subscribers
func (s *StorageImpl[T]) broadcastEvent(event WatchEvent[T]) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()

	for _, sub := range s.subscribers {
		select {
		case sub.Chan <- event:
			// Event sent successfully
		case <-sub.Ctx.Done():
			// Subscriber context is done, will be cleaned up separately
		default:
			// Channel is full, skip this subscriber
			fmt.Printf("Subscriber %d channel is full, skipping event\n", sub.ID)
		}
	}
}

// GetCache returns the cache implementation
func (s *StorageImpl[T]) GetCache() cache.Cache[T] {
	return s.cache
}

// Close closes the storage
func (s *StorageImpl[T]) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return nil
	}

	s.closed = true
	s.cancel()

	// Close all subscriber channels
	s.subMu.Lock()
	for id, sub := range s.subscribers {
		sub.Cancel()
		close(sub.Chan)
		delete(s.subscribers, id)
	}
	s.subMu.Unlock()

	// Close connections
	var errs []error

	// Note: We don't close the cache here as it was provided externally
	// and might be shared with other components

	if err := s.client.Disconnect(context.Background()); err != nil {
		errs = append(errs, fmt.Errorf("failed to close MongoDB connection: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing storage: %v", errs)
	}

	return nil
}

// startWatching starts watching for changes
func (s *StorageImpl[T]) startWatching() error {
	// Configure change stream options

	opts := options.ChangeStream()
	if s.options.WatchFullDocument != "" {
		switch s.options.WatchFullDocument {
		case "updateLookup":
			opts.SetFullDocument(options.UpdateLookup)
		case "required":
			opts.SetFullDocument(options.Required)
		}
	} else {
		opts.SetFullDocument(options.UpdateLookup)
	}

	if s.options.WatchMaxAwaitTime > 0 {
		opts.SetMaxAwaitTime(s.options.WatchMaxAwaitTime)
	}

	if s.options.WatchBatchSize > 0 {
		opts.SetBatchSize(s.options.WatchBatchSize)
	}

	// Create pipeline
	var pipeline mongo.Pipeline
	if len(s.options.WatchFilter) > 0 {
		pipeline = mongo.Pipeline(s.options.WatchFilter)
	} else {
		pipeline = mongo.Pipeline{
			bson.D{{Key: "$match", Value: bson.D{{Key: "operationType", Value: bson.D{{Key: "$in", Value: bson.A{"insert", "update", "delete"}}}}}}},
		}
	}

	// Create change stream
	stream, err := s.collection.Watch(s.ctx, pipeline, opts)
	if err != nil {
		return fmt.Errorf("failed to create change stream: %w", err)
	}

	// Note: We're using a dynamic approach to handle the version field
	// so we don't need to explicitly reference s.options.VersionField here

	// Start goroutine to handle database events
	go func() {
		defer stream.Close(context.Background())

		for stream.Next(s.ctx) {
			// Create a dynamic event structure
			var rawEvent bson.M
			if err := stream.Decode(&rawEvent); err != nil {
				// Log error but continue
				fmt.Printf("Error decoding change stream event: %v\n", err)
				continue
			}

			// Extract operation type
			operationType, _ := rawEvent["operationType"].(string)

			// Extract document ID
			var docID primitive.ObjectID
			if docKey, ok := rawEvent["documentKey"].(bson.M); ok {
				if id, ok := docKey["_id"].(primitive.ObjectID); ok {
					docID = id
				}
			}

			// Extract document data
			var docData T
			if fullDoc, ok := rawEvent["fullDocument"].(bson.M); ok {
				// Convert to bytes and unmarshal
				dataBytes, err := bson.Marshal(fullDoc)
				if err == nil {
					_ = bson.Unmarshal(dataBytes, &docData)
				}
			}

			// Map database operation to watch operation
			operation := operationType
			if operation == "insert" {
				operation = "create"
			}

			// Create watch event
			watchEvent := WatchEvent[T]{
				ID:        docID,
				Operation: operation,
				Data:      docData,
			}

			// Broadcast event to all subscribers
			s.broadcastEvent(watchEvent)
		}

		if err := stream.Err(); err != nil {
			// Log error
			fmt.Printf("Change stream error: %v\n", err)
		}
	}()

	return nil
}

// generateDiff generates a diff between two documents
func generateDiff[T Cachable[T]](oldDoc, newDoc T) (*Diff, error) {
	// This is a simplified diff implementation
	// In a real implementation, you would use a proper diff algorithm
	// to generate more granular operations

	// For now, we just create a simple replacement operation
	return &Diff{
		Operations: []Operation{
			{
				Type:  "replace",
				Path:  "",
				Value: newDoc,
			},
		},
	}, nil
}
