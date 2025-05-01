package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// skipIfNoRedis skips the test if Redis is not available
func skipIfNoRedis(t *testing.T) string {
	// Check if Redis address is provided via environment variable
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // Default Redis address
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Try to connect to Redis
	cache, err := NewRedisCache[*TestDocument](redisAddr, nil)
	if err != nil {
		t.Skipf("Skipping Redis test: %v", err)
		return ""
	}
	defer cache.Close()

	// Ping Redis to ensure it's responsive
	err = cache.client.Ping(ctx).Err()
	if err != nil {
		t.Skipf("Skipping Redis test: %v", err)
		return ""
	}

	return redisAddr
}

// setupRedisCache creates a Redis cache for testing
func setupRedisCache(t *testing.T) (*RedisCache[*TestDocument], func()) {
	redisAddr := skipIfNoRedis(t)
	if redisAddr == "" {
		return nil, func() {}
	}

	// Create a unique prefix for this test to avoid conflicts
	prefix := "test:" + primitive.NewObjectID().Hex() + ":"

	// Create Redis cache with custom options
	options := DefaultCacheOptions()
	options.DefaultTTL = time.Hour

	cache, err := NewRedisCache[*TestDocument](redisAddr, options)
	require.NoError(t, err, "Failed to create Redis cache")

	// Set custom prefix
	cache.prefix = prefix

	// Return cache and cleanup function
	cleanup := func() {
		// Clear all keys with this prefix
		ctx := context.Background()
		cache.Clear(ctx)
		cache.Close()
	}

	return cache, cleanup
}

// TestRedisCacheBasicOperations tests basic CRUD operations on the Redis cache
func TestRedisCacheBasicOperations(t *testing.T) {
	// Set up Redis cache
	cache, cleanup := setupRedisCache(t)
	if cache == nil {
		return // Redis not available
	}
	defer cleanup()

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
	err := cache.Set(ctx, id.Hex(), doc, 0)
	assert.NoError(t, err, "Set should not return an error")

	// Test Get
	retrievedDoc, err := cache.Get(ctx, id.Hex())
	assert.NoError(t, err, "Get should not return an error")
	assert.Equal(t, doc.ID, retrievedDoc.ID, "Document ID should match")
	assert.Equal(t, doc.Name, retrievedDoc.Name, "Document Name should match")
	assert.Equal(t, doc.Age, retrievedDoc.Age, "Document Age should match")

	// Test Delete
	err = cache.Delete(ctx, id.Hex())
	assert.NoError(t, err, "Delete should not return an error")

	// Test Get after Delete
	_, err = cache.Get(ctx, id.Hex())
	assert.Error(t, err, "Get after Delete should return an error")
	assert.Equal(t, ErrCacheMiss, err, "Error should be ErrCacheMiss")

	// Test Clear
	err = cache.Set(ctx, id.Hex(), doc, 0)
	assert.NoError(t, err, "Set should not return an error")
	err = cache.Clear(ctx)
	assert.NoError(t, err, "Clear should not return an error")
	_, err = cache.Get(ctx, id.Hex())
	assert.Error(t, err, "Get after Clear should return an error")
	assert.Equal(t, ErrCacheMiss, err, "Error should be ErrCacheMiss")
}

// TestRedisCacheTTL tests the TTL functionality of the Redis cache
func TestRedisCacheTTL(t *testing.T) {
	// Set up Redis cache
	cache, cleanup := setupRedisCache(t)
	if cache == nil {
		return // Redis not available
	}
	defer cleanup()

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
	err := cache.Set(ctx, id.Hex(), doc, 100*time.Millisecond)
	assert.NoError(t, err, "Set with TTL should not return an error")

	// Test Get immediately after Set
	retrievedDoc, err := cache.Get(ctx, id.Hex())
	assert.NoError(t, err, "Get immediately after Set should not return an error")
	assert.Equal(t, doc.ID, retrievedDoc.ID, "Document ID should match")

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Test Get after TTL expiration
	_, err = cache.Get(ctx, id.Hex())
	assert.Error(t, err, "Get after TTL expiration should return an error")
	assert.Equal(t, ErrCacheMiss, err, "Error should be ErrCacheMiss")

	// Test Set with default TTL
	redisAddr := skipIfNoRedis(t)
	if redisAddr == "" {
		return
	}

	options := DefaultCacheOptions()
	options.DefaultTTL = 100 * time.Millisecond

	cacheWithDefaultTTL, err := NewRedisCache[*TestDocument](redisAddr, options)
	require.NoError(t, err, "Failed to create Redis cache with custom options")
	defer func() {
		cacheWithDefaultTTL.Clear(ctx)
		cacheWithDefaultTTL.Close()
	}()

	// Set custom prefix
	cacheWithDefaultTTL.prefix = "test:" + primitive.NewObjectID().Hex() + ":"

	err = cacheWithDefaultTTL.Set(ctx, id.Hex(), doc, 0) // Use default TTL
	assert.NoError(t, err, "Set with default TTL should not return an error")

	// Test Get immediately after Set
	retrievedDoc, err = cacheWithDefaultTTL.Get(ctx, id.Hex())
	assert.NoError(t, err, "Get immediately after Set should not return an error")
	assert.Equal(t, doc.ID, retrievedDoc.ID, "Document ID should match")

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)

	// Test Get after TTL expiration
	_, err = cacheWithDefaultTTL.Get(ctx, id.Hex())
	assert.Error(t, err, "Get after TTL expiration should return an error")
	assert.Equal(t, ErrCacheMiss, err, "Error should be ErrCacheMiss")
}

// TestRedisCacheConcurrency tests concurrent access to the Redis cache
func TestRedisCacheConcurrency(t *testing.T) {
	// Set up Redis cache
	cache, cleanup := setupRedisCache(t)
	if cache == nil {
		return // Redis not available
	}
	defer cleanup()

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
	err := cache.Set(ctx, id.Hex(), doc, 0)
	assert.NoError(t, err, "Set should not return an error")

	// Run concurrent Get operations
	done := make(chan bool)
	for i := 0; i < numOps; i++ {
		go func() {
			retrievedDoc, err := cache.Get(ctx, id.Hex())
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
			err := cache.Set(ctx, id.Hex(), newDoc, 0)
			assert.NoError(t, err, "Concurrent Set should not return an error")
			done <- true
		}(i)
	}

	// Wait for all operations to complete
	for i := 0; i < numOps; i++ {
		<-done
	}

	// Verify the document is still accessible
	retrievedDoc, err := cache.Get(ctx, id.Hex())
	assert.NoError(t, err, "Get after concurrent operations should not return an error")
	assert.Equal(t, id, retrievedDoc.ID, "Document ID should match")
}

// TestRedisCacheOptions tests the options functionality of the Redis cache
func TestRedisCacheOptions(t *testing.T) {
	redisAddr := skipIfNoRedis(t)
	if redisAddr == "" {
		return
	}

	// Test default options
	defaultOptions := DefaultRedisCacheOptions()
	assert.Equal(t, time.Hour*24, defaultOptions.DefaultTTL, "Default TTL should be 24 hours")
	assert.Equal(t, 10, defaultOptions.PoolSize, "Default PoolSize should be 10")
	assert.Equal(t, "nodestorage:", defaultOptions.KeyPrefix, "Default KeyPrefix should be 'nodestorage:'")

	// Test custom options
	customOptions := &RedisCacheOptions{
		CacheOptions: CacheOptions{
			DefaultTTL: time.Hour,
			MaxItems:   100,
			LogEnabled: false,
		},
		Username:     "",
		Password:     "",
		DB:           1,
		PoolSize:     20,
		MinIdleConns: 5,
		KeyPrefix:    "custom:",
	}

	cache, err := NewRedisCacheWithOptions[*TestDocument](redisAddr, customOptions)
	require.NoError(t, err, "Failed to create Redis cache with custom options")
	defer cache.Close()

	// Verify options are applied (indirectly by testing functionality)
	ctx := context.Background()
	id := primitive.NewObjectID()
	doc := &TestDocument{
		ID:   id,
		Name: "Test Document",
		Age:  30,
	}

	err = cache.Set(ctx, id.Hex(), doc, 0)
	assert.NoError(t, err, "Set should not return an error")

	retrievedDoc, err := cache.Get(ctx, id.Hex())
	assert.NoError(t, err, "Get should not return an error")
	assert.Equal(t, doc.ID, retrievedDoc.ID, "Document ID should match")

	// Clean up
	cache.Clear(ctx)
}

// TestRedisCacheMultipleDocuments tests storing and retrieving multiple documents
func TestRedisCacheMultipleDocuments(t *testing.T) {
	// Set up Redis cache
	cache, cleanup := setupRedisCache(t)
	if cache == nil {
		return // Redis not available
	}
	defer cleanup()

	// Test context
	ctx := context.Background()

	// Create test documents
	numDocs := 100
	ids := make([]primitive.ObjectID, numDocs)
	docs := make([]*TestDocument, numDocs)

	for i := 0; i < numDocs; i++ {
		ids[i] = primitive.NewObjectID()
		docs[i] = &TestDocument{
			ID:   ids[i],
			Name: "Document " + string(rune('A'+i%26)),
			Age:  30 + i%50,
		}

		// Set document
		err := cache.Set(ctx, ids[i].Hex(), docs[i], 0)
		assert.NoError(t, err, "Set should not return an error")
	}

	// Retrieve and verify all documents
	for i := 0; i < numDocs; i++ {
		retrievedDoc, err := cache.Get(ctx, ids[i].Hex())
		assert.NoError(t, err, "Get should not return an error")
		assert.Equal(t, docs[i].ID, retrievedDoc.ID, "Document ID should match")
		assert.Equal(t, docs[i].Name, retrievedDoc.Name, "Document Name should match")
		assert.Equal(t, docs[i].Age, retrievedDoc.Age, "Document Age should match")
	}

	// Delete half of the documents
	for i := 0; i < numDocs/2; i++ {
		err := cache.Delete(ctx, ids[i].Hex())
		assert.NoError(t, err, "Delete should not return an error")
	}

	// Verify deleted documents are gone
	for i := 0; i < numDocs/2; i++ {
		_, err := cache.Get(ctx, ids[i].Hex())
		assert.Error(t, err, "Get after Delete should return an error")
		assert.Equal(t, ErrCacheMiss, err, "Error should be ErrCacheMiss")
	}

	// Verify remaining documents are still accessible
	for i := numDocs / 2; i < numDocs; i++ {
		retrievedDoc, err := cache.Get(ctx, ids[i].Hex())
		assert.NoError(t, err, "Get should not return an error")
		assert.Equal(t, docs[i].ID, retrievedDoc.ID, "Document ID should match")
	}
}
