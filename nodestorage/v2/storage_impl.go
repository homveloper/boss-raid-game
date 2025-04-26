package v2

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"nodestorage/v2/cache"
	"nodestorage/v2/core"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
	"go.uber.org/zap"
)

// Subscriber represents a watch subscriber
type Subscriber[T Cachable[T]] struct {
	ID     int
	Chan   chan WatchEvent[T]
	Ctx    context.Context
	Cancel context.CancelFunc
}

// StorageImpl implements the Storage interface
type StorageImpl[T Cachable[T]] struct {
	client         *mongo.Client
	collection     *mongo.Collection
	cache          cache.Cache[T]
	options        *Options
	ctx            context.Context
	cancel         context.CancelFunc
	closed         bool
	closeMu        sync.Mutex
	subscribers    map[int]*Subscriber[T]
	subMu          sync.RWMutex
	nextSubID      int
	versionField   string // Struct field name for version
	versionBSONTag string // BSON tag name for version field
}

// NewStorage creates a new storage instance
func NewStorage[T Cachable[T]](
	ctx context.Context,
	client *mongo.Client,
	collection *mongo.Collection,
	cacheImpl cache.Cache[T],
	options *Options,
) (*StorageImpl[T], error) {
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

	// Validate that the version field exists in the struct and get its BSON tag
	var doc T
	versionField, versionBSONTag, err := validateVersionField[T](doc, options.VersionField)
	if err != nil {
		return nil, err
	}

	storage := &StorageImpl[T]{
		client:         client,
		collection:     collection,
		cache:          cacheImpl,
		options:        options,
		ctx:            storageCtx,
		cancel:         cancel,
		subscribers:    make(map[int]*Subscriber[T]),
		nextSubID:      1,
		versionField:   versionField,
		versionBSONTag: versionBSONTag,
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

// Collection returns the underlying MongoDB collection
func (s *StorageImpl[T]) Collection() *mongo.Collection {
	return s.collection
}

// FindOne retrieves a document by ID with optional MongoDB options
func (s *StorageImpl[T]) FindOne(
	ctx context.Context,
	id primitive.ObjectID,
	opts ...*options.FindOneOptions,
) (T, error) {
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
	findOpts := options.FindOne()
	if len(opts) > 0 {
		findOpts = opts[0]
	}

	var dbDoc bson.M
	err = s.collection.FindOne(ctx, bson.M{"_id": id}, findOpts).Decode(&dbDoc)
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
		core.Error("Failed to cache document",
			zap.Error(err),
			zap.String("id", id.Hex()))
	}

	return result, nil
}

// FindOneAndUpsert creates a new document or returns the existing one if it already exists.
// This function is safe to use in distributed environments as it implements "CreateIfNotExists" semantics.
func (s *StorageImpl[T]) FindOneAndUpsert(ctx context.Context, data T) (T, error) {
	var empty T

	if s.closed {
		return empty, ErrClosed
	}

	// Get the document ID
	id, err := getDocumentID(data)
	if err != nil {
		return empty, err
	}

	// Initialize version to 1 for new documents
	if err := setVersion(data, s.versionField, 1); err != nil {
		return empty, fmt.Errorf("failed to set initial version: %w", err)
	}

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
	err = s.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// This shouldn't happen with upsert:true, but handle it anyway
			return empty, fmt.Errorf("failed to create or get document: %w", err)
		}
		return empty, fmt.Errorf("failed to create or get document: %w", err)
	}

	// Store in cache
	if err := s.cache.Set(ctx, id, result, s.options.CacheTTL); err != nil {
		core.Warn("Document created/retrieved but failed to cache",
			zap.Error(err),
			zap.String("id", id.Hex()))
		return result, fmt.Errorf("document created/retrieved but failed to cache: %w", err)
	}

	// Return the document
	return result, nil
}

// FindOneAndUpdate edits a document with optimistic concurrency control using a function
func (s *StorageImpl[T]) FindOneAndUpdate(
	ctx context.Context,
	id primitive.ObjectID,
	updateFn EditFunc[T],
	opts ...EditOption,
) (T, *Diff, error) {
	var empty T

	if s.closed {
		return empty, nil, ErrClosed
	}

	// Create options with defaults and apply provided options
	editOpts := NewEditOptions(opts...)

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(editOpts.Timeout))
	defer cancel()

	// Use the version field name for reflection operations
	versionField := s.versionField
	// Use the BSON tag name for MongoDB operations
	versionBSONTag := s.versionBSONTag

	var (
		retries    int
		retryDelay = time.Duration(editOpts.RetryDelay)
		lastErr    error
	)

	for editOpts.MaxRetries == 0 || retries < editOpts.MaxRetries {
		// Get the document
		doc, err := s.FindOne(timeoutCtx, id)
		if err != nil {
			return empty, nil, err
		}

		// Get current version
		currentVersion, err := getVersion(doc, versionField)
		if err != nil {
			return empty, nil, fmt.Errorf("failed to get current version: %w", err)
		}

		// Create a copy of the document for editing
		docCopy := doc.Copy()

		// Apply edit function to the document
		updatedDoc, err := updateFn(docCopy)
		if err != nil {
			return empty, nil, fmt.Errorf("edit function failed: %w", err)
		}

		// Increment version
		newVersion := currentVersion + 1
		if err := setVersion(updatedDoc, versionField, newVersion); err != nil {
			return empty, nil, fmt.Errorf("failed to set new version: %w", err)
		}

		// Generate diff
		diff, err := generateDiff(doc, updatedDoc)
		if err != nil {
			return empty, nil, fmt.Errorf("failed to generate diff: %w", err)
		}

		// Update in database with version check
		result, err := s.collection.UpdateOne(
			timeoutCtx,
			bson.M{
				"_id":          id,
				versionBSONTag: currentVersion, // Use BSON tag name for MongoDB query
			},
			bson.M{
				"$set": updatedDoc,
			},
		)

		if err == nil && result.MatchedCount > 0 {
			// Update succeeded

			// Update cache
			if err := s.cache.Set(timeoutCtx, id, updatedDoc, s.options.CacheTTL); err != nil {
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
			jitter := float64(retryDelay) * editOpts.RetryJitter * (rand.Float64()*2 - 1)
			delay := time.Duration(float64(retryDelay) + jitter)

			// Exponential backoff with cap
			retryDelay = time.Duration(math.Min(
				float64(time.Duration(editOpts.MaxRetryDelay)),
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

// DeleteOne deletes a document
func (s *StorageImpl[T]) DeleteOne(ctx context.Context, id primitive.ObjectID) error {
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
		return fmt.Errorf("document deleted from database but failed to delete from cache: %w", err)
	}

	return nil
}

// Close closes the storage
func (s *StorageImpl[T]) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return nil
	}

	// Set closed flag first to prevent error logging
	s.closed = true

	// Cancel context (this will terminate the change stream)
	s.cancel()

	// Wait briefly to allow change stream to terminate gracefully
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

	// MongoDB client was injected externally, so we don't close it here
	// Client closure should be handled by the component that created it

	return nil
}

// This is a placeholder for a utility function

// Helper function to extract document ID
func getDocumentID[T Cachable[T]](data T) (primitive.ObjectID, error) {
	// Use reflection to get the ID field
	v := reflect.ValueOf(data)

	// Check if it's a pointer and not nil
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		// Get the element the pointer points to
		elem := v.Elem()

		// Check if the struct has an ID field of type ObjectID
		idField := elem.FieldByName("ID")
		if idField.IsValid() && idField.Type() == reflect.TypeOf(primitive.ObjectID{}) {
			// If the document already has an ID, use it
			id := idField.Interface().(primitive.ObjectID)
			if id == primitive.NilObjectID {
				// Generate a new ID if it's nil
				id = primitive.NewObjectID()
				idField.Set(reflect.ValueOf(id))
			}
			return id, nil
		}
		// If the document doesn't have an ID field, generate a new one
		return primitive.NewObjectID(), nil
	}

	// Not a valid pointer
	return primitive.NilObjectID, fmt.Errorf("invalid document: not a pointer or nil pointer")
}

// generateDiff generates a diff between two documents
func generateDiff[T Cachable[T]](oldDoc, newDoc T) (*Diff, error) {
	// This is a placeholder implementation
	// The actual implementation would generate a proper diff
	return &Diff{
		JSONPatch:  nil, // Implement JSON patch generation
		MergePatch: nil, // Implement merge patch generation
	}, nil
}

// The following methods are stubs that need to be implemented:

// FindMany retrieves documents using a query with optional MongoDB options
func (s *StorageImpl[T]) FindMany(
	ctx context.Context,
	filter interface{},
	opts ...*options.FindOptions,
) ([]T, error) {
	if s.closed {
		return nil, ErrClosed
	}

	// Apply options
	findOpts := options.Find()
	if len(opts) > 0 {
		findOpts = opts[0]
	}

	// Execute query
	cursor, err := s.collection.Find(ctx, filter, findOpts)
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

		// Add to results
		results = append(results, doc)

		// Cache the document if caching is enabled
		if s.options.CacheQueryResults {
			id, err := getDocumentID(doc)
			if err == nil {
				if err := s.cache.Set(ctx, id, doc, s.options.CacheTTL); err != nil {
					core.Warn("Failed to cache query result",
						zap.Error(err),
						zap.String("id", id.Hex()))
				}
			}
		}
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return results, nil
}

// UpdateOne allows direct use of MongoDB update operators while maintaining optimistic concurrency control
func (s *StorageImpl[T]) UpdateOne(
	ctx context.Context,
	id primitive.ObjectID,
	update bson.M,
	opts ...EditOption,
) (T, error) {
	var empty T

	if s.closed {
		return empty, ErrClosed
	}

	// Create options with defaults and apply provided options
	editOpts := NewEditOptions(opts...)

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(editOpts.Timeout))
	defer cancel()

	// Use the version field name for reflection operations
	versionField := s.versionField
	// Use the BSON tag name for MongoDB operations
	versionBSONTag := s.versionBSONTag

	// Get current version
	doc, err := s.FindOne(timeoutCtx, id)
	if err != nil {
		return empty, err
	}

	currentVersion, err := getVersion(doc, versionField)
	if err != nil {
		return empty, fmt.Errorf("failed to get current version: %w", err)
	}

	// Add version check to filter
	filter := bson.M{
		"_id":          id,
		versionBSONTag: currentVersion, // Use BSON tag name for MongoDB query
	}

	// Add version increment to update
	if _, ok := update["$inc"]; !ok {
		update["$inc"] = bson.M{}
	}
	update["$inc"].(bson.M)[versionBSONTag] = 1 // Use BSON tag name for MongoDB update

	// Update in database with version check
	result, err := s.collection.UpdateOne(timeoutCtx, filter, update)

	if err != nil {
		return empty, fmt.Errorf("failed to update document: %w", err)
	}

	if result.MatchedCount == 0 {
		return empty, ErrVersionMismatch
	}

	// Get updated document
	updatedDoc, err := s.FindOne(timeoutCtx, id)
	if err != nil {
		return empty, fmt.Errorf("document updated but failed to retrieve: %w", err)
	}

	// Update cache
	if err := s.cache.Set(timeoutCtx, id, updatedDoc, s.options.CacheTTL); err != nil {
		return updatedDoc, fmt.Errorf("document updated but failed to update cache: %w", err)
	}

	return updatedDoc, nil
}

// UpdateOneWithPipeline allows use of MongoDB aggregation pipeline for updates while maintaining optimistic concurrency control
func (s *StorageImpl[T]) UpdateOneWithPipeline(
	ctx context.Context,
	id primitive.ObjectID,
	pipeline mongo.Pipeline,
	opts ...EditOption,
) (T, error) {
	var empty T

	if s.closed {
		return empty, ErrClosed
	}

	// Create options with defaults and apply provided options
	editOpts := NewEditOptions(opts...)

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(editOpts.Timeout))
	defer cancel()

	// Use the version field name for reflection operations
	versionField := s.versionField
	// Use the BSON tag name for MongoDB operations
	versionBSONTag := s.versionBSONTag

	// Get current version
	doc, err := s.FindOne(timeoutCtx, id)
	if err != nil {
		return empty, err
	}

	currentVersion, err := getVersion(doc, versionField)
	if err != nil {
		return empty, fmt.Errorf("failed to get current version: %w", err)
	}

	// Create match stage for ID and version
	matchStage := bson.D{
		{Key: "$match", Value: bson.M{
			"_id":          id,
			versionBSONTag: currentVersion, // Use BSON tag name for MongoDB query
		}},
	}

	// Add version increment stage
	incStage := bson.D{
		{Key: "$set", Value: bson.M{
			versionBSONTag: bson.M{"$add": []interface{}{fmt.Sprintf("$%s", versionBSONTag), 1}}, // Use BSON tag name for MongoDB update
		}},
	}

	// Combine stages
	fullPipeline := mongo.Pipeline{matchStage, incStage}
	fullPipeline = append(fullPipeline, pipeline...)

	// Execute update with pipeline
	updateOpts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var result bson.M
	err = s.collection.FindOneAndUpdate(timeoutCtx, bson.M{"_id": id}, fullPipeline, updateOpts).Decode(&result)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return empty, ErrVersionMismatch
		}
		return empty, fmt.Errorf("failed to update document: %w", err)
	}

	// Convert result to document
	var updatedDoc T
	resultBytes, err := bson.Marshal(result)
	if err != nil {
		return empty, fmt.Errorf("failed to marshal result: %w", err)
	}

	if err := bson.Unmarshal(resultBytes, &updatedDoc); err != nil {
		return empty, fmt.Errorf("failed to unmarshal result: %w", err)
	}

	// Update cache
	if err := s.cache.Set(timeoutCtx, id, updatedDoc, s.options.CacheTTL); err != nil {
		return updatedDoc, fmt.Errorf("document updated but failed to update cache: %w", err)
	}

	return updatedDoc, nil
}

// UpdateSection edits a specific section of a document with optimistic concurrency control
func (s *StorageImpl[T]) UpdateSection(
	ctx context.Context,
	id primitive.ObjectID,
	sectionPath string,
	updateFn func(interface{}) (interface{}, error),
	opts ...EditOption,
) (T, error) {
	var empty T

	if s.closed {
		return empty, ErrClosed
	}

	// Create options with defaults and apply provided options
	editOpts := NewEditOptions(opts...)

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(editOpts.Timeout))
	defer cancel()

	// Extract section version field (using BSON tag name for MongoDB operations)
	sectionVersionField := fmt.Sprintf("%s.%s", sectionPath, s.options.SectionVersionField)

	// Get current section version using MongoDB projection
	var docResult bson.M
	err := s.collection.FindOne(
		timeoutCtx,
		bson.M{"_id": id},
		options.FindOne().SetProjection(bson.M{sectionVersionField: 1, sectionPath: 1}),
	).Decode(&docResult)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return empty, ErrNotFound
		}
		return empty, fmt.Errorf("failed to get section version: %w", err)
	}

	// Extract current version and section data
	var currentVersion int64 = 1
	var sectionData interface{} = bson.M{}

	// Navigate through the document to find the section
	parts := strings.Split(sectionPath, ".")
	current := docResult

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part - this is our section
			if section, ok := current[part]; ok {
				sectionData = section

				// Extract version if available
				if sectionMap, ok := section.(bson.M); ok {
					if version, ok := sectionMap[s.options.SectionVersionField]; ok {
						if versionInt, ok := version.(int64); ok {
							currentVersion = versionInt
						}
					}
				}
			} else {
				// Section doesn't exist yet, create it with default version
				sectionData = bson.M{s.options.SectionVersionField: currentVersion}
			}
		} else {
			// Navigate deeper
			if next, ok := current[part]; ok {
				if nextMap, ok := next.(bson.M); ok {
					current = nextMap
				} else {
					return empty, fmt.Errorf("invalid path: %s is not an object", part)
				}
			} else {
				return empty, fmt.Errorf("invalid path: %s not found", part)
			}
		}
	}

	// Apply edit function to section
	updatedSection, err := updateFn(sectionData)
	if err != nil {
		return empty, fmt.Errorf("edit function failed: %w", err)
	}

	// Ensure section has a version field
	updatedSectionMap, ok := updatedSection.(bson.M)
	if !ok {
		return empty, fmt.Errorf("updated section must be a map")
	}

	// Increment version
	updatedSectionMap[s.options.SectionVersionField] = currentVersion + 1

	// Update in database with version check
	update := bson.M{
		"$set": bson.M{
			sectionPath: updatedSectionMap,
		},
	}

	filter := bson.M{
		"_id": id,
	}

	// Add version check if section exists
	if currentVersion > 0 {
		filter[sectionVersionField] = currentVersion
	}

	updateResult, err := s.collection.UpdateOne(timeoutCtx, filter, update)

	if err != nil {
		return empty, fmt.Errorf("failed to update section: %w", err)
	}

	if updateResult.MatchedCount == 0 {
		return empty, NewSectionVersionError(id, sectionPath, currentVersion, -1)
	}

	// Get updated document
	updatedDoc, err := s.FindOne(timeoutCtx, id)
	if err != nil {
		return empty, fmt.Errorf("section updated but failed to retrieve document: %w", err)
	}

	// Update cache
	if err := s.cache.Set(timeoutCtx, id, updatedDoc, s.options.CacheTTL); err != nil {
		return updatedDoc, fmt.Errorf("section updated but failed to update cache: %w", err)
	}

	return updatedDoc, nil
}

// WithTransaction executes the provided function within a MongoDB transaction
func (s *StorageImpl[T]) WithTransaction(
	ctx context.Context,
	fn func(sessCtx mongo.SessionContext) error,
) error {
	if s.closed {
		return ErrClosed
	}

	// Start a session
	session, err := s.client.StartSession()
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}
	defer session.EndSession(ctx)

	// Apply transaction options if provided
	txnOpts := s.options.DefaultTransactionOptions
	opts := options.Transaction()

	if txnOpts != nil {
		// Set read preference
		if txnOpts.ReadPreference != "" {
			var readPref *readpref.ReadPref
			switch txnOpts.ReadPreference {
			case "primary":
				readPref = readpref.Primary()
			case "primaryPreferred":
				readPref = readpref.PrimaryPreferred()
			case "secondary":
				readPref = readpref.Secondary()
			case "secondaryPreferred":
				readPref = readpref.SecondaryPreferred()
			case "nearest":
				readPref = readpref.Nearest()
			}
			if readPref != nil {
				opts.SetReadPreference(readPref)
			}
		}

		// Set read concern
		if txnOpts.ReadConcern != "" {
			var readConcern *readconcern.ReadConcern
			switch txnOpts.ReadConcern {
			case "local":
				readConcern = readconcern.Local()
			case "majority":
				readConcern = readconcern.Majority()
			case "linearizable":
				readConcern = readconcern.Linearizable()
			case "snapshot":
				readConcern = readconcern.Snapshot()
			case "available":
				readConcern = readconcern.Available()
			}
			if readConcern != nil {
				opts.SetReadConcern(readConcern)
			}
		}

		// Set write concern
		if txnOpts.WriteConcern != "" {
			var writeConcern *writeconcern.WriteConcern
			if txnOpts.WriteConcern == "majority" {
				writeConcern = writeconcern.Majority()
			} else if w, err := strconv.Atoi(txnOpts.WriteConcern); err == nil {
				writeConcern = writeconcern.New(writeconcern.W(w))
			}
			if writeConcern != nil {
				opts.SetWriteConcern(writeConcern)
			}
		}

		// Set max commit time
		if txnOpts.MaxCommitTime > 0 {
			opts.SetMaxCommitTime(&txnOpts.MaxCommitTime)
		}
	}

	// Execute the transaction
	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		return nil, fn(sessCtx)
	}, opts)

	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

// Watch watches for changes to documents with optional MongoDB pipeline and options
func (s *StorageImpl[T]) Watch(
	ctx context.Context,
	pipeline mongo.Pipeline,
	opts ...*options.ChangeStreamOptions,
) (<-chan WatchEvent[T], error) {
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

	// Configure change stream options
	watchOpts := options.ChangeStream()
	if len(opts) > 0 {
		watchOpts = opts[0]
	} else {
		// Default options
		watchOpts.SetFullDocument(options.UpdateLookup)
		if s.options.WatchMaxAwaitTime > 0 {
			watchOpts.SetMaxAwaitTime(s.options.WatchMaxAwaitTime)
		}
		if s.options.WatchBatchSize > 0 {
			watchOpts.SetBatchSize(s.options.WatchBatchSize)
		}
	}

	// Use provided pipeline or default
	if len(pipeline) == 0 {
		pipeline = mongo.Pipeline{
			bson.D{{Key: "$match", Value: bson.D{{Key: "operationType", Value: bson.D{{Key: "$in", Value: bson.A{"insert", "update", "delete"}}}}}}},
		}
	}

	// Create change stream
	stream, err := s.collection.Watch(subCtx, pipeline, watchOpts)
	if err != nil {
		s.removeSubscriber(subID)
		return nil, fmt.Errorf("failed to create change stream: %w", err)
	}

	// Start goroutine to handle database events
	go func() {
		defer stream.Close(context.Background())
		defer s.removeSubscriber(subID)

		for stream.Next(subCtx) {
			// Create a dynamic event structure
			var rawEvent bson.M
			if err := stream.Decode(&rawEvent); err != nil {
				core.Error("Error decoding change stream event", zap.Error(err))
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

			// Send event to subscriber
			select {
			case sub.Chan <- watchEvent:
				// Event sent successfully
			case <-subCtx.Done():
				// Subscriber context is done
				return
			default:
				// Channel is full, log warning and continue
				core.Warn("Subscriber channel is full, skipping event",
					zap.Int("subscriber_id", subID),
					zap.String("document_id", docID.Hex()),
					zap.String("operation", operation))
			}
		}

		if err := stream.Err(); err != nil {
			// Context cancellation is normal termination
			if !s.closed && !errors.Is(err, context.Canceled) {
				core.Error("Change stream error", zap.Error(err))
			}
		}
	}()

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

	// Start goroutine to handle database events
	go func() {
		defer stream.Close(context.Background())

		for stream.Next(s.ctx) {
			// Create a dynamic event structure
			var rawEvent bson.M
			if err := stream.Decode(&rawEvent); err != nil {
				core.Error("Error decoding change stream event", zap.Error(err))
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
			// Context cancellation is normal termination
			if !s.closed && !errors.Is(err, context.Canceled) {
				core.Error("Change stream error", zap.Error(err))
			}
		}
	}()

	return nil
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
			core.Warn("Subscriber channel is full, skipping event",
				zap.Int("subscriber_id", sub.ID),
				zap.String("document_id", event.ID.Hex()),
				zap.String("operation", event.Operation))
		}
	}
}
