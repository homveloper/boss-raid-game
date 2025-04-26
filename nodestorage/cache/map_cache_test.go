package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDocument는 테스트용 문서 구조체입니다.
type TestDocument struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Value int      `json:"value"`
	Tags  []string `json:"tags,omitempty"`
}

// TestNewMapCache는 NewMapCache 함수를 테스트합니다.
func TestNewMapCache(t *testing.T) {
	// 기본 옵션으로 생성
	cache := NewMapCache[*TestDocument]()
	assert.NotNil(t, cache)
	assert.NotNil(t, cache.data)
	assert.NotNil(t, cache.options)
	assert.NotNil(t, cache.closeChan)
	assert.False(t, cache.closed)

	// 커스텀 옵션으로 생성
	cache = NewMapCache[*TestDocument](
		WithMapMaxSize(1000),
		WithMapDefaultTTL(time.Hour),
		WithMapEvictionInterval(time.Minute),
	)
	assert.NotNil(t, cache)
	assert.Equal(t, int64(1000), cache.options.MaxSize)
	assert.Equal(t, time.Hour, cache.options.DefaultTTL)
	assert.Equal(t, time.Minute, cache.options.EvictionInterval)
}

// TestMapCacheGetSet는 MapCache의 Get과 Set 메서드를 테스트합니다.
func TestMapCacheGetSet(t *testing.T) {
	// 캐시 생성
	cache := NewMapCache[*TestDocument]()
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
	err := cache.Set(ctx, doc.ID, doc, 0)
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

	// TTL 설정하여 저장
	shortTTL := 50 * time.Millisecond
	err = cache.Set(ctx, "short-ttl", doc, shortTTL)
	assert.NoError(t, err)

	// TTL 만료 전에 가져오기
	_, err = cache.Get(ctx, "short-ttl")
	assert.NoError(t, err)

	// TTL 만료 후에 가져오기
	time.Sleep(shortTTL * 2)
	_, err = cache.Get(ctx, "short-ttl")
	assert.Error(t, err)
	assert.Equal(t, ErrCacheMiss, err)

	// 닫힌 캐시에 저장
	cache.Close()
	err = cache.Set(ctx, doc.ID, doc, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")

	// 닫힌 캐시에서 가져오기
	_, err = cache.Get(ctx, doc.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

// TestMapCacheDelete는 MapCache의 Delete 메서드를 테스트합니다.
func TestMapCacheDelete(t *testing.T) {
	// 캐시 생성
	cache := NewMapCache[*TestDocument]()
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
	err := cache.Set(ctx, doc.ID, doc, 0)
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

	// 닫힌 캐시에서 삭제
	cache.Close()
	err = cache.Delete(ctx, doc.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

// TestMapCacheClear는 MapCache의 Clear 메서드를 테스트합니다.
func TestMapCacheClear(t *testing.T) {
	// 캐시 생성
	cache := NewMapCache[*TestDocument]()
	defer cache.Close()

	// 컨텍스트 생성
	ctx := context.Background()

	// 테스트 문서 생성
	doc1 := &TestDocument{ID: "test-1", Name: "Document 1"}
	doc2 := &TestDocument{ID: "test-2", Name: "Document 2"}

	// 문서 저장
	err := cache.Set(ctx, doc1.ID, doc1, 0)
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

	// 닫힌 캐시 비우기
	cache.Close()
	err = cache.Clear(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

// TestMapCacheClose는 MapCache의 Close 메서드를 테스트합니다.
func TestMapCacheClose(t *testing.T) {
	// 캐시 생성
	cache := NewMapCache[*TestDocument]()

	// 캐시 닫기
	err := cache.Close()
	assert.NoError(t, err)
	assert.True(t, cache.closed)

	// 이미 닫힌 캐시 닫기
	err = cache.Close()
	assert.NoError(t, err)
}

// TestMapCacheEviction은 MapCache의 만료 항목 제거 기능을 테스트합니다.
func TestMapCacheEviction(t *testing.T) {
	// 짧은 제거 간격으로 캐시 생성
	evictionInterval := 50 * time.Millisecond
	cache := NewMapCache[*TestDocument](
		WithMapEvictionInterval(evictionInterval),
	)
	defer cache.Close()

	// 컨텍스트 생성
	ctx := context.Background()

	// 테스트 문서 생성
	doc := &TestDocument{ID: "test-1", Name: "Test Document"}

	// 짧은 TTL로 문서 저장
	ttl := 10 * time.Millisecond
	err := cache.Set(ctx, doc.ID, doc, ttl)
	assert.NoError(t, err)

	// TTL 만료 후 제거 간격 대기
	time.Sleep(ttl + evictionInterval*2)

	// 제거된 문서 가져오기
	_, err = cache.Get(ctx, doc.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrCacheMiss, err)
}

// TestMapCacheEvictionLoop는 evictionLoop 함수를 테스트합니다.
func TestMapCacheEvictionLoop(t *testing.T) {
	// 짧은 제거 간격으로 캐시 생성
	evictionInterval := 50 * time.Millisecond
	cache := NewMapCache[*TestDocument](
		WithMapEvictionInterval(evictionInterval),
	)

	// 컨텍스트 생성
	ctx := context.Background()

	// 테스트 문서 생성
	doc1 := &TestDocument{ID: "test-1", Name: "Document 1"}
	doc2 := &TestDocument{ID: "test-2", Name: "Document 2"}

	// 다른 TTL로 문서 저장
	err := cache.Set(ctx, doc1.ID, doc1, 10*time.Millisecond)
	assert.NoError(t, err)
	err = cache.Set(ctx, doc2.ID, doc2, time.Hour)
	assert.NoError(t, err)

	// 첫 번째 문서의 TTL 만료 후 제거 간격 대기
	time.Sleep(evictionInterval * 2)

	// 첫 번째 문서는 제거되고 두 번째 문서는 남아있어야 함
	_, err = cache.Get(ctx, doc1.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrCacheMiss, err)

	_, err = cache.Get(ctx, doc2.ID)
	assert.NoError(t, err)

	// 캐시 닫기 (evictionLoop 종료)
	cache.Close()

	// evictionLoop가 종료되었는지 확인하기 위해 잠시 대기
	time.Sleep(evictionInterval * 2)
}
