package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestMemoryCacheBasicOperations tests basic CRUD operations on the memory cache
func TestMemoryCacheBasicOperations(t *testing.T) {
	// Create a new memory cache
	cache := NewMemoryCache[*TestDocument](nil)
	defer cache.Close()

	// Create a test document
	id := primitive.NewObjectID()
	doc := &TestDocument{
		ID:   id,
		Name: "Test Document",
		Age:  30,
	}

	// Test context
	ctx := context.Background()

	// Test Set
	err := cache.Set(ctx, id, doc, 0)
	assert.NoError(t, err, "Set should not return an error")

	// Test Get
	retrievedDoc, err := cache.Get(ctx, id)
	assert.NoError(t, err, "Get should not return an error")
	assert.Equal(t, doc.ID, retrievedDoc.ID, "Document ID should match")
	assert.Equal(t, doc.Name, retrievedDoc.Name, "Document Name should match")
	assert.Equal(t, doc.Age, retrievedDoc.Age, "Document Age should match")

	// Test Delete
	err = cache.Delete(ctx, id)
	assert.NoError(t, err, "Delete should not return an error")

	// Test Get after Delete
	_, err = cache.Get(ctx, id)
	assert.Error(t, err, "Get after Delete should return an error")
	assert.Equal(t, ErrCacheMiss, err, "Error should be ErrCacheMiss")

	// Test Clear
	err = cache.Set(ctx, id, doc, 0)
	assert.NoError(t, err, "Set should not return an error")
	err = cache.Clear(ctx)
	assert.NoError(t, err, "Clear should not return an error")
	_, err = cache.Get(ctx, id)
	assert.Error(t, err, "Get after Clear should return an error")
	assert.Equal(t, ErrCacheMiss, err, "Error should be ErrCacheMiss")
}

// TestMemoryCacheTTL tests the TTL functionality of the memory cache
func TestMemoryCacheTTL(t *testing.T) {
	// Create a new memory cache
	cache := NewMemoryCache[*TestDocument](nil)
	defer cache.Close()

	// Create a test document
	id := primitive.NewObjectID()
	doc := &TestDocument{
		ID:   id,
		Name: "Test Document",
		Age:  30,
	}

	// Test context
	ctx := context.Background()

	// Test Set with short TTL
	err := cache.Set(ctx, id, doc, 100*time.Millisecond)
	assert.NoError(t, err, "Set with TTL should not return an error")

	// Test Get immediately after Set
	retrievedDoc, err := cache.Get(ctx, id)
	assert.NoError(t, err, "Get immediately after Set should not return an error")
	assert.Equal(t, doc.ID, retrievedDoc.ID, "Document ID should match")

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Test Get after TTL expiration
	_, err = cache.Get(ctx, id)
	assert.Error(t, err, "Get after TTL expiration should return an error")
	assert.Equal(t, ErrCacheMiss, err, "Error should be ErrCacheMiss")

	// Test Set with default TTL
	options := DefaultCacheOptions()
	options.DefaultTTL = 100 * time.Millisecond
	cacheWithDefaultTTL := NewMemoryCache[*TestDocument](options)
	defer cacheWithDefaultTTL.Close()

	err = cacheWithDefaultTTL.Set(ctx, id, doc, 0) // Use default TTL
	assert.NoError(t, err, "Set with default TTL should not return an error")

	// Test Get immediately after Set
	retrievedDoc, err = cacheWithDefaultTTL.Get(ctx, id)
	assert.NoError(t, err, "Get immediately after Set should not return an error")
	assert.Equal(t, doc.ID, retrievedDoc.ID, "Document ID should match")

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Test Get after TTL expiration
	_, err = cacheWithDefaultTTL.Get(ctx, id)
	assert.Error(t, err, "Get after TTL expiration should return an error")
	assert.Equal(t, ErrCacheMiss, err, "Error should be ErrCacheMiss")
}

// TestMemoryCacheMaxItems tests the MaxItems functionality of the memory cache
func TestMemoryCacheMaxItems(t *testing.T) {
	// Create a new memory cache with MaxItems=3
	options := DefaultCacheOptions()
	options.MaxItems = 3
	cache := NewMemoryCache[*TestDocument](options)
	defer cache.Close()

	// Test context
	ctx := context.Background()

	// Create test documents
	docs := make([]*TestDocument, 5)
	ids := make([]primitive.ObjectID, 5)
	for i := 0; i < 5; i++ {
		ids[i] = primitive.NewObjectID()
		docs[i] = &TestDocument{
			ID:   ids[i],
			Name: "Document " + string(rune('A'+i)),
			Age:  30 + i,
		}
	}

	// Add first 3 documents (should all be cached)
	for i := 0; i < 3; i++ {
		err := cache.Set(ctx, ids[i], docs[i], 0)
		assert.NoError(t, err, "Set should not return an error")
	}

	// Verify first 3 documents are cached
	for i := 0; i < 3; i++ {
		doc, err := cache.Get(ctx, ids[i])
		assert.NoError(t, err, "Get should not return an error")
		assert.Equal(t, docs[i].ID, doc.ID, "Document ID should match")
	}

	// Add 2 more documents (should evict some documents to maintain MaxItems=3)
	for i := 3; i < 5; i++ {
		err := cache.Set(ctx, ids[i], docs[i], 0)
		assert.NoError(t, err, "Set should not return an error")
	}

	// Count how many documents are still in the cache
	// We expect exactly 3 documents to be in the cache
	cacheHits := 0
	for i := 0; i < 5; i++ {
		_, err := cache.Get(ctx, ids[i])
		if err == nil {
			cacheHits++
		}
	}
	assert.Equal(t, 3, cacheHits, "Cache should contain exactly 3 documents")

	// Verify the newest documents (ids[3] and ids[4]) are definitely in the cache
	for i := 3; i < 5; i++ {
		doc, err := cache.Get(ctx, ids[i])
		assert.NoError(t, err, "Get for newest documents should not return an error")
		assert.Equal(t, docs[i].ID, doc.ID, "Document ID should match")
	}

	// At least one of the older documents should be evicted
	evicted := false
	for i := 0; i < 3; i++ {
		_, err := cache.Get(ctx, ids[i])
		if err != nil {
			evicted = true
			assert.Equal(t, ErrCacheMiss, err, "Error should be ErrCacheMiss")
			break
		}
	}
	assert.True(t, evicted, "At least one of the older documents should be evicted")
}

// TestMemoryCacheConcurrency tests concurrent access to the memory cache
func TestMemoryCacheConcurrency(t *testing.T) {
	// Create a new memory cache
	cache := NewMemoryCache[*TestDocument](nil)
	defer cache.Close()

	// Test context
	ctx := context.Background()

	// Number of concurrent operations
	numOps := 100

	// Create a test document
	id := primitive.NewObjectID()
	doc := &TestDocument{
		ID:   id,
		Name: "Test Document",
		Age:  30,
	}

	// Set the document
	err := cache.Set(ctx, id, doc, 0)
	assert.NoError(t, err, "Set should not return an error")

	// Run concurrent Get operations
	done := make(chan bool)
	for i := 0; i < numOps; i++ {
		go func() {
			retrievedDoc, err := cache.Get(ctx, id)
			assert.NoError(t, err, "Concurrent Get should not return an error")
			assert.Equal(t, doc.ID, retrievedDoc.ID, "Document ID should match")
			done <- true
		}()
	}

	// Wait for all operations to complete
	for i := 0; i < numOps; i++ {
		<-done
	}

	// Run concurrent Set operations
	for i := 0; i < numOps; i++ {
		go func(i int) {
			newDoc := &TestDocument{
				ID:   id,
				Name: "Test Document " + string(rune('A'+i%26)),
				Age:  30 + i%10,
			}
			err := cache.Set(ctx, id, newDoc, 0)
			assert.NoError(t, err, "Concurrent Set should not return an error")
			done <- true
		}(i)
	}

	// Wait for all operations to complete
	for i := 0; i < numOps; i++ {
		<-done
	}

	// Verify the document is still accessible
	retrievedDoc, err := cache.Get(ctx, id)
	assert.NoError(t, err, "Get after concurrent operations should not return an error")
	assert.Equal(t, id, retrievedDoc.ID, "Document ID should match")
}

// TestMemoryCacheCleanup tests the automatic cleanup of expired items
func TestMemoryCacheCleanup(t *testing.T) {
	// Create a new memory cache with custom cleanup interval
	options := &MemoryCacheOptions{
		CacheOptions: CacheOptions{
			DefaultTTL: time.Hour,
			MaxItems:   1000,
			LogEnabled: false,
		},
		CleanupInterval: 100 * time.Millisecond,
	}
	cache := NewMemoryCacheWithOptions[*TestDocument](options)
	defer cache.Close()

	// Test context
	ctx := context.Background()

	// Create test documents
	numDocs := 10
	ids := make([]primitive.ObjectID, numDocs)
	for i := 0; i < numDocs; i++ {
		ids[i] = primitive.NewObjectID()
		doc := &TestDocument{
			ID:   ids[i],
			Name: "Document " + string(rune('A'+i)),
			Age:  30 + i,
		}
		// Set with short TTL
		err := cache.Set(ctx, ids[i], doc, 50*time.Millisecond)
		assert.NoError(t, err, "Set should not return an error")
	}

	// Verify all documents are initially cached
	for i := 0; i < numDocs; i++ {
		_, err := cache.Get(ctx, ids[i])
		assert.NoError(t, err, "Get should not return an error")
	}

	// Wait for TTL to expire and cleanup to run
	time.Sleep(200 * time.Millisecond)

	// Verify all documents are removed
	for i := 0; i < numDocs; i++ {
		_, err := cache.Get(ctx, ids[i])
		assert.Error(t, err, "Get after cleanup should return an error")
		assert.Equal(t, ErrCacheMiss, err, "Error should be ErrCacheMiss")
	}
}

// TestMemoryCacheOptions tests the options functionality of the memory cache
func TestMemoryCacheOptions(t *testing.T) {
	// Test default options
	defaultOptions := DefaultMemoryCacheOptions()
	assert.Equal(t, time.Hour*24, defaultOptions.DefaultTTL, "Default TTL should be 24 hours")
	assert.Equal(t, 10000, defaultOptions.MaxItems, "Default MaxItems should be 10000")
	assert.Equal(t, true, defaultOptions.LogEnabled, "Default LogEnabled should be true")
	assert.Equal(t, time.Minute, defaultOptions.CleanupInterval, "Default CleanupInterval should be 1 minute")

	// Test custom options
	customOptions := &MemoryCacheOptions{
		CacheOptions: CacheOptions{
			DefaultTTL: time.Hour,
			MaxItems:   100,
			LogEnabled: false,
		},
		CleanupInterval: 30 * time.Second,
	}
	cache := NewMemoryCacheWithOptions[*TestDocument](customOptions)
	defer cache.Close()

	// Verify options are applied
	assert.Equal(t, time.Hour, cache.options.DefaultTTL, "Custom DefaultTTL should be applied")
	assert.Equal(t, 100, cache.options.MaxItems, "Custom MaxItems should be applied")
	assert.Equal(t, false, cache.options.LogEnabled, "Custom LogEnabled should be applied")
}
