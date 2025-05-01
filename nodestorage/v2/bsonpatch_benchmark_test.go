package nodestorage

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 벤치마크를 위한 복잡한 중첩 구조체 정의
type ComplexNestedStruct struct {
	ID          primitive.ObjectID `bson:"_id"`
	Name        string             `bson:"name"`
	Description string             `bson:"description"`
	CreatedAt   time.Time          `bson:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt"`
	Version     int64              `bson:"version"`
	IsActive    bool               `bson:"isActive"`
	Tags        []string           `bson:"tags"`
	Metadata    map[string]string  `bson:"metadata"`
	Counter     int                `bson:"counter"`
	Score       float64            `bson:"score"`

	// 중첩 구조체
	Config *ConfigStruct `bson:"config"`

	// 구조체 배열
	Items []*ItemStruct `bson:"items"`

	// 구조체 맵
	Properties map[string]*PropertyStruct `bson:"properties"`

	// 복잡한 중첩 배열
	Matrix [][]int `bson:"matrix"`

	// 복잡한 중첩 맵
	NestedMaps map[string]map[string]interface{} `bson:"nestedMaps"`
}

type ConfigStruct struct {
	Enabled   bool                   `bson:"enabled"`
	MaxItems  int                    `bson:"maxItems"`
	Threshold float64                `bson:"threshold"`
	Options   []string               `bson:"options"`
	Settings  map[string]interface{} `bson:"settings"`
}

type ItemStruct struct {
	ID         string            `bson:"id"`
	Name       string            `bson:"name"`
	Value      float64           `bson:"value"`
	Timestamp  time.Time         `bson:"timestamp"`
	Tags       []string          `bson:"tags"`
	Attributes map[string]string `bson:"attributes"`
	SubItems   []*SubItemStruct  `bson:"subItems"`
}

type SubItemStruct struct {
	ID        string `bson:"id"`
	Name      string `bson:"name"`
	Value     int    `bson:"value"`
	IsEnabled bool   `bson:"isEnabled"`
}

type PropertyStruct struct {
	Key        string            `bson:"key"`
	Value      interface{}       `bson:"value"`
	Type       string            `bson:"type"`
	IsRequired bool              `bson:"isRequired"`
	Validators []ValidatorStruct `bson:"validators"`
}

type ValidatorStruct struct {
	Type    string                 `bson:"type"`
	Message string                 `bson:"message"`
	Params  map[string]interface{} `bson:"params"`
}

// 랜덤 문자열 생성 함수
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// 랜덤 구조체 생성 함수
func generateRandomComplexStruct(itemCount, subItemCount, propertyCount int) *ComplexNestedStruct {
	// 기본 구조체 생성
	doc := &ComplexNestedStruct{
		ID:          primitive.NewObjectID(),
		Name:        randomString(10),
		Description: randomString(50),
		CreatedAt:   time.Now().Add(-24 * time.Hour),
		UpdatedAt:   time.Now(),
		Version:     1,
		IsActive:    rand.Intn(2) == 1,
		Tags:        make([]string, 0, 5),
		Metadata:    make(map[string]string),
		Counter:     rand.Intn(1000),
		Score:       rand.Float64() * 100,
		Config: &ConfigStruct{
			Enabled:   rand.Intn(2) == 1,
			MaxItems:  rand.Intn(100),
			Threshold: rand.Float64() * 10,
			Options:   make([]string, 0, 3),
			Settings:  make(map[string]interface{}),
		},
		Items:      make([]*ItemStruct, 0, itemCount),
		Properties: make(map[string]*PropertyStruct),
		Matrix:     make([][]int, 5),
		NestedMaps: make(map[string]map[string]interface{}),
	}

	// 태그 추가
	for i := 0; i < 5; i++ {
		doc.Tags = append(doc.Tags, randomString(8))
	}

	// 메타데이터 추가
	for i := 0; i < 10; i++ {
		doc.Metadata[randomString(8)] = randomString(15)
	}

	// Config 옵션 추가
	for i := 0; i < 3; i++ {
		doc.Config.Options = append(doc.Config.Options, randomString(8))
	}

	// Config 설정 추가
	for i := 0; i < 5; i++ {
		key := randomString(8)
		switch rand.Intn(3) {
		case 0:
			doc.Config.Settings[key] = rand.Intn(100)
		case 1:
			doc.Config.Settings[key] = rand.Float64() * 100
		case 2:
			doc.Config.Settings[key] = randomString(10)
		}
	}

	// 아이템 추가
	for i := 0; i < itemCount; i++ {
		item := &ItemStruct{
			ID:         fmt.Sprintf("item-%d", i),
			Name:       fmt.Sprintf("Item %d", i),
			Value:      rand.Float64() * 1000,
			Timestamp:  time.Now().Add(time.Duration(rand.Intn(100)) * time.Hour),
			Tags:       make([]string, 0, 3),
			Attributes: make(map[string]string),
			SubItems:   make([]*SubItemStruct, 0, subItemCount),
		}

		// 아이템 태그 추가
		for j := 0; j < 3; j++ {
			item.Tags = append(item.Tags, randomString(6))
		}

		// 아이템 속성 추가
		for j := 0; j < 5; j++ {
			item.Attributes[randomString(6)] = randomString(10)
		}

		// 서브아이템 추가
		for j := 0; j < subItemCount; j++ {
			subItem := &SubItemStruct{
				ID:        fmt.Sprintf("subitem-%d-%d", i, j),
				Name:      fmt.Sprintf("SubItem %d.%d", i, j),
				Value:     rand.Intn(500),
				IsEnabled: rand.Intn(2) == 1,
			}
			item.SubItems = append(item.SubItems, subItem)
		}

		doc.Items = append(doc.Items, item)
	}

	// 속성 추가
	for i := 0; i < propertyCount; i++ {
		key := fmt.Sprintf("prop-%d", i)
		prop := &PropertyStruct{
			Key:        key,
			Value:      randomString(10),
			Type:       "string",
			IsRequired: rand.Intn(2) == 1,
			Validators: make([]ValidatorStruct, 0, 2),
		}

		// 검증기 추가
		for j := 0; j < 2; j++ {
			validator := ValidatorStruct{
				Type:    fmt.Sprintf("validator-%d", j),
				Message: fmt.Sprintf("Validation message %d", j),
				Params:  make(map[string]interface{}),
			}

			// 검증기 매개변수 추가
			for k := 0; k < 3; k++ {
				paramKey := randomString(5)
				switch rand.Intn(3) {
				case 0:
					validator.Params[paramKey] = rand.Intn(100)
				case 1:
					validator.Params[paramKey] = rand.Float64() * 100
				case 2:
					validator.Params[paramKey] = randomString(8)
				}
			}

			prop.Validators = append(prop.Validators, validator)
		}

		doc.Properties[key] = prop
	}

	// 행렬 추가
	for i := 0; i < 5; i++ {
		doc.Matrix[i] = make([]int, 5)
		for j := 0; j < 5; j++ {
			doc.Matrix[i][j] = rand.Intn(100)
		}
	}

	// 중첩 맵 추가
	for i := 0; i < 5; i++ {
		key := randomString(8)
		doc.NestedMaps[key] = make(map[string]interface{})

		for j := 0; j < 5; j++ {
			nestedKey := randomString(6)
			switch rand.Intn(3) {
			case 0:
				doc.NestedMaps[key][nestedKey] = rand.Intn(100)
			case 1:
				doc.NestedMaps[key][nestedKey] = rand.Float64() * 100
			case 2:
				doc.NestedMaps[key][nestedKey] = randomString(10)
			}
		}
	}

	return doc
}

// 구조체 변경 함수 - 일부 필드만 변경
func modifyComplexStruct(doc *ComplexNestedStruct, changeLevel string) *ComplexNestedStruct {
	// 원본 복사
	modified := *doc

	// 기본 필드 변경
	modified.Name = "Modified " + doc.Name
	modified.Description = "Updated: " + doc.Description
	modified.UpdatedAt = time.Now()
	modified.Version = doc.Version + 1
	modified.Counter = doc.Counter + 10
	modified.Score = doc.Score * 1.1

	// 변경 수준에 따라 추가 변경
	switch changeLevel {
	case "minimal":
		// 최소 변경 - 기본 필드만 변경

	case "moderate":
		// 중간 수준 변경 - 기본 필드 + 배열/맵 일부 변경
		if len(modified.Tags) > 0 {
			modified.Tags[0] = "modified-tag"
		}
		modified.Metadata["new-key"] = "new-value"

		// Config 변경
		modifiedConfig := *doc.Config
		modifiedConfig.MaxItems += 5
		modifiedConfig.Threshold *= 1.2
		modified.Config = &modifiedConfig

		// 일부 아이템 변경
		if len(modified.Items) > 0 {
			modifiedItems := make([]*ItemStruct, len(doc.Items))
			for i, item := range doc.Items {
				itemCopy := *item
				if i == 0 {
					itemCopy.Name = "Modified " + item.Name
					itemCopy.Value *= 1.5
				}
				modifiedItems[i] = &itemCopy
			}
			modified.Items = modifiedItems
		}

	case "extensive":
		// 광범위한 변경 - 깊은 중첩 구조까지 변경
		modified.Tags = append(modified.Tags, "new-tag-1", "new-tag-2")
		modified.Metadata["new-key-1"] = "new-value-1"
		modified.Metadata["new-key-2"] = "new-value-2"

		// Config 변경
		modifiedConfig := *doc.Config
		modifiedConfig.MaxItems += 10
		modifiedConfig.Threshold *= 1.5
		modifiedConfig.Options = append(modifiedConfig.Options, "new-option")
		modifiedConfig.Settings["new-setting"] = "new-value"
		modified.Config = &modifiedConfig

		// 아이템 변경 및 추가
		modifiedItems := make([]*ItemStruct, len(doc.Items)+1)
		for i, item := range doc.Items {
			itemCopy := *item
			itemCopy.Name = "Modified " + item.Name
			itemCopy.Value *= 1.5

			// 서브아이템 변경
			modifiedSubItems := make([]*SubItemStruct, len(item.SubItems))
			for j, subItem := range item.SubItems {
				subItemCopy := *subItem
				subItemCopy.Name = "Modified " + subItem.Name
				subItemCopy.Value += 100
				modifiedSubItems[j] = &subItemCopy
			}

			// 새 서브아이템 추가
			newSubItem := &SubItemStruct{
				ID:        "new-subitem",
				Name:      "New SubItem",
				Value:     999,
				IsEnabled: true,
			}
			modifiedSubItems = append(modifiedSubItems, newSubItem)

			itemCopy.SubItems = modifiedSubItems
			modifiedItems[i] = &itemCopy
		}

		// 새 아이템 추가
		newItem := &ItemStruct{
			ID:         "new-item",
			Name:       "New Item",
			Value:      1000.0,
			Timestamp:  time.Now(),
			Tags:       []string{"new", "item", "tags"},
			Attributes: map[string]string{"attr1": "val1", "attr2": "val2"},
			SubItems:   []*SubItemStruct{},
		}
		modifiedItems[len(doc.Items)] = newItem
		modified.Items = modifiedItems

		// 속성 변경 및 추가
		modifiedProperties := make(map[string]*PropertyStruct)
		for key, prop := range doc.Properties {
			propCopy := *prop
			propCopy.IsRequired = !prop.IsRequired
			modifiedProperties[key] = &propCopy
		}
		modifiedProperties["new-property"] = &PropertyStruct{
			Key:        "new-property",
			Value:      "new-value",
			Type:       "string",
			IsRequired: true,
			Validators: []ValidatorStruct{
				{
					Type:    "length",
					Message: "Length validation",
					Params:  map[string]interface{}{"min": 5, "max": 20},
				},
			},
		}
		modified.Properties = modifiedProperties

		// 행렬 변경
		for i := 0; i < len(modified.Matrix); i++ {
			for j := 0; j < len(modified.Matrix[i]); j++ {
				modified.Matrix[i][j] += 10
			}
		}

		// 중첩 맵 변경
		for key := range modified.NestedMaps {
			modified.NestedMaps[key]["new-nested-key"] = "new-nested-value"
		}
		modified.NestedMaps["new-map"] = map[string]interface{}{
			"key1": "value1",
			"key2": 123,
			"key3": true,
		}
	}

	return &modified
}

// 벤치마크 함수 - 작은 구조체 (10개 아이템, 5개 서브아이템, 5개 속성)
func BenchmarkCreateBsonPatch_SmallStruct(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	original := generateRandomComplexStruct(10, 5, 5)
	modified := modifyComplexStruct(original, "minimal")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CreateBsonPatch(original, modified)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCreateBsonPatchBad_SmallStruct(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	original := generateRandomComplexStruct(10, 5, 5)
	modified := modifyComplexStruct(original, "minimal")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CreateBsonPatchBad(original, modified)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// 벤치마크 함수 - 중간 크기 구조체 (50개 아이템, 10개 서브아이템, 20개 속성)
func BenchmarkCreateBsonPatch_MediumStruct(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	original := generateRandomComplexStruct(50, 10, 20)
	modified := modifyComplexStruct(original, "moderate")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CreateBsonPatch(original, modified)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCreateBsonPatchBad_MediumStruct(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	original := generateRandomComplexStruct(50, 10, 20)
	modified := modifyComplexStruct(original, "moderate")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CreateBsonPatchBad(original, modified)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// 벤치마크 함수 - 대형 구조체 (200개 아이템, 20개 서브아이템, 50개 속성)
func BenchmarkCreateBsonPatch_LargeStruct(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	original := generateRandomComplexStruct(200, 20, 50)
	modified := modifyComplexStruct(original, "extensive")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CreateBsonPatch(original, modified)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCreateBsonPatchBad_LargeStruct(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	original := generateRandomComplexStruct(200, 20, 50)
	modified := modifyComplexStruct(original, "extensive")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CreateBsonPatchBad(original, modified)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// 벤치마크 함수 - 초대형 구조체 (500개 아이템, 50개 서브아이템, 100개 속성)
func BenchmarkCreateBsonPatch_XLargeStruct(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	original := generateRandomComplexStruct(500, 50, 100)
	modified := modifyComplexStruct(original, "extensive")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CreateBsonPatch(original, modified)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCreateBsonPatchBad_XLargeStruct(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	original := generateRandomComplexStruct(500, 50, 100)
	modified := modifyComplexStruct(original, "extensive")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := CreateBsonPatchBad(original, modified)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// 벤치마크 함수 - 반복 호출 (캐싱 효과 측정)
func BenchmarkCreateBsonPatch_RepeatedCalls(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	original := generateRandomComplexStruct(100, 10, 30)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 매번 약간씩 다른 수정본 생성
		modified := modifyComplexStruct(original, "moderate")
		modified.Counter = i // 매번 다른 값으로 변경

		_, err := CreateBsonPatch(original, modified)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCreateBsonPatchBad_RepeatedCalls(b *testing.B) {
	rand.Seed(time.Now().UnixNano())
	original := generateRandomComplexStruct(100, 10, 30)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 매번 약간씩 다른 수정본 생성
		modified := modifyComplexStruct(original, "moderate")
		modified.Counter = i // 매번 다른 값으로 변경

		_, err := CreateBsonPatchBad(original, modified)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// 벤치마크 함수 - 동일한 타입의 다른 인스턴스 (타입 캐싱 효과 측정)
func BenchmarkCreateBsonPatch_DifferentInstances(b *testing.B) {
	rand.Seed(time.Now().UnixNano())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 매번 새로운 인스턴스 생성
		original := generateRandomComplexStruct(20, 5, 10)
		modified := modifyComplexStruct(original, "minimal")

		_, err := CreateBsonPatch(original, modified)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCreateBsonPatchBad_DifferentInstances(b *testing.B) {
	rand.Seed(time.Now().UnixNano())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 매번 새로운 인스턴스 생성
		original := generateRandomComplexStruct(20, 5, 10)
		modified := modifyComplexStruct(original, "minimal")

		_, err := CreateBsonPatchBad(original, modified)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// 벤치마크 함수 - 정확성 테스트
func TestBsonPatchAccuracy(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	// 다양한 크기의 구조체로 테스트
	testCases := []struct {
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			original := generateRandomComplexStruct(tc.itemCount, tc.subItemCount, tc.propCount)
			modified := modifyComplexStruct(original, tc.changeLevel)

			// 두 방식으로 패치 생성
			patch1, err := CreateBsonPatch(original, modified)
			if err != nil {
				t.Fatalf("CreateBsonPatch failed: %v", err)
			}

			patch2, err := CreateBsonPatchBad(original, modified)
			if err != nil {
				t.Fatalf("CreateBsonPatchBad failed: %v", err)
			}

			// 패치 내용 비교 (간단한 비교)
			if (patch1 == nil) != (patch2 == nil) {
				t.Errorf("Patch nil status differs: patch1=%v, patch2=%v", patch1 == nil, patch2 == nil)
			}

			if patch1 != nil && patch2 != nil {
				// Set 필드 개수 비교
				if len(patch1.Set) != len(patch2.Set) {
					t.Logf("Set field count differs: patch1=%d, patch2=%d", len(patch1.Set), len(patch2.Set))
				}

				// Unset 필드 개수 비교
				if len(patch1.Unset) != len(patch2.Unset) {
					t.Logf("Unset field count differs: patch1=%d, patch2=%d", len(patch1.Unset), len(patch2.Unset))
				}

				// 추가 필드 비교 (Bad에만 있는 필드)
				if len(patch2.Push) > 0 || len(patch2.Pull) > 0 || len(patch2.AddToSet) > 0 || len(patch2.PullAll) > 0 {
					t.Logf("Bad has additional array operations: Push=%d, Pull=%d, AddToSet=%d, PullAll=%d",
						len(patch2.Push), len(patch2.Pull), len(patch2.AddToSet), len(patch2.PullAll))
				}
			}
		})
	}
}
