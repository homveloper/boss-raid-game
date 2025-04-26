package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDefaultMapCacheOptions는 DefaultMapCacheOptions 함수를 테스트합니다.
func TestDefaultMapCacheOptions(t *testing.T) {
	options := DefaultMapCacheOptions()

	// 기본 옵션 값 확인
	assert.Equal(t, int64(10000), options.MaxSize)
	assert.Equal(t, time.Hour*24, options.DefaultTTL)
	assert.Equal(t, time.Minute*5, options.EvictionInterval)
}

// TestDefaultBadgerCacheOptions는 DefaultBadgerCacheOptions 함수를 테스트합니다.
func TestDefaultBadgerCacheOptions(t *testing.T) {
	options := DefaultBadgerCacheOptions()

	// 기본 옵션 값 확인
	assert.Equal(t, "./badger-data", options.Path)
	assert.False(t, options.InMemory)
	assert.Equal(t, time.Hour*24, options.DefaultTTL)
	assert.Equal(t, time.Minute*5, options.GCInterval)
}

// TestMapCacheOptions는 MapCache 옵션 함수들을 테스트합니다.
func TestMapCacheOptions(t *testing.T) {
	// 기본 옵션 생성
	options := DefaultMapCacheOptions()

	// WithMapMaxSize 옵션 적용
	WithMapMaxSize(5000)(options)
	assert.Equal(t, int64(5000), options.MaxSize)

	// WithMapDefaultTTL 옵션 적용
	WithMapDefaultTTL(time.Hour)(options)
	assert.Equal(t, time.Hour, options.DefaultTTL)

	// WithMapEvictionInterval 옵션 적용
	WithMapEvictionInterval(time.Minute)(options)
	assert.Equal(t, time.Minute, options.EvictionInterval)
}

// TestBadgerCacheOptions는 BadgerCache 옵션 함수들을 테스트합니다.
func TestBadgerCacheOptions(t *testing.T) {
	// 기본 옵션 생성
	options := DefaultBadgerCacheOptions()

	// WithBadgerPath 옵션 적용
	WithBadgerPath("/tmp/badger-test")(options)
	assert.Equal(t, "/tmp/badger-test", options.Path)

	// WithBadgerInMemory 옵션 적용
	WithBadgerInMemory(true)(options)
	assert.True(t, options.InMemory)

	// WithBadgerDefaultTTL 옵션 적용
	WithBadgerDefaultTTL(time.Hour)(options)
	assert.Equal(t, time.Hour, options.DefaultTTL)

	// WithBadgerGCInterval 옵션 적용
	WithBadgerGCInterval(time.Minute)(options)
	assert.Equal(t, time.Minute, options.GCInterval)
}
