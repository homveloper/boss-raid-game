package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
)

// RedisCache implements the Cache interface using Redis
type RedisCache[T any] struct {
	client  *redis.Client
	options *CacheOptions
	prefix  string
}

// NewRedisCache creates a new RedisCache instance
func NewRedisCache[T any](redisAddr string, options *CacheOptions) (*RedisCache[T], error) {
	if options == nil {
		options = DefaultCacheOptions()
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache[T]{
		client:  client,
		options: options,
		prefix:  "nodestorage:",
	}, nil
}

// Get retrieves a document from the cache
func (c *RedisCache[T]) Get(ctx context.Context, key string) (T, error) {
	var result T

	// Create key from ID
	prefixkey := c.getKey(key)

	// Get from Redis
	data, err := c.client.Get(ctx, prefixkey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return result, ErrCacheMiss
		}
		return result, fmt.Errorf("failed to get from Redis: %w", err)
	}

	// Unmarshal the data
	if err := bson.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return result, nil
}

// Set stores a document in the cache with an optional TTL
func (c *RedisCache[T]) Set(ctx context.Context, key string, data T, ttl time.Duration) error {
	// Create key from ID
	prefixkey := c.getKey(key)

	// Marshal the data
	bytes, err := bson.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Use default TTL if not provided
	if ttl <= 0 {
		ttl = c.options.DefaultTTL
	}

	// Set in Redis
	if err := c.client.Set(ctx, prefixkey, bytes, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set in Redis: %w", err)
	}

	return nil
}

// Delete removes a document from the cache
func (c *RedisCache[T]) Delete(ctx context.Context, key string) error {
	// Create key from ID
	prefixkey := c.getKey(key)

	// Delete from Redis
	if err := c.client.Del(ctx, prefixkey).Err(); err != nil {
		return fmt.Errorf("failed to delete from Redis: %w", err)
	}

	return nil
}

// Clear removes all documents from the cache with the same prefix
func (c *RedisCache[T]) Clear(ctx context.Context) error {
	// Get all keys with the prefix
	pattern := c.prefix + "*"
	keys, err := c.client.Keys(ctx, pattern).Result()
	if err != nil {
		return fmt.Errorf("failed to get keys from Redis: %w", err)
	}

	// Delete all keys
	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("failed to delete keys from Redis: %w", err)
		}
	}

	return nil
}

// Close closes the cache
func (c *RedisCache[T]) Close() error {
	return c.client.Close()
}

// Helper function to create a key from an ObjectID
func (c *RedisCache[T]) getKey(key string) string {
	return c.prefix + key
}

// RedisCacheOptions represents additional options for RedisCache
type RedisCacheOptions struct {
	// Base cache options
	CacheOptions

	// Redis specific options
	Username     string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	KeyPrefix    string
}

// DefaultRedisCacheOptions returns the default RedisCache options
func DefaultRedisCacheOptions() *RedisCacheOptions {
	return &RedisCacheOptions{
		CacheOptions: *DefaultCacheOptions(),

		// Redis specific defaults
		Username:     "",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		KeyPrefix:    "nodestorage:",
	}
}

// NewRedisCacheWithOptions creates a new RedisCache with custom options
func NewRedisCacheWithOptions[T any](redisAddr string, options *RedisCacheOptions) (*RedisCache[T], error) {
	if options == nil {
		options = DefaultRedisCacheOptions()
	}

	// Create Redis client with custom options
	client := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		Username:     options.Username,
		Password:     options.Password,
		DB:           options.DB,
		PoolSize:     options.PoolSize,
		MinIdleConns: options.MinIdleConns,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache[T]{
		client:  client,
		options: &options.CacheOptions,
		prefix:  options.KeyPrefix,
	}, nil
}
