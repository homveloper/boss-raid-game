package nodestorage

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestCreateIfNotExists는 Create 함수의 CreateIfNotExists 동작을 테스트합니다.
func TestCreateIfNotExists(t *testing.T) {
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
	doc := &TestDocument{
		Name:  "CreateIfNotExists Test",
		Value: 42,
		Tags:  []string{"test", "create", "idempotent"},
	}

	// 첫 번째 생성 시도 (새로운 문서)
	createdDoc, err := testStorage.CreateAndGet(ctx, doc)
	require.NoError(t, err)
	require.NotNil(t, createdDoc)
	id1 := createdDoc.ID

	// 문서가 DB에 저장되었는지 확인
	var savedDoc TestDocument
	err = testColl.FindOne(ctx, bson.M{"_id": id1}).Decode(&savedDoc)
	assert.NoError(t, err)
	assert.Equal(t, createdDoc.Name, savedDoc.Name)
	assert.Equal(t, createdDoc.Value, savedDoc.Value)
	assert.Equal(t, createdDoc.Tags, savedDoc.Tags)
	assert.Equal(t, int64(1), savedDoc.Version())

	// 문서가 캐시에 저장되었는지 확인
	cachedDoc, err := testCache.Get(ctx, id1)
	assert.NoError(t, err)
	assert.Equal(t, createdDoc.Name, cachedDoc.Name)
	assert.Equal(t, createdDoc.Value, cachedDoc.Value)
	assert.Equal(t, createdDoc.Tags, cachedDoc.Tags)
	assert.Equal(t, int64(1), cachedDoc.Version())

	// 두 번째 생성 시도 (이미 존재하는 문서)
	doc2 := &TestDocument{
		ID:    id1, // 같은 ID 사용
		Name:  "This should not be saved",
		Value: 100,
		Tags:  []string{"should", "not", "be", "saved"},
	}

	// 두 번째 생성 시도
	existingDoc, err := testStorage.CreateAndGet(ctx, doc2)
	require.NoError(t, err)
	require.Equal(t, id1, existingDoc.ID) // 같은 ID가 반환되어야 함

	// 문서가 변경되지 않았는지 확인
	var savedDoc2 TestDocument
	err = testColl.FindOne(ctx, bson.M{"_id": id1}).Decode(&savedDoc2)
	assert.NoError(t, err)
	assert.Equal(t, doc.Name, savedDoc2.Name)      // 원래 이름이 유지되어야 함
	assert.Equal(t, doc.Value, savedDoc2.Value)    // 원래 값이 유지되어야 함
	assert.Equal(t, doc.Tags, savedDoc2.Tags)      // 원래 태그가 유지되어야 함
	assert.Equal(t, int64(1), savedDoc2.Version()) // 버전이 변경되지 않아야 함

	// 캐시에서도 원래 문서가 유지되는지 확인
	cachedDoc2, err := testCache.Get(ctx, id1)
	assert.NoError(t, err)
	assert.Equal(t, doc.Name, cachedDoc2.Name)
	assert.Equal(t, doc.Value, cachedDoc2.Value)
	assert.Equal(t, doc.Tags, cachedDoc2.Tags)
	assert.Equal(t, int64(1), cachedDoc2.Version())
}

// TestCreateWithCustomID는 사용자 지정 ID로 문서 생성을 테스트합니다.
func TestCreateWithCustomID(t *testing.T) {
	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	// 테스트 전 컬렉션 비우기
	err := testColl.Drop(ctx)
	assert.NoError(t, err)

	// 테스트 전 캐시 비우기
	err = testCache.Clear(ctx)
	assert.NoError(t, err)

	// 사용자 지정 ID 생성
	customID := primitive.NewObjectID()

	// 테스트 문서 생성 (사용자 지정 ID 사용)
	doc := &TestDocument{
		ID:    customID,
		Name:  "Custom ID Test",
		Value: 42,
		Tags:  []string{"test", "custom", "id"},
	}

	// 문서 생성
	createdDoc, err := testStorage.CreateAndGet(ctx, doc)
	require.NoError(t, err)
	require.Equal(t, customID, createdDoc.ID) // 사용자 지정 ID가 반환되어야 함

	// 문서가 DB에 저장되었는지 확인
	var savedDoc TestDocument
	err = testColl.FindOne(ctx, bson.M{"_id": customID}).Decode(&savedDoc)
	assert.NoError(t, err)
	assert.Equal(t, doc.Name, savedDoc.Name)
	assert.Equal(t, doc.Value, savedDoc.Value)
	assert.Equal(t, doc.Tags, savedDoc.Tags)
	assert.Equal(t, int64(1), savedDoc.Version())

	// 문서가 캐시에 저장되었는지 확인
	cachedDoc, err := testCache.Get(ctx, customID)
	assert.NoError(t, err)
	assert.Equal(t, doc.Name, cachedDoc.Name)
	assert.Equal(t, doc.Value, cachedDoc.Value)
	assert.Equal(t, doc.Tags, cachedDoc.Tags)
	assert.Equal(t, int64(1), cachedDoc.Version())
}

// TestCreateWithNilID는 nil ID로 문서 생성을 테스트합니다.
func TestCreateWithNilID(t *testing.T) {
	// 테스트 컨텍스트
	ctx, cancel := context.WithTimeout(testCtx, 10*time.Second)
	defer cancel()

	// 테스트 전 컬렉션 비우기
	err := testColl.Drop(ctx)
	assert.NoError(t, err)

	// 테스트 전 캐시 비우기
	err = testCache.Clear(ctx)
	assert.NoError(t, err)

	// 테스트 문서 생성 (nil ID 사용)
	doc := &TestDocument{
		ID:    primitive.NilObjectID, // nil ID
		Name:  "Nil ID Test",
		Value: 42,
		Tags:  []string{"test", "nil", "id"},
	}

	// 문서 생성
	createdDoc, err := testStorage.CreateAndGet(ctx, doc)
	require.NoError(t, err)
	require.NotEqual(t, primitive.NilObjectID, createdDoc.ID) // 새로운 ID가 생성되어야 함

	// 문서가 DB에 저장되었는지 확인
	var savedDoc TestDocument
	err = testColl.FindOne(ctx, bson.M{"_id": createdDoc.ID}).Decode(&savedDoc)
	assert.NoError(t, err)
	assert.Equal(t, doc.Name, savedDoc.Name)
	assert.Equal(t, doc.Value, savedDoc.Value)
	assert.Equal(t, doc.Tags, savedDoc.Tags)
	assert.Equal(t, int64(1), savedDoc.Version())

	// 문서가 캐시에 저장되었는지 확인
	cachedDoc, err := testCache.Get(ctx, createdDoc.ID)
	assert.NoError(t, err)
	assert.Equal(t, doc.Name, cachedDoc.Name)
	assert.Equal(t, doc.Value, cachedDoc.Value)
	assert.Equal(t, doc.Tags, cachedDoc.Tags)
	assert.Equal(t, int64(1), cachedDoc.Version())
}
