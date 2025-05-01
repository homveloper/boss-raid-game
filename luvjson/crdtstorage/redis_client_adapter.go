package crdtstorage

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisClientAdapter는 go-redis/redis 클라이언트를 RedisClient 인터페이스에 맞게 어댑터합니다.
type RedisClientAdapter struct {
	client *redis.Client
}

// NewRedisClientAdapter는 새 Redis 클라이언트 어댑터를 생성합니다.
func NewRedisClientAdapter(client *redis.Client) *RedisClientAdapter {
	return &RedisClientAdapter{
		client: client,
	}
}

// SetNX는 키가 존재하지 않는 경우에만 값을 설정합니다.
func (a *RedisClientAdapter) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	return a.client.SetNX(ctx, key, value, expiration).Result()
}

// Eval은 Lua 스크립트를 실행합니다.
func (a *RedisClientAdapter) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return a.client.Eval(ctx, script, keys, args...).Result()
}

// Del은 키를 삭제합니다.
func (a *RedisClientAdapter) Del(ctx context.Context, keys ...string) (int64, error) {
	return a.client.Del(ctx, keys...).Result()
}

// Expire는 키의 만료 시간을 설정합니다.
func (a *RedisClientAdapter) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	return a.client.Expire(ctx, key, expiration).Result()
}

// Get은 키의 값을 가져옵니다.
func (a *RedisClientAdapter) Get(ctx context.Context, key string) (string, error) {
	return a.client.Get(ctx, key).Result()
}

// Set은 키에 값을 설정합니다.
func (a *RedisClientAdapter) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return a.client.Set(ctx, key, value, expiration).Err()
}
