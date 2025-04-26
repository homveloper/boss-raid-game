package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDefaultCacheOptions tests the default cache options
func TestDefaultCacheOptions(t *testing.T) {
	options := DefaultCacheOptions()

	// Verify default values
	assert.Equal(t, time.Hour*24, options.DefaultTTL, "Default TTL should be 24 hours")
	assert.Equal(t, 10000, options.MaxItems, "Default MaxItems should be 10000")
	assert.Equal(t, true, options.LogEnabled, "Default LogEnabled should be true")
}

// TestCacheErrors tests the cache error definitions
func TestCacheErrors(t *testing.T) {
	// Verify error messages
	assert.Equal(t, "cache miss", ErrCacheMiss.Error())
	assert.Equal(t, "cache is full", ErrCacheFull.Error())
	assert.Equal(t, "cache is closed", ErrCacheClosed.Error())
	assert.Equal(t, "invalid cache key", ErrInvalidKey.Error())
	assert.Equal(t, "invalid cache value", ErrInvalidValue.Error())
	assert.Equal(t, "failed to serialize cache value", ErrSerializationFailed.Error())
	assert.Equal(t, "failed to deserialize cache value", ErrDeserializationFailed.Error())
}

// Note: The following tests are commented out because they require importing
// the specific cache implementations, which would create circular dependencies.
// These tests are covered in the individual implementation test files.

/*
// TestCacheImplementations verifies that all cache implementations satisfy the Cache interface
func TestCacheImplementations(t *testing.T) {
	// This is a compile-time check to ensure all implementations satisfy the Cache interface
	var _ Cache[*TestDocument] = (*MemoryCache[*TestDocument])(nil)
	var _ Cache[*TestDocument] = (*BadgerCache[*TestDocument])(nil)
	var _ Cache[*TestDocument] = (*RedisCache[*TestDocument])(nil)
}

// TestCacheOptionsImplementations verifies that all cache options implementations are valid
func TestCacheOptionsImplementations(t *testing.T) {
	// Memory cache options
	memoryOptions := DefaultMemoryCacheOptions()
	assert.NotNil(t, memoryOptions)
	assert.Equal(t, time.Minute, memoryOptions.CleanupInterval)

	// BadgerDB cache options
	badgerOptions := DefaultBadgerCacheOptions()
	assert.NotNil(t, badgerOptions)
	assert.Equal(t, int64(1<<28), badgerOptions.ValueLogFileSize)

	// Redis cache options
	redisOptions := DefaultRedisCacheOptions()
	assert.NotNil(t, redisOptions)
	assert.Equal(t, "nodestorage:", redisOptions.KeyPrefix)
}
*/
