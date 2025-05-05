package crdtstorage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
)

// TestDocumentRootType은 CRDT 문서의 루트 노드 타입을 테스트합니다.
func TestDocumentRootType(t *testing.T) {
	// 새 문서 생성
	sessionID := common.NewSessionID()
	doc := crdt.NewDocument(sessionID)

	// 문서 뷰 가져오기
	view, err := doc.View()
	require.NoError(t, err)

	// 뷰 타입 확인
	fmt.Printf("Initial document view type: %T\n", view)
	fmt.Printf("Initial document view value: %v\n", view)

	// 루트를 객체로 초기화
	pb := crdtpatch.NewPatchBuilder(sessionID, 1)
	rootOp := pb.NewObject()
	patch := pb.Flush()

	// 패치 적용
	err = patch.Apply(doc)
	require.NoError(t, err)

	// 패치 적용 후 뷰 가져오기
	viewAfter, err := doc.View()
	require.NoError(t, err)

	// 패치 적용 후 뷰 타입 확인
	fmt.Printf("Document view type after patch: %T\n", viewAfter)
	fmt.Printf("Document view value after patch: %v\n", viewAfter)

	// 맵으로 변환 가능한지 확인
	_, ok := viewAfter.(map[string]interface{})
	assert.True(t, ok, "Document view should be convertible to map[string]interface{}")

	// Todo 항목 추가
	pb = crdtpatch.NewPatchBuilder(sessionID, 2)
	todoItem := map[string]interface{}{
		"title":     "Test Todo",
		"completed": false,
		"createdAt": time.Now().Format(time.RFC3339),
		"updatedAt": time.Now().Format(time.RFC3339),
	}
	pb.InsertObjectField(rootOp.ID, "todo-1", todoItem)
	patch = pb.Flush()

	// 패치 적용
	err = patch.Apply(doc)
	require.NoError(t, err)

	// Todo 항목 추가 후 뷰 가져오기
	viewWithTodo, err := doc.View()
	require.NoError(t, err)

	// Todo 항목 추가 후 뷰 타입 확인
	fmt.Printf("Document view type after adding todo: %T\n", viewWithTodo)
	fmt.Printf("Document view value after adding todo: %v\n", viewWithTodo)

	// 맵으로 변환
	docMap, ok := viewWithTodo.(map[string]interface{})
	assert.True(t, ok, "Document view should be convertible to map[string]interface{}")

	// Todo 항목 확인
	todoMap, ok := docMap["todo-1"].(map[string]interface{})
	assert.True(t, ok, "Todo item should be convertible to map[string]interface{}")
	assert.Equal(t, "Test Todo", todoMap["title"])
}

// TestStorageDocumentInitialization은 Storage 문서 초기화를 테스트합니다.
func TestStorageDocumentInitialization(t *testing.T) {
	// 임시 스토리지 생성
	storage, err := NewStorageWithCustomPersistence(context.Background(), DefaultStorageOptions(), NewMemoryPersistence())
	require.NoError(t, err)

	// 문서 생성
	docID := "test-doc"
	doc, err := storage.CreateDocument(context.Background(), docID)
	require.NoError(t, err)

	// 문서 뷰 가져오기
	view, err := doc.CRDTDoc.View()
	require.NoError(t, err)

	// 뷰 타입 확인
	fmt.Printf("Storage document view type: %T\n", view)
	fmt.Printf("Storage document view value: %v\n", view)

	// 세션 ID 생성
	sessionID := common.NewSessionID()

	// 루트를 객체로 초기화
	pb := crdtpatch.NewPatchBuilder(sessionID, 1)
	rootOp := pb.NewObject()
	patch := pb.Flush()

	// 패치 적용
	err = patch.Apply(doc.CRDTDoc)
	require.NoError(t, err)

	// 패치 적용 후 뷰 가져오기
	viewAfter, err := doc.CRDTDoc.View()
	require.NoError(t, err)

	// 패치 적용 후 뷰 타입 확인
	fmt.Printf("Storage document view type after patch: %T\n", viewAfter)
	fmt.Printf("Storage document view value after patch: %v\n", viewAfter)

	// 맵으로 변환 가능한지 확인
	_, ok := viewAfter.(map[string]interface{})
	assert.True(t, ok, "Storage document view should be convertible to map[string]interface{}")

	// Todo 항목 추가
	pb = crdtpatch.NewPatchBuilder(sessionID, 2)
	todoItem := map[string]interface{}{
		"title":     "Test Todo",
		"completed": false,
		"createdAt": time.Now().Format(time.RFC3339),
		"updatedAt": time.Now().Format(time.RFC3339),
	}
	pb.InsertObjectField(rootOp.ID, "todo-1", todoItem)
	patch = pb.Flush()

	// 패치 적용
	err = patch.Apply(doc.CRDTDoc)
	require.NoError(t, err)

	// Todo 항목 추가 후 뷰 가져오기
	viewWithTodo, err := doc.CRDTDoc.View()
	require.NoError(t, err)

	// Todo 항목 추가 후 뷰 타입 확인
	fmt.Printf("Storage document view type after adding todo: %T\n", viewWithTodo)

	// 맵으로 변환
	docMap, ok := viewWithTodo.(map[string]interface{})
	assert.True(t, ok, "Storage document view should be convertible to map[string]interface{}")

	// Todo 항목 확인
	todoMap, ok := docMap["todo-1"].(map[string]interface{})
	assert.True(t, ok, "Todo item should be convertible to map[string]interface{}")
	assert.Equal(t, "Test Todo", todoMap["title"])

	// 문서 저장
	err = doc.Save(context.Background())
	require.NoError(t, err)

	// 문서 로드
	loadedDoc, err := storage.GetDocument(context.Background(), docID)
	require.NoError(t, err)

	// 로드된 문서 뷰 가져오기
	loadedView, err := loadedDoc.CRDTDoc.View()
	require.NoError(t, err)

	// 로드된 문서 뷰 타입 확인
	fmt.Printf("Loaded document view type: %T\n", loadedView)

	// 맵으로 변환
	loadedMap, ok := loadedView.(map[string]interface{})
	assert.True(t, ok, "Loaded document view should be convertible to map[string]interface{}")

	// Todo 항목 확인
	loadedTodo, ok := loadedMap["todo-1"].(map[string]interface{})
	assert.True(t, ok, "Loaded todo item should be convertible to map[string]interface{}")
	assert.Equal(t, "Test Todo", loadedTodo["title"])
}

// TestTodoAppScenario는 Todo 앱 시나리오를 테스트합니다.
func TestTodoAppScenario(t *testing.T) {
	// 임시 스토리지 생성
	storage, err := NewStorageWithCustomPersistence(context.Background(), DefaultStorageOptions(), NewMemoryPersistence())
	require.NoError(t, err)

	// 문서 생성
	docID := "todos"
	doc, err := storage.CreateDocument(context.Background(), docID)
	require.NoError(t, err)

	// 세션 ID 생성
	sessionID := common.NewSessionID()

	// 루트를 객체로 초기화
	pb := crdtpatch.NewPatchBuilder(sessionID, 1)
	rootOp := pb.NewObject()
	patch := pb.Flush()

	// 패치 적용
	err = patch.Apply(doc.CRDTDoc)
	require.NoError(t, err)

	// Todo 항목 추가
	for i := 1; i <= 3; i++ {
		pb = crdtpatch.NewPatchBuilder(sessionID, uint64(i+1))
		todoID := fmt.Sprintf("todo-%d", i)
		todoItem := map[string]interface{}{
			"title":     fmt.Sprintf("Test Todo %d", i),
			"completed": false,
			"createdAt": time.Now().Format(time.RFC3339),
			"updatedAt": time.Now().Format(time.RFC3339),
		}
		pb.InsertObjectField(rootOp.ID, todoID, todoItem)
		patch = pb.Flush()

		// 패치 적용
		err = patch.Apply(doc.CRDTDoc)
		require.NoError(t, err)
	}

	// 문서 뷰 가져오기
	view, err := doc.CRDTDoc.View()
	require.NoError(t, err)

	// 맵으로 변환
	docMap, ok := view.(map[string]interface{})
	assert.True(t, ok, "Document view should be convertible to map[string]interface{}")

	// Todo 항목 수 확인
	assert.Equal(t, 3, len(docMap), "Document should have 3 todo items")

	// Todo 항목 내용 확인
	for i := 1; i <= 3; i++ {
		todoID := fmt.Sprintf("todo-%d", i)
		todoMap, ok := docMap[todoID].(map[string]interface{})
		assert.True(t, ok, fmt.Sprintf("Todo item %s should be convertible to map[string]interface{}", todoID))
		assert.Equal(t, fmt.Sprintf("Test Todo %d", i), todoMap["title"])
	}

	// 문서 저장
	err = doc.Save(context.Background())
	require.NoError(t, err)

	// 문서 로드
	loadedDoc, err := storage.GetDocument(context.Background(), docID)
	require.NoError(t, err)

	// 로드된 문서 뷰 가져오기
	loadedView, err := loadedDoc.CRDTDoc.View()
	require.NoError(t, err)

	// 맵으로 변환
	loadedMap, ok := loadedView.(map[string]interface{})
	assert.True(t, ok, "Loaded document view should be convertible to map[string]interface{}")

	// Todo 항목 수 확인
	assert.Equal(t, 3, len(loadedMap), "Loaded document should have 3 todo items")

	// Todo 항목 내용 확인
	for i := 1; i <= 3; i++ {
		todoID := fmt.Sprintf("todo-%d", i)
		todoMap, ok := loadedMap[todoID].(map[string]interface{})
		assert.True(t, ok, fmt.Sprintf("Loaded todo item %s should be convertible to map[string]interface{}", todoID))
		assert.Equal(t, fmt.Sprintf("Test Todo %d", i), todoMap["title"])
	}

	// 문서 내용 출력
	docJSON, _ := json.MarshalIndent(loadedView, "", "  ")
	log.Printf("최종 문서 내용: %s", string(docJSON))
}
