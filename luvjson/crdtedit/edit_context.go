package crdtedit

import (
	"fmt"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
)

// EditContext provides methods for editing a document during an edit operation
type EditContext struct {
	doc          *crdt.Document
	pathResolver *PathResolver
	patchBuilder *PatchBuilder
}

// NewEditContext creates a new EditContext
func NewEditContext(doc *crdt.Document, pathResolver *PathResolver, patchBuilder *PatchBuilder) *EditContext {
	return &EditContext{
		doc:          doc,
		pathResolver: pathResolver,
		patchBuilder: patchBuilder,
	}
}

// GetRootNode returns the root node of the document
func (ctx *EditContext) GetRootNode() (crdt.Node, error) {
	return ctx.doc.GetNode(ctx.doc.GetRootID())
}

// GetRootID returns the root ID of the document
func (ctx *EditContext) GetRootID() common.LogicalTimestamp {
	return ctx.doc.GetRootID()
}

// SetValue sets a value at the given path
func (ctx *EditContext) SetValue(path string, value any) error {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path %s: %w", path, err)
	}

	return ctx.patchBuilder.AddSetOperation(nodeID, value)
}

// GetValue gets the value at the given path
func (ctx *EditContext) GetValue(path string) (any, error) {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
	}

	node, err := ctx.doc.GetNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node at path %s: %w", path, err)
	}

	return ctx.doc.GetNodeValue(node)
}

// CreateObject creates an object at the given path
func (ctx *EditContext) CreateObject(path string) error {
	// 경로가 비어 있거나 "root"인 경우 루트 노드에 객체 생성
	if path == "" || path == "root" {
		// 루트 노드에 직접 객체 생성
		objID, err := ctx.doc.CreateObject()
		if err != nil {
			return fmt.Errorf("failed to create object: %w", err)
		}

		// 루트 노드 설정
		rootID := ctx.doc.GetRootID()
		return ctx.patchBuilder.AddSetOperation(rootID, objID)
	}

	// 일반적인 경우: 부모 경로에 객체 생성
	parentPath, key, err := ctx.pathResolver.GetParentPath(path)
	if err != nil {
		return fmt.Errorf("failed to get parent path for %s: %w", path, err)
	}

	parentID, err := ctx.pathResolver.ResolveNodePath(parentPath)
	if err != nil {
		return fmt.Errorf("failed to resolve parent path %s: %w", parentPath, err)
	}

	// Create an empty object
	objID, err := ctx.doc.CreateObject()
	if err != nil {
		return fmt.Errorf("failed to create object: %w", err)
	}

	// Set the object at the parent path
	return ctx.patchBuilder.AddObjectInsertOperation(parentID, key, objID)
}

// CreateArray creates an array at the given path
func (ctx *EditContext) CreateArray(path string) error {
	// 경로가 비어 있거나 "root"인 경우 루트 노드에 배열 생성
	if path == "" || path == "root" {
		// 루트 노드에 직접 배열 생성
		arrID, err := ctx.doc.CreateArray()
		if err != nil {
			return fmt.Errorf("failed to create array: %w", err)
		}

		// 루트 노드 설정
		rootID := ctx.doc.GetRootID()
		return ctx.patchBuilder.AddSetOperation(rootID, arrID)
	}

	// 일반적인 경우: 부모 경로에 배열 생성
	parentPath, key, err := ctx.pathResolver.GetParentPath(path)
	if err != nil {
		return fmt.Errorf("failed to get parent path for %s: %w", path, err)
	}

	parentID, err := ctx.pathResolver.ResolveNodePath(parentPath)
	if err != nil {
		return fmt.Errorf("failed to resolve parent path %s: %w", parentPath, err)
	}

	// Create an empty array
	arrID, err := ctx.doc.CreateArray()
	if err != nil {
		return fmt.Errorf("failed to create array: %w", err)
	}

	// Set the array at the parent path
	return ctx.patchBuilder.AddObjectInsertOperation(parentID, key, arrID)
}

// AsObject returns an ObjectEditor for the object at the given path
func (ctx *EditContext) AsObject(path string) (ObjectEditor, error) {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
	}

	nodeType, err := ctx.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node type at path %s: %w", path, err)
	}

	if nodeType != NodeTypeObject {
		return nil, fmt.Errorf("node at path %s is not an object", path)
	}

	return newObjectEditor(ctx, path, nodeID), nil
}

// AsArray returns an ArrayEditor for the array at the given path
func (ctx *EditContext) AsArray(path string) (ArrayEditor, error) {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
	}

	nodeType, err := ctx.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node type at path %s: %w", path, err)
	}

	if nodeType != NodeTypeArray {
		return nil, fmt.Errorf("node at path %s is not an array", path)
	}

	return newArrayEditor(ctx, path, nodeID), nil
}

// AsString returns a StringEditor for the string at the given path
func (ctx *EditContext) AsString(path string) (StringEditor, error) {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
	}

	nodeType, err := ctx.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node type at path %s: %w", path, err)
	}

	if nodeType != NodeTypeString {
		return nil, fmt.Errorf("node at path %s is not a string", path)
	}

	return newStringEditor(ctx, path, nodeID), nil
}

// AsNumber returns a NumberEditor for the number at the given path
func (ctx *EditContext) AsNumber(path string) (NumberEditor, error) {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
	}

	nodeType, err := ctx.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node type at path %s: %w", path, err)
	}

	if nodeType != NodeTypeNumber {
		return nil, fmt.Errorf("node at path %s is not a number", path)
	}

	return newNumberEditor(ctx, path, nodeID), nil
}

// AsBoolean returns a BooleanEditor for the boolean at the given path
func (ctx *EditContext) AsBoolean(path string) (BooleanEditor, error) {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
	}

	nodeType, err := ctx.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node type at path %s: %w", path, err)
	}

	if nodeType != NodeTypeBoolean {
		return nil, fmt.Errorf("node at path %s is not a boolean", path)
	}

	return newBooleanEditor(ctx, path, nodeID), nil
}

// GetNodeType returns the type of the node at the given path
func (ctx *EditContext) GetNodeType(path string) (NodeType, error) {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return NodeTypeUnknown, fmt.Errorf("failed to resolve path %s: %w", path, err)
	}

	return ctx.pathResolver.GetNodeType(nodeID)
}

// SetObjectKey sets a key in an object to the given value
func (ctx *EditContext) SetObjectKey(path string, key string, value any) error {
	objEditor, err := ctx.AsObject(path)
	if err != nil {
		return err
	}

	_, err = objEditor.SetKey(key, value)
	return err
}

// DeleteObjectKey deletes a key from an object
func (ctx *EditContext) DeleteObjectKey(path string, key string) error {
	objEditor, err := ctx.AsObject(path)
	if err != nil {
		return err
	}

	_, err = objEditor.DeleteKey(key)
	return err
}

// AppendArrayElement appends a value to an array
func (ctx *EditContext) AppendArrayElement(path string, value any) error {
	arrEditor, err := ctx.AsArray(path)
	if err != nil {
		return err
	}

	_, err = arrEditor.Append(value)
	return err
}

// InsertArrayElement inserts a value into an array at the given index
func (ctx *EditContext) InsertArrayElement(path string, index int, value any) error {
	arrEditor, err := ctx.AsArray(path)
	if err != nil {
		return err
	}

	_, err = arrEditor.Insert(index, value)
	return err
}

// DeleteArrayElement deletes an element from an array at the given index
func (ctx *EditContext) DeleteArrayElement(path string, index int) error {
	arrEditor, err := ctx.AsArray(path)
	if err != nil {
		return err
	}

	_, err = arrEditor.Delete(index)
	return err
}

// IncrementNumber increments a number by the given delta
func (ctx *EditContext) IncrementNumber(path string, delta float64) error {
	numEditor, err := ctx.AsNumber(path)
	if err != nil {
		return err
	}

	_, err = numEditor.Increment(delta)
	return err
}

// AppendString appends text to a string
func (ctx *EditContext) AppendString(path string, text string) error {
	strEditor, err := ctx.AsString(path)
	if err != nil {
		return err
	}

	_, err = strEditor.Append(text)
	return err
}

// DeleteString deletes text from a string
func (ctx *EditContext) DeleteString(path string, start, end int) error {
	strEditor, err := ctx.AsString(path)
	if err != nil {
		return err
	}

	_, err = strEditor.Delete(start, end)
	return err
}
