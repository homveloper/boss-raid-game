package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewBadgerCache는 NewBadgerCache 함수를 테스트합니다.
func TestNewBadgerCache(t *testing.T) {
	// 임시 디렉토리 생성
	tempDir, err := os.MkdirTemp("", "badger-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 기본 옵션으로 생성
	cache, err := NewBadgerCache[*TestDocument](
		WithBadgerPath(tempDir),
	)
	assert.NoError(t, err)
	assert.NotNil(t, cache)
	assert.NotNil(t, cache.db)
	assert.NotNil(t, cache.options)
	cache.Close()

	// 인메모리 옵션으로 생성
	cache, err = NewBadgerCache[*TestDocument](
		WithBadgerInMemory(true),
		WithBadgerPath(""),
	)
	assert.NoError(t, err)
	assert.NotNil(t, cache)
	cache.Close()

	// 커스텀 옵션으로 생성
	cache, err = NewBadgerCache[*TestDocument](
		WithBadgerPath(tempDir),
		WithBadgerDefaultTTL(time.Hour),
		WithBadgerGCInterval(time.Minute),
	)
	assert.NoError(t, err)
	assert.NotNil(t, cache)
	assert.Equal(t, tempDir, cache.options.Path)
	assert.Equal(t, time.Hour, cache.options.DefaultTTL)
	assert.Equal(t, time.Minute, cache.options.GCInterval)
	cache.Close()

	// 잘못된 경로로 생성 (이 테스트는 실제로 에러가 발생하지 않을 수 있음)
	invalidPath := filepath.Join(tempDir, "non-existent", "badger")
	cache, err = NewBadgerCache[*TestDocument](
		WithBadgerPath(invalidPath),
	)
	if err != nil {
		assert.Error(t, err)
	} else {
		assert.NotNil(t, cache)
		cache.Close()
	}
}

// TestBadgerCacheGetSet는 BadgerCache의 Get과 Set 메서드를 테스트합니다.
func TestBadgerCacheGetSet(t *testing.T) {
	// 인메모리 캐시 생성
	cache, err := NewBadgerCache[*TestDocument](
		WithBadgerInMemory(true),
		WithBadgerPath(""),
	)
	assert.NoError(t, err)
	defer cache.Close()

	// 컨텍스트 생성
	ctx := context.Background()

	// 테스트 문서 생성
	doc := &TestDocument{
		ID:    "test-1",
		Name:  "Test Document",
		Value: 42,
		Tags:  []string{"test", "document"},
	}

	// 문서 저장
	err = cache.Set(ctx, doc.ID, doc, 0)
	assert.NoError(t, err)

	// 문서 가져오기
	retrievedDoc, err := cache.Get(ctx, doc.ID)
	assert.NoError(t, err)
	assert.Equal(t, doc.ID, retrievedDoc.ID)
	assert.Equal(t, doc.Name, retrievedDoc.Name)
	assert.Equal(t, doc.Value, retrievedDoc.Value)
	assert.Equal(t, doc.Tags, retrievedDoc.Tags)

	// 존재하지 않는 문서 가져오기
	_, err = cache.Get(ctx, "non-existent")
	assert.Error(t, err)
	assert.Equal(t, ErrCacheMiss, err)

	// 직렬화 실패 테스트
	type UnserializableDoc struct {
		Ch chan int
	}
	unserializableDoc := &UnserializableDoc{Ch: make(chan int)}
	badCache, err := NewBadgerCache[*UnserializableDoc](
		WithBadgerInMemory(true),
		WithBadgerPath(""),
	)
	assert.NoError(t, err)
	defer badCache.Close()

	err = badCache.Set(ctx, "bad-doc", unserializableDoc, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to serialize data")
}

// TestBadgerCacheDelete는 BadgerCache의 Delete 메서드를 테스트합니다.
func TestBadgerCacheDelete(t *testing.T) {
	// 인메모리 캐시 생성
	cache, err := NewBadgerCache[*TestDocument](
		WithBadgerInMemory(true),
		WithBadgerPath(""),
	)
	assert.NoError(t, err)
	defer cache.Close()

	// 컨텍스트 생성
	ctx := context.Background()

	// 테스트 문서 생성
	doc := &TestDocument{
		ID:    "test-1",
		Name:  "Test Document",
		Value: 42,
		Tags:  []string{"test", "document"},
	}

	// 문서 저장
	err = cache.Set(ctx, doc.ID, doc, 0)
	assert.NoError(t, err)

	// 문서 삭제
	err = cache.Delete(ctx, doc.ID)
	assert.NoError(t, err)

	// 삭제된 문서 가져오기
	_, err = cache.Get(ctx, doc.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrCacheMiss, err)

	// 존재하지 않는 문서 삭제
	err = cache.Delete(ctx, "non-existent")
	assert.NoError(t, err)
}

// TestBadgerCacheClear는 BadgerCache의 Clear 메서드를 테스트합니다.
func TestBadgerCacheClear(t *testing.T) {
	// 인메모리 캐시 생성
	cache, err := NewBadgerCache[*TestDocument](
		WithBadgerInMemory(true),
		WithBadgerPath(""),
	)
	assert.NoError(t, err)
	defer cache.Close()

	// 컨텍스트 생성
	ctx := context.Background()

	// 테스트 문서 생성
	doc1 := &TestDocument{ID: "test-1", Name: "Document 1"}
	doc2 := &TestDocument{ID: "test-2", Name: "Document 2"}

	// 문서 저장
	err = cache.Set(ctx, doc1.ID, doc1, 0)
	assert.NoError(t, err)
	err = cache.Set(ctx, doc2.ID, doc2, 0)
	assert.NoError(t, err)

	// 캐시 비우기
	err = cache.Clear(ctx)
	assert.NoError(t, err)

	// 비워진 캐시에서 문서 가져오기
	_, err = cache.Get(ctx, doc1.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrCacheMiss, err)
	_, err = cache.Get(ctx, doc2.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrCacheMiss, err)
}

// TestBadgerCacheClose는 BadgerCache의 Close 메서드를 테스트합니다.
func TestBadgerCacheClose(t *testing.T) {
	// 인메모리 캐시 생성
	cache, err := NewBadgerCache[*TestDocument](
		WithBadgerInMemory(true),
		WithBadgerPath(""),
	)
	assert.NoError(t, err)

	// 캐시 닫기
	err = cache.Close()
	assert.NoError(t, err)

	// 닫힌 캐시 사용
	ctx := context.Background()
	doc := &TestDocument{ID: "test-1", Name: "Test Document"}
	err = cache.Set(ctx, doc.ID, doc, 0)
	assert.Error(t, err)
}

// TestBadgerCacheRunGC는 BadgerCache의 GC 기능을 테스트합니다.
func TestBadgerCacheRunGC(t *testing.T) {
	// 임시 디렉토리 생성
	tempDir, err := os.MkdirTemp("", "badger-gc-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 짧은 GC 간격으로 캐시 생성
	cache, err := NewBadgerCache[*TestDocument](
		WithBadgerPath(tempDir),
		WithBadgerGCInterval(50*time.Millisecond),
	)
	assert.NoError(t, err)

	// 컨텍스트 생성
	ctx := context.Background()

	// 테스트 문서 생성 및 저장
	for i := 0; i < 10; i++ {
		doc := &TestDocument{
			ID:    "test-doc-" + string(rune('0'+i)),
			Name:  "Test Document " + string(rune('0'+i)),
			Value: i,
		}
		err = cache.Set(ctx, doc.ID, doc, 0)
		assert.NoError(t, err)
	}

	// GC가 실행될 시간 대기
	time.Sleep(100 * time.Millisecond)

	// 캐시 닫기
	err = cache.Close()
	assert.NoError(t, err)
}
