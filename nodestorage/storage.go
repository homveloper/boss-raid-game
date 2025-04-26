package nodestorage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"nodestorage/cache"
	"nodestorage/core/nstlog"
	"reflect"
	"sync"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// Cachable is an interface for objects that can be cached and versioned
// T must be a pointer type to ensure proper modification
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

	// CreateAndGet creates a new document or returns the existing one if it already exists.
	// This function is safe to use in distributed environments as it implements "CreateIfNotExists" semantics.
	// It returns the created or existing document directly.
	CreateAndGet(ctx context.Context, data T) (T, error)

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
	// RFC 6902 JSON Patch
	JSONPatch jsonpatch.Patch `json:"jsonPatch,omitempty"`
	// RFC 7396 JSON Merge Patch
	MergePatch []byte `json:"mergePatch,omitempty"`
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
		nstlog.Error("Failed to cache document", zap.Error(err), zap.String("id", id.Hex()))
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

// CreateAndGet creates a new document or returns the existing one if it already exists.
// This function is safe to use in distributed environments as it implements "CreateIfNotExists" semantics.
// It returns the created or existing document directly.
// Implementation uses MongoDB's FindOneAndUpdate with upsert option for atomic operation.
func (s *StorageImpl[T]) CreateAndGet(ctx context.Context, data T) (T, error) {
	var empty T

	if s.closed {
		return empty, ErrClosed
	}

	// We'll check for nil using reflection instead

	// Get the document ID
	var id primitive.ObjectID
	v := reflect.ValueOf(data)

	// Check if it's a pointer and not nil
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		// Get the element the pointer points to
		elem := v.Elem()

		// Check if the struct has an ID field of type ObjectID
		idField := elem.FieldByName("ID")
		if idField.IsValid() && idField.Type() == reflect.TypeOf(primitive.ObjectID{}) {
			// If the document already has an ID, use it
			id = idField.Interface().(primitive.ObjectID)
			if id == primitive.NilObjectID {
				// Generate a new ID if it's nil
				id = primitive.NewObjectID()
				idField.Set(reflect.ValueOf(id))
			}
		} else {
			// If the document doesn't have an ID field, generate a new one
			id = primitive.NewObjectID()
		}
	} else {
		// Not a valid pointer
		return empty, fmt.Errorf("invalid document: not a pointer or nil pointer")
	}

	// Initialize version to 1 for new documents
	data.Version(1)

	// Set up options for FindOneAndUpdate
	opts := options.FindOneAndUpdate()
	opts.SetUpsert(true)                  // Create if not exists
	opts.SetReturnDocument(options.After) // Return the document after update

	// Create filter for the document ID
	filter := bson.M{"_id": id}

	// Create update document with $setOnInsert to only set fields when document is created
	// If document already exists, this won't modify it
	update := bson.M{
		"$setOnInsert": data,
	}

	// Execute FindOneAndUpdate operation
	var result T
	err := s.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// This shouldn't happen with upsert:true, but handle it anyway
			return empty, fmt.Errorf("failed to create or get document: %w", err)
		}
		return empty, fmt.Errorf("failed to create or get document: %w", err)
	}

	// Store in cache
	if err := s.cache.Set(ctx, id, result, s.options.CacheTTL); err != nil {
		// 캐시 저장 실패는 경고로 로그하고 반환하여 사용자가 인지할 수 있도록 함
		nstlog.Warn("Document created/retrieved but failed to cache",
			zap.Error(err),
			zap.String("id", id.Hex()))
		return result, fmt.Errorf("document created/retrieved but failed to cache: %w", err)
	}

	// Return the document
	return result, nil
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
		newVersion := currentVersion + 1
		updatedDoc.Version(newVersion)

		// Generate diff
		diff, err := generateDiff(doc, updatedDoc)
		if err != nil {
			return empty, nil, fmt.Errorf("failed to generate diff: %w", err)
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
				// 캐시 업데이트 실패는 경고로 로그하고 반환하여 사용자가 인지할 수 있도록 함
				return updatedDoc, diff, fmt.Errorf("document updated but failed to update cache: %w", err)
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
				// 캐시 무효화 실패는 경고로 로그하고 반환하여 사용자가 인지할 수 있도록 함
				return empty, nil, fmt.Errorf("failed to invalidate cache for retry: %w", err)
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
		// 캐시 삭제 실패는 경고로 로그하고 반환하여 사용자가 인지할 수 있도록 함
		return fmt.Errorf("document deleted from database but failed to delete from cache: %w", err)
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
			// 이 에러는 내부적으로 발생하므로 호출자에게 직접 반환할 수 없음
			// 대신 에러 채널을 통해 에러를 전달하는 방식을 고려할 수 있음
			nstlog.Error("Subscriber channel is full, skipping event",
				zap.Int("subscriber_id", sub.ID),
				zap.String("document_id", event.ID.Hex()),
				zap.String("operation", event.Operation))
			// 향후 개선: 에러 채널을 통해 에러를 전달하는 방식 구현 고려
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

	// 먼저 closed 플래그를 설정하여 에러 로깅을 방지
	s.closed = true

	// 컨텍스트 취소 (이로 인해 Change Stream이 종료됨)
	s.cancel()

	// 잠시 대기하여 Change Stream이 정상적으로 종료되도록 함
	time.Sleep(100 * time.Millisecond)

	// Close all subscriber channels
	s.subMu.Lock()
	for id, sub := range s.subscribers {
		sub.Cancel()
		close(sub.Chan)
		delete(s.subscribers, id)
	}
	s.subMu.Unlock()

	// Note: We don't close the cache here as it was provided externally
	// and might be shared with other components

	// MongoDB 클라이언트는 외부에서 주입받았으므로 여기서 종료하지 않음
	// 클라이언트 종료는 클라이언트를 생성한 쪽에서 담당해야 함

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
				// 디코딩 에러는 심각한 문제이므로 로그만 남기고 계속 진행
				// 이 에러는 스트림 내부에서 발생하므로 호출자에게 직접 반환할 수 없음
				nstlog.Error("Error decoding change stream event", zap.Error(err))
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
			// 컨텍스트 취소로 인한 오류는 정상적인 종료이므로 로그를 출력하지 않음
			// 이 에러는 스트림 내부에서 발생하므로 호출자에게 직접 반환할 수 없음
			// 대신 에러 채널을 통해 에러를 전달하는 방식을 고려할 수 있음
			if !s.closed && !errors.Is(err, context.Canceled) {
				nstlog.Error("Change stream error", zap.Error(err))
				// 향후 개선: 에러 채널을 통해 에러를 전달하는 방식 구현 고려
			}
		}
	}()

	return nil
}

// generateDiff generates a diff between two documents
func generateDiff[T Cachable[T]](oldDoc, newDoc T) (*Diff, error) {
	// Create a basic diff
	diff := &Diff{}

	// Generate JSON Patch (RFC 6902) and JSON Merge Patch (RFC 7396)
	oldJSON, err := json.Marshal(oldDoc)
	if err != nil {
		return diff, fmt.Errorf("failed to marshal old document: %w", err)
	}

	newJSON, err := json.Marshal(newDoc)
	if err != nil {
		return diff, fmt.Errorf("failed to marshal new document: %w", err)
	}

	// Generate RFC 6902 JSON Patch
	// We need to manually create the patch since there's no direct CreatePatch function
	// First, we'll use the Equal function to check if the documents are different
	if !jsonpatch.Equal(oldJSON, newJSON) {
		// Create a patch manually
		patchJSON := []byte(fmt.Sprintf(`[{"op":"replace","path":"","value":%s}]`, string(newJSON)))
		patch, err := jsonpatch.DecodePatch(patchJSON)
		if err != nil {
			nstlog.Warn("Failed to create JSON patch", zap.Error(err))
		} else {
			diff.JSONPatch = patch
		}
	}

	// Generate RFC 7396 JSON Merge Patch
	mergePatch, err := jsonpatch.CreateMergePatch(oldJSON, newJSON)
	if err != nil {
		nstlog.Warn("Failed to create merge patch", zap.Error(err))
	} else {
		diff.MergePatch = mergePatch
	}

	return diff, nil
}
