package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestErrorVariables는 에러 변수들을 테스트합니다.
func TestErrorVariables(t *testing.T) {
	// 모든 에러 변수 확인
	assert.Equal(t, "cache miss", ErrCacheMiss.Error())
	assert.Equal(t, "cache is closed", ErrClosed.Error())
}
