package nodestorage

import (
	"context"
	"nodestorage/cache"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// TestStorageDelete는 Storage.Delete 메서드를 테스트합니다.
func TestStorageDelete(t *testing.T) {
	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	// 테스트 전 캐시 비우기
	err := testCache.Clear(ctx)
	assert.NoError(t, err)

	// 테스트 문서 생성
	doc := &TestDocument{
		Name:  "Test Document for Delete",
		Value: 42,
		Tags:  []string{"test", "document", "delete"},
	}

	// 문서 저장
	createdDoc, err := testStorage.CreateAndGet(ctx, doc)
	require.NoError(t, err)
	require.NotNil(t, createdDoc)
	id := createdDoc.ID
	require.NotEqual(t, primitive.NilObjectID, id)

	// 테스트 케이스
	tests := []struct {
		name        string
		id          primitive.ObjectID
		expectError bool
		setup       func()
		verify      func()
	}{
		{
			name:        "문서 삭제",
			id:          id,
			expectError: false,
			setup: func() {
				// 문서가 DB에 있는지 확인
				var savedDoc TestDocument
				err := testColl.FindOne(ctx, bson.M{"_id": id}).Decode(&savedDoc)
				assert.NoError(t, err)

				// 문서가 캐시에 있는지 확인
				_, err = testCache.Get(ctx, id)
				assert.NoError(t, err)
			},
			verify: func() {
				// 문서가 DB에서 삭제되었는지 확인
				err := testColl.FindOne(ctx, bson.M{"_id": id}).Decode(&TestDocument{})
				assert.Error(t, err)
				assert.Equal(t, mongo.ErrNoDocuments, err)

				// 문서가 캐시에서 삭제되었는지 확인
				_, err = testCache.Get(ctx, id)
				assert.Error(t, err)
				assert.Equal(t, cache.ErrCacheMiss, err)
			},
		},
		{
			name:        "존재하지 않는 문서 삭제",
			id:          primitive.NewObjectID(),
			expectError: false,
			setup:       func() {},
			verify:      func() {},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// 테스트 설정
			tc.setup()

			// 문서 삭제
			err := testStorage.Delete(ctx, tc.id)

			// 결과 확인
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				tc.verify()
			}
		})
	}

	// 닫힌 스토리지에서 문서 삭제 테스트
	t.Run("닫힌 스토리지에서 문서 삭제", func(t *testing.T) {
		// 임시 스토리지 생성
		tempCache := cache.NewMapCache[*TestDocument]()
		tempStorage, err := NewStorage[*TestDocument](ctx, testClient, testColl, tempCache, DefaultOptions())
		assert.NoError(t, err)

		// 스토리지 닫기
		err = tempStorage.Close()
		assert.NoError(t, err)

		// 닫힌 스토리지에서 문서 삭제 시도
		err = tempStorage.Delete(ctx, primitive.NewObjectID())
		assert.Error(t, err)
		assert.Equal(t, ErrClosed, err)
	})
}
