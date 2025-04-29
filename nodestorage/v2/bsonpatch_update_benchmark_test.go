package v2

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// 벤치마크를 위한 테스트 문서 구조체
type BenchmarkDoc struct {
	ID          primitive.ObjectID         `bson:"_id"`
	Name        string                     `bson:"name"`
	Description string                     `bson:"description"`
	CreatedAt   time.Time                  `bson:"createdAt"`
	UpdatedAt   time.Time                  `bson:"updatedAt"`
	Version     int64                      `bson:"version"`
	Tags        []string                   `bson:"tags"`
	Metadata    map[string]string          `bson:"metadata"`
	Stats       map[string]int             `bson:"stats"`
	Settings    map[string]interface{}     `bson:"settings"`
	Nested      BenchmarkNestedDoc         `bson:"nested"`
	Items       []BenchmarkItem            `bson:"items"`
	Flags       map[string]bool            `bson:"flags"`
	Permissions map[string]map[string]bool `bson:"permissions"`
	History     []BenchmarkHistoryEntry    `bson:"history"`
}

// 중첩 문서 구조체
type BenchmarkNestedDoc struct {
	Title       string                 `bson:"title"`
	Description string                 `bson:"description"`
	Properties  map[string]interface{} `bson:"properties"`
	Status      string                 `bson:"status"`
	Priority    int                    `bson:"priority"`
}

// 아이템 구조체
type BenchmarkItem struct {
	ID       string            `bson:"id"`
	Name     string            `bson:"name"`
	Quantity int               `bson:"quantity"`
	Price    float64           `bson:"price"`
	Tags     []string          `bson:"tags"`
	Metadata map[string]string `bson:"metadata"`
}

// 히스토리 항목 구조체
type BenchmarkHistoryEntry struct {
	Timestamp time.Time              `bson:"timestamp"`
	Action    string                 `bson:"action"`
	User      string                 `bson:"user"`
	Changes   map[string]interface{} `bson:"changes"`
}

// 벤치마크 설정을 위한 MongoDB 연결 설정
func setupUpdateBenchmarkDB(b *testing.B) (*mongo.Client, *mongo.Collection, func()) {
	// MongoDB 연결 설정
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		b.Fatalf("MongoDB 연결 실패: %v", err)
	}

	// 테스트용 데이터베이스 및 컬렉션 설정
	db := client.Database("test_benchmark")
	collName := fmt.Sprintf("benchmark_%d", time.Now().UnixNano())
	collection := db.Collection(collName)

	// 정리 함수
	cleanup := func() {
		collection.Drop(ctx)
		client.Disconnect(ctx)
	}

	return client, collection, cleanup
}

// 벤치마크용 대형 문서 생성
func createLargeDocument() *BenchmarkDoc {
	doc := &BenchmarkDoc{
		ID:          primitive.NewObjectID(),
		Name:        "Benchmark Document",
		Description: "This is a large document for benchmarking purposes",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Version:     1,
		Tags:        []string{"benchmark", "test", "performance", "mongodb", "large"},
		Metadata: map[string]string{
			"source":      "benchmark",
			"environment": "test",
			"category":    "performance",
			"type":        "document",
		},
		Stats: map[string]int{
			"views":     1000,
			"likes":     500,
			"comments":  200,
			"shares":    100,
			"downloads": 50,
		},
		Settings: map[string]interface{}{
			"notifications": true,
			"privacy":       "public",
			"theme":         "dark",
			"fontSize":      14,
			"language":      "en",
			"autoSave":      true,
			"refreshRate":   60,
		},
		Nested: BenchmarkNestedDoc{
			Title:       "Nested Document",
			Description: "This is a nested document for testing nested fields",
			Properties: map[string]interface{}{
				"color":    "blue",
				"size":     "large",
				"material": "metal",
				"weight":   10.5,
				"height":   20,
				"width":    30,
				"depth":    15,
			},
			Status:   "active",
			Priority: 1,
		},
		Flags: map[string]bool{
			"isActive":    true,
			"isArchived":  false,
			"isPublished": true,
			"isDeleted":   false,
			"isLocked":    false,
			"isPrivate":   false,
			"isVerified":  true,
		},
		Permissions: map[string]map[string]bool{
			"admin": {
				"read":   true,
				"write":  true,
				"delete": true,
				"share":  true,
			},
			"user": {
				"read":   true,
				"write":  false,
				"delete": false,
				"share":  false,
			},
			"guest": {
				"read":   true,
				"write":  false,
				"delete": false,
				"share":  false,
			},
		},
	}

	// 아이템 추가
	for i := 0; i < 20; i++ {
		item := BenchmarkItem{
			ID:       fmt.Sprintf("item_%d", i),
			Name:     fmt.Sprintf("Item %d", i),
			Quantity: rand.Intn(100) + 1,
			Price:    float64(rand.Intn(10000)) / 100.0,
			Tags:     []string{fmt.Sprintf("tag%d", i%5), fmt.Sprintf("category%d", i%3)},
			Metadata: map[string]string{
				"color":    fmt.Sprintf("color%d", i%10),
				"size":     fmt.Sprintf("size%d", i%3),
				"material": fmt.Sprintf("material%d", i%5),
			},
		}
		doc.Items = append(doc.Items, item)
	}

	// 히스토리 항목 추가
	for i := 0; i < 10; i++ {
		historyEntry := BenchmarkHistoryEntry{
			Timestamp: time.Now().Add(-time.Duration(i) * time.Hour),
			Action:    fmt.Sprintf("action_%d", i),
			User:      fmt.Sprintf("user_%d", i%3),
			Changes: map[string]interface{}{
				"field1": fmt.Sprintf("value%d", i),
				"field2": i,
				"field3": i%2 == 0,
			},
		}
		doc.History = append(doc.History, historyEntry)
	}

	return doc
}

// 문서의 일부 필드만 수정한 새 문서 생성
func createModifiedDocument(original *BenchmarkDoc) *BenchmarkDoc {
	// 원본 문서 복사
	modified := *original

	// 일부 필드만 수정
	modified.Name = "Updated Benchmark Document"
	modified.Description = "This document has been updated for benchmarking"
	modified.UpdatedAt = time.Now()
	modified.Version = original.Version + 1

	// 태그 수정
	modified.Tags = append(modified.Tags, "updated")

	// 메타데이터 수정
	modified.Metadata["status"] = "updated"
	modified.Metadata["updatedBy"] = "benchmark"

	// 설정 수정
	modified.Settings["theme"] = "light"
	modified.Settings["fontSize"] = 16

	// 중첩 문서 수정
	modified.Nested.Status = "updated"
	modified.Nested.Priority = 2
	modified.Nested.Properties["color"] = "red"

	// 아이템 수정 (첫 번째 아이템만)
	if len(modified.Items) > 0 {
		modified.Items[0].Name = "Updated Item"
		modified.Items[0].Price = 999.99
	}

	// 플래그 수정
	modified.Flags["isUpdated"] = true

	return &modified
}

// BsonPatch를 사용한 부분 업데이트 벤치마크
func BenchmarkPartialUpdate_BsonPatch(b *testing.B) {
	// MongoDB 설정
	_, collection, cleanup := setupUpdateBenchmarkDB(b)
	defer cleanup()

	// 원본 문서 생성 및 삽입
	original := createLargeDocument()
	ctx := context.Background()
	_, err := collection.InsertOne(ctx, original)
	require.NoError(b, err, "문서 삽입 실패")

	// 수정된 문서 생성
	modified := createModifiedDocument(original)

	// 벤치마크 리셋
	b.ResetTimer()

	// 벤치마크 실행
	for i := 0; i < b.N; i++ {
		// BsonPatch 생성
		patch, err := CreateBsonPatch(original, modified)
		if err != nil {
			b.Fatalf("패치 생성 실패: %v", err)
		}

		// 패치를 MongoDB 업데이트 문서로 변환
		updateDoc, err := patch.MarshalBSON()
		if err != nil {
			b.Fatalf("패치 마샬링 실패: %v", err)
		}

		// MongoDB에 업데이트 적용
		filter := bson.M{"_id": original.ID}
		var updateBSON bson.M
		err = bson.Unmarshal(updateDoc, &updateBSON)
		if err != nil {
			b.Fatalf("BSON 언마샬링 실패: %v", err)
		}

		_, err = collection.UpdateOne(ctx, filter, updateBSON)
		if err != nil {
			b.Fatalf("MongoDB 업데이트 실패: %v", err)
		}
	}
}

// 전체 문서 업데이트 벤치마크
func BenchmarkFullUpdate(b *testing.B) {
	// MongoDB 설정
	_, collection, cleanup := setupUpdateBenchmarkDB(b)
	defer cleanup()

	// 원본 문서 생성 및 삽입
	original := createLargeDocument()
	ctx := context.Background()
	_, err := collection.InsertOne(ctx, original)
	require.NoError(b, err, "문서 삽입 실패")

	// 수정된 문서 생성
	modified := createModifiedDocument(original)

	// 벤치마크 리셋
	b.ResetTimer()

	// 벤치마크 실행
	for i := 0; i < b.N; i++ {
		// MongoDB에 전체 문서 업데이트 적용
		filter := bson.M{"_id": original.ID}
		update := bson.M{"$set": modified}

		_, err = collection.UpdateOne(ctx, filter, update)
		if err != nil {
			b.Fatalf("MongoDB 업데이트 실패: %v", err)
		}
	}
}

// 부분 업데이트 직접 구성 벤치마크 (BsonPatch 없이)
func BenchmarkPartialUpdate_Manual(b *testing.B) {
	// MongoDB 설정
	_, collection, cleanup := setupUpdateBenchmarkDB(b)
	defer cleanup()

	// 원본 문서 생성 및 삽입
	original := createLargeDocument()
	ctx := context.Background()
	_, err := collection.InsertOne(ctx, original)
	require.NoError(b, err, "문서 삽입 실패")

	// 수정된 문서 생성
	modified := createModifiedDocument(original)

	// 벤치마크 리셋
	b.ResetTimer()

	// 벤치마크 실행
	for i := 0; i < b.N; i++ {
		// 수동으로 업데이트 문서 구성
		update := bson.M{
			"$set": bson.M{
				"name":                    modified.Name,
				"description":             modified.Description,
				"updatedAt":               modified.UpdatedAt,
				"version":                 modified.Version,
				"tags":                    modified.Tags,
				"metadata.status":         "updated",
				"metadata.updatedBy":      "benchmark",
				"settings.theme":          "light",
				"settings.fontSize":       16,
				"nested.status":           "updated",
				"nested.priority":         2,
				"nested.properties.color": "red",
				"items.0.name":            "Updated Item",
				"items.0.price":           999.99,
				"flags.isUpdated":         true,
			},
		}

		// MongoDB에 업데이트 적용
		filter := bson.M{"_id": original.ID}
		_, err = collection.UpdateOne(ctx, filter, update)
		if err != nil {
			b.Fatalf("MongoDB 업데이트 실패: %v", err)
		}
	}
}

// 다양한 크기의 문서에 대한 벤치마크
func BenchmarkUpdateByDocumentSize(b *testing.B) {
	// 다양한 크기의 문서에 대한 벤치마크
	sizes := []struct {
		name         string
		itemCount    int
		historyCount int
	}{
		{"Small", 5, 2},
		{"Medium", 20, 10},
		{"Large", 100, 50},
		{"XLarge", 500, 200},
	}

	for _, size := range sizes {
		// BsonPatch 벤치마크
		b.Run(fmt.Sprintf("BsonPatch_%s", size.name), func(b *testing.B) {
			_, collection, cleanup := setupUpdateBenchmarkDB(b)
			defer cleanup()

			// 원본 문서 생성 및 삽입
			original := createLargeDocument()

			// 크기 조정 - 안전하게 처리
			if len(original.Items) > size.itemCount {
				original.Items = original.Items[:size.itemCount]
			} else {
				// 필요한 크기만큼 아이템 추가
				for i := len(original.Items); i < size.itemCount; i++ {
					item := BenchmarkItem{
						ID:       fmt.Sprintf("item_%d", i),
						Name:     fmt.Sprintf("Item %d", i),
						Quantity: rand.Intn(100) + 1,
						Price:    float64(rand.Intn(10000)) / 100.0,
						Tags:     []string{fmt.Sprintf("tag%d", i%5), fmt.Sprintf("category%d", i%3)},
						Metadata: map[string]string{
							"color":    fmt.Sprintf("color%d", i%10),
							"size":     fmt.Sprintf("size%d", i%3),
							"material": fmt.Sprintf("material%d", i%5),
						},
					}
					original.Items = append(original.Items, item)
				}
			}

			if len(original.History) > size.historyCount {
				original.History = original.History[:size.historyCount]
			} else {
				// 필요한 크기만큼 히스토리 항목 추가
				for i := len(original.History); i < size.historyCount; i++ {
					historyEntry := BenchmarkHistoryEntry{
						Timestamp: time.Now().Add(-time.Duration(i) * time.Hour),
						Action:    fmt.Sprintf("action_%d", i),
						User:      fmt.Sprintf("user_%d", i%3),
						Changes: map[string]interface{}{
							"field1": fmt.Sprintf("value%d", i),
							"field2": i,
							"field3": i%2 == 0,
						},
					}
					original.History = append(original.History, historyEntry)
				}
			}

			ctx := context.Background()
			_, err := collection.InsertOne(ctx, original)
			require.NoError(b, err, "문서 삽입 실패")

			// 수정된 문서 생성
			modified := createModifiedDocument(original)

			// 벤치마크 리셋
			b.ResetTimer()

			// 벤치마크 실행
			for i := 0; i < b.N; i++ {
				patch, _ := CreateBsonPatch(original, modified)
				updateDoc, _ := patch.MarshalBSON()
				filter := bson.M{"_id": original.ID}
				var updateBSON bson.M
				bson.Unmarshal(updateDoc, &updateBSON)
				collection.UpdateOne(ctx, filter, updateBSON)
			}
		})

		// 전체 업데이트 벤치마크
		b.Run(fmt.Sprintf("FullUpdate_%s", size.name), func(b *testing.B) {
			_, collection, cleanup := setupUpdateBenchmarkDB(b)
			defer cleanup()

			// 원본 문서 생성 및 삽입
			original := createLargeDocument()

			// 크기 조정 - 안전하게 처리
			if len(original.Items) > size.itemCount {
				original.Items = original.Items[:size.itemCount]
			} else {
				// 필요한 크기만큼 아이템 추가
				for i := len(original.Items); i < size.itemCount; i++ {
					item := BenchmarkItem{
						ID:       fmt.Sprintf("item_%d", i),
						Name:     fmt.Sprintf("Item %d", i),
						Quantity: rand.Intn(100) + 1,
						Price:    float64(rand.Intn(10000)) / 100.0,
						Tags:     []string{fmt.Sprintf("tag%d", i%5), fmt.Sprintf("category%d", i%3)},
						Metadata: map[string]string{
							"color":    fmt.Sprintf("color%d", i%10),
							"size":     fmt.Sprintf("size%d", i%3),
							"material": fmt.Sprintf("material%d", i%5),
						},
					}
					original.Items = append(original.Items, item)
				}
			}

			if len(original.History) > size.historyCount {
				original.History = original.History[:size.historyCount]
			} else {
				// 필요한 크기만큼 히스토리 항목 추가
				for i := len(original.History); i < size.historyCount; i++ {
					historyEntry := BenchmarkHistoryEntry{
						Timestamp: time.Now().Add(-time.Duration(i) * time.Hour),
						Action:    fmt.Sprintf("action_%d", i),
						User:      fmt.Sprintf("user_%d", i%3),
						Changes: map[string]interface{}{
							"field1": fmt.Sprintf("value%d", i),
							"field2": i,
							"field3": i%2 == 0,
						},
					}
					original.History = append(original.History, historyEntry)
				}
			}

			ctx := context.Background()
			_, err := collection.InsertOne(ctx, original)
			require.NoError(b, err, "문서 삽입 실패")

			// 수정된 문서 생성
			modified := createModifiedDocument(original)

			// 벤치마크 리셋
			b.ResetTimer()

			// 벤치마크 실행
			for i := 0; i < b.N; i++ {
				filter := bson.M{"_id": original.ID}
				update := bson.M{"$set": modified}
				collection.UpdateOne(ctx, filter, update)
			}
		})

		// 수동 부분 업데이트 벤치마크
		b.Run(fmt.Sprintf("ManualUpdate_%s", size.name), func(b *testing.B) {
			_, collection, cleanup := setupUpdateBenchmarkDB(b)
			defer cleanup()

			// 원본 문서 생성 및 삽입
			original := createLargeDocument()

			// 크기 조정 - 안전하게 처리
			if len(original.Items) > size.itemCount {
				original.Items = original.Items[:size.itemCount]
			} else {
				// 필요한 크기만큼 아이템 추가
				for i := len(original.Items); i < size.itemCount; i++ {
					item := BenchmarkItem{
						ID:       fmt.Sprintf("item_%d", i),
						Name:     fmt.Sprintf("Item %d", i),
						Quantity: rand.Intn(100) + 1,
						Price:    float64(rand.Intn(10000)) / 100.0,
						Tags:     []string{fmt.Sprintf("tag%d", i%5), fmt.Sprintf("category%d", i%3)},
						Metadata: map[string]string{
							"color":    fmt.Sprintf("color%d", i%10),
							"size":     fmt.Sprintf("size%d", i%3),
							"material": fmt.Sprintf("material%d", i%5),
						},
					}
					original.Items = append(original.Items, item)
				}
			}

			if len(original.History) > size.historyCount {
				original.History = original.History[:size.historyCount]
			} else {
				// 필요한 크기만큼 히스토리 항목 추가
				for i := len(original.History); i < size.historyCount; i++ {
					historyEntry := BenchmarkHistoryEntry{
						Timestamp: time.Now().Add(-time.Duration(i) * time.Hour),
						Action:    fmt.Sprintf("action_%d", i),
						User:      fmt.Sprintf("user_%d", i%3),
						Changes: map[string]interface{}{
							"field1": fmt.Sprintf("value%d", i),
							"field2": i,
							"field3": i%2 == 0,
						},
					}
					original.History = append(original.History, historyEntry)
				}
			}

			ctx := context.Background()
			_, err := collection.InsertOne(ctx, original)
			require.NoError(b, err, "문서 삽입 실패")

			// 수정된 문서 생성
			modified := createModifiedDocument(original)

			// 벤치마크 리셋
			b.ResetTimer()

			// 벤치마크 실행
			for i := 0; i < b.N; i++ {
				update := bson.M{
					"$set": bson.M{
						"name":                    modified.Name,
						"description":             modified.Description,
						"updatedAt":               modified.UpdatedAt,
						"version":                 modified.Version,
						"tags":                    modified.Tags,
						"metadata.status":         "updated",
						"metadata.updatedBy":      "benchmark",
						"settings.theme":          "light",
						"settings.fontSize":       16,
						"nested.status":           "updated",
						"nested.priority":         2,
						"nested.properties.color": "red",
						"items.0.name":            "Updated Item",
						"items.0.price":           999.99,
						"flags.isUpdated":         true,
					},
				}

				filter := bson.M{"_id": original.ID}
				collection.UpdateOne(ctx, filter, update)
			}
		})
	}
}
