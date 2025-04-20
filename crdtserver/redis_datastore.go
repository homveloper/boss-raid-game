package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	ds "github.com/ipfs/go-datastore"
	dsq "github.com/ipfs/go-datastore/query"
)

var _ ds.Datastore = (*RedisDatastore)(nil)
var _ ds.Batching = (*RedisDatastore)(nil)

// Options Redis 데이터스토어 옵션
type Options struct {
	TTL time.Duration
}

// DefaultOptions 기본 옵션 반환
func DefaultOptions() *Options {
	return &Options{
		TTL: time.Hour * 24 * 7, // 1주일
	}
}

// RedisDatastore Redis 기반 데이터스토어
type RedisDatastore struct {
	client *redis.Client
	ttl    time.Duration
	mu     sync.Mutex
}

// NewRedisDatastore 새 Redis 데이터스토어 생성
func NewRedisDatastore(client *redis.Client, opts *Options) (*RedisDatastore, error) {
	if client == nil {
		return nil, errors.New("redis client is nil")
	}

	if opts == nil {
		opts = DefaultOptions()
	}

	return &RedisDatastore{
		client: client,
		ttl:    opts.TTL,
	}, nil
}

// Put 데이터 저장
func (rd *RedisDatastore) Put(ctx context.Context, key ds.Key, value []byte) error {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	if rd.ttl > 0 {
		return rd.client.Set(ctx, key.String(), value, rd.ttl).Err()
	}
	return rd.client.Set(ctx, key.String(), value, 0).Err()
}

// Get 데이터 조회
func (rd *RedisDatastore) Get(ctx context.Context, key ds.Key) ([]byte, error) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	data, err := rd.client.Get(ctx, key.String()).Bytes()
	if err == redis.Nil {
		return nil, ds.ErrNotFound
	}
	return data, err
}

// Has 키 존재 여부 확인
func (rd *RedisDatastore) Has(ctx context.Context, key ds.Key) (bool, error) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	val, err := rd.client.Exists(ctx, key.String()).Result()
	if err != nil {
		return false, err
	}
	return val > 0, nil
}

// GetSize 데이터 크기 조회
func (rd *RedisDatastore) GetSize(ctx context.Context, key ds.Key) (int, error) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	size, err := rd.client.StrLen(ctx, key.String()).Result()
	if err == redis.Nil {
		return 0, ds.ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	return int(size), nil
}

// Delete 데이터 삭제
func (rd *RedisDatastore) Delete(ctx context.Context, key ds.Key) error {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	return rd.client.Del(ctx, key.String()).Err()
}

// Query 데이터 쿼리
func (rd *RedisDatastore) Query(ctx context.Context, q dsq.Query) (dsq.Results, error) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	// 패턴 생성
	pattern := "*"
	if q.Prefix != "" {
		pattern = q.Prefix + "*"
	}

	// 키 스캔
	var keys []string
	var cursor uint64
	var err error

	for {
		var scanKeys []string
		scanKeys, cursor, err = rd.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, scanKeys...)
		if cursor == 0 {
			break
		}
	}

	// 결과 생성
	entries := make([]dsq.Entry, 0, len(keys))
	for _, key := range keys {
		var value []byte
		if !q.KeysOnly {
			value, err = rd.client.Get(ctx, key).Bytes()
			if err != nil && err != redis.Nil {
				continue
			}
		}

		entry := dsq.Entry{
			Key: key,
		}
		if !q.KeysOnly {
			entry.Value = value
		}
		entries = append(entries, entry)
	}

	// 정렬 및 필터링
	if q.Offset > 0 && q.Offset < len(entries) {
		entries = entries[q.Offset:]
	}

	if q.Limit > 0 && q.Limit < len(entries) {
		entries = entries[:q.Limit]
	}

	// 결과 반환
	return dsq.ResultsWithEntries(q, entries), nil
}

// Batch 배치 작업 생성
func (rd *RedisDatastore) Batch(ctx context.Context) (ds.Batch, error) {
	return &redisBatch{
		ctx:      ctx,
		ds:       rd,
		pipeline: rd.client.Pipeline(),
	}, nil
}

// Close 데이터스토어 종료
func (rd *RedisDatastore) Close() error {
	return rd.client.Close()
}

// Sync 동기화 (Redis는 필요 없음)
func (rd *RedisDatastore) Sync(ctx context.Context, prefix ds.Key) error {
	return nil
}

// redisBatch Redis 배치 작업
type redisBatch struct {
	ctx      context.Context
	ds       *RedisDatastore
	pipeline redis.Pipeliner
	size     int
}

// Put 배치에 데이터 추가
func (rb *redisBatch) Put(ctx context.Context, key ds.Key, value []byte) error {
	if rb.ds.ttl > 0 {
		rb.pipeline.Set(rb.ctx, key.String(), value, rb.ds.ttl)
	} else {
		rb.pipeline.Set(rb.ctx, key.String(), value, 0)
	}
	rb.size++
	return nil
}

// Delete 배치에서 데이터 삭제
func (rb *redisBatch) Delete(ctx context.Context, key ds.Key) error {
	rb.pipeline.Del(ctx, key.String())
	rb.size++
	return nil
}

// Commit 배치 작업 커밋
func (rb *redisBatch) Commit(ctx context.Context) error {
	if rb.size == 0 {
		return nil
	}

	_, err := rb.pipeline.Exec(ctx)
	return err
}
