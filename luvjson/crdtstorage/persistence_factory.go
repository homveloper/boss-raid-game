package crdtstorage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/mongo"
)

// NewMemoryPersistence는 메모리 기반 영구 저장소를 생성합니다.
func NewMemoryPersistence() PersistenceAdapter {
	return NewMemoryAdapter()
}

// NewFilePersistence는 파일 기반 영구 저장소를 생성합니다.
func NewFilePersistence(basePath string) (PersistenceAdapter, error) {
	return NewFileAdapter(basePath)
}

// NewRedisPersistence는 Redis 기반 영구 저장소를 생성합니다.
func NewRedisPersistence(client *redis.Client, keyPrefix string) PersistenceAdapter {
	return NewRedisAdapter(client, keyPrefix)
}

// NewSQLPersistence는 SQL 데이터베이스 기반 영구 저장소를 생성합니다.
func NewSQLPersistence(db *sql.DB, tableName string) (PersistenceAdapter, error) {
	return NewSQLAdapter(db, tableName)
}

// NewAdvancedSQLPersistence는 스냅샷을 지원하는 SQL 데이터베이스 기반 영구 저장소를 생성합니다.
func NewAdvancedSQLPersistence(db *sql.DB, tableName string, snapshotTableName string) (AdvancedPersistenceProvider, error) {
	return NewAdvancedSQLAdapter(db, tableName, snapshotTableName)
}

// NewMongoDBPersistence는 MongoDB 기반 영구 저장소를 생성합니다.
func NewMongoDBPersistence(collection *mongo.Collection) PersistenceAdapter {
	return NewMongoDBAdapter(collection)
}

// createPersistenceAdapter는 영구 저장소 어댑터를 생성합니다.
func createPersistenceAdapter(ctx context.Context, options *StorageOptions, customPersistence PersistenceAdapter) (PersistenceAdapter, error) {
	// 사용자 정의 영구 저장소가 있는 경우 사용
	if customPersistence != nil {
		return customPersistence, nil
	}

	// 기본 영구 저장소 생성
	switch options.PersistenceType {
	case "memory":
		return NewMemoryPersistence(), nil
	case "file":
		return NewFilePersistence(options.PersistencePath)
	case "redis":
		// Redis 클라이언트 생성
		redisClient := redis.NewClient(&redis.Options{
			Addr:     options.RedisAddr,
			Password: options.RedisPassword,
			DB:       options.RedisDB,
		})

		// Redis 연결 테스트
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("failed to connect to Redis: %w", err)
		}

		return NewRedisPersistence(redisClient, options.KeyPrefix), nil
	case "sql":
		// SQL 어댑터는 외부에서 생성해야 함
		return nil, fmt.Errorf("SQL persistence type requires a custom persistence adapter")
	case "custom":
		return nil, fmt.Errorf("custom persistence type requires a custom persistence adapter")
	default:
		return nil, fmt.Errorf("unsupported persistence type: %s", options.PersistenceType)
	}
}
