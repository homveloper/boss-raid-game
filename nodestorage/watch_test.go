package nodestorage

import (
	"context"
	"nodestorage/cache"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestStorageWatch는 Storage.Watch 메서드를 테스트합니다.
func TestStorageWatch(t *testing.T) {
	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(testCtx, 30*time.Second)
	defer cancel()

	// 테스트 전 캐시 비우기
	err := testCache.Clear(ctx)
	assert.NoError(t, err)

	// Watch 채널 생성
	watchCh, err := testStorage.Watch(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, watchCh)

	// 이벤트 수신 고루틴 (이벤트는 무시)
	go func() {
		for range watchCh {
			// 이벤트 무시
		}
	}()

	// 테스트 문서 생성
	doc := &TestDocument{
		Name:  "Test Document for Watch",
		Value: 42,
		Tags:  []string{"test", "document", "watch"},
	}

	// 문서 생성 이벤트 테스트
	t.Run("문서 생성 이벤트", func(t *testing.T) {
		// 문서 저장
		createdDoc, err := testStorage.CreateAndGet(ctx, doc)
		assert.NoError(t, err)
		assert.NotNil(t, createdDoc)
		id := createdDoc.ID
		assert.NotEqual(t, primitive.NilObjectID, id)

		// 문서 편집 이벤트 테스트
		t.Run("문서 편집 이벤트", func(t *testing.T) {
			// 문서 편집
			updatedDoc, diff, err := testStorage.Edit(ctx, id, func(d *TestDocument) (*TestDocument, error) {
				d.Name = "Updated Document"
				d.Value = 84
				d.Tags = append(d.Tags, "updated")
				return d, nil
			})
			assert.NoError(t, err)
			assert.NotNil(t, updatedDoc)
			assert.NotNil(t, diff)

			// 문서 삭제 이벤트 테스트
			t.Run("문서 삭제 이벤트", func(t *testing.T) {
				// 문서 삭제
				err := testStorage.Delete(ctx, id)
				assert.NoError(t, err)
			})
		})
	})

	// 컨텍스트 취소 시 채널 닫힘 확인
	t.Run("컨텍스트 취소 시 채널 닫힘", func(t *testing.T) {
		// 새 컨텍스트 생성
		watchCtx, watchCancel := context.WithCancel(ctx)

		// Watch 채널 생성
		ch, err := testStorage.Watch(watchCtx)
		assert.NoError(t, err)
		assert.NotNil(t, ch)

		// 채널 닫힘 확인을 위한 채널
		closed := make(chan struct{})
		go func() {
			for range ch {
				// 이벤트 무시
			}
			close(closed)
		}()

		// 컨텍스트 취소
		watchCancel()

		// 채널 닫힘 확인
		select {
		case <-closed:
			// 채널이 닫힘
		case <-time.After(5 * time.Second):
			t.Fatal("채널이 닫히지 않음")
		}
	})

	// 닫힌 스토리지에서 Watch 호출 테스트
	t.Run("닫힌 스토리지에서 Watch 호출", func(t *testing.T) {
		// 임시 스토리지 생성
		tempCache := cache.NewMapCache[*TestDocument]()
		tempStorage, err := NewStorage[*TestDocument](ctx, testClient, testColl, tempCache, DefaultOptions())
		assert.NoError(t, err)

		// 스토리지 닫기
		err = tempStorage.Close()
		assert.NoError(t, err)

		// 닫힌 스토리지에서 Watch 호출 시도
		_, err = tempStorage.Watch(ctx)
		assert.Error(t, err)
		assert.Equal(t, ErrClosed, err)
	})

	// 메인 컨텍스트 취소
	cancel()
}
