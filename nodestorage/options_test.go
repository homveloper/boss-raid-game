package nodestorage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDefaultOptions는 DefaultOptions 함수를 테스트합니다.
func TestDefaultOptions(t *testing.T) {
	options := DefaultOptions()

	// 기본 옵션 값 확인
	assert.Equal(t, 0, options.MaxRetries)
	assert.Equal(t, time.Millisecond*100, options.RetryDelay)
	assert.Equal(t, time.Second*2, options.MaxRetryDelay)
	assert.Equal(t, 0.1, options.RetryJitter)
	assert.Equal(t, time.Second*30, options.OperationTimeout)
	assert.Equal(t, "version", options.VersionField)
	assert.Equal(t, time.Hour*24, options.CacheTTL)
	assert.True(t, options.WatchEnabled)
	assert.Equal(t, "updateLookup", options.WatchFullDocument)
	assert.Equal(t, time.Second*1, options.WatchMaxAwaitTime)
	assert.Equal(t, int32(100), options.WatchBatchSize)

	// WatchFilter 확인
	assert.Len(t, options.WatchFilter, 1)
	filter := options.WatchFilter[0]
	assert.Equal(t, "$match", filter[0].Key)
}

// TestEditOptions는 EditOptions 관련 함수들을 테스트합니다.
func TestEditOptions(t *testing.T) {
	// 기본 EditOptions 생성
	options := NewEditOptions()

	// 기본 값 확인
	assert.Equal(t, 0, options.MaxRetries)
	assert.Equal(t, time.Millisecond*10, options.RetryDelay)
	assert.Equal(t, time.Millisecond*100, options.MaxRetryDelay)
	assert.Equal(t, 0.1, options.RetryJitter)
	assert.Equal(t, time.Second*10, options.Timeout)

	// WithMaxRetries 옵션 적용
	options = NewEditOptions(WithMaxRetries(5))
	assert.Equal(t, 5, options.MaxRetries)

	// WithRetryDelay 옵션 적용
	options = NewEditOptions(WithRetryDelay(time.Second))
	assert.Equal(t, time.Second, options.RetryDelay)

	// WithMaxRetryDelay 옵션 적용
	options = NewEditOptions(WithMaxRetryDelay(time.Second * 5))
	assert.Equal(t, time.Second*5, options.MaxRetryDelay)

	// WithRetryJitter 옵션 적용
	options = NewEditOptions(WithRetryJitter(0.2))
	assert.Equal(t, 0.2, options.RetryJitter)

	// WithTimeout 옵션 적용
	options = NewEditOptions(WithTimeout(time.Minute))
	assert.Equal(t, time.Minute, options.Timeout)

	// 여러 옵션 함께 적용
	options = NewEditOptions(
		WithMaxRetries(3),
		WithRetryDelay(time.Second*2),
		WithMaxRetryDelay(time.Second*10),
		WithRetryJitter(0.3),
		WithTimeout(time.Second*30),
	)

	assert.Equal(t, 3, options.MaxRetries)
	assert.Equal(t, time.Second*2, options.RetryDelay)
	assert.Equal(t, time.Second*10, options.MaxRetryDelay)
	assert.Equal(t, 0.3, options.RetryJitter)
	assert.Equal(t, time.Second*30, options.Timeout)
}
