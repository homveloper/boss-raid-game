package eventsync

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"nodestorage/v2"
	"nodestorage/v2/cache"
)

// setupTestDatabase는 테스트용 MongoDB 데이터베이스를 설정합니다.
func setupTestDatabase(t *testing.T) (*mongo.Client, *mongo.Database) {
	// MongoDB 연결 설정
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// MongoDB 클라이언트 생성
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	require.NoError(t, err)

	// 테스트용 데이터베이스 이름 생성
	dbName := fmt.Sprintf("eventsync_test_%s", time.Now().Format("20060102150405"))
	t.Logf("테스트 데이터베이스 생성: %s", dbName)

	// 데이터베이스 생성
	db := client.Database(dbName)

	// 데이터베이스 존재 확인
	err = db.RunCommand(ctx, bson.D{{Key: "ping", Value: 1}}).Err()
	require.NoError(t, err)
	t.Logf("테스트 데이터베이스 확인 완료: %s", dbName)

	return client, db
}

// teardownTestDatabase는 테스트용 MongoDB 데이터베이스를 정리합니다.
func teardownTestDatabase(t *testing.T, client *mongo.Client, db *mongo.Database) {
	// 데이터베이스 삭제
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Logf("테스트 데이터베이스 삭제: %s", db.Name())
	err := db.Drop(ctx)
	require.NoError(t, err)

	// MongoDB 클라이언트 연결 종료
	err = client.Disconnect(ctx)
	require.NoError(t, err)
}

// setupTestStorage는 테스트용 nodestorage.Storage를 설정합니다.
func setupTestStorage[T nodestorage.Cachable[T]](t *testing.T, db *mongo.Database) nodestorage.Storage[T] {
	// 컬렉션 생성
	collection := db.Collection("documents")

	// 캐시 설정
	cachestorage := cache.NewMemoryCache[T](nil)

	// 스토리지 설정
	storage, err := nodestorage.NewStorage[T](
		context.Background(),
		collection,
		cachestorage,
		&nodestorage.Options{
			VersionField: "Version",
			CacheTTL:     time.Hour,
		},
	)
	require.NoError(t, err)

	return storage
}
