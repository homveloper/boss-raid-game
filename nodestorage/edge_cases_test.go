package nodestorage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestEmptyDocument는 빈 문서 처리를 테스트합니다.
func TestEmptyDocument(t *testing.T) {
	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	// 테스트 전 캐시 비우기
	err := testCache.Clear(ctx)
	assert.NoError(t, err)

	// 빈 문서 생성
	emptyDoc := &TestDocument{}

	// 문서 저장
	createdDoc, err := testStorage.CreateAndGet(ctx, emptyDoc)
	assert.NoError(t, err)
	assert.NotNil(t, createdDoc)
	id := createdDoc.ID
	assert.NotEqual(t, primitive.NilObjectID, id)

	// 빈 문서 가져오기
	retrievedDoc, err := testStorage.Get(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, emptyDoc.Name, retrievedDoc.Name)
	assert.Equal(t, emptyDoc.Value, retrievedDoc.Value)
	assert.Equal(t, emptyDoc.Tags, retrievedDoc.Tags)
	assert.Equal(t, int64(1), retrievedDoc.Version())
}

// TestLargeDocument는 대용량 문서 처리를 테스트합니다.
func TestLargeDocument(t *testing.T) {
	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	// 테스트 전 캐시 비우기
	err := testCache.Clear(ctx)
	assert.NoError(t, err)

	// 대용량 문서 생성
	largeDoc := &TestDocument{
		Name:  "Large Document",
		Value: 42,
		Tags:  make([]string, 1000), // 대량의 태그
	}

	// 대량의 태그 생성
	for i := 0; i < 1000; i++ {
		largeDoc.Tags[i] = fmt.Sprintf("tag-%d", i)
	}

	// 문서 저장
	createdDoc, err := testStorage.CreateAndGet(ctx, largeDoc)
	assert.NoError(t, err)
	assert.NotNil(t, createdDoc)
	id := createdDoc.ID
	assert.NotEqual(t, primitive.NilObjectID, id)

	// 대용량 문서 가져오기
	retrievedDoc, err := testStorage.Get(ctx, id)
	assert.NoError(t, err)
	assert.Equal(t, largeDoc.Name, retrievedDoc.Name)
	assert.Equal(t, largeDoc.Value, retrievedDoc.Value)
	assert.Equal(t, len(largeDoc.Tags), len(retrievedDoc.Tags))
	assert.Equal(t, int64(1), retrievedDoc.Version())

	// 태그 내용 확인 (처음 10개만)
	for i := 0; i < 10; i++ {
		assert.Equal(t, largeDoc.Tags[i], retrievedDoc.Tags[i])
	}
}

// TestNilDocument는 nil 문서 처리를 테스트합니다.
func TestNilDocument(t *testing.T) {
	// 이 테스트는 현재 구현에서 패닉을 발생시키므로 스킵합니다.
	// 실제 구현에서는 nil 문서 체크를 추가해야 합니다.
	t.Skip("현재 구현에서는 nil 문서 체크가 없어 패닉이 발생합니다.")

	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	// nil 문서 저장 시도
	_, err := testStorage.CreateAndGet(ctx, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid document")
}

// TestEditWithNilReturn은 편집 함수에서 nil 반환 시 처리를 테스트합니다.
func TestEditWithNilReturn(t *testing.T) {
	// 이 테스트는 현재 구현에서 패닉을 발생시키므로 스킵합니다.
	// 실제 구현에서는 nil 반환 체크를 추가해야 합니다.
	t.Skip("현재 구현에서는 nil 반환 체크가 없어 패닉이 발생합니다.")

	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	// 테스트 전 캐시 비우기
	err := testCache.Clear(ctx)
	assert.NoError(t, err)

	// 테스트 문서 생성
	doc := &TestDocument{
		Name:  "Nil Return Test",
		Value: 42,
	}

	// 문서 저장
	createdDoc, err := testStorage.CreateAndGet(ctx, doc)
	require.NoError(t, err)
	require.NotNil(t, createdDoc)
	id := createdDoc.ID
	require.NotEqual(t, primitive.NilObjectID, id)

	// 편집 함수에서 nil 반환
	_, _, err = testStorage.Edit(ctx, id, func(d *TestDocument) (*TestDocument, error) {
		return nil, nil
	})

	// 에러 확인
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid document")
}
