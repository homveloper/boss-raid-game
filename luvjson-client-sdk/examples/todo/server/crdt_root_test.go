package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
)

// TestCRDTDocumentRootType은 CRDT 문서의 루트 노드 타입을 테스트합니다.
func TestCRDTDocumentRootType(t *testing.T) {
	// 새 문서 생성
	sessionID := common.NewSessionID()
	doc := crdt.NewDocument(sessionID)

	// 문서 뷰 가져오기
	view, err := doc.View()
	require.NoError(t, err)

	// 뷰 타입 확인
	fmt.Printf("Initial document view type: %T\n", view)
	fmt.Printf("Initial document view value: %v\n", view)

	// 맵으로 변환 가능한지 확인
	_, ok := view.(map[string]interface{})
	assert.False(t, ok, "Initial document view should not be convertible to map[string]interface{}")

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
	_, ok = viewAfter.(map[string]interface{})
	assert.True(t, ok, "Document view after patch should be convertible to map[string]interface{}")

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
	docMapWithTodo, ok := viewWithTodo.(map[string]interface{})
	assert.True(t, ok, "Document view with todo should be convertible to map[string]interface{}")

	// Todo 항목 확인
	todoMap, ok := docMapWithTodo["todo-1"].(map[string]interface{})
	assert.True(t, ok, "Todo item should be convertible to map[string]interface{}")
	assert.Equal(t, "Test Todo", todoMap["title"])
}
