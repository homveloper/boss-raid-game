package eventsync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"

	"nodestorage/v2"
	"nodestorage/v2/cache"
)

// setupStorage는 테스트용 nodestorage.Storage를 설정합니다.
func setupStorage(t *testing.T, collection *mongo.Collection) (nodestorage.Storage[*TestDocument], error) {
	// 컨텍스트 설정
	ctx := context.Background()

	// 캐시 설정
	cacheStorage := cache.NewMemoryCache[*TestDocument](nil)

	// 스토리지 옵션 설정
	storageOptions := &nodestorage.Options{
		VersionField:      "Version",
		WatchEnabled:      true,
		WatchFullDocument: "updateLookup",
	}

	// 스토리지 설정
	storage, err := nodestorage.NewStorage[*TestDocument](
		ctx,
		collection,
		cacheStorage,
		storageOptions,
	)
	require.NoError(t, err)

	return storage, nil
}
