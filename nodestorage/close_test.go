package nodestorage

import (
	"context"
	"nodestorage/cache"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestStorageClose는 Storage.Close 메서드를 테스트합니다.
func TestStorageClose(t *testing.T) {
	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	// 테스트 케이스
	tests := []struct {
		name        string
		setup       func() Storage[*TestDocument]
		expectError bool
	}{
		{
			name: "스토리지 닫기",
			setup: func() Storage[*TestDocument] {
				// 임시 스토리지 생성
				tempCache := cache.NewMapCache[*TestDocument]()
				tempStorage, err := NewStorage[*TestDocument](ctx, testClient, testColl, tempCache, DefaultOptions())
				assert.NoError(t, err)
				return tempStorage
			},
			expectError: false,
		},
		{
			name: "이미 닫힌 스토리지 닫기",
			setup: func() Storage[*TestDocument] {
				// 임시 스토리지 생성
				tempCache := cache.NewMapCache[*TestDocument]()
				tempStorage, err := NewStorage[*TestDocument](ctx, testClient, testColl, tempCache, DefaultOptions())
				assert.NoError(t, err)

				// 스토리지 닫기
				err = tempStorage.Close()
				assert.NoError(t, err)

				return tempStorage
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// 테스트 설정
			storage := tc.setup()

			// 스토리지 닫기
			err := storage.Close()

			// 결과 확인
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// 닫은 후 다른 메서드 호출 시 에러 확인
			_, err = storage.Get(ctx, primitive.NewObjectID())
			assert.Error(t, err)
			assert.Equal(t, ErrClosed, err)

			_, err = storage.GetByQuery(ctx, nil)
			assert.Error(t, err)
			assert.Equal(t, ErrClosed, err)

			_, err = storage.CreateAndGet(ctx, &TestDocument{})
			assert.Error(t, err)
			assert.Equal(t, ErrClosed, err)

			_, _, err = storage.Edit(ctx, primitive.NewObjectID(), func(d *TestDocument) (*TestDocument, error) {
				return d, nil
			})
			assert.Error(t, err)
			assert.Equal(t, ErrClosed, err)

			err = storage.Delete(ctx, primitive.NewObjectID())
			assert.Error(t, err)
			assert.Equal(t, ErrClosed, err)

			_, err = storage.Watch(ctx)
			assert.Error(t, err)
			assert.Equal(t, ErrClosed, err)
		})
	}
}
