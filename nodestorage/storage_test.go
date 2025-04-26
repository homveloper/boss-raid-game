package nodestorage

import (
	"context"
	"errors"
	"fmt"
	"nodestorage/cache"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TestDocument는 테스트에 사용할 문서 구조체입니다.
type TestDocument struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	Name         string             `bson:"name"`
	Value        int                `bson:"value"`
	Tags         []string           `bson:"tags"`
	VersionField int64              `bson:"version"`
}

// Version은 문서의 버전을 가져오거나 설정합니다.
func (d *TestDocument) Version(v ...int64) int64 {
	if len(v) > 0 {
		d.VersionField = v[0]
	}
	return d.VersionField
}

// Copy는 문서의 복사본을 반환합니다.
func (d *TestDocument) Copy() *TestDocument {
	if d == nil {
		return nil
	}
	copy := *d
	if d.Tags != nil {
		copy.Tags = make([]string, len(d.Tags))
		for i, tag := range d.Tags {
			copy.Tags[i] = tag
		}
	}
	return &copy
}

var (
	testCtx     context.Context
	testClient  *mongo.Client
	testColl    *mongo.Collection
	testCache   cache.Cache[*TestDocument]
	testStorage Storage[*TestDocument]
)

// setupTestEnvironment는 테스트 환경을 설정합니다.
func setupTestEnvironment() error {
	// 테스트 컨텍스트 생성
	testCtx = context.Background()

	// MongoDB 클라이언트 생성
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	var err error
	testClient, err = mongo.Connect(testCtx, clientOptions)
	if err != nil {
		return err
	}

	// 테스트 컬렉션 생성
	testColl = testClient.Database("test_db").Collection("test_collection")

	// 테스트 캐시 생성
	testCache = cache.NewMapCache[*TestDocument]()

	// 테스트 스토리지 생성
	opts := DefaultOptions()
	testStorage, err = NewStorage[*TestDocument](testCtx, testClient, testColl, testCache, opts)
	if err != nil {
		return err
	}

	return nil
}

// cleanupTestEnvironment는 테스트 환경을 정리합니다.
func cleanupTestEnvironment() error {
	// 스토리지 닫기
	if testStorage != nil {
		if err := testStorage.Close(); err != nil {
			return err
		}
	}

	// 캐시 닫기
	if testCache != nil {
		if err := testCache.Close(); err != nil {
			return err
		}
	}

	// 테스트 컬렉션 삭제
	if testColl != nil {
		if err := testColl.Drop(testCtx); err != nil {
			return err
		}
	}

	// MongoDB 클라이언트 연결 종료
	if testClient != nil {
		if err := testClient.Disconnect(testCtx); err != nil {
			return err
		}
	}

	return nil
}

// TestMain은 테스트 전체의 설정과 정리를 담당합니다.
func TestMain(m *testing.M) {
	// 테스트 환경 설정
	if err := setupTestEnvironment(); err != nil {
		fmt.Printf("Failed to setup test environment: %v\n", err)
		os.Exit(1)
	}

	// 테스트 실행
	code := m.Run()

	// 테스트 환경 정리
	if err := cleanupTestEnvironment(); err != nil {
		fmt.Printf("Failed to cleanup test environment: %v\n", err)
	}

	os.Exit(code)
}

// TestStorageGet은 Storage.Get 메서드를 테스트합니다.
func TestStorageGet(t *testing.T) {
	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	// 테스트 문서 생성
	doc := &TestDocument{
		Name:  "Test Document for Get",
		Value: 42,
		Tags:  []string{"test", "document", "get"},
	}

	// 문서 저장
	createdDoc, err := testStorage.CreateAndGet(ctx, doc)
	assert.NoError(t, err)
	assert.NotNil(t, createdDoc)
	id := createdDoc.ID
	assert.NotEqual(t, primitive.NilObjectID, id)

	// 테스트 케이스
	tests := []struct {
		name        string
		id          primitive.ObjectID
		expectError bool
		setup       func()
	}{
		{
			name:        "존재하는 문서 가져오기",
			id:          id,
			expectError: false,
			setup:       func() {},
		},
		{
			name:        "존재하지 않는 문서 가져오기",
			id:          primitive.NewObjectID(),
			expectError: true,
			setup:       func() {},
		},
		{
			name:        "캐시에서 문서 가져오기",
			id:          id,
			expectError: false,
			setup: func() {
				// 캐시에 문서 저장
				err := testCache.Set(ctx, id, createdDoc, 0)
				assert.NoError(t, err)
			},
		},
		{
			name:        "캐시 미스 후 DB에서 문서 가져오기",
			id:          id,
			expectError: false,
			setup: func() {
				// 캐시 비우기
				err := testCache.Clear(ctx)
				assert.NoError(t, err)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// 테스트 설정
			tc.setup()

			// 문서 가져오기
			retrievedDoc, err := testStorage.Get(ctx, tc.id)

			// 결과 확인
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, createdDoc.Name, retrievedDoc.Name)
				assert.Equal(t, createdDoc.Value, retrievedDoc.Value)
				assert.Equal(t, createdDoc.Tags, retrievedDoc.Tags)
				assert.Equal(t, int64(1), retrievedDoc.Version())
			}
		})
	}

	// 닫힌 스토리지에서 문서 가져오기 테스트
	t.Run("닫힌 스토리지에서 문서 가져오기", func(t *testing.T) {
		// 임시 스토리지 생성
		tempCache := cache.NewMapCache[*TestDocument]()
		tempStorage, err := NewStorage[*TestDocument](ctx, testClient, testColl, tempCache, DefaultOptions())
		assert.NoError(t, err)

		// 스토리지 닫기
		err = tempStorage.Close()
		assert.NoError(t, err)

		// 닫힌 스토리지에서 문서 가져오기 시도
		_, err = tempStorage.Get(ctx, id)
		assert.Error(t, err)
		assert.Equal(t, ErrClosed, err)
	})
}

// TestStorageGetByQuery는 Storage.GetByQuery 메서드를 테스트합니다.
func TestStorageGetByQuery(t *testing.T) {
	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	// 테스트 전 컬렉션 비우기
	err := testColl.Drop(ctx)
	assert.NoError(t, err)

	// 테스트 전 캐시 비우기
	err = testCache.Clear(ctx)
	assert.NoError(t, err)

	// 테스트 문서 생성
	doc1 := &TestDocument{
		Name:  "Test Document 1 for Query",
		Value: 42,
		Tags:  []string{"test", "document", "query", "first"},
	}

	doc2 := &TestDocument{
		Name:  "Test Document 2 for Query",
		Value: 84,
		Tags:  []string{"test", "document", "query", "second"},
	}

	// 문서 저장
	createdDoc1, err := testStorage.CreateAndGet(ctx, doc1)
	assert.NoError(t, err)
	assert.NotNil(t, createdDoc1)
	id1 := createdDoc1.ID
	assert.NotEqual(t, primitive.NilObjectID, id1)

	createdDoc2, err := testStorage.CreateAndGet(ctx, doc2)
	assert.NoError(t, err)
	assert.NotNil(t, createdDoc2)
	id2 := createdDoc2.ID
	assert.NotEqual(t, primitive.NilObjectID, id2)

	// 테스트 케이스
	tests := []struct {
		name        string
		query       interface{}
		expectCount int
		expectError bool
	}{
		{
			name:        "모든 문서 가져오기",
			query:       bson.M{},
			expectCount: 2,
			expectError: false,
		},
		{
			name:        "필터로 문서 가져오기",
			query:       bson.M{"value": 42},
			expectCount: 1,
			expectError: false,
		},
		{
			name:        "결과가 없는 쿼리 실행",
			query:       bson.M{"value": 999},
			expectCount: 0,
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// 문서 가져오기
			docs, err := testStorage.GetByQuery(ctx, tc.query)

			// 결과 확인
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectCount, len(docs))

				// 결과 내용 확인
				if tc.expectCount > 0 && len(docs) > 0 {
					// 첫 번째 문서가 doc1 또는 doc2와 일치하는지 확인
					found := false
					for _, d := range docs {
						if d.Name == createdDoc1.Name || d.Name == createdDoc2.Name {
							found = true
							break
						}
					}
					assert.True(t, found, "결과에 예상된 문서가 없습니다")
				}
			}
		})
	}

	// 닫힌 스토리지에서 쿼리 실행 테스트
	t.Run("닫힌 스토리지에서 쿼리 실행", func(t *testing.T) {
		// 임시 스토리지 생성
		tempCache := cache.NewMapCache[*TestDocument]()
		tempStorage, err := NewStorage[*TestDocument](ctx, testClient, testColl, tempCache, DefaultOptions())
		assert.NoError(t, err)

		// 스토리지 닫기
		err = tempStorage.Close()
		assert.NoError(t, err)

		// 닫힌 스토리지에서 쿼리 실행 시도
		_, err = tempStorage.GetByQuery(ctx, bson.M{})
		assert.Error(t, err)
		assert.Equal(t, ErrClosed, err)
	})
}

// TestStorageEdit는 Storage.Edit 메서드를 테스트합니다.
func TestStorageEdit(t *testing.T) {
	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	// 테스트 전 캐시 비우기
	err := testCache.Clear(ctx)
	assert.NoError(t, err)

	// 테스트 문서 생성
	doc := &TestDocument{
		Name:  "Test Document for Edit",
		Value: 42,
		Tags:  []string{"test", "document", "edit"},
	}

	// 문서 저장
	createdDoc, err := testStorage.CreateAndGet(ctx, doc)
	assert.NoError(t, err)
	assert.NotNil(t, createdDoc)
	id := createdDoc.ID
	assert.NotEqual(t, primitive.NilObjectID, id)

	// 테스트 케이스
	tests := []struct {
		name        string
		id          primitive.ObjectID
		editFn      EditFunc[*TestDocument]
		expectError bool
		setup       func()
	}{
		{
			name: "문서 편집 (기본 케이스)",
			id:   id,
			editFn: func(d *TestDocument) (*TestDocument, error) {
				d.Name = "Updated Document"
				d.Value = 84
				d.Tags = append(d.Tags, "updated")
				return d, nil
			},
			expectError: false,
			setup:       func() {},
		},
		{
			name: "편집 함수에서 에러 반환",
			id:   id,
			editFn: func(d *TestDocument) (*TestDocument, error) {
				return nil, errors.New("edit function error")
			},
			expectError: true,
			setup:       func() {},
		},
		{
			name: "존재하지 않는 문서 편집",
			id:   primitive.NewObjectID(),
			editFn: func(d *TestDocument) (*TestDocument, error) {
				return d, nil
			},
			expectError: true,
			setup:       func() {},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// 테스트 설정
			tc.setup()

			// 문서 편집
			updatedDoc, diff, err := testStorage.Edit(ctx, tc.id, tc.editFn)

			// 결과 확인
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, updatedDoc)
				assert.NotNil(t, diff)

				// 버전 필드 확인
				assert.Equal(t, int64(2), updatedDoc.Version())

				// 문서가 DB에 저장되었는지 확인
				var savedDoc TestDocument
				err = testColl.FindOne(ctx, bson.M{"_id": tc.id}).Decode(&savedDoc)
				assert.NoError(t, err)
				assert.Equal(t, updatedDoc.Name, savedDoc.Name)
				assert.Equal(t, updatedDoc.Value, savedDoc.Value)
				assert.Equal(t, updatedDoc.Tags, savedDoc.Tags)
				assert.Equal(t, int64(2), savedDoc.Version())

				// 문서가 캐시에 저장되었는지 확인
				cachedDoc, err := testCache.Get(ctx, tc.id)
				assert.NoError(t, err)
				assert.Equal(t, updatedDoc.Name, cachedDoc.Name)
				assert.Equal(t, updatedDoc.Value, cachedDoc.Value)
				assert.Equal(t, updatedDoc.Tags, cachedDoc.Tags)
				assert.Equal(t, int64(2), cachedDoc.Version())
			}
		})
	}

	// 닫힌 스토리지에서 문서 편집 테스트
	t.Run("닫힌 스토리지에서 문서 편집", func(t *testing.T) {
		// 임시 스토리지 생성
		tempCache := cache.NewMapCache[*TestDocument]()
		tempStorage, err := NewStorage[*TestDocument](ctx, testClient, testColl, tempCache, DefaultOptions())
		assert.NoError(t, err)

		// 스토리지 닫기
		err = tempStorage.Close()
		assert.NoError(t, err)

		// 닫힌 스토리지에서 문서 편집 시도
		_, _, err = tempStorage.Edit(ctx, id, func(d *TestDocument) (*TestDocument, error) {
			return d, nil
		})
		assert.Error(t, err)
		assert.Equal(t, ErrClosed, err)
	})
}

// TestStorageCreate는 Storage.Create 메서드를 테스트합니다.
func TestStorageCreate(t *testing.T) {
	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	// 테스트 케이스
	tests := []struct {
		name        string
		doc         *TestDocument
		expectError bool
	}{
		{
			name: "새 문서 생성",
			doc: &TestDocument{
				Name:  "Test Document 1",
				Value: 42,
				Tags:  []string{"test", "document", "create"},
			},
			expectError: false,
		},
		{
			name: "ID가 이미 있는 문서 생성",
			doc: &TestDocument{
				ID:    primitive.NewObjectID(),
				Name:  "Test Document 2",
				Value: 84,
				Tags:  []string{"test", "document", "create", "with-id"},
			},
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// 테스트 전 캐시 비우기
			err := testCache.Clear(ctx)
			assert.NoError(t, err)

			// 문서 생성
			createdDoc, err := testStorage.CreateAndGet(ctx, tc.doc)

			// 결과 확인
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, createdDoc)
				assert.NotEqual(t, primitive.NilObjectID, createdDoc.ID)

				// 버전 필드 확인
				assert.Equal(t, int64(1), createdDoc.Version())

				// 문서가 DB에 저장되었는지 확인
				var savedDoc TestDocument
				err = testColl.FindOne(ctx, bson.M{"_id": createdDoc.ID}).Decode(&savedDoc)
				assert.NoError(t, err)
				assert.Equal(t, createdDoc.Name, savedDoc.Name)
				assert.Equal(t, createdDoc.Value, savedDoc.Value)
				assert.Equal(t, createdDoc.Tags, savedDoc.Tags)
				assert.Equal(t, int64(1), savedDoc.Version())

				// 문서가 캐시에 저장되었는지 확인
				cachedDoc, err := testCache.Get(ctx, createdDoc.ID)
				assert.NoError(t, err)
				assert.Equal(t, createdDoc.Name, cachedDoc.Name)
				assert.Equal(t, createdDoc.Value, cachedDoc.Value)
				assert.Equal(t, createdDoc.Tags, cachedDoc.Tags)
				assert.Equal(t, int64(1), cachedDoc.Version())
			}
		})
	}

	// 닫힌 스토리지에서 문서 생성 테스트
	t.Run("닫힌 스토리지에서 문서 생성", func(t *testing.T) {
		// 임시 스토리지 생성
		tempCache := cache.NewMapCache[*TestDocument]()
		tempStorage, err := NewStorage[*TestDocument](ctx, testClient, testColl, tempCache, DefaultOptions())
		assert.NoError(t, err)

		// 스토리지 닫기
		err = tempStorage.Close()
		assert.NoError(t, err)

		// 닫힌 스토리지에서 문서 생성 시도
		doc := &TestDocument{
			Name:  "Test Document Closed",
			Value: 100,
			Tags:  []string{"test", "closed"},
		}
		_, err = tempStorage.CreateAndGet(ctx, doc)
		assert.Error(t, err)
		assert.Equal(t, ErrClosed, err)
	})
}
