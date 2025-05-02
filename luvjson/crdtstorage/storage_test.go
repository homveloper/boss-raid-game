package crdtstorage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
)

// TestDocument는 테스트용 문서 구조체입니다.
type TestDocument struct {
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Authors  []string `json:"authors"`
	Modified string   `json:"modified"`
}

// TestStorage_CreateDocument는 문서 생성을 테스트합니다.
func TestStorage_CreateDocument(t *testing.T) {
	// 컨텍스트 생성
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 저장소 옵션 생성
	options := DefaultStorageOptions()
	options.PubSubType = "memory"
	options.PersistenceType = "memory"

	// 저장소 생성
	storage, err := NewStorage(ctx, options)
	assert.NoError(t, err)
	defer storage.Close()

	// 문서 생성
	doc, err := storage.CreateDocument(ctx, "test-doc")
	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, "test-doc", doc.ID)

	// 초기 문서 내용 설정
	result := doc.Edit(ctx, func(crdtDoc *crdt.Document, patchBuilder *crdtpatch.PatchBuilder) error {
		// 루트 노드 생성
		rootID := crdtDoc.NextTimestamp()
		rootOp := &crdtpatch.NewOperation{
			ID:       rootID,
			NodeType: common.NodeTypeCon,
			Value: map[string]interface{}{
				"title":    "Test Document",
				"content":  "Test content",
				"authors":  []string{"tester"},
				"modified": time.Now().Format(time.RFC3339),
			},
		}

		// 패치 생성
		patchBuilder.AddOperation(rootOp)

		// 루트 설정 작업 추가
		rootSetOp := &crdtpatch.InsOperation{
			ID:       crdtDoc.NextTimestamp(),
			TargetID: common.RootID,
			Value:    rootID,
		}
		patchBuilder.AddOperation(rootSetOp)

		return nil
	})
	assert.True(t, result.Success)
	assert.Nil(t, result.Error)

	// 문서 내용 확인
	var content TestDocument
	err = doc.GetContentAs(&content)
	assert.NoError(t, err)
	assert.Equal(t, "Test Document", content.Title)
	assert.Equal(t, "Test content", content.Content)
	assert.Equal(t, []string{"tester"}, content.Authors)
}

// TestStorage_GetDocument는 문서 로드를 테스트합니다.
func TestStorage_GetDocument(t *testing.T) {
	// 컨텍스트 생성
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 저장소 옵션 생성
	options := DefaultStorageOptions()
	options.PubSubType = "memory"
	options.PersistenceType = "memory"

	// 저장소 생성
	storage, err := NewStorage(ctx, options)
	assert.NoError(t, err)
	defer storage.Close()

	// 문서 생성
	doc1, err := storage.CreateDocument(ctx, "test-doc")
	assert.NoError(t, err)

	// 초기 문서 내용 설정
	result := doc1.Edit(ctx, func(crdtDoc *crdt.Document, patchBuilder *crdtpatch.PatchBuilder) error {
		// 루트 노드 생성
		rootID := crdtDoc.NextTimestamp()
		rootOp := &crdtpatch.NewOperation{
			ID:       rootID,
			NodeType: common.NodeTypeCon,
			Value: map[string]interface{}{
				"title":    "Test Document",
				"content":  "Test content",
				"authors":  []string{"tester"},
				"modified": time.Now().Format(time.RFC3339),
			},
		}

		// 패치 생성
		patchBuilder.AddOperation(rootOp)

		// 루트 설정 작업 추가
		rootSetOp := &crdtpatch.InsOperation{
			ID:       crdtDoc.NextTimestamp(),
			TargetID: common.RootID,
			Value:    rootID,
		}
		patchBuilder.AddOperation(rootSetOp)

		return nil
	})
	assert.True(t, result.Success)

	// 문서 저장
	err = doc1.Save(ctx)
	assert.NoError(t, err)

	// 문서 로드
	doc2, err := storage.GetDocument(ctx, "test-doc")
	assert.NoError(t, err)
	assert.NotNil(t, doc2)
	assert.Equal(t, "test-doc", doc2.ID)

	// 문서 내용 확인
	var content TestDocument
	err = doc2.GetContentAs(&content)
	assert.NoError(t, err)
	assert.Equal(t, "Test Document", content.Title)
	assert.Equal(t, "Test content", content.Content)
	assert.Equal(t, []string{"tester"}, content.Authors)
}

// TestStorage_DeleteDocument는 문서 삭제를 테스트합니다.
func TestStorage_DeleteDocument(t *testing.T) {
	// 컨텍스트 생성
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 저장소 옵션 생성
	options := DefaultStorageOptions()
	options.PubSubType = "memory"
	options.PersistenceType = "memory"

	// 저장소 생성
	storage, err := NewStorage(ctx, options)
	assert.NoError(t, err)
	defer storage.Close()

	// 문서 생성
	doc, err := storage.CreateDocument(ctx, "test-doc")
	assert.NoError(t, err)

	// 초기 문서 내용 설정
	result := doc.Edit(ctx, func(crdtDoc *crdt.Document, patchBuilder *crdtpatch.PatchBuilder) error {
		// 루트 노드 생성
		rootID := crdtDoc.NextTimestamp()
		rootOp := &crdtpatch.NewOperation{
			ID:       rootID,
			NodeType: common.NodeTypeCon,
			Value: map[string]interface{}{
				"title":    "Test Document",
				"content":  "Test content",
				"authors":  []string{"tester"},
				"modified": time.Now().Format(time.RFC3339),
			},
		}

		// 패치 생성
		patchBuilder.AddOperation(rootOp)

		// 루트 설정 작업 추가
		rootSetOp := &crdtpatch.InsOperation{
			ID:       crdtDoc.NextTimestamp(),
			TargetID: common.RootID,
			Value:    rootID,
		}
		patchBuilder.AddOperation(rootSetOp)

		return nil
	})
	assert.True(t, result.Success)

	// 문서 저장
	err = doc.Save(ctx)
	assert.NoError(t, err)

	// 문서 삭제
	err = storage.DeleteDocument(ctx, "test-doc")
	assert.NoError(t, err)

	// 문서 로드 시도 (실패해야 함)
	_, err = storage.GetDocument(ctx, "test-doc")
	assert.Error(t, err)
}

// TestDocument_Edit는 문서 편집을 테스트합니다.
func TestDocument_Edit(t *testing.T) {
	// 컨텍스트 생성
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 저장소 옵션 생성
	options := DefaultStorageOptions()
	options.PubSubType = "memory"
	options.PersistenceType = "memory"

	// 저장소 생성
	storage, err := NewStorage(ctx, options)
	assert.NoError(t, err)
	defer storage.Close()

	// 문서 생성
	doc, err := storage.CreateDocument(ctx, "test-doc")
	assert.NoError(t, err)

	// 초기 문서 내용 설정
	result := doc.Edit(ctx, func(crdtDoc *crdt.Document, patchBuilder *crdtpatch.PatchBuilder) error {
		// 루트 노드 생성
		rootID := crdtDoc.NextTimestamp()
		rootOp := &crdtpatch.NewOperation{
			ID:       rootID,
			NodeType: common.NodeTypeCon,
			Value: map[string]interface{}{
				"title":    "Test Document",
				"content":  "Test content",
				"authors":  []string{"tester"},
				"modified": time.Now().Format(time.RFC3339),
			},
		}

		// 패치 생성
		patchBuilder.AddOperation(rootOp)

		// 루트 설정 작업 추가
		rootSetOp := &crdtpatch.InsOperation{
			ID:       crdtDoc.NextTimestamp(),
			TargetID: common.RootID,
			Value:    rootID,
		}
		patchBuilder.AddOperation(rootSetOp)

		return nil
	})
	assert.True(t, result.Success)

	// 문서 편집
	result = doc.Edit(ctx, func(crdtDoc *crdt.Document, patchBuilder *crdtpatch.PatchBuilder) error {
		// 현재 내용 가져오기
		view, err := crdtDoc.View()
		if err != nil {
			return err
		}

		currentContent, ok := view.(map[string]interface{})
		if !ok {
			return fmt.Errorf("root node value is not a map")
		}

		// 내용 수정
		updatedContent := make(map[string]interface{})
		for k, v := range currentContent {
			updatedContent[k] = v
		}
		updatedContent["title"] = "Updated Title"
		updatedContent["content"] = "Updated content"
		updatedContent["modified"] = time.Now().Format(time.RFC3339)

		// 새 루트 노드 생성
		newRootID := crdtDoc.NextTimestamp()
		newRootOp := &crdtpatch.NewOperation{
			ID:       newRootID,
			NodeType: common.NodeTypeCon,
			Value:    updatedContent,
		}

		// 패치 생성
		patchBuilder.AddOperation(newRootOp)

		// 루트 설정 작업 추가
		rootSetOp := &crdtpatch.InsOperation{
			ID:       crdtDoc.NextTimestamp(),
			TargetID: common.RootID,
			Value:    newRootID,
		}
		patchBuilder.AddOperation(rootSetOp)

		return nil
	})
	assert.True(t, result.Success)

	// 문서 내용 확인
	var content TestDocument
	err = doc.GetContentAs(&content)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Title", content.Title)
	assert.Equal(t, "Updated content", content.Content)
	assert.Equal(t, []string{"tester"}, content.Authors)
}

// TestDocument_OnChange는 문서 변경 이벤트를 테스트합니다.
func TestDocument_OnChange(t *testing.T) {
	// 컨텍스트 생성
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 저장소 옵션 생성
	options := DefaultStorageOptions()
	options.PubSubType = "memory"
	options.PersistenceType = "memory"

	// 저장소 생성
	storage, err := NewStorage(ctx, options)
	assert.NoError(t, err)
	defer storage.Close()

	// 문서 생성
	doc, err := storage.CreateDocument(ctx, "test-doc")
	assert.NoError(t, err)

	// 변경 이벤트 플래그
	changed := false

	// 문서 변경 콜백 등록
	doc.OnChange(func(d *Document, patch *crdtpatch.Patch) {
		changed = true
	})

	// 초기 문서 내용 설정
	result := doc.Edit(ctx, func(crdtDoc *crdt.Document, patchBuilder *crdtpatch.PatchBuilder) error {
		// 루트 노드 생성
		rootID := crdtDoc.NextTimestamp()
		rootOp := &crdtpatch.NewOperation{
			ID:       rootID,
			NodeType: common.NodeTypeCon,
			Value: map[string]interface{}{
				"title":    "Test Document",
				"content":  "Test content",
				"authors":  []string{"tester"},
				"modified": time.Now().Format(time.RFC3339),
			},
		}

		// 패치 생성
		patchBuilder.AddOperation(rootOp)

		// 루트 설정 작업 추가
		rootSetOp := &crdtpatch.InsOperation{
			ID:       crdtDoc.NextTimestamp(),
			TargetID: common.RootID,
			Value:    rootID,
		}
		patchBuilder.AddOperation(rootSetOp)

		return nil
	})
	assert.True(t, result.Success)

	// 변경 이벤트 확인
	assert.True(t, changed)
}

// TestMemoryPersistence는 메모리 영구 저장소를 테스트합니다.
func TestMemoryPersistence(t *testing.T) {
	// 컨텍스트 생성
	ctx := context.Background()

	// 영구 저장소 생성
	persistence := NewMemoryPersistence()

	// 테스트용 문서 객체 생성
	doc := &Document{
		ID:           "test-doc",
		LastModified: time.Now(),
		Metadata: map[string]interface{}{
			"test": "metadata",
		},
	}

	// 문서 저장
	err := persistence.SaveDocument(ctx, doc)
	assert.NoError(t, err)

	// 문서 로드
	data, err := persistence.LoadDocument(ctx, "test-doc")
	assert.NoError(t, err)
	assert.Equal(t, []byte(`{"title":"Test Document"}`), data)

	// 문서 목록
	docs, err := persistence.ListDocuments(ctx)
	assert.NoError(t, err)
	assert.Contains(t, docs, "test-doc")

	// 문서 삭제
	err = persistence.DeleteDocument(ctx, "test-doc")
	assert.NoError(t, err)

	// 문서 로드 시도 (실패해야 함)
	_, err = persistence.LoadDocument(ctx, "test-doc")
	assert.Error(t, err)
}
