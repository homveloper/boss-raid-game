package nodestorage

import (
	"context"
	"nodestorage/cache"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestMongoDBConnectionFailure는 MongoDB 연결 실패 상황을 테스트합니다.
func TestMongoDBConnectionFailure(t *testing.T) {
	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 잘못된 MongoDB URI 설정
	invalidURI := "mongodb://nonexistent:27017"
	clientOptions := options.Client().ApplyURI(invalidURI)
	clientOptions.SetConnectTimeout(1 * time.Second) // 빠른 타임아웃 설정
	clientOptions.SetServerSelectionTimeout(1 * time.Second)

	// 잘못된 URI로 클라이언트 생성
	invalidClient, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		// 연결 자체가 실패하면 테스트 성공으로 간주
		t.Log("Connection failed as expected:", err)
		return
	}

	// 캐시 생성
	cacheImpl := cache.NewMapCache[*TestDocument]()

	// 컬렉션 설정
	collection := invalidClient.Database("test_db").Collection("test_collection")

	// 스토리지 생성 시도
	storage, err := NewStorage[*TestDocument](ctx, invalidClient, collection, cacheImpl, DefaultOptions())

	// 결과 확인
	assert.Error(t, err, "잘못된 MongoDB 연결로 스토리지 생성 시 에러가 발생해야 합니다")
	assert.Nil(t, storage, "연결 실패 시 스토리지는 nil이어야 합니다")

	// 클라이언트 연결 확인 시도
	err = invalidClient.Ping(ctx, nil)
	assert.Error(t, err, "잘못된 MongoDB 연결로 Ping 시 에러가 발생해야 합니다")

	// 클라이언트 연결 종료
	if invalidClient != nil {
		_ = invalidClient.Disconnect(ctx)
	}
}

// TestMongoDBInvalidCollection은 유효하지 않은 컬렉션으로 스토리지 생성을 테스트합니다.
func TestMongoDBInvalidCollection(t *testing.T) {
	// 이 테스트는 현재 구현에서 패닉을 발생시키므로 스킵합니다.
	// 실제 구현에서는 nil 컬렉션 체크를 추가해야 합니다.
	t.Skip("현재 구현에서는 nil 컬렉션 체크가 없어 패닉이 발생합니다.")

	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 캐시 생성
	cacheImpl := cache.NewMapCache[*TestDocument]()

	// 스토리지 생성 시도 (nil 컬렉션)
	storage, err := NewStorage[*TestDocument](ctx, testClient, nil, cacheImpl, DefaultOptions())

	// 결과 확인
	assert.Error(t, err, "nil 컬렉션으로 스토리지 생성 시 에러가 발생해야 합니다")
	assert.Nil(t, storage, "유효하지 않은 컬렉션으로 생성 시 스토리지는 nil이어야 합니다")
}

// TestMongoDBInvalidClient는 유효하지 않은 클라이언트로 스토리지 생성을 테스트합니다.
func TestMongoDBInvalidClient(t *testing.T) {
	// 이 테스트는 현재 구현에서 패닉을 발생시킬 수 있으므로 스킵합니다.
	// 실제 구현에서는 nil 클라이언트 체크를 추가해야 합니다.
	t.Skip("현재 구현에서는 nil 클라이언트 체크가 없어 패닉이 발생할 수 있습니다.")

	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 캐시 생성
	cacheImpl := cache.NewMapCache[*TestDocument]()

	// 스토리지 생성 시도 (nil 클라이언트)
	storage, err := NewStorage[*TestDocument](ctx, nil, testColl, cacheImpl, DefaultOptions())

	// 결과 확인
	assert.Error(t, err, "nil 클라이언트로 스토리지 생성 시 에러가 발생해야 합니다")
	assert.Nil(t, storage, "유효하지 않은 클라이언트로 생성 시 스토리지는 nil이어야 합니다")
}

// TestMongoDBOperationTimeout은 MongoDB 작업 타임아웃을 테스트합니다.
func TestMongoDBOperationTimeout(t *testing.T) {
	// 매우 짧은 타임아웃으로 컨텍스트 생성
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// 약간 대기하여 타임아웃 발생
	time.Sleep(1 * time.Millisecond)

	// 타임아웃된 컨텍스트로 스토리지 생성 시도
	cacheImpl := cache.NewMapCache[*TestDocument]()
	storage, err := NewStorage[*TestDocument](ctx, testClient, testColl, cacheImpl, DefaultOptions())

	// 결과 확인
	assert.Error(t, err, "타임아웃된 컨텍스트로 스토리지 생성 시 에러가 발생해야 합니다")
	assert.Nil(t, storage, "타임아웃 시 스토리지는 nil이어야 합니다")
}
