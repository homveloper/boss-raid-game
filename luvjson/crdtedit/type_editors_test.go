package crdtedit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
)

// setupTestDocument creates a test document with a root object
func setupTestDocument(t *testing.T) *crdt.Document {
	sessionID := common.NewSessionID()
	doc := crdt.NewDocument(sessionID)

	// 루트 노드에 객체 노드 생성
	objID := doc.NextTimestamp()
	objNode := crdt.NewLWWObjectNode(objID)
	doc.AddNode(objNode)

	// 루트 노드의 값을 객체 노드로 설정
	rootNode := doc.Root().(*crdt.RootNode)
	rootNode.NodeValue = objNode

	return doc
}

// TestObjectEditor tests the ObjectEditor implementation
func TestObjectEditor(t *testing.T) {
	doc := setupTestDocument(t)
	editor := NewDocumentEditor(doc)

	// 사용자 객체 생성
	err := editor.CreateObject("user")
	require.NoError(t, err, "Failed to create user object")

	// 객체 에디터 가져오기
	objEditor, err := editor.AsObject("user")
	require.NoError(t, err, "Failed to get object editor")

	// 키 설정
	objEditor, err = objEditor.SetKey("name", "John Doe")
	require.NoError(t, err, "Failed to set key")

	objEditor, err = objEditor.SetKey("age", 30)
	require.NoError(t, err, "Failed to set key")

	objEditor, err = objEditor.SetKey("active", true)
	require.NoError(t, err, "Failed to set key")

	// 키 확인
	hasKey, err := objEditor.HasKey("name")
	require.NoError(t, err, "Failed to check key")
	assert.True(t, hasKey, "Key should exist")

	hasKey, err = objEditor.HasKey("nonexistent")
	require.NoError(t, err, "Failed to check key")
	assert.False(t, hasKey, "Key should not exist")

	// 값 가져오기
	name, err := objEditor.GetValue("name")
	require.NoError(t, err, "Failed to get value")
	assert.Equal(t, "John Doe", name, "Value should match")

	age, err := objEditor.GetValue("age")
	require.NoError(t, err, "Failed to get value")
	assert.Equal(t, float64(30), age, "Value should match")

	active, err := objEditor.GetValue("active")
	require.NoError(t, err, "Failed to get value")
	assert.Equal(t, true, active, "Value should match")

	// 키 목록 가져오기
	keys, err := objEditor.GetKeys()
	require.NoError(t, err, "Failed to get keys")
	assert.ElementsMatch(t, []string{"name", "age", "active"}, keys, "Keys should match")

	// 키 삭제
	objEditor, err = objEditor.DeleteKey("age")
	require.NoError(t, err, "Failed to delete key")

	// 삭제 확인
	hasKey, err = objEditor.HasKey("age")
	require.NoError(t, err, "Failed to check key")
	assert.False(t, hasKey, "Key should not exist after deletion")

	// 경로 확인
	assert.Equal(t, "user", objEditor.GetPath(), "Path should match")
}

// TestArrayEditor tests the ArrayEditor implementation
func TestArrayEditor(t *testing.T) {
	doc := setupTestDocument(t)
	editor := NewDocumentEditor(doc)

	// 배열 생성
	err := editor.CreateArray("items")
	require.NoError(t, err, "Failed to create array")

	// 배열 에디터 가져오기
	arrEditor, err := editor.AsArray("items")
	require.NoError(t, err, "Failed to get array editor")

	// 요소 추가
	arrEditor, err = arrEditor.Append("Item 1")
	require.NoError(t, err, "Failed to append item")

	arrEditor, err = arrEditor.Append("Item 2")
	require.NoError(t, err, "Failed to append item")

	arrEditor, err = arrEditor.Append("Item 3")
	require.NoError(t, err, "Failed to append item")

	// 길이 확인
	length, err := arrEditor.GetLength()
	require.NoError(t, err, "Failed to get length")
	assert.Equal(t, 3, length, "Length should match")

	// 요소 가져오기
	item, err := arrEditor.GetElement(1)
	require.NoError(t, err, "Failed to get element")
	assert.Equal(t, "Item 2", item, "Element should match")

	// 요소 삽입
	arrEditor, err = arrEditor.Insert(1, "New Item")
	require.NoError(t, err, "Failed to insert item")

	// 삽입 확인
	item, err = arrEditor.GetElement(1)
	require.NoError(t, err, "Failed to get element")
	assert.Equal(t, "New Item", item, "Element should match")

	// 길이 재확인
	length, err = arrEditor.GetLength()
	require.NoError(t, err, "Failed to get length")
	assert.Equal(t, 4, length, "Length should match after insertion")

	// 요소 삭제
	arrEditor, err = arrEditor.Delete(1)
	require.NoError(t, err, "Failed to delete item")

	// 삭제 확인
	item, err = arrEditor.GetElement(1)
	require.NoError(t, err, "Failed to get element")
	assert.Equal(t, "Item 2", item, "Element should match after deletion")

	// 길이 재확인
	length, err = arrEditor.GetLength()
	require.NoError(t, err, "Failed to get length")
	assert.Equal(t, 3, length, "Length should match after deletion")

	// 경로 확인
	assert.Equal(t, "items", arrEditor.GetPath(), "Path should match")
}

// TestStringEditor tests the StringEditor implementation
func TestStringEditor(t *testing.T) {
	doc := setupTestDocument(t)
	editor := NewDocumentEditor(doc)

	// 문자열 생성
	err := editor.SetValue("text", "Hello")
	require.NoError(t, err, "Failed to create string")

	// 문자열 에디터 가져오기
	strEditor, err := editor.AsString("text")
	require.NoError(t, err, "Failed to get string editor")

	// 문자열 추가
	strEditor, err = strEditor.Append(" World")
	require.NoError(t, err, "Failed to append text")

	// 값 확인
	value, err := strEditor.GetValue()
	require.NoError(t, err, "Failed to get value")
	assert.Equal(t, "Hello World", value, "Value should match")

	// 문자열 삽입
	strEditor, err = strEditor.Insert(5, ",")
	require.NoError(t, err, "Failed to insert text")

	// 삽입 확인
	value, err = strEditor.GetValue()
	require.NoError(t, err, "Failed to get value")
	assert.Equal(t, "Hello, World", value, "Value should match after insertion")

	// 길이 확인
	length, err := strEditor.GetLength()
	require.NoError(t, err, "Failed to get length")
	assert.Equal(t, 12, length, "Length should match")

	// 부분 문자열 가져오기
	substring, err := strEditor.GetSubstring(0, 5)
	require.NoError(t, err, "Failed to get substring")
	assert.Equal(t, "Hello", substring, "Substring should match")

	// 문자열 삭제
	strEditor, err = strEditor.Delete(5, 7)
	require.NoError(t, err, "Failed to delete text")

	// 삭제 확인
	value, err = strEditor.GetValue()
	require.NoError(t, err, "Failed to get value")
	assert.Equal(t, "Hello World", value, "Value should match after deletion")

	// 경로 확인
	assert.Equal(t, "text", strEditor.GetPath(), "Path should match")
}

// TestNumberEditor tests the NumberEditor implementation
func TestNumberEditor(t *testing.T) {
	doc := setupTestDocument(t)
	editor := NewDocumentEditor(doc)

	// 숫자 생성
	err := editor.SetValue("count", 10)
	require.NoError(t, err, "Failed to create number")

	// 숫자 에디터 가져오기
	numEditor, err := editor.AsNumber("count")
	require.NoError(t, err, "Failed to get number editor")

	// 값 확인
	value, err := numEditor.GetValue()
	require.NoError(t, err, "Failed to get value")
	assert.Equal(t, float64(10), value, "Value should match")

	// 값 설정
	numEditor, err = numEditor.SetValue(20)
	require.NoError(t, err, "Failed to set value")

	// 설정 확인
	value, err = numEditor.GetValue()
	require.NoError(t, err, "Failed to get value")
	assert.Equal(t, float64(20), value, "Value should match after setting")

	// 증가
	numEditor, err = numEditor.Increment(5)
	require.NoError(t, err, "Failed to increment value")

	// 증가 확인
	value, err = numEditor.GetValue()
	require.NoError(t, err, "Failed to get value")
	assert.Equal(t, float64(25), value, "Value should match after incrementing")

	// 경로 확인
	assert.Equal(t, "count", numEditor.GetPath(), "Path should match")
}

// TestBooleanEditor tests the BooleanEditor implementation
func TestBooleanEditor(t *testing.T) {
	doc := setupTestDocument(t)
	editor := NewDocumentEditor(doc)

	// 불리언 생성
	err := editor.SetValue("flag", false)
	require.NoError(t, err, "Failed to create boolean")

	// 불리언 에디터 가져오기
	boolEditor, err := editor.AsBoolean("flag")
	require.NoError(t, err, "Failed to get boolean editor")

	// 값 확인
	value, err := boolEditor.GetValue()
	require.NoError(t, err, "Failed to get value")
	assert.Equal(t, false, value, "Value should match")

	// 값 설정
	boolEditor, err = boolEditor.SetValue(true)
	require.NoError(t, err, "Failed to set value")

	// 설정 확인
	value, err = boolEditor.GetValue()
	require.NoError(t, err, "Failed to get value")
	assert.Equal(t, true, value, "Value should match after setting")

	// 토글
	boolEditor, err = boolEditor.Toggle()
	require.NoError(t, err, "Failed to toggle value")

	// 토글 확인
	value, err = boolEditor.GetValue()
	require.NoError(t, err, "Failed to get value")
	assert.Equal(t, false, value, "Value should match after toggling")

	// 경로 확인
	assert.Equal(t, "flag", boolEditor.GetPath(), "Path should match")
}
