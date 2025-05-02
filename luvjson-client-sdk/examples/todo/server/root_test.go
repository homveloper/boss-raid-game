package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDocumentRootType은 CRDT 문서의 루트 노드 타입을 테스트합니다.
func TestDocumentRootType(t *testing.T) {
	// 서버 생성
	server, err := NewServer()
	require.NoError(t, err)

	// 문서 ID
	docID := "test-doc"

	// 문서 초기화
	err = server.createDefaultDocument(docID)
	require.NoError(t, err)

	// 문서 가져오기
	server.documentsMu.RLock()
	doc := server.documents[docID]
	server.documentsMu.RUnlock()

	// 문서 뷰 가져오기
	view, err := doc.View()
	require.NoError(t, err)

	// 뷰 타입 확인
	fmt.Printf("Initial document view type: %T\n", view)
	fmt.Printf("Initial document view value: %v\n", view)

	// 맵으로 변환 가능한지 확인
	_, ok := view.(map[string]interface{})
	assert.False(t, ok, "Initial document view should not be convertible to map[string]interface{}")

	// Todo 항목 추가
	clientPatch := &ClientPatch{
		DocumentID:  docID,
		BaseVersion: 1,
		ClientID:    "test-client",
		Operations: []Operation{
			{
				Type:      "add",
				Path:      "todo-1",
				Value:     json.RawMessage(`{"title":"Test Todo","completed":false}`),
				Timestamp: 1234567890,
				ClientID:  "test-client",
			},
		},
	}

	// 패치 적용
	err = server.applyClientPatch(docID, clientPatch)
	require.NoError(t, err)

	// 패치 적용 후 뷰 가져오기
	viewAfter, err := doc.View()
	require.NoError(t, err)

	// 패치 적용 후 뷰 타입 확인
	fmt.Printf("Document view type after patch: %T\n", viewAfter)
	fmt.Printf("Document view value after patch: %v\n", viewAfter)

	// 맵으로 변환 가능한지 확인
	docMap, ok := viewAfter.(map[string]interface{})
	assert.True(t, ok, "Document view after patch should be convertible to map[string]interface{}")

	// Todo 항목 확인
	if ok {
		todoMap, ok := docMap["todo-1"].(map[string]interface{})
		assert.True(t, ok, "Todo item should be convertible to map[string]interface{}")
		if ok {
			assert.Equal(t, "Test Todo", todoMap["title"])
		}
	}
}

// TestStorageDocumentInitialization은 Storage 문서 초기화를 테스트합니다.
func TestStorageDocumentInitialization(t *testing.T) {
	// 서버 생성
	server, err := NewServer()
	require.NoError(t, err)

	// 문서 ID
	docID := "test-storage-doc"

	// 문서 초기화
	err = server.createDefaultDocument(docID)
	require.NoError(t, err)

	// 스토리지에서 문서 로드
	doc, err := server.storage.LoadDocument(context.Background(), docID)
	require.NoError(t, err)

	// 문서 뷰 가져오기
	view, err := doc.CRDTDoc.View()
	require.NoError(t, err)

	// 뷰 타입 확인
	fmt.Printf("Storage document view type: %T\n", view)
	fmt.Printf("Storage document view value: %v\n", view)

	// 맵으로 변환 가능한지 확인
	_, ok := view.(map[string]interface{})
	assert.True(t, ok, "Storage document view should be convertible to map[string]interface{}")
}
