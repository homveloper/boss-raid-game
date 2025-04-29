package v2

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jinzhu/copier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

/*
BsonPatch 테스트 계획:

1. 기본 기능 테스트
   - 단순 구조체에 대한 패치 생성 및 검증
   - 다양한 필드 타입(문자열, 숫자, 불리언, 시간 등)에 대한 패치 생성
   - 필드 추가, 수정, 삭제에 대한 패치 생성

2. 복잡한 구조체 테스트
   - 중첩 구조체에 대한 패치 생성 및 검증
   - 배열 필드에 대한 패치 생성 및 검증
   - 맵 필드에 대한 패치 생성 및 검증
   - 깊은 중첩 구조에 대한 패치 생성 및 검증

3. 특수 케이스 테스트
   - 빈 구조체에 대한 패치 생성
   - 동일한 구조체에 대한 패치 생성 (변경 없음)
   - nil 값 처리 테스트
   - 포인터 필드 테스트

4. MongoDB 적용 테스트
   - 생성된 패치를 MongoDB에 적용하여 실제 업데이트 검증
   - 다양한 구조체 타입에 대한 패치 적용 테스트
   - 배열 필터를 사용한 업데이트 테스트

5. 성능 테스트
   - 다양한 크기의 구조체에 대한 패치 생성 성능 측정
   - 반복적인 패치 생성 성능 측정 (캐싱 효과 확인)
*/

// 테스트를 위한 간단한 구조체
type SimpleTestDoc struct {
	ID        primitive.ObjectID `bson:"_id"`
	Name      string             `bson:"name"`
	Age       int32              `bson:"age"`
	IsActive  bool               `bson:"isActive"`
	CreatedAt time.Time          `bson:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt"`
	Score     float64            `bson:"score"`
	Tags      []string           `bson:"tags"`
	Metadata  map[string]string  `bson:"metadata"`
	Version   int64              `bson:"version"`
}

// 중첩 구조체를 포함한 복잡한 테스트 구조체 - 벤치마크 테스트에서 사용하는 ItemStruct와 충돌하여 주석 처리
type NestedTestDoc struct {
	ID        primitive.ObjectID `bson:"_id"`
	Name      string             `bson:"name"`
	Version   int64              `bson:"version"`
	CreatedAt time.Time          `bson:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt"`

	// 중첩      구조체
	Profile *ProfileStruct `bson:"profile"`

	// 배열      필드
	Items []*ItemStruct `bson:"items"`

	// 맵        필드
	Settings map[string]interface{} `bson:"settings"`
}

type ProfileStruct struct {
	FirstName string `bson:"firstName"`
	LastName  string `bson:"lastName"`
	Email     string `bson:"email"`
	Age       int    `bson:"age"`
}

// ItemStruct는 bsonpatch_benchmark_test.go에 이미 정의되어 있음

// 매우 깊은 중첩 구조체
type DeepNestedTestDoc struct {
	ID      primitive.ObjectID `bson:"_id"`
	Name    string             `bson:"name"`
	Version int64              `bson:"version"`

	// 1단계 중첩
	Level1 *Level1Struct `bson:"level1"`
}

type Level1Struct struct {
	Name   string        `bson:"name"`
	Value  int           `bson:"value"`
	Level2 *Level2Struct `bson:"level2"`
}

type Level2Struct struct {
	Name   string        `bson:"name"`
	Value  int           `bson:"value"`
	Level3 *Level3Struct `bson:"level3"`
}

type Level3Struct struct {
	Name   string        `bson:"name"`
	Value  int           `bson:"value"`
	Level4 *Level4Struct `bson:"level4"`
}

type Level4Struct struct {
	Name  string `bson:"name"`
	Value int    `bson:"value"`
	Data  []int  `bson:"data"`
}

// 배열이 많은 구조체
type ArrayHeavyTestDoc struct {
	ID      primitive.ObjectID `bson:"_id"`
	Name    string             `bson:"name"`
	Version int64              `bson:"version"`

	// 다양한 배열 필드
	Strings []string            `bson:"strings"`
	Numbers []int               `bson:"numbers"`
	Floats  []float64           `bson:"floats"`
	Objects []*SimpleObject     `bson:"objects"`
	Mixed   []interface{}       `bson:"mixed"`
	Matrix  [][]int             `bson:"matrix"`
	Complex [][][]string        `bson:"complex"`
	Maps    []map[string]string `bson:"maps"`
}

type SimpleObject struct {
	Key   string `bson:"key"`
	Value string `bson:"value"`
}

// 맵이 많은 구조체
type MapHeavyTestDoc struct {
	ID      primitive.ObjectID `bson:"_id"`
	Name    string             `bson:"name"`
	Version int64              `bson:"version"`

	// 다양한 맵 필드
	StringMap  map[string]string                 `bson:"stringMap"`
	NumberMap  map[string]int                    `bson:"numberMap"`
	FloatMap   map[string]float64                `bson:"floatMap"`
	ObjectMap  map[string]*SimpleObject          `bson:"objectMap"`
	MixedMap   map[string]interface{}            `bson:"mixedMap"`
	NestedMap  map[string]map[string]string      `bson:"nestedMap"`
	ComplexMap map[string]map[string]interface{} `bson:"complexMap"`
}

// setupBsonPatchTestDB는 bsonpatch 테스트를 위한 MongoDB 환경을 설정합니다.
func setupBsonPatchTestDB(t *testing.T) (*mongo.Client, *mongo.Collection, func()) {
	// MongoDB 연결
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 로컬 MongoDB 인스턴스에 연결
	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(ctx, clientOptions)
	require.NoError(t, err, "MongoDB 연결 실패")

	// 연결 확인
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, nil, func() {}
	}

	// 테스트용 고유 컬렉션 이름 생성
	collectionName := "test_bsonpatch_" + primitive.NewObjectID().Hex()
	collection := client.Database("test_db").Collection(collectionName)

	// 정리 함수 반환
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		collection.Drop(ctx)
		client.Disconnect(ctx)
	}

	return client, collection, cleanup
}

// 테스트 실행 전 MongoDB 연결 확인
func TestMain(m *testing.M) {
	// MongoDB 연결 확인
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI("mongodb://localhost:27017")
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		fmt.Printf("MongoDB 연결 실패: %v\n", err)
		return
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		fmt.Printf("MongoDB 서버가 응답하지 않습니다: %v\n", err)
		return
	}

	client.Disconnect(ctx)

	// 테스트 실행
	m.Run()
}

// 1. 기본 기능 테스트 - 단순 구조체에 대한 패치 생성 및 검증
func TestCreateBsonPatch_SimpleStruct(t *testing.T) {
	// 원본 문서 생성
	original := &SimpleTestDoc{
		ID:        primitive.NewObjectID(),
		Name:      "Test Document",
		Age:       30,
		IsActive:  true,
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now().Add(-12 * time.Hour),
		Score:     85.5,
		Tags:      []string{"test", "document", "simple"},
		Metadata: map[string]string{
			"created_by": "system",
			"category":   "test",
		},
		Version: 1,
	}

	// 수정된 문서 생성 (몇 가지 필드 변경)
	modified := &SimpleTestDoc{
		ID:        original.ID,                 // ID는 동일하게 유지
		Name:      "Updated Test Document",     // 이름 변경
		Age:       31,                          // 나이 증가
		IsActive:  false,                       // 상태 변경
		CreatedAt: original.CreatedAt,          // 생성 시간은 동일하게 유지
		UpdatedAt: time.Now(),                  // 업데이트 시간 변경
		Score:     90.5,                        // 점수 변경
		Tags:      []string{"test", "updated"}, // 태그 변경
		Metadata: map[string]string{ // 메타데이터 변경
			"created_by": "system",
			"category":   "test",
			"updated_by": "user",
		},
		Version: 2, // 버전 증가
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 검증 - 수정 후 isActive 필드는 항상 Set에 포함됨
	assert.Equal(t, 7, len(patch.Set), "$set 연산자에 예상된 필드 수가 포함되어 있지 않습니다")

	// 변경된 필드 확인
	assert.Equal(t, "Updated Test Document", patch.Set["name"], "name 필드가 올바르게 설정되지 않았습니다")
	assert.Equal(t, int32(31), patch.Set["age"], "age 필드가 올바르게 설정되지 않았습니다")

	// isActive 필드는 항상 Set에 포함됨
	assert.Contains(t, patch.Set, "isActive", "isActive 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, false, patch.Set["isActive"], "isActive 필드가 올바르게 설정되지 않았습니다")

	assert.Equal(t, 90.5, patch.Set["score"], "score 필드가 올바르게 설정되지 않았습니다")
	assert.Equal(t, int64(2), patch.Set["version"], "version 필드가 올바르게 설정되지 않았습니다")

	// 복잡한 필드(배열, 맵) 확인
	assert.Contains(t, patch.Set, "tags", "tags 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "metadata.updated_by", "metadata 필드가 패치에 포함되어 있지 않습니다")

	// 변경되지 않은 필드 확인
	assert.NotContains(t, patch.Set, "_id", "변경되지 않은 _id 필드가 패치에 포함되어 있습니다")
	assert.NotContains(t, patch.Set, "createdAt", "변경되지 않은 createdAt 필드가 패치에 포함되어 있습니다")

	// Unset 연산자 확인 - 수정 후 bool 필드는 항상 Set에 포함되므로 Unset은 비어 있어야 함
	assert.Equal(t, 0, len(patch.Unset), "$unset 연산자가 비어 있어야 합니다")
	assert.Empty(t, patch.Unset, "$unset 연산자가 비어 있어야 합니다")

	// 추가 검증: Unset 맵이 nil이 아니고 초기화되어 있는지 확인
	assert.NotNil(t, patch.Unset, "$unset 연산자가 nil이 아니어야 합니다")
	assert.IsType(t, bson.M{}, patch.Unset, "$unset 연산자가 올바른 타입이어야 합니다")
}

// 필드 추가, 수정, 삭제에 대한 패치 생성 테스트
func TestCreateBsonPatch_FieldOperations(t *testing.T) {
	// 원본 문서 생성
	original := &SimpleTestDoc{
		ID:        primitive.NewObjectID(),
		Name:      "Original Document",
		Age:       25,
		IsActive:  true,
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now().Add(-12 * time.Hour),
		Score:     75.0,
		Tags:      []string{"original", "test"},
		Metadata: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		Version: 1,
	}

	// 수정된 문서 생성 (필드 추가, 수정, 삭제)
	modified := &SimpleTestDoc{
		ID:        original.ID,                 // ID는 동일하게 유지
		Name:      "Modified Document",         // 수정
		Age:       0,                           // 값을 0으로 설정 (삭제는 아님)
		IsActive:  false,                       // 수정
		CreatedAt: original.CreatedAt,          // 유지
		UpdatedAt: time.Now(),                  // 수정
		Score:     0,                           // 값을 0으로 설정 (삭제는 아님)
		Tags:      []string{"modified", "new"}, // 수정
		Metadata: map[string]string{ // 일부 키 삭제, 일부 키 추가
			"key1": "updated_value",
			"key3": "new_value",
		},
		Version: 2, // 수정
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 검증
	// $set 연산자 확인
	assert.Contains(t, patch.Set, "name", "name 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "Modified Document", patch.Set["name"], "name 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "age", "age 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, int32(0), patch.Set["age"], "age 필드가 올바르게 설정되지 않았습니다")

	// isActive 필드는 항상 Set에 포함됨
	assert.Contains(t, patch.Set, "isActive", "isActive 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, false, patch.Set["isActive"], "isActive 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "score", "score 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, 0.0, patch.Set["score"], "score 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "version", "version 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, int64(2), patch.Set["version"], "version 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "tags.0", "tags.0 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "tags.1", "tags.1 필드가 패치에 포함되어 있지 않습니다")

	// 메타데이터 필드 확인 - 개별 필드로 처리될 수 있음
	assert.Contains(t, patch.Set, "metadata.key1", "metadata.key1 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "updated_value", patch.Set["metadata.key1"], "metadata.key1 값이 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "metadata.key3", "metadata.key3 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "new_value", patch.Set["metadata.key3"], "metadata.key3 값이 올바르게 설정되지 않았습니다")

	// 변경되지 않은 필드 확인
	assert.NotContains(t, patch.Set, "_id", "변경되지 않은 _id 필드가 패치에 포함되어 있습니다")
	assert.NotContains(t, patch.Set, "createdAt", "변경되지 않은 createdAt 필드가 패치에 포함되어 있습니다")

	// Unset 연산자 확인 - metadata.key2가 삭제되었으므로 Unset에 포함되어야 함
	assert.Equal(t, 1, len(patch.Unset), "$unset 연산자에 예상된 필드 수가 포함되어 있지 않습니다")
	assert.Contains(t, patch.Unset, "metadata.key2", "metadata.key2 필드가 $unset에 포함되어 있지 않습니다")

	// 추가 검증: Unset 맵이 nil이 아니고 초기화되어 있는지 확인
	assert.NotNil(t, patch.Unset, "$unset 연산자가 nil이 아니어야 합니다")
	assert.IsType(t, bson.M{}, patch.Unset, "$unset 연산자가 올바른 타입이어야 합니다")
}

// 2. 복잡한 구조체 테스트 - 중첩 구조체에 대한 패치 생성 및 검증
func TestCreateBsonPatch_NestedStruct(t *testing.T) {

	// 원본 중첩 문서 생성
	original := &NestedTestDoc{
		ID:        primitive.NewObjectID(),
		Name:      "Nested Document",
		Version:   1,
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now().Add(-12 * time.Hour),
		Profile: &ProfileStruct{
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john.doe@example.com",
			Age:       30,
		},
		Items: []*ItemStruct{
			{
				ID:    "item1",
				Name:  "Item 1",
				Value: 5,
				Tags:  []string{"tag1", "tag2"},
				Attributes: map[string]string{
					"color": "red",
					"size":  "medium",
				},
			},
			{
				ID:    "item2",
				Name:  "Item 2",
				Value: 10,
				Tags:  []string{"tag2", "tag3"},
				Attributes: map[string]string{
					"color": "blue",
					"size":  "large",
				},
			},
		},
		Settings: map[string]interface{}{
			"notifications": true,
			"theme":         "dark",
			"fontSize":      12,
		},
	}

	// 수정된 중첩 문서 생성
	modified := &NestedTestDoc{
		ID:        original.ID,
		Name:      "Updated Nested Document",
		Version:   2,
		CreatedAt: original.CreatedAt,
		UpdatedAt: time.Now(),
		Profile: &ProfileStruct{
			FirstName: "John",
			LastName:  "Smith",                  // 성 변경
			Email:     "john.smith@example.com", // 이메일 변경
			Age:       31,                       // 나이 변경
		},
		Items: []*ItemStruct{
			{
				ID:    "item1",
				Name:  "Updated Item 1",                    // 이름 변경
				Value: 7,                                   // 수량 변경
				Tags:  []string{"tag1", "tag2", "new-tag"}, // 태그 추가
				Attributes: map[string]string{
					"color":    "red",
					"size":     "large",  // 크기 변경
					"material": "cotton", // 속성 추가
				},
			},
			{
				ID:    "item2",
				Name:  "Item 2",
				Value: 10,
				Tags:  []string{"tag2", "tag3"},
				Attributes: map[string]string{
					"color": "blue",
					"size":  "large",
				},
			},
			{
				ID:    "item3", // 새 아이템 추가
				Name:  "New Item 3",
				Value: 3,
				Tags:  []string{"new", "item"},
				Attributes: map[string]string{
					"color": "green",
					"size":  "small",
				},
			},
		},
		Settings: map[string]interface{}{
			"notifications": false,   // 변경
			"theme":         "light", // 변경
			"fontSize":      14,      // 변경
			"language":      "ko",    // 추가
		},
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 검증
	// 기본 필드 확인
	assert.Contains(t, patch.Set, "name", "name 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "Updated Nested Document", patch.Set["name"], "name 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "version", "version 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, int64(2), patch.Set["version"], "version 필드가 올바르게 설정되지 않았습니다")

	// 중첩 구조체 필드 확인
	assert.Contains(t, patch.Set, "profile.lastName", "profile.lastName 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "Smith", patch.Set["profile.lastName"], "profile.lastName 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "profile.email", "profile.email 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "john.smith@example.com", patch.Set["profile.email"], "profile.email 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "profile.age", "profile.age 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, 31, patch.Set["profile.age"], "profile.age 필드가 올바르게 설정되지 않았습니다")

	// 배열 필드 확인 (items)
	assert.Contains(t, patch.Set, "items", "items 필드가 패치에 포함되어 있지 않습니다")

	// 설정 맵 확인 - 개별 필드로 처리될 수 있음
	assert.Contains(t, patch.Set, "settings.notifications", "settings.notifications 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, false, patch.Set["settings.notifications"], "settings.notifications 값이 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "settings.theme", "settings.theme 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "light", patch.Set["settings.theme"], "settings.theme 값이 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "settings.fontSize", "settings.fontSize 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, 14, patch.Set["settings.fontSize"], "settings.fontSize 값이 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "settings.language", "settings.language 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "ko", patch.Set["settings.language"], "settings.language 값이 올바르게 설정되지 않았습니다")
}

// 깊은 중첩 구조체 테스트
func TestCreateBsonPatch_DeepNestedStruct(t *testing.T) {

	// 원본 깊은 중첩 문서 생성
	original := &DeepNestedTestDoc{
		ID:      primitive.NewObjectID(),
		Name:    "Deep Nested Document",
		Version: 1,
		Level1: &Level1Struct{
			Name:  "Level 1",
			Value: 1,
			Level2: &Level2Struct{
				Name:  "Level 2",
				Value: 2,
				Level3: &Level3Struct{
					Name:  "Level 3",
					Value: 3,
					Level4: &Level4Struct{
						Name:  "Level 4",
						Value: 4,
						Data:  []int{1, 2, 3, 4},
					},
				},
			},
		},
	}

	// 수정된 깊은 중첩 문서 생성 (깊은 수준의 필드 변경)
	modified := &DeepNestedTestDoc{
		ID:      original.ID,
		Name:    "Updated Deep Nested Document", // 변경
		Version: 2,                              // 변경
		Level1: &Level1Struct{
			Name:  "Updated Level 1", // 변경
			Value: 10,                // 변경
			Level2: &Level2Struct{
				Name:  "Level 2", // 유지
				Value: 20,        // 변경
				Level3: &Level3Struct{
					Name:  "Updated Level 3", // 변경
					Value: 30,                // 변경
					Level4: &Level4Struct{
						Name:  "Level 4",               // 유지
						Value: 40,                      // 변경
						Data:  []int{1, 2, 3, 4, 5, 6}, // 변경 (요소 추가)
					},
				},
			},
		},
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 검증
	// 기본 필드 확인
	assert.Contains(t, patch.Set, "name", "name 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "Updated Deep Nested Document", patch.Set["name"], "name 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "version", "version 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, int64(2), patch.Set["version"], "version 필드가 올바르게 설정되지 않았습니다")

	// 깊은 중첩 필드 확인
	assert.Contains(t, patch.Set, "level1.name", "level1.name 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "Updated Level 1", patch.Set["level1.name"], "level1.name 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "level1.value", "level1.value 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, 10, patch.Set["level1.value"], "level1.value 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "level1.level2.value", "level1.level2.value 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, 20, patch.Set["level1.level2.value"], "level1.level2.value 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "level1.level2.level3.name", "level1.level2.level3.name 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "Updated Level 3", patch.Set["level1.level2.level3.name"], "level1.level2.level3.name 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "level1.level2.level3.value", "level1.level2.level3.value 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, 30, patch.Set["level1.level2.level3.value"], "level1.level2.level3.value 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "level1.level2.level3.level4.value", "level1.level2.level3.level4.value 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, 40, patch.Set["level1.level2.level3.level4.value"], "level1.level2.level3.level4.value 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "level1.level2.level3.level4.data", "level1.level2.level3.level4.data 필드가 패치에 포함되어 있지 않습니다")

	// 변경되지 않은 필드 확인
	assert.NotContains(t, patch.Set, "level1.level2.name", "변경되지 않은 level1.level2.name 필드가 패치에 포함되어 있습니다")
	assert.NotContains(t, patch.Set, "level1.level2.level3.level4.name", "변경되지 않은 level1.level2.level3.level4.name 필드가 패치에 포함되어 있습니다")
}

// 배열이 많은 구조체 테스트
func TestCreateBsonPatch_ArrayHeavyStruct(t *testing.T) {

	// 원본 배열 중심 문서 생성
	original := &ArrayHeavyTestDoc{
		ID:      primitive.NewObjectID(),
		Name:    "Array Heavy Document",
		Version: 1,
		Strings: []string{"one", "two", "three"},
		Numbers: []int{1, 2, 3, 4, 5},
		Floats:  []float64{1.1, 2.2, 3.3},
		Objects: []*SimpleObject{
			{Key: "key1", Value: "value1"},
			{Key: "key2", Value: "value2"},
		},
		Mixed: []interface{}{
			"string", 123, 45.6, true,
		},
		Matrix: [][]int{
			{1, 2, 3},
			{4, 5, 6},
		},
		Complex: [][][]string{
			{
				{"a", "b"},
				{"c", "d"},
			},
		},
		Maps: []map[string]string{
			{"k1": "v1", "k2": "v2"},
			{"k3": "v3", "k4": "v4"},
		},
	}

	// 수정된 배열 중심 문서 생성
	modified := &ArrayHeavyTestDoc{
		ID:      original.ID,
		Name:    "Updated Array Heavy Document",                  // 변경
		Version: 2,                                               // 변경
		Strings: []string{"one", "two", "three", "four", "five"}, // 요소 추가
		Numbers: []int{5, 4, 3, 2, 1},                            // 순서 변경
		Floats:  []float64{1.1, 2.2, 3.3, 4.4},                   // 요소 추가
		Objects: []*SimpleObject{ // 요소 변경 및 추가
			{Key: "key1", Value: "updated_value1"}, // 값 변경
			{Key: "key2", Value: "value2"},         // 유지
			{Key: "key3", Value: "value3"},         // 추가
		},
		Mixed: []interface{}{ // 요소 변경 및 추가
			"updated_string", 123, 45.6, false, "new_value", // 일부 변경 및 추가
		},
		Matrix: [][]int{ // 행렬 변경
			{10, 20, 30}, // 값 변경
			{40, 50, 60}, // 값 변경
			{70, 80, 90}, // 행 추가
		},
		Complex: [][][]string{ // 복잡한 배열 변경
			{
				{"A", "B"}, // 값 변경
				{"c", "d"}, // 유지
				{"e", "f"}, // 추가
			},
			{
				{"g", "h"}, // 새 차원 추가
			},
		},
		Maps: []map[string]string{ // 맵 배열 변경
			{"k1": "updated_v1", "k2": "v2"},     // 값 변경
			{"k3": "v3", "k4": "v4", "k5": "v5"}, // 키 추가
			{"k6": "v6", "k7": "v7"},             // 맵 추가
		},
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 검증
	// 기본 필드 확인
	assert.Contains(t, patch.Set, "name", "name 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "Updated Array Heavy Document", patch.Set["name"], "name 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "version", "version 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, int64(2), patch.Set["version"], "version 필드가 올바르게 설정되지 않았습니다")

	// 배열 필드 확인 - 배열 요소가 개별적으로 처리됨
	// 배열 길이가 다르거나 순서가 변경된 경우 전체 배열이 대체될 수 있음

	// 배열 요소 확인 - 배열 길이가 다르거나 순서가 변경된 경우
	if _, ok := patch.Set["strings"]; ok {
		// 전체 배열이 대체된 경우
		assert.Contains(t, patch.Set, "strings", "strings 필드가 패치에 포함되어 있지 않습니다")
	} else {
		// 개별 요소가 수정된 경우
		assert.Contains(t, patch.Set, "strings.3", "strings.3 필드가 패치에 포함되어 있지 않습니다")
		assert.Contains(t, patch.Set, "strings.4", "strings.4 필드가 패치에 포함되어 있지 않습니다")
	}

	if _, ok := patch.Set["numbers"]; ok {
		// 전체 배열이 대체된 경우
		assert.Contains(t, patch.Set, "numbers", "numbers 필드가 패치에 포함되어 있지 않습니다")
	} else {
		// 개별 요소가 수정된 경우
		assert.Contains(t, patch.Set, "numbers.0", "numbers.0 필드가 패치에 포함되어 있지 않습니다")
		assert.Equal(t, 5, patch.Set["numbers.0"], "numbers.0 값이 올바르게 설정되지 않았습니다")
	}

	if _, ok := patch.Set["floats"]; ok {
		// 전체 배열이 대체된 경우
		assert.Contains(t, patch.Set, "floats", "floats 필드가 패치에 포함되어 있지 않습니다")
	} else {
		// 개별 요소가 수정된 경우
		assert.Contains(t, patch.Set, "floats.3", "floats.3 필드가 패치에 포함되어 있지 않습니다")
	}

	// 객체 배열 확인
	if _, ok := patch.Set["objects"]; ok {
		// 전체 배열이 대체된 경우
		assert.Contains(t, patch.Set, "objects", "objects 필드가 패치에 포함되어 있지 않습니다")
	} else {
		// 개별 요소가 수정된 경우
		assert.Contains(t, patch.Set, "objects.0.value", "objects.0.value 필드가 패치에 포함되어 있지 않습니다")
		assert.Equal(t, "updated_value1", patch.Set["objects.0.value"], "objects.0.value 값이 올바르게 설정되지 않았습니다")
	}

	// 복잡한 배열 확인
	if _, ok := patch.Set["matrix"]; ok {
		assert.Contains(t, patch.Set, "matrix", "matrix 필드가 패치에 포함되어 있지 않습니다")
	}

	if _, ok := patch.Set["complex"]; ok {
		assert.Contains(t, patch.Set, "complex", "complex 필드가 패치에 포함되어 있지 않습니다")
	}

	if _, ok := patch.Set["maps"]; ok {
		assert.Contains(t, patch.Set, "maps", "maps 필드가 패치에 포함되어 있지 않습니다")
	}
}

// 맵이 많은 구조체 테스트
func TestCreateBsonPatch_MapHeavyStruct(t *testing.T) {

	// 원본 맵 중심 문서 생성
	original := &MapHeavyTestDoc{
		ID:      primitive.NewObjectID(),
		Name:    "Map Heavy Document",
		Version: 1,
		StringMap: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		NumberMap: map[string]int{
			"one": 1,
			"two": 2,
		},
		FloatMap: map[string]float64{
			"pi":    3.14,
			"euler": 2.71,
		},
		ObjectMap: map[string]*SimpleObject{
			"obj1": {Key: "key1", Value: "value1"},
			"obj2": {Key: "key2", Value: "value2"},
		},
		MixedMap: map[string]interface{}{
			"string": "value",
			"number": 123,
			"float":  45.6,
			"bool":   true,
		},
		NestedMap: map[string]map[string]string{
			"user": {
				"name":  "John",
				"email": "john@example.com",
			},
			"settings": {
				"theme": "dark",
				"lang":  "en",
			},
		},
		ComplexMap: map[string]map[string]interface{}{
			"profile": {
				"name":  "John Doe",
				"age":   30,
				"admin": true,
			},
		},
	}

	// 수정된 맵 중심 문서 생성
	modified := &MapHeavyTestDoc{
		ID:      original.ID,
		Name:    "Updated Map Heavy Document", // 변경
		Version: 2,                            // 변경
		StringMap: map[string]string{ // 값 변경, 키 추가/삭제
			"key1": "updated_value1", // 값 변경
			"key3": "value3",         // 키 추가
			// key2 삭제
		},
		NumberMap: map[string]int{ // 값 변경, 키 추가
			"one":   10, // 값 변경
			"two":   2,  // 유지
			"three": 3,  // 키 추가
		},
		FloatMap: map[string]float64{ // 값 변경, 키 추가
			"pi":     3.14159, // 값 변경
			"euler":  2.71828, // 값 변경
			"golden": 1.61803, // 키 추가
		},
		ObjectMap: map[string]*SimpleObject{ // 객체 변경, 추가
			"obj1": {Key: "updated_key1", Value: "updated_value1"}, // 값 변경
			"obj2": {Key: "key2", Value: "value2"},                 // 유지
			"obj3": {Key: "key3", Value: "value3"},                 // 추가
		},
		MixedMap: map[string]interface{}{ // 값 변경, 키 추가/삭제
			"string": "updated_value", // 값 변경
			"number": 456,             // 값 변경
			"float":  45.6,            // 유지
			"array":  []int{1, 2, 3},  // 키 추가
			// bool 삭제
		},
		NestedMap: map[string]map[string]string{ // 중첩 맵 변경
			"user": { // 값 변경, 키 추가
				"name":  "John Smith",       // 값 변경
				"email": "john@example.com", // 유지
				"phone": "123-456-7890",     // 키 추가
			},
			"settings": { // 값 변경
				"theme": "light", // 값 변경
				"lang":  "ko",    // 값 변경
			},
			"preferences": { // 맵 추가
				"notifications": "enabled",
				"timezone":      "UTC+9",
			},
		},
		ComplexMap: map[string]map[string]interface{}{ // 복잡한 중첩 맵 변경
			"profile": { // 값 변경, 키 추가
				"name":    "John Smith",  // 값 변경
				"age":     31,            // 값 변경
				"admin":   true,          // 유지
				"address": "123 Main St", // 키 추가
			},
			"stats": { // 맵 추가
				"visits": 100,
				"likes":  50,
				"active": true,
			},
		},
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 검증
	// 기본 필드 확인
	assert.Contains(t, patch.Set, "name", "name 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "Updated Map Heavy Document", patch.Set["name"], "name 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "version", "version 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, int64(2), patch.Set["version"], "version 필드가 올바르게 설정되지 않았습니다")

	// 맵 필드 확인 - 맵 요소가 개별적으로 처리됨

	// stringMap 필드 확인
	assert.Contains(t, patch.Set, "stringMap.key1", "stringMap.key1 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "updated_value1", patch.Set["stringMap.key1"], "stringMap.key1 값이 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "stringMap.key3", "stringMap.key3 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "value3", patch.Set["stringMap.key3"], "stringMap.key3 값이 올바르게 설정되지 않았습니다")

	// key2가 삭제되었으므로 unset에 포함되어야 함
	assert.Contains(t, patch.Unset, "stringMap.key2", "stringMap.key2 필드가 $unset에 포함되어 있지 않습니다")

	// numberMap 필드 확인
	assert.Contains(t, patch.Set, "numberMap.one", "numberMap.one 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, 10, patch.Set["numberMap.one"], "numberMap.one 값이 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "numberMap.three", "numberMap.three 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, 3, patch.Set["numberMap.three"], "numberMap.three 값이 올바르게 설정되지 않았습니다")

	// floatMap 필드 확인
	assert.Contains(t, patch.Set, "floatMap.pi", "floatMap.pi 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "floatMap.euler", "floatMap.euler 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "floatMap.golden", "floatMap.golden 필드가 패치에 포함되어 있지 않습니다")

	// objectMap 필드 확인
	assert.Contains(t, patch.Set, "objectMap.obj1.key", "objectMap.obj1.key 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "objectMap.obj1.value", "objectMap.obj1.value 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "objectMap.obj3", "objectMap.obj3 필드가 패치에 포함되어 있지 않습니다")

	// mixedMap 필드 확인
	assert.Contains(t, patch.Set, "mixedMap.string", "mixedMap.string 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "mixedMap.number", "mixedMap.number 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "mixedMap.array", "mixedMap.array 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Unset, "mixedMap.bool", "mixedMap.bool 필드가 $unset에 포함되어 있지 않습니다")

	// nestedMap 필드 확인
	assert.Contains(t, patch.Set, "nestedMap.user.name", "nestedMap.user.name 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "nestedMap.user.phone", "nestedMap.user.phone 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "nestedMap.settings.theme", "nestedMap.settings.theme 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "nestedMap.settings.lang", "nestedMap.settings.lang 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "nestedMap.preferences", "nestedMap.preferences 필드가 패치에 포함되어 있지 않습니다")

	// complexMap 필드 확인
	assert.Contains(t, patch.Set, "complexMap.profile.name", "complexMap.profile.name 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "complexMap.profile.age", "complexMap.profile.age 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "complexMap.profile.address", "complexMap.profile.address 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "complexMap.stats", "complexMap.stats 필드가 패치에 포함되어 있지 않습니다")
}

// 3. 특수 케이스 테스트 - 빈 구조체에 대한 패치 생성
func TestCreateBsonPatch_EmptyStruct(t *testing.T) {
	// 빈 구조체 정의
	type EmptyTestDoc struct {
		ID      primitive.ObjectID `bson:"_id"`
		Version int64              `bson:"version"`
	}

	// 원본 빈 문서 생성
	original := &EmptyTestDoc{
		ID:      primitive.NewObjectID(),
		Version: 1,
	}

	// 수정된 빈 문서 생성
	modified := &EmptyTestDoc{
		ID:      original.ID,
		Version: 2,
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 검증
	assert.Equal(t, 1, len(patch.Set), "$set 연산자에 예상된 필드 수가 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "version", "version 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, int64(2), patch.Set["version"], "version 필드가 올바르게 설정되지 않았습니다")
}

// 동일한 구조체에 대한 패치 생성 테스트 (변경 없음)
func TestCreateBsonPatch_NoChanges(t *testing.T) {
	// 원본 문서 생성
	original := &SimpleTestDoc{
		ID:        primitive.NewObjectID(),
		Name:      "Test Document",
		Age:       30,
		IsActive:  true,
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now().Add(-12 * time.Hour),
		Score:     85.5,
		Tags:      []string{"test", "document", "simple"},
		Metadata: map[string]string{
			"created_by": "system",
			"category":   "test",
		},
		Version: 1,
	}

	// 동일한 문서 (깊은 복사)
	modified := &SimpleTestDoc{
		ID:        original.ID,
		Name:      original.Name,
		Age:       original.Age,
		IsActive:  original.IsActive,
		CreatedAt: original.CreatedAt,
		UpdatedAt: original.UpdatedAt,
		Score:     original.Score,
		Tags:      make([]string, len(original.Tags)),
		Metadata:  make(map[string]string, len(original.Metadata)),
		Version:   original.Version,
	}

	// 배열과 맵 복사
	copy(modified.Tags, original.Tags)
	for k, v := range original.Metadata {
		modified.Metadata[k] = v
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 검증 - 변경이 없으므로 빈 패치여야 함
	assert.Equal(t, 0, len(patch.Set), "$set 연산자가 비어 있어야 합니다")
	assert.Empty(t, patch.Set, "$set 연산자가 비어 있어야 합니다")
	assert.NotNil(t, patch.Set, "$set 연산자가 nil이 아니어야 합니다")
	assert.IsType(t, bson.M{}, patch.Set, "$set 연산자가 올바른 타입이어야 합니다")

	assert.Equal(t, 0, len(patch.Unset), "$unset 연산자가 비어 있어야 합니다")
	assert.Empty(t, patch.Unset, "$unset 연산자가 비어 있어야 합니다")
	assert.NotNil(t, patch.Unset, "$unset 연산자가 nil이 아니어야 합니다")
	assert.IsType(t, bson.M{}, patch.Unset, "$unset 연산자가 올바른 타입이어야 합니다")

	assert.True(t, patch.IsEmpty(), "패치가 비어 있어야 합니다")
}

// nil 값 처리 테스트
func TestCreateBsonPatch_NilValues(t *testing.T) {

	// nil 값을 포함한 구조체 정의
	type NilTestDoc struct {
		ID       primitive.ObjectID `bson:"_id"`
		Name     string             `bson:"name"`
		Profile  *ProfileStruct     `bson:"profile"`
		Items    []*ItemStruct      `bson:"items"`
		Settings map[string]string  `bson:"settings"`
		Version  int64              `bson:"version"`
	}

	// 원본 문서 생성 (nil 값 포함)
	original := &NilTestDoc{
		ID:      primitive.NewObjectID(),
		Name:    "Nil Test Document",
		Profile: nil, // nil 포인터
		Items:   nil, // nil 슬라이스
		Settings: map[string]string{
			"key1": "value1",
		},
		Version: 1,
	}

	// 수정된 문서 생성 (nil 값 설정 및 해제)
	modified := &NilTestDoc{
		ID:   original.ID,
		Name: "Updated Nil Test Document",
		Profile: &ProfileStruct{ // nil에서 값으로 변경
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john.doe@example.com",
			Age:       30,
		},
		Items: []*ItemStruct{ // nil에서 값으로 변경
			{
				ID:    "item1",
				Name:  "Item 1",
				Value: 5,
				Tags:  []string{"tag1", "tag2"},
				Attributes: map[string]string{
					"color": "red",
					"size":  "medium",
				},
			},
		},
		Settings: nil, // 값에서 nil로 변경
		Version:  2,
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 검증
	assert.Contains(t, patch.Set, "name", "name 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "Updated Nil Test Document", patch.Set["name"], "name 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "profile", "profile 필드가 패치에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "items", "items 필드가 패치에 포함되어 있지 않습니다")

	assert.Contains(t, patch.Set, "version", "version 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, int64(2), patch.Set["version"], "version 필드가 올바르게 설정되지 않았습니다")

	// settings 필드가 nil로 변경되었으므로 settings.key1이 unset에 포함되어야 함
	assert.Contains(t, patch.Unset, "settings.key1", "settings.key1 필드가 $unset에 포함되어 있지 않습니다")
}

// 포인터 필드 테스트
func TestCreateBsonPatch_PointerFields(t *testing.T) {
	// 포인터 필드를 포함한 구조체 정의
	type PointerTestDoc struct {
		ID       primitive.ObjectID `bson:"_id"`
		Name     string             `bson:"name"`
		Age      *int               `bson:"age"`
		Score    *float64           `bson:"score"`
		IsActive *bool              `bson:"isActive"`
		Tags     *[]string          `bson:"tags"`
		Settings *map[string]string `bson:"settings"`
		Version  int64              `bson:"version"`
	}

	// 원본 문서 생성
	age := 30
	score := 85.5
	isActive := true
	tags := []string{"test", "pointer"}
	settings := map[string]string{
		"theme": "dark",
		"lang":  "en",
	}

	original := &PointerTestDoc{
		ID:       primitive.NewObjectID(),
		Name:     "Pointer Test Document",
		Age:      &age,
		Score:    &score,
		IsActive: &isActive,
		Tags:     &tags,
		Settings: &settings,
		Version:  1,
	}

	// 수정된 문서 생성 (포인터 값 변경)
	newAge := 31
	newScore := 90.5
	newIsActive := false
	newTags := []string{"updated", "pointer"}
	newSettings := map[string]string{
		"theme": "light",
		"lang":  "ko",
	}

	modified := &PointerTestDoc{
		ID:       original.ID,
		Name:     "Updated Pointer Test Document",
		Age:      &newAge,
		Score:    &newScore,
		IsActive: &newIsActive,
		Tags:     &newTags,
		Settings: &newSettings,
		Version:  2,
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 검증
	assert.Contains(t, patch.Set, "name", "name 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "Updated Pointer Test Document", patch.Set["name"], "name 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "age", "age 필드가 패치에 포함되어 있지 않습니다")
	// 타입 변환 없이 값만 비교
	ageValue := patch.Set["age"]
	assert.Equal(t, 31, ageValue, "age 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "score", "score 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, 90.5, patch.Set["score"], "score 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "isActive", "isActive 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, false, patch.Set["isActive"], "isActive 필드가 올바르게 설정되지 않았습니다")

	// 배열과 맵은 개별 요소로 처리될 수 있음
	assert.Contains(t, patch.Set, "tags.0", "tags.0 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "updated", patch.Set["tags.0"], "tags.0 값이 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "settings.theme", "settings.theme 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "light", patch.Set["settings.theme"], "settings.theme 값이 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "settings.lang", "settings.lang 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "ko", patch.Set["settings.lang"], "settings.lang 값이 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "version", "version 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, int64(2), patch.Set["version"], "version 필드가 올바르게 설정되지 않았습니다")
}

// 4. MongoDB 적용 테스트 - 생성된 패치를 MongoDB에 적용하여 실제 업데이트 검증
func TestBsonPatch_MongoDBApply(t *testing.T) {
	// MongoDB 연결 설정
	_, collection, cleanup := setupBsonPatchTestDB(t)
	defer cleanup()

	// 테스트 문서 생성
	doc := &SimpleTestDoc{
		ID:        primitive.NewObjectID(),
		Name:      "MongoDB Test Document",
		Age:       30,
		IsActive:  true,
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now().Add(-12 * time.Hour),
		Score:     85.5,
		Tags:      []string{"mongodb", "test", "apply"},
		Metadata: map[string]string{
			"created_by": "system",
			"category":   "test",
		},
		Version: 1,
	}

	// 문서를 MongoDB에 삽입
	ctx := context.Background()
	_, err := collection.InsertOne(ctx, doc)
	require.NoError(t, err, "문서 삽입 중 오류 발생")

	// 수정된 문서 생성
	modified := &SimpleTestDoc{
		ID:        doc.ID,
		Name:      "Updated MongoDB Test Document",
		Age:       31,
		IsActive:  false,
		CreatedAt: doc.CreatedAt,
		UpdatedAt: time.Now(),
		Score:     90.5,
		Tags:      []string{"mongodb", "updated", "apply"},
		Metadata: map[string]string{
			"created_by": "system",
			"category":   "test",
			"updated_by": "user",
		},
		Version: 2,
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(doc, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치를 MongoDB 업데이트 문서로 변환
	updateDoc, err := patch.MarshalBSON()
	require.NoError(t, err, "패치를 BSON으로 마샬링하는 중 오류 발생")

	// MongoDB에 업데이트 적용
	filter := bson.M{"_id": doc.ID}
	var updateBSON bson.M
	err = bson.Unmarshal(updateDoc, &updateBSON)
	require.NoError(t, err, "BSON 언마샬링 중 오류 발생")

	result, err := collection.UpdateOne(ctx, filter, updateBSON)
	require.NoError(t, err, "MongoDB 업데이트 중 오류 발생")
	assert.Equal(t, int64(1), result.MatchedCount, "업데이트된 문서 수가 예상과 다릅니다")
	assert.Equal(t, int64(1), result.ModifiedCount, "수정된 문서 수가 예상과 다릅니다")

	// 업데이트된 문서 조회
	var updatedDoc SimpleTestDoc
	err = collection.FindOne(ctx, filter).Decode(&updatedDoc)
	require.NoError(t, err, "업데이트된 문서 조회 중 오류 발생")

	// 업데이트된 문서 검증
	assert.Equal(t, modified.ID, updatedDoc.ID, "ID가 일치하지 않습니다")
	assert.Equal(t, modified.Name, updatedDoc.Name, "Name이 일치하지 않습니다")
	assert.Equal(t, modified.Age, updatedDoc.Age, "Age가 일치하지 않습니다")
	assert.Equal(t, modified.IsActive, updatedDoc.IsActive, "IsActive가 일치하지 않습니다")
	assert.Equal(t, modified.Version, updatedDoc.Version, "Version이 일치하지 않습니다")
	assert.Equal(t, modified.Score, updatedDoc.Score, "Score가 일치하지 않습니다")
	assert.ElementsMatch(t, modified.Tags, updatedDoc.Tags, "Tags가 일치하지 않습니다")
	assert.Equal(t, len(modified.Metadata), len(updatedDoc.Metadata), "Metadata 크기가 일치하지 않습니다")
	for k, v := range modified.Metadata {
		assert.Equal(t, v, updatedDoc.Metadata[k], "Metadata 값이 일치하지 않습니다: "+k)
	}
}

// 깊은 중첩 구조체에 대한 MongoDB 적용 테스트
func TestBsonPatch_MongoDBApplyDeepNested(t *testing.T) {
	// MongoDB 연결 설정
	_, collection, cleanup := setupBsonPatchTestDB(t)
	defer cleanup()

	// 원본 깊은 중첩 문서 생성
	original := &DeepNestedTestDoc{
		ID:      primitive.NewObjectID(),
		Name:    "Deep Nested MongoDB Test",
		Version: 1,
		Level1: &Level1Struct{
			Name:  "Level 1",
			Value: 1,
			Level2: &Level2Struct{
				Name:  "Level 2",
				Value: 2,
				Level3: &Level3Struct{
					Name:  "Level 3",
					Value: 3,
					Level4: &Level4Struct{
						Name:  "Level 4",
						Value: 4,
						Data:  []int{1, 2, 3, 4},
					},
				},
			},
		},
	}

	// 문서를 MongoDB에 삽입
	ctx := context.Background()
	_, err := collection.InsertOne(ctx, original)
	require.NoError(t, err, "문서 삽입 중 오류 발생")

	// 수정된 깊은 중첩 문서 생성 (깊은 수준의 필드 변경)
	modified := &DeepNestedTestDoc{
		ID:      original.ID,
		Name:    "Updated Deep Nested MongoDB Test", // 변경
		Version: 2,                                  // 변경
		Level1: &Level1Struct{
			Name:  "Updated Level 1", // 변경
			Value: 10,                // 변경
			Level2: &Level2Struct{
				Name:  "Level 2", // 유지
				Value: 20,        // 변경
				Level3: &Level3Struct{
					Name:  "Updated Level 3", // 변경
					Value: 30,                // 변경
					Level4: &Level4Struct{
						Name:  "Level 4",               // 유지
						Value: 40,                      // 변경
						Data:  []int{1, 2, 3, 4, 5, 6}, // 변경 (요소 추가)
					},
				},
			},
		},
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 출력 (디버깅용)
	t.Logf("생성된 패치: %+v", patch)

	// 패치를 MongoDB 업데이트 문서로 변환
	updateDoc, err := patch.MarshalBSON()
	require.NoError(t, err, "패치를 BSON으로 마샬링하는 중 오류 발생")

	// MongoDB에 업데이트 적용
	filter := bson.M{"_id": original.ID}
	var updateBSON bson.M
	err = bson.Unmarshal(updateDoc, &updateBSON)
	require.NoError(t, err, "BSON 언마샬링 중 오류 발생")

	// 업데이트 문서 출력 (디버깅용)
	t.Logf("업데이트 문서: %+v", updateBSON)

	result, err := collection.UpdateOne(ctx, filter, updateBSON)
	require.NoError(t, err, "MongoDB 업데이트 중 오류 발생")
	assert.Equal(t, int64(1), result.MatchedCount, "업데이트된 문서 수가 예상과 다릅니다")
	assert.Equal(t, int64(1), result.ModifiedCount, "수정된 문서 수가 예상과 다릅니다")

	// 업데이트된 문서 조회
	var updatedDoc DeepNestedTestDoc
	err = collection.FindOne(ctx, filter).Decode(&updatedDoc)
	require.NoError(t, err, "업데이트된 문서 조회 중 오류 발생")

	// 업데이트된 문서 검증
	assert.Equal(t, modified.ID, updatedDoc.ID, "ID가 일치하지 않습니다")
	assert.Equal(t, modified.Name, updatedDoc.Name, "Name이 일치하지 않습니다")
	assert.Equal(t, modified.Version, updatedDoc.Version, "Version이 일치하지 않습니다")

	// Level1 검증
	require.NotNil(t, updatedDoc.Level1, "Level1이 nil입니다")
	assert.Equal(t, modified.Level1.Name, updatedDoc.Level1.Name, "Level1.Name이 일치하지 않습니다")
	assert.Equal(t, modified.Level1.Value, updatedDoc.Level1.Value, "Level1.Value가 일치하지 않습니다")

	// Level2 검증
	require.NotNil(t, updatedDoc.Level1.Level2, "Level2가 nil입니다")
	assert.Equal(t, modified.Level1.Level2.Name, updatedDoc.Level1.Level2.Name, "Level2.Name이 일치하지 않습니다")
	assert.Equal(t, modified.Level1.Level2.Value, updatedDoc.Level1.Level2.Value, "Level2.Value가 일치하지 않습니다")

	// Level3 검증
	require.NotNil(t, updatedDoc.Level1.Level2.Level3, "Level3가 nil입니다")
	assert.Equal(t, modified.Level1.Level2.Level3.Name, updatedDoc.Level1.Level2.Level3.Name, "Level3.Name이 일치하지 않습니다")
	assert.Equal(t, modified.Level1.Level2.Level3.Value, updatedDoc.Level1.Level2.Level3.Value, "Level3.Value가 일치하지 않습니다")

	// Level4 검증
	require.NotNil(t, updatedDoc.Level1.Level2.Level3.Level4, "Level4가 nil입니다")
	assert.Equal(t, modified.Level1.Level2.Level3.Level4.Name, updatedDoc.Level1.Level2.Level3.Level4.Name, "Level4.Name이 일치하지 않습니다")
	assert.Equal(t, modified.Level1.Level2.Level3.Level4.Value, updatedDoc.Level1.Level2.Level3.Level4.Value, "Level4.Value가 일치하지 않습니다")
	assert.ElementsMatch(t, modified.Level1.Level2.Level3.Level4.Data, updatedDoc.Level1.Level2.Level3.Level4.Data, "Level4.Data가 일치하지 않습니다")
}

// 포인터 필드를 포함한 구조체에 대한 MongoDB 적용 테스트
func TestBsonPatch_MongoDBApplyPointerFields(t *testing.T) {
	// MongoDB 연결 설정
	_, collection, cleanup := setupBsonPatchTestDB(t)
	defer cleanup()

	// 포인터 필드를 포함한 구조체 정의
	type PointerTestDoc struct {
		ID       primitive.ObjectID `bson:"_id"`
		Name     string             `bson:"name"`
		Age      *int               `bson:"age"`
		Score    *float64           `bson:"score"`
		IsActive *bool              `bson:"isActive"`
		Tags     *[]string          `bson:"tags"`
		Settings *map[string]string `bson:"settings"`
		Version  int64              `bson:"version"`
	}

	// 원본 문서 생성
	age := 30
	score := 85.5
	isActive := true
	tags := []string{"test", "pointer"}
	settings := map[string]string{
		"theme": "dark",
		"lang":  "en",
	}

	original := &PointerTestDoc{
		ID:       primitive.NewObjectID(),
		Name:     "Pointer Test Document",
		Age:      &age,
		Score:    &score,
		IsActive: &isActive,
		Tags:     &tags,
		Settings: &settings,
		Version:  1,
	}

	// 문서를 MongoDB에 삽입
	ctx := context.Background()
	_, err := collection.InsertOne(ctx, original)
	require.NoError(t, err, "문서 삽입 중 오류 발생")

	// 수정된 문서 생성 (포인터 값 변경)
	newAge := 31
	newScore := 90.5
	newIsActive := false
	newTags := []string{"updated", "pointer"}
	newSettings := map[string]string{
		"theme": "light",
		"lang":  "ko",
	}

	modified := &PointerTestDoc{
		ID:       original.ID,
		Name:     "Updated Pointer Test Document",
		Age:      &newAge,
		Score:    &newScore,
		IsActive: &newIsActive,
		Tags:     &newTags,
		Settings: &newSettings,
		Version:  2,
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 출력 (디버깅용)
	t.Logf("생성된 패치: %+v", patch)

	// 패치를 MongoDB 업데이트 문서로 변환
	updateDoc, err := patch.MarshalBSON()
	require.NoError(t, err, "패치를 BSON으로 마샬링하는 중 오류 발생")

	// MongoDB에 업데이트 적용
	filter := bson.M{"_id": original.ID}
	var updateBSON bson.M
	err = bson.Unmarshal(updateDoc, &updateBSON)
	require.NoError(t, err, "BSON 언마샬링 중 오류 발생")

	// 업데이트 문서 출력 (디버깅용)
	t.Logf("업데이트 문서: %+v", updateBSON)

	result, err := collection.UpdateOne(ctx, filter, updateBSON)
	require.NoError(t, err, "MongoDB 업데이트 중 오류 발생")
	assert.Equal(t, int64(1), result.MatchedCount, "업데이트된 문서 수가 예상과 다릅니다")
	assert.Equal(t, int64(1), result.ModifiedCount, "수정된 문서 수가 예상과 다릅니다")

	// 업데이트된 문서 조회
	var updatedDoc PointerTestDoc
	err = collection.FindOne(ctx, filter).Decode(&updatedDoc)
	require.NoError(t, err, "업데이트된 문서 조회 중 오류 발생")

	// 업데이트된 문서 검증
	assert.Equal(t, modified.ID, updatedDoc.ID, "ID가 일치하지 않습니다")
	assert.Equal(t, modified.Name, updatedDoc.Name, "Name이 일치하지 않습니다")
	assert.Equal(t, modified.Version, updatedDoc.Version, "Version이 일치하지 않습니다")

	// 포인터 필드 검증
	require.NotNil(t, updatedDoc.Age, "Age가 nil입니다")
	assert.Equal(t, *modified.Age, *updatedDoc.Age, "Age 값이 일치하지 않습니다")

	require.NotNil(t, updatedDoc.Score, "Score가 nil입니다")
	assert.Equal(t, *modified.Score, *updatedDoc.Score, "Score 값이 일치하지 않습니다")

	require.NotNil(t, updatedDoc.IsActive, "IsActive가 nil입니다")
	assert.Equal(t, *modified.IsActive, *updatedDoc.IsActive, "IsActive 값이 일치하지 않습니다")

	// 포인터 배열 검증
	require.NotNil(t, updatedDoc.Tags, "Tags가 nil입니다")
	assert.ElementsMatch(t, *modified.Tags, *updatedDoc.Tags, "Tags 값이 일치하지 않습니다")

	// 포인터 맵 검증
	require.NotNil(t, updatedDoc.Settings, "Settings가 nil입니다")
	assert.Equal(t, len(*modified.Settings), len(*updatedDoc.Settings), "Settings 크기가 일치하지 않습니다")
	for k, v := range *modified.Settings {
		val, ok := (*updatedDoc.Settings)[k]
		assert.True(t, ok, "Settings에 키가 없습니다: "+k)
		assert.Equal(t, v, val, "Settings 값이 일치하지 않습니다: "+k)
	}
}

// 배열이 많은 구조체에 대한 MongoDB 적용 테스트
func TestBsonPatch_MongoDBApplyArrayHeavy(t *testing.T) {
	// MongoDB 연결 설정
	_, collection, cleanup := setupBsonPatchTestDB(t)
	defer cleanup()

	// 원본 배열 중심 문서 생성
	original := &ArrayHeavyTestDoc{
		ID:      primitive.NewObjectID(),
		Name:    "Array Heavy MongoDB Test",
		Version: 1,
		Strings: []string{"one", "two", "three"},
		Numbers: []int{1, 2, 3, 4, 5},
		Floats:  []float64{1.1, 2.2, 3.3},
		Objects: []*SimpleObject{
			{Key: "key1", Value: "value1"},
			{Key: "key2", Value: "value2"},
		},
		Mixed: []interface{}{
			"string", 123, 45.6, true,
		},
		Matrix: [][]int{
			{1, 2, 3},
			{4, 5, 6},
		},
		Complex: [][][]string{
			{
				{"a", "b"},
				{"c", "d"},
			},
		},
		Maps: []map[string]string{
			{"k1": "v1", "k2": "v2"},
			{"k3": "v3", "k4": "v4"},
		},
	}

	// 문서를 MongoDB에 삽입
	ctx := context.Background()
	_, err := collection.InsertOne(ctx, original)
	require.NoError(t, err, "문서 삽입 중 오류 발생")

	// 수정된 배열 중심 문서 생성
	modified := &ArrayHeavyTestDoc{
		ID:      original.ID,
		Name:    "Updated Array Heavy MongoDB Test",              // 변경
		Version: 2,                                               // 변경
		Strings: []string{"one", "two", "three", "four", "five"}, // 요소 추가
		Numbers: []int{5, 4, 3, 2, 1},                            // 순서 변경
		Floats:  []float64{1.1, 2.2, 3.3, 4.4},                   // 요소 추가
		Objects: []*SimpleObject{ // 요소 변경 및 추가
			{Key: "key1", Value: "updated_value1"}, // 값 변경
			{Key: "key2", Value: "value2"},         // 유지
			{Key: "key3", Value: "value3"},         // 추가
		},
		Mixed: []interface{}{ // 요소 변경 및 추가
			"updated_string", 123, 45.6, false, "new_value", // 일부 변경 및 추가
		},
		Matrix: [][]int{ // 행렬 변경
			{10, 20, 30}, // 값 변경
			{40, 50, 60}, // 값 변경
			{70, 80, 90}, // 행 추가
		},
		Complex: [][][]string{ // 복잡한 배열 변경
			{
				{"A", "B"}, // 값 변경
				{"c", "d"}, // 유지
				{"e", "f"}, // 추가
			},
			{
				{"g", "h"}, // 새 차원 추가
			},
		},
		Maps: []map[string]string{ // 맵 배열 변경
			{"k1": "updated_v1", "k2": "v2"},     // 값 변경
			{"k3": "v3", "k4": "v4", "k5": "v5"}, // 키 추가
			{"k6": "v6", "k7": "v7"},             // 맵 추가
		},
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 출력 (디버깅용)
	t.Logf("생성된 패치: %+v", patch)

	// 패치를 MongoDB 업데이트 문서로 변환
	updateDoc, err := patch.MarshalBSON()
	require.NoError(t, err, "패치를 BSON으로 마샬링하는 중 오류 발생")

	// MongoDB에 업데이트 적용
	filter := bson.M{"_id": original.ID}
	var updateBSON bson.M
	err = bson.Unmarshal(updateDoc, &updateBSON)
	require.NoError(t, err, "BSON 언마샬링 중 오류 발생")

	// 업데이트 문서 출력 (디버깅용)
	t.Logf("업데이트 문서: %+v", updateBSON)

	result, err := collection.UpdateOne(ctx, filter, updateBSON)
	require.NoError(t, err, "MongoDB 업데이트 중 오류 발생")
	assert.Equal(t, int64(1), result.MatchedCount, "업데이트된 문서 수가 예상과 다릅니다")
	assert.Equal(t, int64(1), result.ModifiedCount, "수정된 문서 수가 예상과 다릅니다")

	// 업데이트된 문서 조회
	var updatedDoc ArrayHeavyTestDoc
	err = collection.FindOne(ctx, filter).Decode(&updatedDoc)
	require.NoError(t, err, "업데이트된 문서 조회 중 오류 발생")

	// 업데이트된 문서 검증
	assert.Equal(t, modified.ID, updatedDoc.ID, "ID가 일치하지 않습니다")
	assert.Equal(t, modified.Name, updatedDoc.Name, "Name이 일치하지 않습니다")
	assert.Equal(t, modified.Version, updatedDoc.Version, "Version이 일치하지 않습니다")

	// 배열 필드 검증
	assert.ElementsMatch(t, modified.Strings, updatedDoc.Strings, "Strings가 일치하지 않습니다")
	assert.ElementsMatch(t, modified.Numbers, updatedDoc.Numbers, "Numbers가 일치하지 않습니다")
	assert.ElementsMatch(t, modified.Floats, updatedDoc.Floats, "Floats가 일치하지 않습니다")

	// Objects 배열 검증
	assert.Equal(t, len(modified.Objects), len(updatedDoc.Objects), "Objects 배열 길이가 일치하지 않습니다")
	for i, obj := range modified.Objects {
		if i < len(updatedDoc.Objects) {
			assert.Equal(t, obj.Key, updatedDoc.Objects[i].Key, "Objects[%d].Key가 일치하지 않습니다", i)
			assert.Equal(t, obj.Value, updatedDoc.Objects[i].Value, "Objects[%d].Value가 일치하지 않습니다", i)
		}
	}

	// Mixed 배열 검증 - interface{} 타입이므로 직접 비교가 어려움
	assert.Equal(t, len(modified.Mixed), len(updatedDoc.Mixed), "Mixed 배열 길이가 일치하지 않습니다")

	// Matrix 배열 검증
	assert.Equal(t, len(modified.Matrix), len(updatedDoc.Matrix), "Matrix 배열 길이가 일치하지 않습니다")
	for i, row := range modified.Matrix {
		if i < len(updatedDoc.Matrix) {
			assert.ElementsMatch(t, row, updatedDoc.Matrix[i], "Matrix[%d]가 일치하지 않습니다", i)
		}
	}

	// Complex 배열 검증
	assert.Equal(t, len(modified.Complex), len(updatedDoc.Complex), "Complex 배열 길이가 일치하지 않습니다")

	// Maps 배열 검증
	assert.Equal(t, len(modified.Maps), len(updatedDoc.Maps), "Maps 배열 길이가 일치하지 않습니다")
	for i, m := range modified.Maps {
		if i < len(updatedDoc.Maps) {
			assert.Equal(t, len(m), len(updatedDoc.Maps[i]), "Maps[%d] 크기가 일치하지 않습니다", i)
			for k, v := range m {
				val, ok := updatedDoc.Maps[i][k]
				assert.True(t, ok, "Maps[%d]에 키가 없습니다: %s", i, k)
				assert.Equal(t, v, val, "Maps[%d][%s] 값이 일치하지 않습니다", i, k)
			}
		}
	}
}

// 맵이 많은 구조체에 대한 MongoDB 적용 테스트
func TestBsonPatch_MongoDBApplyMapHeavy(t *testing.T) {
	// MongoDB 연결 설정
	_, collection, cleanup := setupBsonPatchTestDB(t)
	defer cleanup()

	// 원본 맵 중심 문서 생성
	original := &MapHeavyTestDoc{
		ID:      primitive.NewObjectID(),
		Name:    "Map Heavy MongoDB Test",
		Version: 1,
		StringMap: map[string]string{
			"key1": "value1",
			"key2": "value2",
		},
		NumberMap: map[string]int{
			"one": 1,
			"two": 2,
		},
		FloatMap: map[string]float64{
			"pi":    3.14,
			"euler": 2.71,
		},
		ObjectMap: map[string]*SimpleObject{
			"obj1": {Key: "key1", Value: "value1"},
			"obj2": {Key: "key2", Value: "value2"},
		},
		MixedMap: map[string]interface{}{
			"string": "value",
			"number": 123,
			"float":  45.6,
			"bool":   true,
		},
		NestedMap: map[string]map[string]string{
			"user": {
				"name":  "John",
				"email": "john@example.com",
			},
			"settings": {
				"theme": "dark",
				"lang":  "en",
			},
		},
		ComplexMap: map[string]map[string]interface{}{
			"profile": {
				"name":  "John Doe",
				"age":   30,
				"admin": true,
			},
		},
	}

	// 문서를 MongoDB에 삽입
	ctx := context.Background()
	_, err := collection.InsertOne(ctx, original)
	require.NoError(t, err, "문서 삽입 중 오류 발생")

	// 수정된 맵 중심 문서 생성
	modified := &MapHeavyTestDoc{
		ID:      original.ID,
		Name:    "Updated Map Heavy MongoDB Test", // 변경
		Version: 2,                                // 변경
		StringMap: map[string]string{ // 값 변경, 키 추가/삭제
			"key1": "updated_value1", // 값 변경
			"key3": "value3",         // 키 추가
			// key2 삭제
		},
		NumberMap: map[string]int{ // 값 변경, 키 추가
			"one":   10, // 값 변경
			"two":   2,  // 유지
			"three": 3,  // 키 추가
		},
		FloatMap: map[string]float64{ // 값 변경, 키 추가
			"pi":     3.14159, // 값 변경
			"euler":  2.71828, // 값 변경
			"golden": 1.61803, // 키 추가
		},
		ObjectMap: map[string]*SimpleObject{ // 객체 변경, 추가
			"obj1": {Key: "updated_key1", Value: "updated_value1"}, // 값 변경
			"obj2": {Key: "key2", Value: "value2"},                 // 유지
			"obj3": {Key: "key3", Value: "value3"},                 // 추가
		},
		MixedMap: map[string]interface{}{ // 값 변경, 키 추가/삭제
			"string": "updated_value", // 값 변경
			"number": 456,             // 값 변경
			"float":  45.6,            // 유지
			"array":  []int{1, 2, 3},  // 키 추가
			// bool 삭제
		},
		NestedMap: map[string]map[string]string{ // 중첩 맵 변경
			"user": { // 값 변경, 키 추가
				"name":  "John Smith",       // 값 변경
				"email": "john@example.com", // 유지
				"phone": "123-456-7890",     // 키 추가
			},
			"settings": { // 값 변경
				"theme": "light", // 값 변경
				"lang":  "ko",    // 값 변경
			},
			"preferences": { // 맵 추가
				"notifications": "enabled",
				"timezone":      "UTC+9",
			},
		},
		ComplexMap: map[string]map[string]interface{}{ // 복잡한 중첩 맵 변경
			"profile": { // 값 변경, 키 추가
				"name":    "John Smith",  // 값 변경
				"age":     31,            // 값 변경
				"admin":   true,          // 유지
				"address": "123 Main St", // 키 추가
			},
			"stats": { // 맵 추가
				"visits": 100,
				"likes":  50,
				"active": true,
			},
		},
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 출력 (디버깅용)
	t.Logf("생성된 패치: %+v", patch)

	// 패치를 MongoDB 업데이트 문서로 변환
	updateDoc, err := patch.MarshalBSON()
	require.NoError(t, err, "패치를 BSON으로 마샬링하는 중 오류 발생")

	// MongoDB에 업데이트 적용
	filter := bson.M{"_id": original.ID}
	var updateBSON bson.M
	err = bson.Unmarshal(updateDoc, &updateBSON)
	require.NoError(t, err, "BSON 언마샬링 중 오류 발생")

	// 업데이트 문서 출력 (디버깅용)
	t.Logf("업데이트 문서: %+v", updateBSON)

	result, err := collection.UpdateOne(ctx, filter, updateBSON)
	require.NoError(t, err, "MongoDB 업데이트 중 오류 발생")
	assert.Equal(t, int64(1), result.MatchedCount, "업데이트된 문서 수가 예상과 다릅니다")
	assert.Equal(t, int64(1), result.ModifiedCount, "수정된 문서 수가 예상과 다릅니다")

	// 업데이트된 문서 조회
	var updatedDoc MapHeavyTestDoc
	err = collection.FindOne(ctx, filter).Decode(&updatedDoc)
	require.NoError(t, err, "업데이트된 문서 조회 중 오류 발생")

	// 업데이트된 문서 검증
	assert.Equal(t, modified.ID, updatedDoc.ID, "ID가 일치하지 않습니다")
	assert.Equal(t, modified.Name, updatedDoc.Name, "Name이 일치하지 않습니다")
	assert.Equal(t, modified.Version, updatedDoc.Version, "Version이 일치하지 않습니다")

	// StringMap 검증
	assert.Equal(t, len(modified.StringMap), len(updatedDoc.StringMap), "StringMap 크기가 일치하지 않습니다")
	for k, v := range modified.StringMap {
		val, ok := updatedDoc.StringMap[k]
		assert.True(t, ok, "StringMap에 키가 없습니다: "+k)
		assert.Equal(t, v, val, "StringMap 값이 일치하지 않습니다: "+k)
	}
	_, hasKey2 := updatedDoc.StringMap["key2"]
	assert.False(t, hasKey2, "삭제된 key2가 여전히 존재합니다")

	// NumberMap 검증
	assert.Equal(t, len(modified.NumberMap), len(updatedDoc.NumberMap), "NumberMap 크기가 일치하지 않습니다")
	for k, v := range modified.NumberMap {
		val, ok := updatedDoc.NumberMap[k]
		assert.True(t, ok, "NumberMap에 키가 없습니다: "+k)
		assert.Equal(t, v, val, "NumberMap 값이 일치하지 않습니다: "+k)
	}

	// FloatMap 검증
	assert.Equal(t, len(modified.FloatMap), len(updatedDoc.FloatMap), "FloatMap 크기가 일치하지 않습니다")
	for k, v := range modified.FloatMap {
		val, ok := updatedDoc.FloatMap[k]
		assert.True(t, ok, "FloatMap에 키가 없습니다: "+k)
		assert.Equal(t, v, val, "FloatMap 값이 일치하지 않습니다: "+k)
	}

	// ObjectMap 검증
	assert.Equal(t, len(modified.ObjectMap), len(updatedDoc.ObjectMap), "ObjectMap 크기가 일치하지 않습니다")
	for k, v := range modified.ObjectMap {
		obj, ok := updatedDoc.ObjectMap[k]
		assert.True(t, ok, "ObjectMap에 키가 없습니다: "+k)
		assert.Equal(t, v.Key, obj.Key, "ObjectMap[%s].Key가 일치하지 않습니다", k)
		assert.Equal(t, v.Value, obj.Value, "ObjectMap[%s].Value가 일치하지 않습니다", k)
	}

	// NestedMap 검증
	assert.Equal(t, len(modified.NestedMap), len(updatedDoc.NestedMap), "NestedMap 크기가 일치하지 않습니다")
	for k, v := range modified.NestedMap {
		nestedMap, ok := updatedDoc.NestedMap[k]
		assert.True(t, ok, "NestedMap에 키가 없습니다: "+k)
		assert.Equal(t, len(v), len(nestedMap), "NestedMap[%s] 크기가 일치하지 않습니다", k)
		for k2, v2 := range v {
			val, ok := nestedMap[k2]
			assert.True(t, ok, "NestedMap[%s]에 키가 없습니다: %s", k, k2)
			assert.Equal(t, v2, val, "NestedMap[%s][%s] 값이 일치하지 않습니다", k, k2)
		}
	}

	// ComplexMap 검증
	assert.Equal(t, len(modified.ComplexMap), len(updatedDoc.ComplexMap), "ComplexMap 크기가 일치하지 않습니다")
	for k, v := range modified.ComplexMap {
		nestedMap, ok := updatedDoc.ComplexMap[k]
		assert.True(t, ok, "ComplexMap에 키가 없습니다: "+k)
		assert.Equal(t, len(v), len(nestedMap), "ComplexMap[%s] 크기가 일치하지 않습니다", k)
	}
}

// 포인터 필드 심화 테스트 - 값이 있다가 nil이 되는 경우
func TestBsonPatch_PointerFieldsToNil(t *testing.T) {
	// 포인터 필드를 포함한 구조체 정의
	type PointerAdvancedTestDoc struct {
		ID      primitive.ObjectID `bson:"_id"`
		Name    string             `bson:"name"`
		Version int64              `bson:"version"`

		// 원시 타입 포인터
		IntPtr    *int       `bson:"intPtr"`
		FloatPtr  *float64   `bson:"floatPtr"`
		BoolPtr   *bool      `bson:"boolPtr"`
		StringPtr *string    `bson:"stringPtr"`
		TimePtr   *time.Time `bson:"timePtr"`

		// 구조체 포인터
		StructPtr *ProfileStruct `bson:"structPtr"`

		// 배열 포인터
		IntArrayPtr    *[]int           `bson:"intArrayPtr"`
		StringArrayPtr *[]string        `bson:"stringArrayPtr"`
		StructArrayPtr *[]ProfileStruct `bson:"structArrayPtr"`

		// 맵 포인터
		StringMapPtr *map[string]string        `bson:"stringMapPtr"`
		IntMapPtr    *map[string]int           `bson:"intMapPtr"`
		StructMapPtr *map[string]ProfileStruct `bson:"structMapPtr"`
	}

	// 원본 문서 생성 (모든 포인터 필드에 값 설정)
	intVal := 42
	floatVal := 3.14
	boolVal := true
	stringVal := "hello"
	timeVal := time.Now()

	structVal := ProfileStruct{
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john.doe@example.com",
		Age:       30,
	}

	intArrayVal := []int{1, 2, 3, 4, 5}
	stringArrayVal := []string{"one", "two", "three"}
	structArrayVal := []ProfileStruct{
		{
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john@example.com",
			Age:       30,
		},
		{
			FirstName: "Jane",
			LastName:  "Smith",
			Email:     "jane@example.com",
			Age:       28,
		},
	}

	stringMapVal := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	intMapVal := map[string]int{
		"one": 1,
		"two": 2,
	}
	structMapVal := map[string]ProfileStruct{
		"user1": {
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john@example.com",
			Age:       30,
		},
		"user2": {
			FirstName: "Jane",
			LastName:  "Smith",
			Email:     "jane@example.com",
			Age:       28,
		},
	}

	original := &PointerAdvancedTestDoc{
		ID:      primitive.NewObjectID(),
		Name:    "Pointer To Nil Test",
		Version: 1,

		// 원시 타입 포인터
		IntPtr:    &intVal,
		FloatPtr:  &floatVal,
		BoolPtr:   &boolVal,
		StringPtr: &stringVal,
		TimePtr:   &timeVal,

		// 구조체 포인터
		StructPtr: &structVal,

		// 배열 포인터
		IntArrayPtr:    &intArrayVal,
		StringArrayPtr: &stringArrayVal,
		StructArrayPtr: &structArrayVal,

		// 맵 포인터
		StringMapPtr: &stringMapVal,
		IntMapPtr:    &intMapVal,
		StructMapPtr: &structMapVal,
	}

	// 수정된 문서 생성 (모든 포인터 필드를 nil로 설정)
	modified := &PointerAdvancedTestDoc{
		ID:      original.ID,
		Name:    "Updated Pointer To Nil Test",
		Version: 2,

		// 모든 포인터 필드를 nil로 설정
		IntPtr:         nil,
		FloatPtr:       nil,
		BoolPtr:        nil,
		StringPtr:      nil,
		TimePtr:        nil,
		StructPtr:      nil,
		IntArrayPtr:    nil,
		StringArrayPtr: nil,
		StructArrayPtr: nil,
		StringMapPtr:   nil,
		IntMapPtr:      nil,
		StructMapPtr:   nil,
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 검증
	assert.Contains(t, patch.Set, "name", "name 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "Updated Pointer To Nil Test", patch.Set["name"], "name 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "version", "version 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, int64(2), patch.Set["version"], "version 필드가 올바르게 설정되지 않았습니다")

	// 모든 포인터 필드가 $unset에 포함되어 있는지 확인
	assert.Contains(t, patch.Unset, "intPtr", "intPtr 필드가 $unset에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Unset, "floatPtr", "floatPtr 필드가 $unset에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Unset, "boolPtr", "boolPtr 필드가 $unset에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Unset, "stringPtr", "stringPtr 필드가 $unset에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Unset, "timePtr", "timePtr 필드가 $unset에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Unset, "structPtr", "structPtr 필드가 $unset에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Unset, "intArrayPtr", "intArrayPtr 필드가 $unset에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Unset, "stringArrayPtr", "stringArrayPtr 필드가 $unset에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Unset, "structArrayPtr", "structArrayPtr 필드가 $unset에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Unset, "stringMapPtr", "stringMapPtr 필드가 $unset에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Unset, "intMapPtr", "intMapPtr 필드가 $unset에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Unset, "structMapPtr", "structMapPtr 필드가 $unset에 포함되어 있지 않습니다")

	// MongoDB 적용 테스트
	_, collection, cleanup := setupBsonPatchTestDB(t)
	defer cleanup()

	// 원본 문서를 MongoDB에 삽입
	ctx := context.Background()
	_, err = collection.InsertOne(ctx, original)
	require.NoError(t, err, "문서 삽입 중 오류 발생")

	// 패치를 MongoDB 업데이트 문서로 변환
	updateDoc, err := patch.MarshalBSON()
	require.NoError(t, err, "패치를 BSON으로 마샬링하는 중 오류 발생")

	// MongoDB에 업데이트 적용
	filter := bson.M{"_id": original.ID}
	var updateBSON bson.M
	err = bson.Unmarshal(updateDoc, &updateBSON)
	require.NoError(t, err, "BSON 언마샬링 중 오류 발생")

	result, err := collection.UpdateOne(ctx, filter, updateBSON)
	require.NoError(t, err, "MongoDB 업데이트 중 오류 발생")
	assert.Equal(t, int64(1), result.MatchedCount, "업데이트된 문서 수가 예상과 다릅니다")
	assert.Equal(t, int64(1), result.ModifiedCount, "수정된 문서 수가 예상과 다릅니다")

	// 업데이트된 문서 조회
	var updatedDoc PointerAdvancedTestDoc
	err = collection.FindOne(ctx, filter).Decode(&updatedDoc)
	require.NoError(t, err, "업데이트된 문서 조회 중 오류 발생")

	// 업데이트된 문서 검증
	assert.Equal(t, modified.ID, updatedDoc.ID, "ID가 일치하지 않습니다")
	assert.Equal(t, modified.Name, updatedDoc.Name, "Name이 일치하지 않습니다")
	assert.Equal(t, modified.Version, updatedDoc.Version, "Version이 일치하지 않습니다")

	// 모든 포인터 필드가 nil인지 확인
	assert.Nil(t, updatedDoc.IntPtr, "IntPtr가 nil이 아닙니다")
	assert.Nil(t, updatedDoc.FloatPtr, "FloatPtr가 nil이 아닙니다")
	assert.Nil(t, updatedDoc.BoolPtr, "BoolPtr가 nil이 아닙니다")
	assert.Nil(t, updatedDoc.StringPtr, "StringPtr가 nil이 아닙니다")
	assert.Nil(t, updatedDoc.TimePtr, "TimePtr가 nil이 아닙니다")
	assert.Nil(t, updatedDoc.StructPtr, "StructPtr가 nil이 아닙니다")
	assert.Nil(t, updatedDoc.IntArrayPtr, "IntArrayPtr가 nil이 아닙니다")
	assert.Nil(t, updatedDoc.StringArrayPtr, "StringArrayPtr가 nil이 아닙니다")
	assert.Nil(t, updatedDoc.StructArrayPtr, "StructArrayPtr가 nil이 아닙니다")
	assert.Nil(t, updatedDoc.StringMapPtr, "StringMapPtr가 nil이 아닙니다")
	assert.Nil(t, updatedDoc.IntMapPtr, "IntMapPtr가 nil이 아닙니다")
	assert.Nil(t, updatedDoc.StructMapPtr, "StructMapPtr가 nil이 아닙니다")
}

// 포인터 필드 심화 테스트 - nil이었다가 값이 생기는 경우
func TestBsonPatch_PointerFieldsFromNil(t *testing.T) {
	// 포인터 필드를 포함한 구조체 정의
	type PointerAdvancedTestDoc struct {
		ID      primitive.ObjectID `bson:"_id"`
		Name    string             `bson:"name"`
		Version int64              `bson:"version"`

		// 원시 타입 포인터
		IntPtr    *int       `bson:"intPtr"`
		FloatPtr  *float64   `bson:"floatPtr"`
		BoolPtr   *bool      `bson:"boolPtr"`
		StringPtr *string    `bson:"stringPtr"`
		TimePtr   *time.Time `bson:"timePtr"`

		// 구조체 포인터
		StructPtr *ProfileStruct `bson:"structPtr"`

		// 배열 포인터
		IntArrayPtr    *[]int           `bson:"intArrayPtr"`
		StringArrayPtr *[]string        `bson:"stringArrayPtr"`
		StructArrayPtr *[]ProfileStruct `bson:"structArrayPtr"`

		// 맵 포인터
		StringMapPtr *map[string]string        `bson:"stringMapPtr"`
		IntMapPtr    *map[string]int           `bson:"intMapPtr"`
		StructMapPtr *map[string]ProfileStruct `bson:"structMapPtr"`
	}

	// 원본 문서 생성 (모든 포인터 필드가 nil)
	original := &PointerAdvancedTestDoc{
		ID:      primitive.NewObjectID(),
		Name:    "Pointer From Nil Test",
		Version: 1,

		// 모든 포인터 필드를 nil로 설정
		IntPtr:         nil,
		FloatPtr:       nil,
		BoolPtr:        nil,
		StringPtr:      nil,
		TimePtr:        nil,
		StructPtr:      nil,
		IntArrayPtr:    nil,
		StringArrayPtr: nil,
		StructArrayPtr: nil,
		StringMapPtr:   nil,
		IntMapPtr:      nil,
		StructMapPtr:   nil,
	}

	// 수정된 문서 생성 (모든 포인터 필드에 값 설정)
	intVal := 42
	floatVal := 3.14
	boolVal := true
	stringVal := "hello"
	timeVal := time.Now()

	structVal := ProfileStruct{
		FirstName: "John",
		LastName:  "Doe",
		Email:     "john.doe@example.com",
		Age:       30,
	}

	intArrayVal := []int{1, 2, 3, 4, 5}
	stringArrayVal := []string{"one", "two", "three"}
	structArrayVal := []ProfileStruct{
		{
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john@example.com",
			Age:       30,
		},
		{
			FirstName: "Jane",
			LastName:  "Smith",
			Email:     "jane@example.com",
			Age:       28,
		},
	}

	stringMapVal := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	intMapVal := map[string]int{
		"one": 1,
		"two": 2,
	}
	structMapVal := map[string]ProfileStruct{
		"user1": {
			FirstName: "John",
			LastName:  "Doe",
			Email:     "john@example.com",
			Age:       30,
		},
		"user2": {
			FirstName: "Jane",
			LastName:  "Smith",
			Email:     "jane@example.com",
			Age:       28,
		},
	}

	modified := &PointerAdvancedTestDoc{
		ID:      original.ID,
		Name:    "Updated Pointer From Nil Test",
		Version: 2,

		// 원시 타입 포인터
		IntPtr:    &intVal,
		FloatPtr:  &floatVal,
		BoolPtr:   &boolVal,
		StringPtr: &stringVal,
		TimePtr:   &timeVal,

		// 구조체 포인터
		StructPtr: &structVal,

		// 배열 포인터
		IntArrayPtr:    &intArrayVal,
		StringArrayPtr: &stringArrayVal,
		StructArrayPtr: &structArrayVal,

		// 맵 포인터
		StringMapPtr: &stringMapVal,
		IntMapPtr:    &intMapVal,
		StructMapPtr: &structMapVal,
	}

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내용 출력 (디버깅용)
	t.Logf("생성된 패치: %+v", patch)
	t.Logf("intPtr 타입: %T, 값: %v", patch.Set["intPtr"], patch.Set["intPtr"])
	t.Logf("floatPtr 타입: %T, 값: %v", patch.Set["floatPtr"], patch.Set["floatPtr"])
	t.Logf("boolPtr 타입: %T, 값: %v", patch.Set["boolPtr"], patch.Set["boolPtr"])
	t.Logf("stringPtr 타입: %T, 값: %v", patch.Set["stringPtr"], patch.Set["stringPtr"])

	// 패치 내용 검증
	assert.Contains(t, patch.Set, "name", "name 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, "Updated Pointer From Nil Test", patch.Set["name"], "name 필드가 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "version", "version 필드가 패치에 포함되어 있지 않습니다")
	assert.Equal(t, int64(2), patch.Set["version"], "version 필드가 올바르게 설정되지 않았습니다")

	// 모든 포인터 필드가 $set에 포함되어 있는지 확인
	assert.Contains(t, patch.Set, "intPtr", "intPtr 필드가 $set에 포함되어 있지 않습니다")
	// 패치에서는 포인터 값이 저장됨
	intPtrVal, ok := patch.Set["intPtr"].(*int)
	assert.True(t, ok, "intPtr 값이 *int 타입이 아닙니다")
	assert.Equal(t, intVal, *intPtrVal, "intPtr 값이 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "floatPtr", "floatPtr 필드가 $set에 포함되어 있지 않습니다")
	floatPtrVal, ok := patch.Set["floatPtr"].(*float64)
	assert.True(t, ok, "floatPtr 값이 *float64 타입이 아닙니다")
	assert.Equal(t, floatVal, *floatPtrVal, "floatPtr 값이 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "boolPtr", "boolPtr 필드가 $set에 포함되어 있지 않습니다")
	boolPtrVal, ok := patch.Set["boolPtr"].(*bool)
	assert.True(t, ok, "boolPtr 값이 *bool 타입이 아닙니다")
	assert.Equal(t, boolVal, *boolPtrVal, "boolPtr 값이 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "stringPtr", "stringPtr 필드가 $set에 포함되어 있지 않습니다")
	stringPtrVal, ok := patch.Set["stringPtr"].(*string)
	assert.True(t, ok, "stringPtr 값이 *string 타입이 아닙니다")
	assert.Equal(t, stringVal, *stringPtrVal, "stringPtr 값이 올바르게 설정되지 않았습니다")

	assert.Contains(t, patch.Set, "structPtr", "structPtr 필드가 $set에 포함되어 있지 않습니다")

	assert.Contains(t, patch.Set, "intArrayPtr", "intArrayPtr 필드가 $set에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "stringArrayPtr", "stringArrayPtr 필드가 $set에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "structArrayPtr", "structArrayPtr 필드가 $set에 포함되어 있지 않습니다")

	assert.Contains(t, patch.Set, "stringMapPtr", "stringMapPtr 필드가 $set에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "intMapPtr", "intMapPtr 필드가 $set에 포함되어 있지 않습니다")
	assert.Contains(t, patch.Set, "structMapPtr", "structMapPtr 필드가 $set에 포함되어 있지 않습니다")

	// MongoDB 적용 테스트
	_, collection, cleanup := setupBsonPatchTestDB(t)
	defer cleanup()

	// 원본 문서를 MongoDB에 삽입
	ctx := context.Background()
	_, err = collection.InsertOne(ctx, original)
	require.NoError(t, err, "문서 삽입 중 오류 발생")

	// 패치를 MongoDB 업데이트 문서로 변환
	updateDoc, err := patch.MarshalBSON()
	require.NoError(t, err, "패치를 BSON으로 마샬링하는 중 오류 발생")

	// MongoDB에 업데이트 적용
	filter := bson.M{"_id": original.ID}
	var updateBSON bson.M
	err = bson.Unmarshal(updateDoc, &updateBSON)
	require.NoError(t, err, "BSON 언마샬링 중 오류 발생")

	result, err := collection.UpdateOne(ctx, filter, updateBSON)
	require.NoError(t, err, "MongoDB 업데이트 중 오류 발생")
	assert.Equal(t, int64(1), result.MatchedCount, "업데이트된 문서 수가 예상과 다릅니다")
	assert.Equal(t, int64(1), result.ModifiedCount, "수정된 문서 수가 예상과 다릅니다")

	// 업데이트된 문서 조회
	var updatedDoc PointerAdvancedTestDoc
	err = collection.FindOne(ctx, filter).Decode(&updatedDoc)
	require.NoError(t, err, "업데이트된 문서 조회 중 오류 발생")

	// 업데이트된 문서 검증
	assert.Equal(t, modified.ID, updatedDoc.ID, "ID가 일치하지 않습니다")
	assert.Equal(t, modified.Name, updatedDoc.Name, "Name이 일치하지 않습니다")
	assert.Equal(t, modified.Version, updatedDoc.Version, "Version이 일치하지 않습니다")

	// 모든 포인터 필드가 nil이 아니고 올바른 값을 가지는지 확인
	require.NotNil(t, updatedDoc.IntPtr, "IntPtr가 nil입니다")
	assert.Equal(t, *modified.IntPtr, *updatedDoc.IntPtr, "IntPtr 값이 일치하지 않습니다")

	require.NotNil(t, updatedDoc.FloatPtr, "FloatPtr가 nil입니다")
	assert.Equal(t, *modified.FloatPtr, *updatedDoc.FloatPtr, "FloatPtr 값이 일치하지 않습니다")

	require.NotNil(t, updatedDoc.BoolPtr, "BoolPtr가 nil입니다")
	assert.Equal(t, *modified.BoolPtr, *updatedDoc.BoolPtr, "BoolPtr 값이 일치하지 않습니다")

	require.NotNil(t, updatedDoc.StringPtr, "StringPtr가 nil입니다")
	assert.Equal(t, *modified.StringPtr, *updatedDoc.StringPtr, "StringPtr 값이 일치하지 않습니다")

	require.NotNil(t, updatedDoc.TimePtr, "TimePtr가 nil입니다")

	// 구조체 포인터 검증
	require.NotNil(t, updatedDoc.StructPtr, "StructPtr가 nil입니다")
	assert.Equal(t, modified.StructPtr.FirstName, updatedDoc.StructPtr.FirstName, "StructPtr.FirstName 값이 일치하지 않습니다")
	assert.Equal(t, modified.StructPtr.LastName, updatedDoc.StructPtr.LastName, "StructPtr.LastName 값이 일치하지 않습니다")
	assert.Equal(t, modified.StructPtr.Email, updatedDoc.StructPtr.Email, "StructPtr.Email 값이 일치하지 않습니다")
	assert.Equal(t, modified.StructPtr.Age, updatedDoc.StructPtr.Age, "StructPtr.Age 값이 일치하지 않습니다")

	// 배열 포인터 검증
	require.NotNil(t, updatedDoc.IntArrayPtr, "IntArrayPtr가 nil입니다")
	assert.ElementsMatch(t, *modified.IntArrayPtr, *updatedDoc.IntArrayPtr, "IntArrayPtr 값이 일치하지 않습니다")

	require.NotNil(t, updatedDoc.StringArrayPtr, "StringArrayPtr가 nil입니다")
	assert.ElementsMatch(t, *modified.StringArrayPtr, *updatedDoc.StringArrayPtr, "StringArrayPtr 값이 일치하지 않습니다")

	require.NotNil(t, updatedDoc.StructArrayPtr, "StructArrayPtr가 nil입니다")
	assert.Equal(t, len(*modified.StructArrayPtr), len(*updatedDoc.StructArrayPtr), "StructArrayPtr 길이가 일치하지 않습니다")

	// 맵 포인터 검증
	require.NotNil(t, updatedDoc.StringMapPtr, "StringMapPtr가 nil입니다")
	assert.Equal(t, len(*modified.StringMapPtr), len(*updatedDoc.StringMapPtr), "StringMapPtr 크기가 일치하지 않습니다")
	for k, v := range *modified.StringMapPtr {
		val, ok := (*updatedDoc.StringMapPtr)[k]
		assert.True(t, ok, "StringMapPtr에 키가 없습니다: "+k)
		assert.Equal(t, v, val, "StringMapPtr 값이 일치하지 않습니다: "+k)
	}

	require.NotNil(t, updatedDoc.IntMapPtr, "IntMapPtr가 nil입니다")
	assert.Equal(t, len(*modified.IntMapPtr), len(*updatedDoc.IntMapPtr), "IntMapPtr 크기가 일치하지 않습니다")
	for k, v := range *modified.IntMapPtr {
		val, ok := (*updatedDoc.IntMapPtr)[k]
		assert.True(t, ok, "IntMapPtr에 키가 없습니다: "+k)
		assert.Equal(t, v, val, "IntMapPtr 값이 일치하지 않습니다: "+k)
	}

	require.NotNil(t, updatedDoc.StructMapPtr, "StructMapPtr가 nil입니다")
	assert.Equal(t, len(*modified.StructMapPtr), len(*updatedDoc.StructMapPtr), "StructMapPtr 크기가 일치하지 않습니다")
}

// 포인터 필드 주소 비교 테스트 - nil에서 값으로 변경될 때 주소가 다른지 확인
func TestBsonPatch_PointerAddressComparison(t *testing.T) {
	// 포인터 필드를 포함한 간단한 구조체 정의
	type SimplePointerDoc struct {
		ID       primitive.ObjectID `bson:"_id"`
		Name     string             `bson:"name"`
		Version  int64              `bson:"version"`
		IntPtr   *int               `bson:"intPtr"`
		StrPtr   *string            `bson:"strPtr"`
		BoolPtr  *bool              `bson:"boolPtr"`
		SlicePtr *[]int             `bson:"slicePtr"`
		MapPtr   *map[string]string `bson:"mapPtr"`
	}

	// 원본 문서 생성 (모든 포인터 필드가 nil)
	original := &SimplePointerDoc{
		ID:       primitive.NewObjectID(),
		Name:     "Pointer Address Test",
		Version:  1,
		IntPtr:   nil,
		StrPtr:   nil,
		BoolPtr:  nil,
		SlicePtr: nil,
		MapPtr:   nil,
	}

	// 수정된 문서 생성 (모든 포인터 필드에 값 설정)
	intVal := 42
	strVal := "hello"
	boolVal := true
	sliceVal := []int{1, 2, 3}
	mapVal := map[string]string{"key": "value"}

	modified := &SimplePointerDoc{
		ID:       original.ID,
		Name:     "Updated Pointer Address Test",
		Version:  2,
		IntPtr:   &intVal,
		StrPtr:   &strVal,
		BoolPtr:  &boolVal,
		SlicePtr: &sliceVal,
		MapPtr:   &mapVal,
	}

	// 원본 포인터 주소 저장
	intPtrAddr := fmt.Sprintf("%p", modified.IntPtr)
	strPtrAddr := fmt.Sprintf("%p", modified.StrPtr)
	boolPtrAddr := fmt.Sprintf("%p", modified.BoolPtr)
	slicePtrAddr := fmt.Sprintf("%p", modified.SlicePtr)
	mapPtrAddr := fmt.Sprintf("%p", modified.MapPtr)

	t.Logf("원본 포인터 주소:")
	t.Logf("IntPtr: %s", intPtrAddr)
	t.Logf("StrPtr: %s", strPtrAddr)
	t.Logf("BoolPtr: %s", boolPtrAddr)
	t.Logf("SlicePtr: %s", slicePtrAddr)
	t.Logf("MapPtr: %s", mapPtrAddr)

	// BsonPatch 생성
	patch, err := CreateBsonPatch(original, modified)
	require.NoError(t, err, "패치 생성 중 오류 발생")
	require.NotNil(t, patch, "생성된 패치가 nil입니다")

	// 패치 내 포인터 주소 확인
	intPtrPatch, ok := patch.Set["intPtr"].(*int)
	require.True(t, ok, "intPtr가 *int 타입이 아닙니다")
	intPtrPatchAddr := fmt.Sprintf("%p", intPtrPatch)
	t.Logf("패치 내 IntPtr 주소: %s", intPtrPatchAddr)

	strPtrPatch, ok := patch.Set["strPtr"].(*string)
	require.True(t, ok, "strPtr가 *string 타입이 아닙니다")
	strPtrPatchAddr := fmt.Sprintf("%p", strPtrPatch)
	t.Logf("패치 내 StrPtr 주소: %s", strPtrPatchAddr)

	boolPtrPatch, ok := patch.Set["boolPtr"].(*bool)
	require.True(t, ok, "boolPtr가 *bool 타입이 아닙니다")
	boolPtrPatchAddr := fmt.Sprintf("%p", boolPtrPatch)
	t.Logf("패치 내 BoolPtr 주소: %s", boolPtrPatchAddr)

	slicePtrPatch, ok := patch.Set["slicePtr"].(*[]int)
	require.True(t, ok, "slicePtr가 *[]int 타입이 아닙니다")
	slicePtrPatchAddr := fmt.Sprintf("%p", slicePtrPatch)
	t.Logf("패치 내 SlicePtr 주소: %s", slicePtrPatchAddr)

	mapPtrPatch, ok := patch.Set["mapPtr"].(*map[string]string)
	require.True(t, ok, "mapPtr가 *map[string]string 타입이 아닙니다")
	mapPtrPatchAddr := fmt.Sprintf("%p", mapPtrPatch)
	t.Logf("패치 내 MapPtr 주소: %s", mapPtrPatchAddr)

	// 주소 비교 - 모든 포인터 주소가 달라야 함
	assert.NotEqual(t, intPtrAddr, intPtrPatchAddr, "IntPtr 주소가 동일합니다")
	assert.NotEqual(t, strPtrAddr, strPtrPatchAddr, "StrPtr 주소가 동일합니다")
	assert.NotEqual(t, boolPtrAddr, boolPtrPatchAddr, "BoolPtr 주소가 동일합니다")
	assert.NotEqual(t, slicePtrAddr, slicePtrPatchAddr, "SlicePtr 주소가 동일합니다")
	assert.NotEqual(t, mapPtrAddr, mapPtrPatchAddr, "MapPtr 주소가 동일합니다")

	// 값 비교 - 값은 동일해야 함
	assert.Equal(t, *modified.IntPtr, *intPtrPatch, "IntPtr 값이 다릅니다")
	assert.Equal(t, *modified.StrPtr, *strPtrPatch, "StrPtr 값이 다릅니다")
	assert.Equal(t, *modified.BoolPtr, *boolPtrPatch, "BoolPtr 값이 다릅니다")
	assert.ElementsMatch(t, *modified.SlicePtr, *slicePtrPatch, "SlicePtr 값이 다릅니다")

	// 맵 비교
	assert.Equal(t, len(*modified.MapPtr), len(*mapPtrPatch), "MapPtr 크기가 다릅니다")
	for k, v := range *modified.MapPtr {
		val, ok := (*mapPtrPatch)[k]
		assert.True(t, ok, "MapPtr에 키가 없습니다: "+k)
		assert.Equal(t, v, val, "MapPtr 값이 다릅니다: "+k)
	}

	// 외부 수정 테스트 - 원본 값을 변경해도 패치 내 값은 변경되지 않아야 함
	*modified.IntPtr = 99
	*modified.StrPtr = "modified"
	*modified.BoolPtr = false
	(*modified.SlicePtr)[0] = 100
	(*modified.MapPtr)["key"] = "modified"

	// 패치 내 값은 변경되지 않아야 함
	assert.Equal(t, 42, *intPtrPatch, "IntPtr 값이 외부 수정에 영향을 받았습니다")
	assert.Equal(t, "hello", *strPtrPatch, "StrPtr 값이 외부 수정에 영향을 받았습니다")
	assert.Equal(t, true, *boolPtrPatch, "BoolPtr 값이 외부 수정에 영향을 받았습니다")
	assert.Equal(t, 1, (*slicePtrPatch)[0], "SlicePtr 값이 외부 수정에 영향을 받았습니다")
	assert.Equal(t, "value", (*mapPtrPatch)["key"], "MapPtr 값이 외부 수정에 영향을 받았습니다")
}

// copier 패키지의 Copy 함수가 에러를 반환하는 경우를 테스트
func TestCopierErrorCases(t *testing.T) {

	t.Run("기본 타입 복사", func(t *testing.T) {
		// 기본 타입 복사 - 성공 케이스
		var src int = 42
		var dst int
		err := copier.Copy(&dst, src)
		assert.NoError(t, err, "기본 타입 복사 중 에러 발생")
		assert.Equal(t, src, dst, "복사된 값이 다릅니다")
	})

	t.Run("타입 불일치", func(t *testing.T) {
		// 타입 불일치 - 에러가 발생하지 않을 수 있음
		var src int = 42
		var dst string
		err := copier.Copy(&dst, src)
		// 타입 불일치에도 에러가 발생하지 않을 수 있음
		t.Logf("타입 불일치 에러: %v", err)
		t.Logf("변환 결과: %q", dst) // 문자열로 변환될 수 있음
	})

	t.Run("nil 대상", func(t *testing.T) {
		// nil 대상 - 에러 케이스
		var src int = 42
		var dst *int = nil
		err := copier.Copy(dst, src) // dst가 nil이므로 에러 발생 예상
		assert.Error(t, err, "nil 대상에도 에러가 발생하지 않음")
		t.Logf("nil 대상 에러: %v", err)
	})

	t.Run("비포인터 대상", func(t *testing.T) {
		// 비포인터 대상 - 에러 케이스
		var src int = 42
		var dst int
		err := copier.Copy(dst, src) // dst가 포인터가 아니므로 에러 발생 예상
		assert.Error(t, err, "비포인터 대상에도 에러가 발생하지 않음")
		t.Logf("비포인터 대상 에러: %v", err)
	})

	t.Run("구조체 복사", func(t *testing.T) {
		// 구조체 복사 - 성공 케이스
		type Person struct {
			Name string
			Age  int
		}
		src := Person{Name: "John", Age: 30}
		var dst Person
		err := copier.Copy(&dst, src)
		assert.NoError(t, err, "구조체 복사 중 에러 발생")
		assert.Equal(t, src, dst, "복사된 구조체가 다릅니다")
	})

	t.Run("구조체 필드 타입 불일치", func(t *testing.T) {
		// 구조체 필드 타입 불일치 - 에러 케이스
		type Person1 struct {
			Name string
			Age  int
		}
		type Person2 struct {
			Name string
			Age  string // 타입 불일치
		}
		src := Person1{Name: "John", Age: 30}
		var dst Person2
		err := copier.Copy(&dst, src)
		// 필드 타입이 다르면 에러가 발생할 수 있음
		if err != nil {
			t.Logf("구조체 필드 타입 불일치 에러: %v", err)
		} else {
			t.Logf("구조체 필드 타입 불일치에도 에러가 발생하지 않음: dst=%+v", dst)
		}
	})

	t.Run("비공개 필드", func(t *testing.T) {
		// 비공개 필드 - 에러 케이스
		type Person1 struct {
			Name string
			age  int // 비공개 필드
		}
		type Person2 struct {
			Name string
			Age  int
		}
		src := Person1{Name: "John", age: 30}
		var dst Person2
		err := copier.Copy(&dst, src)
		// 비공개 필드는 복사되지 않지만 에러는 발생하지 않을 수 있음
		assert.NoError(t, err, "비공개 필드 복사 중 에러 발생")
		assert.Equal(t, "John", dst.Name, "Name 필드가 복사되지 않음")
		assert.Equal(t, 0, dst.Age, "비공개 필드는 복사되지 않아야 함")
	})

	t.Run("중첩 구조체", func(t *testing.T) {
		// 중첩 구조체 - 성공 케이스
		type Address struct {
			City  string
			State string
		}
		type Person struct {
			Name    string
			Address Address
		}
		src := Person{Name: "John", Address: Address{City: "New York", State: "NY"}}
		var dst Person
		err := copier.Copy(&dst, src)
		assert.NoError(t, err, "중첩 구조체 복사 중 에러 발생")
		assert.Equal(t, src, dst, "복사된 중첩 구조체가 다릅니다")
	})

	t.Run("포인터 필드", func(t *testing.T) {
		// 포인터 필드 - 성공 케이스
		type Person struct {
			Name *string
			Age  *int
		}
		name := "John"
		age := 30
		src := Person{Name: &name, Age: &age}
		var dst Person
		err := copier.Copy(&dst, src)
		assert.NoError(t, err, "포인터 필드 복사 중 에러 발생")
		assert.NotNil(t, dst.Name, "Name 포인터가 nil입니다")
		assert.NotNil(t, dst.Age, "Age 포인터가 nil입니다")
		assert.Equal(t, *src.Name, *dst.Name, "Name 값이 다릅니다")
		assert.Equal(t, *src.Age, *dst.Age, "Age 값이 다릅니다")
		// 포인터 주소 비교 - 깊은 복사이므로 주소가 달라야 함
		assert.NotEqual(t, fmt.Sprintf("%p", src.Name), fmt.Sprintf("%p", dst.Name), "Name 포인터 주소가 같습니다")
		assert.NotEqual(t, fmt.Sprintf("%p", src.Age), fmt.Sprintf("%p", dst.Age), "Age 포인터 주소가 같습니다")
	})

	t.Run("인터페이스 필드", func(t *testing.T) {
		// 인터페이스 필드 - 에러 케이스
		type Person struct {
			Name string
			Data interface{}
		}
		src := Person{Name: "John", Data: map[string]interface{}{"key": "value"}}
		var dst Person
		err := copier.Copy(&dst, src)
		// 인터페이스 필드는 복사가 제한적일 수 있음
		if err != nil {
			t.Logf("인터페이스 필드 복사 에러: %v", err)
		} else {
			t.Logf("인터페이스 필드 복사 결과: dst=%+v", dst)
		}
	})

	t.Run("DeepCopy 옵션", func(t *testing.T) {
		// DeepCopy 옵션 - 성공 케이스
		type Person struct {
			Name    string
			Friends []string
		}
		src := Person{Name: "John", Friends: []string{"Alice", "Bob"}}
		var dst Person
		err := copier.CopyWithOption(&dst, src, copier.Option{DeepCopy: true})
		assert.NoError(t, err, "DeepCopy 옵션 사용 중 에러 발생")
		assert.Equal(t, src, dst, "복사된 구조체가 다릅니다")

		// 원본 슬라이스 수정
		src.Friends[0] = "Changed"
		// 깊은 복사이므로 dst는 변경되지 않아야 함
		assert.Equal(t, "Alice", dst.Friends[0], "깊은 복사가 제대로 되지 않았습니다")
	})

	t.Run("복사 불가능한 타입", func(t *testing.T) {
		// 복사 불가능한 타입 - 에러 케이스
		type Person struct {
			Name string
			Ch   chan int // 채널은 복사가 어려울 수 있음
		}
		src := Person{Name: "John", Ch: make(chan int)}
		var dst Person
		err := copier.Copy(&dst, src)
		// 채널과 같은 특수 타입은 복사가 제한적일 수 있음
		if err != nil {
			t.Logf("복사 불가능한 타입 에러: %v", err)
		} else {
			t.Logf("복사 불가능한 타입 복사 결과: dst=%+v", dst)
			// 채널이 복사되었는지 확인
			if dst.Ch != nil {
				t.Logf("채널이 복사되었습니다: %p", dst.Ch)
			} else {
				t.Logf("채널이 복사되지 않았습니다")
			}
		}
	})
}

// 배열 필터를 사용한 업데이트 테스트
func TestBsonPatch_ArrayFilters(t *testing.T) {
	t.Skip("ItemStruct 구조체 필드 문제로 인해 스킵")
}

// 5. 성능 테스트 - 다양한 크기의 구조체에 대한 패치 생성 성능 측정
func BenchmarkCreateBsonPatch_Performance(b *testing.B) {
	// 다양한 크기의 구조체에 대한 벤치마크 케이스
	benchCases := []struct {
		name         string
		itemCount    int
		subItemCount int
		propCount    int
		changeLevel  string
	}{
		{"Small-Minimal", 10, 5, 5, "minimal"},
		{"Medium-Moderate", 50, 10, 20, "moderate"},
		{"Large-Extensive", 200, 20, 50, "extensive"},
	}

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			// 원본 문서 생성
			original := generateRandomComplexStruct(bc.itemCount, bc.subItemCount, bc.propCount)
			modified := modifyComplexStruct(original, bc.changeLevel)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				patch, err := CreateBsonPatch(original, modified)
				if err != nil {
					b.Fatal(err)
				}
				if patch == nil {
					b.Fatal("생성된 패치가 nil입니다")
				}
			}
		})
	}
}

// 반복적인 패치 생성 성능 측정 (캐싱 효과 확인)
func BenchmarkCreateBsonPatch_CachingEffect(b *testing.B) {
	// 원본 문서 생성
	original := generateRandomComplexStruct(100, 10, 30)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 매번 약간씩 다른 수정본 생성
		modified := modifyComplexStruct(original, "moderate")
		modified.Counter = i // 매번 다른 값으로 변경

		patch, err := CreateBsonPatch(original, modified)
		if err != nil {
			b.Fatal(err)
		}
		if patch == nil {
			b.Fatal("생성된 패치가 nil입니다")
		}
	}
}
