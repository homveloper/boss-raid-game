package nodestorage

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestConcurrentEdits는 여러 고루틴에서 동시에 같은 문서를 편집하는 상황을 테스트합니다.
func TestConcurrentEdits(t *testing.T) {
	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(testCtx, 30*time.Second)
	defer cancel()

	// 테스트 전 캐시 비우기
	err := testCache.Clear(ctx)
	assert.NoError(t, err)

	// 테스트 문서 생성
	doc := &TestDocument{
		Name:  "Concurrent Test",
		Value: 0,
		Tags:  []string{"test", "concurrent"},
	}

	// 문서 저장
	createdDoc, err := testStorage.CreateAndGet(ctx, doc)
	require.NoError(t, err)
	require.NotNil(t, createdDoc)
	id := createdDoc.ID
	require.NotEqual(t, primitive.NilObjectID, id)

	// 동시 편집 횟수
	const numEdits = 10

	// 동기화를 위한 WaitGroup
	var wg sync.WaitGroup
	wg.Add(numEdits)

	// 에러 채널
	errCh := make(chan error, numEdits)
	successCh := make(chan struct{}, numEdits)

	// 여러 고루틴에서 동시에 같은 문서 편집
	for i := 0; i < numEdits; i++ {
		go func(idx int) {
			defer wg.Done()

			// 약간의 지연 추가 (동시성 증가)
			time.Sleep(time.Duration(idx) * time.Millisecond)

			// 문서 편집
			_, _, err := testStorage.Edit(ctx, id, func(d *TestDocument) (*TestDocument, error) {
				d.Value += 1
				d.Tags = append(d.Tags, "edited")
				return d, nil
			})

			if err != nil {
				errCh <- err
			} else {
				successCh <- struct{}{}
			}
		}(i)
	}

	// 모든 고루틴 완료 대기
	wg.Wait()
	close(errCh)
	close(successCh)

	// 에러 확인
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}

	// 성공 횟수
	successCount := 0
	for range successCh {
		successCount++
	}

	// 최종 문서 확인
	finalDoc, err := testStorage.Get(ctx, id)
	assert.NoError(t, err)
	assert.NotNil(t, finalDoc)

	// 로그 출력
	t.Logf("총 편집 시도: %d", numEdits)
	t.Logf("성공한 편집: %d", successCount)
	t.Logf("실패한 편집: %d", len(errors))
	t.Logf("최종 값: %d", finalDoc.Value)
	t.Logf("최종 태그 수: %d", len(finalDoc.Tags))
	t.Logf("최종 버전: %d", finalDoc.Version())

	// 성공한 편집 횟수 + 실패한 편집 횟수 = 총 편집 시도 횟수
	assert.Equal(t, numEdits, successCount+len(errors))

	// 최종 값은 성공한 편집 횟수와 같아야 함
	assert.Equal(t, successCount, finalDoc.Value)

	// 태그 수는 초기 태그 수 + 성공한 편집 횟수
	assert.Equal(t, 2+successCount, len(finalDoc.Tags))
}

// TestContextCancellation은 컨텍스트 취소 상황을 테스트합니다.
func TestContextCancellation(t *testing.T) {
	// 취소 가능한 컨텍스트 생성
	ctx, cancel := context.WithCancel(context.Background())

	// 컨텍스트 취소
	cancel()

	// 취소된 컨텍스트로 작업 시도
	_, err := testStorage.Get(ctx, primitive.NewObjectID())

	// 컨텍스트 취소 에러 확인
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

// TestOperationTimeout은 작업 타임아웃 상황을 테스트합니다.
func TestOperationTimeout(t *testing.T) {
	// 매우 짧은 타임아웃으로 컨텍스트 생성
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// 약간 대기하여 타임아웃 발생
	time.Sleep(1 * time.Millisecond)

	// 타임아웃된 컨텍스트로 작업 시도
	_, err := testStorage.Get(ctx, primitive.NewObjectID())

	// 컨텍스트 타임아웃 에러 확인
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}
