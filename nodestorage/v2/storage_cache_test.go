package nodestorage

import (
	"context"
	"testing"
	"time"

	"nodestorage/v2/cache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestStorageWithMemoryCache tests the Storage implementation with memory cache
func TestStorageWithMemoryCache(t *testing.T) {
	// Set up test database
	_, collection, dbCleanup := setupTestDB(t)
	defer dbCleanup()

	// Create memory cache
	memCache := cache.NewMemoryCache[*TestDocument](nil)
	defer memCache.Close()

	// Create storage options
	options := &Options{
		VersionField: "VectorClock",
		CacheTTL:     time.Hour,
	}

	// Create storage
	ctx := context.Background()
	storage, err := NewStorage[*TestDocument](ctx, collection, memCache, options)
	require.NoError(t, err, "Failed to create storage")
	defer storage.Close()

	// Test cache hit/miss scenarios

	// 1. Insert a document directly into MongoDB (cache miss)
	id := primitive.NewObjectID()
	doc := &TestDocument{
		ID:          id,
		Name:        "Test Document",
		Value:       42,
		VectorClock: 1,
	}
	_, err = collection.InsertOne(ctx, doc)
	require.NoError(t, err, "Failed to insert test document")

	// First retrieval should be a cache miss (from MongoDB)
	result1, err := storage.FindOne(ctx, id)
	assert.NoError(t, err, "FindOne should not return an error")
	assert.Equal(t, doc.ID, result1.ID, "Document ID should match")
	assert.Equal(t, doc.Name, result1.Name, "Document Name should match")
	assert.Equal(t, doc.Value, result1.Value, "Document Value should match")
	assert.Equal(t, doc.VectorClock, result1.VectorClock, "Document VectorClock should match")

	// Second retrieval should be a cache hit (from memory)
	result2, err := storage.FindOne(ctx, id)
	assert.NoError(t, err, "FindOne should not return an error")
	assert.Equal(t, doc.ID, result2.ID, "Document ID should match")

	// 2. Test cache invalidation on update
	_, err = storage.UpdateOne(ctx, id, bson.M{
		"$set": bson.M{
			"name": "Updated Document",
		},
	})
	assert.NoError(t, err, "UpdateOne should not return an error")

	// Retrieval after update should reflect the changes
	result3, err := storage.FindOne(ctx, id)
	assert.NoError(t, err, "FindOne should not return an error")
	assert.Equal(t, "Updated Document", result3.Name, "Document Name should be updated")
	assert.Equal(t, int64(2), result3.VectorClock, "Document VectorClock should be incremented")

	// 3. Test cache invalidation on delete
	err = storage.DeleteOne(ctx, id)
	assert.NoError(t, err, "DeleteOne should not return an error")

	// Retrieval after delete should return not found
	_, err = storage.FindOne(ctx, id)
	assert.Error(t, err, "FindOne after delete should return an error")
	assert.Equal(t, ErrNotFound, err, "Error should be ErrNotFound")
}

// TestStorageCacheTTL tests the TTL functionality of the cache in Storage
func TestStorageCacheTTL(t *testing.T) {
	// Set up test database
	_, collection, dbCleanup := setupTestDB(t)
	defer dbCleanup()

	// Create memory cache
	memCache := cache.NewMemoryCache[*TestDocument](nil)
	defer memCache.Close()

	// Create storage options with short TTL
	options := &Options{
		VersionField: "VectorClock",
		CacheTTL:     100 * time.Millisecond, // Very short TTL for testing
	}

	// Create storage
	ctx := context.Background()
	storage, err := NewStorage[*TestDocument](ctx, collection, memCache, options)
	require.NoError(t, err, "Failed to create storage")
	defer storage.Close()

	// Insert a document
	id := primitive.NewObjectID()
	doc := &TestDocument{
		ID:          id,
		Name:        "TTL Test Document",
		Value:       42,
		VectorClock: 1,
	}
	_, err = collection.InsertOne(ctx, doc)
	require.NoError(t, err, "Failed to insert test document")

	// First retrieval should be from MongoDB
	result1, err := storage.FindOne(ctx, id)
	assert.NoError(t, err, "FindOne should not return an error")
	assert.Equal(t, doc.ID, result1.ID, "Document ID should match")

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Second retrieval should be from MongoDB again (cache expired)
	result2, err := storage.FindOne(ctx, id)
	assert.NoError(t, err, "FindOne should not return an error")
	assert.Equal(t, doc.ID, result2.ID, "Document ID should match")
}

// TestStorageCacheConcurrency tests concurrent access to the storage with cache
func TestStorageCacheConcurrency(t *testing.T) {
	// Set up test database
	_, collection, dbCleanup := setupTestDB(t)
	defer dbCleanup()

	// Create memory cache
	memCache := cache.NewMemoryCache[*TestDocument](nil)
	defer memCache.Close()

	// Create storage options
	options := &Options{
		VersionField: "VectorClock",
		CacheTTL:     time.Hour,
	}

	// Create storage
	ctx := context.Background()
	storage, err := NewStorage[*TestDocument](ctx, collection, memCache, options)
	require.NoError(t, err, "Failed to create storage")
	defer storage.Close()

	// Insert a document
	id := primitive.NewObjectID()
	doc := &TestDocument{
		ID:          id,
		Name:        "Concurrency Test Document",
		Value:       42,
		VectorClock: 1,
	}
	_, err = collection.InsertOne(ctx, doc)
	require.NoError(t, err, "Failed to insert test document")

	// Number of concurrent operations
	numOps := 50

	// Run concurrent FindOne operations
	done := make(chan bool)
	for i := 0; i < numOps; i++ {
		go func() {
			result, err := storage.FindOne(ctx, id)
			assert.NoError(t, err, "Concurrent FindOne should not return an error")
			assert.Equal(t, doc.ID, result.ID, "Document ID should match")
			done <- true
		}()
	}

	// Wait for all operations to complete
	for i := 0; i < numOps; i++ {
		<-done
	}

	// Run concurrent UpdateOne operations
	for i := 0; i < numOps; i++ {
		go func(i int) {
			_, _ = storage.UpdateOne(ctx, id, bson.M{
				"$set": bson.M{
					"value": 42 + i,
				},
			})
			// Note: Some updates may fail due to version conflicts, which is expected
			// We're testing that the system doesn't crash or corrupt data
			done <- true
		}(i)
	}

	// Wait for all operations to complete
	for i := 0; i < numOps; i++ {
		<-done
	}

	// Verify the document is still accessible and has a valid state
	result, err := storage.FindOne(ctx, id)
	assert.NoError(t, err, "FindOne after concurrent operations should not return an error")
	assert.Equal(t, id, result.ID, "Document ID should match")
	assert.True(t, result.Value >= 42, "Document Value should be updated")
	assert.True(t, result.VectorClock > 1, "Document VectorClock should be incremented")
}

// TestStorageCacheInvalidation tests that the cache is properly invalidated
func TestStorageCacheInvalidation(t *testing.T) {
	// Set up test database
	_, collection, dbCleanup := setupTestDB(t)
	defer dbCleanup()

	// Create memory cache
	memCache := cache.NewMemoryCache[*TestDocument](nil)
	defer memCache.Close()

	// Create storage storageoptions
	storageoptions := &Options{
		VersionField: "VectorClock",
		CacheTTL:     time.Hour,
	}

	// Create storage
	ctx := context.Background()
	storage, err := NewStorage[*TestDocument](ctx, collection, memCache, storageoptions)
	require.NoError(t, err, "Failed to create storage")
	defer storage.Close()

	// Insert a document
	id := primitive.NewObjectID()
	doc := &TestDocument{
		ID:          id,
		Name:        "Cache Invalidation Test",
		Value:       42,
		VectorClock: 1,
	}
	_, err = collection.InsertOne(ctx, doc)
	require.NoError(t, err, "Failed to insert test document")

	// First retrieval should be from MongoDB
	result1, err := storage.FindOne(ctx, id)
	assert.NoError(t, err, "FindOne should not return an error")
	assert.Equal(t, doc.ID, result1.ID, "Document ID should match")

	// Update document directly in MongoDB (bypassing the storage)
	_, err = collection.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{
			"$set": bson.M{
				"name":  "Updated Directly",
				"value": 100,
			},
			"$inc": bson.M{
				"vector_clock": 1,
			},
		},
	)
	require.NoError(t, err, "Failed to update document directly")

	// Second retrieval should still be from cache (stale data)
	result2, err := storage.FindOne(ctx, id)
	assert.NoError(t, err, "FindOne should not return an error")
	assert.Equal(t, "Cache Invalidation Test", result2.Name, "Document Name should be from cache")
	assert.Equal(t, 42, result2.Value, "Document Value should be from cache")

	// Update through storage should invalidate cache
	_, err = storage.UpdateOne(ctx, id, bson.M{
		"$set": bson.M{
			"name": "Updated Through Storage",
		},
	})
	// This should fail due to version mismatch
	assert.Error(t, err, "UpdateOne should return an error due to version mismatch")
	assert.Equal(t, ErrVersionMismatch, err, "Error should be ErrVersionMismatch")

	// Force refresh from database by directly querying MongoDB
	var result3 TestDocument
	err = collection.FindOne(ctx, bson.M{"_id": id}).Decode(&result3)
	assert.NoError(t, err, "FindOne from MongoDB should not return an error")
	assert.Equal(t, "Updated Directly", result3.Name, "Document Name should be from database")
	assert.Equal(t, 100, result3.Value, "Document Value should be from database")
	assert.Equal(t, int64(2), result3.VectorClock, "Document VectorClock should be incremented")

	// Now update should succeed
	_, err = storage.UpdateOne(ctx, id, bson.M{
		"$set": bson.M{
			"name": "Updated After Refresh",
		},
	})
	assert.NoError(t, err, "UpdateOne after refresh should not return an error")

	// Verify update
	result4, err := storage.FindOne(ctx, id)
	assert.NoError(t, err, "FindOne after update should not return an error")
	assert.Equal(t, "Updated After Refresh", result4.Name, "Document Name should be updated")
	assert.Equal(t, int64(3), result4.VectorClock, "Document VectorClock should be incremented")
}
