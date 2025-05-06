package crdtedit

import (
	"github.com/pkg/errors"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
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
		return errors.Wrapf(err, "failed to resolve path %s", path)
	}

	if err := ctx.patchBuilder.AddSetOperation(nodeID, value); err != nil {
		return errors.Wrapf(err, "failed to set value at path %s", path)
	}

	// 패치 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := ctx.patchBuilder.CreatePatch(patchID)
	if err := patch.Apply(ctx.doc); err != nil {
		return errors.Wrapf(err, "failed to apply patch for path %s", path)
	}
	return nil
}

// GetValue gets the value at the given path
func (ctx *EditContext) GetValue(path string) (any, error) {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve path %s", path)
	}

	node, err := ctx.doc.GetNode(nodeID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node at path %s", path)
	}

	value, err := ctx.doc.GetNodeValue(node)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node value at path %s", path)
	}
	return value, nil
}

// CreateObject creates an object at the given path
func (ctx *EditContext) CreateObject(path string) error {
	// 경로가 비어 있거나 "root"인 경우 루트 노드에 객체 생성
	if path == "" || path == "root" {
		// 루트 노드에 직접 객체 생성
		// 새 객체 노드 ID 생성
		objID := ctx.patchBuilder.NextID()

		// 객체 노드 생성
		newOp := &crdtpatch.NewOperation{
			ID:       objID,
			NodeType: common.NodeTypeObj,
		}

		// 루트 노드 설정
		rootID := ctx.doc.GetRootID()
		setOp, err := crdtpatch.NewSetOperation(rootID, objID)
		if err != nil {
			return errors.Wrap(err, "failed to create set operation")
		}

		// 패치 생성 및 적용
		patchID := common.LogicalTimestamp{
			SID:     common.NewSessionID(),
			Counter: 1,
		}
		patch := crdtpatch.NewPatch(patchID)
		patch.AddOperation(newOp)
		patch.AddOperation(setOp)

		if err := patch.Apply(ctx.doc); err != nil {
			return errors.Wrap(err, "failed to apply patch")
		}
		return nil
	}

	// 일반적인 경우: 부모 경로에 객체 생성
	parentPath, key, err := ctx.pathResolver.GetParentPath(path)
	if err != nil {
		return errors.Wrapf(err, "failed to get parent path for %s", path)
	}

	parentID, err := ctx.pathResolver.ResolveNodePath(parentPath)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve parent path %s", parentPath)
	}

	// 새 객체 노드 ID 생성
	objID := ctx.patchBuilder.NextID()

	// 객체 노드 생성
	newOp := &crdtpatch.NewOperation{
		ID:       objID,
		NodeType: common.NodeTypeObj,
	}

	// 부모 객체에 삽입
	insOp, err := crdtpatch.NewObjectInsertOperation(parentID, key, objID)
	if err != nil {
		return errors.Wrapf(err, "failed to create object insert operation for path %s", path)
	}

	// 패치 생성 및 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := crdtpatch.NewPatch(patchID)
	patch.AddOperation(newOp)
	patch.AddOperation(insOp)

	if err := patch.Apply(ctx.doc); err != nil {
		return errors.Wrap(err, "failed to apply patch")
	}
	return nil
}

// CreateArray creates an array at the given path
func (ctx *EditContext) CreateArray(path string) error {
	// 경로가 비어 있거나 "root"인 경우 루트 노드에 배열 생성
	if path == "" || path == "root" {
		// 루트 노드에 직접 배열 생성
		// 새 배열 노드 ID 생성
		arrID := ctx.patchBuilder.NextID()

		// 배열 노드 생성
		newOp := &crdtpatch.NewOperation{
			ID:       arrID,
			NodeType: common.NodeTypeArr,
		}

		// 루트 노드 설정
		rootID := ctx.doc.GetRootID()
		setOp, err := crdtpatch.NewSetOperation(rootID, arrID)
		if err != nil {
			return errors.Wrap(err, "failed to create set operation")
		}

		// 패치 생성 및 적용
		patchID := common.LogicalTimestamp{
			SID:     common.NewSessionID(),
			Counter: 1,
		}
		patch := crdtpatch.NewPatch(patchID)
		patch.AddOperation(newOp)
		patch.AddOperation(setOp)

		if err := patch.Apply(ctx.doc); err != nil {
			return errors.Wrap(err, "failed to apply patch")
		}
		return nil
	}

	// 일반적인 경우: 부모 경로에 배열 생성
	parentPath, key, err := ctx.pathResolver.GetParentPath(path)
	if err != nil {
		return errors.Wrapf(err, "failed to get parent path for %s", path)
	}

	parentID, err := ctx.pathResolver.ResolveNodePath(parentPath)
	if err != nil {
		return errors.Wrapf(err, "failed to resolve parent path %s", parentPath)
	}

	// 새 배열 노드 ID 생성
	arrID := ctx.patchBuilder.NextID()

	// 배열 노드 생성
	newOp := &crdtpatch.NewOperation{
		ID:       arrID,
		NodeType: common.NodeTypeArr,
	}

	// 부모 객체에 삽입
	insOp, err := crdtpatch.NewObjectInsertOperation(parentID, key, arrID)
	if err != nil {
		return errors.Wrapf(err, "failed to create object insert operation for path %s", path)
	}

	// 패치 생성 및 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := crdtpatch.NewPatch(patchID)
	patch.AddOperation(newOp)
	patch.AddOperation(insOp)

	if err := patch.Apply(ctx.doc); err != nil {
		return errors.Wrap(err, "failed to apply patch")
	}
	return nil
}

// AsObject returns an ObjectEditor for the object at the given path
func (ctx *EditContext) AsObject(path string) (ObjectEditor, error) {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve path %s", path)
	}

	nodeType, err := ctx.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node type at path %s", path)
	}

	if nodeType != NodeTypeObject {
		return nil, errors.Errorf("node at path %s is not an object", path)
	}

	return newObjectEditor(ctx, path, nodeID), nil
}

// AsArray returns an ArrayEditor for the array at the given path
func (ctx *EditContext) AsArray(path string) (ArrayEditor, error) {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve path %s", path)
	}

	nodeType, err := ctx.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node type at path %s", path)
	}

	if nodeType != NodeTypeArray {
		return nil, errors.Errorf("node at path %s is not an array", path)
	}

	return newArrayEditor(ctx, path, nodeID), nil
}

// AsString returns a StringEditor for the string at the given path
func (ctx *EditContext) AsString(path string) (StringEditor, error) {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve path %s", path)
	}

	nodeType, err := ctx.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node type at path %s", path)
	}

	if nodeType != NodeTypeString {
		return nil, errors.Errorf("node at path %s is not a string", path)
	}

	return newStringEditor(ctx, path, nodeID), nil
}

// AsNumber returns a NumberEditor for the number at the given path
func (ctx *EditContext) AsNumber(path string) (NumberEditor, error) {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve path %s", path)
	}

	nodeType, err := ctx.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node type at path %s", path)
	}

	if nodeType != NodeTypeNumber {
		return nil, errors.Errorf("node at path %s is not a number", path)
	}

	return newNumberEditor(ctx, path, nodeID), nil
}

// AsBoolean returns a BooleanEditor for the boolean at the given path
func (ctx *EditContext) AsBoolean(path string) (BooleanEditor, error) {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve path %s", path)
	}

	nodeType, err := ctx.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node type at path %s", path)
	}

	if nodeType != NodeTypeBoolean {
		return nil, errors.Errorf("node at path %s is not a boolean", path)
	}

	return newBooleanEditor(ctx, path, nodeID), nil
}

// GetNodeType returns the type of the node at the given path
func (ctx *EditContext) GetNodeType(path string) (NodeType, error) {
	nodeID, err := ctx.pathResolver.ResolveNodePath(path)
	if err != nil {
		return NodeTypeUnknown, errors.Wrapf(err, "failed to resolve path %s", path)
	}

	return ctx.pathResolver.GetNodeType(nodeID)
}

// SetObjectKey sets a key in an object to the given value
func (ctx *EditContext) SetObjectKey(path string, key string, value any) error {
	objEditor, err := ctx.AsObject(path)
	if err != nil {
		return errors.Wrapf(err, "failed to get object editor for path %s", path)
	}

	_, err = objEditor.SetKey(key, value)
	if err != nil {
		return errors.Wrapf(err, "failed to set key %s at path %s", key, path)
	}

	// 패치 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := ctx.patchBuilder.CreatePatch(patchID)
	if err := patch.Apply(ctx.doc); err != nil {
		return errors.Wrap(err, "failed to apply patch")
	}
	return nil
}

// DeleteObjectKey deletes a key from an object
func (ctx *EditContext) DeleteObjectKey(path string, key string) error {
	objEditor, err := ctx.AsObject(path)
	if err != nil {
		return errors.Wrapf(err, "failed to get object editor for path %s", path)
	}

	_, err = objEditor.DeleteKey(key)
	if err != nil {
		return errors.Wrapf(err, "failed to delete key %s at path %s", key, path)
	}

	// 패치 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := ctx.patchBuilder.CreatePatch(patchID)
	if err := patch.Apply(ctx.doc); err != nil {
		return errors.Wrap(err, "failed to apply patch")
	}
	return nil
}

// AppendArrayElement appends a value to an array
func (ctx *EditContext) AppendArrayElement(path string, value any) error {
	arrEditor, err := ctx.AsArray(path)
	if err != nil {
		return errors.Wrapf(err, "failed to get array editor for path %s", path)
	}

	_, err = arrEditor.Append(value)
	if err != nil {
		return errors.Wrapf(err, "failed to append element to array at path %s", path)
	}

	// 패치 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := ctx.patchBuilder.CreatePatch(patchID)
	if err := patch.Apply(ctx.doc); err != nil {
		return errors.Wrap(err, "failed to apply patch")
	}
	return nil
}

// InsertArrayElement inserts a value into an array at the given index
func (ctx *EditContext) InsertArrayElement(path string, index int, value any) error {
	arrEditor, err := ctx.AsArray(path)
	if err != nil {
		return errors.Wrapf(err, "failed to get array editor for path %s", path)
	}

	_, err = arrEditor.Insert(index, value)
	if err != nil {
		return errors.Wrapf(err, "failed to insert element at index %d in array at path %s", index, path)
	}
	return nil
}

// DeleteArrayElement deletes an element from an array at the given index
func (ctx *EditContext) DeleteArrayElement(path string, index int) error {
	arrEditor, err := ctx.AsArray(path)
	if err != nil {
		return errors.Wrapf(err, "failed to get array editor for path %s", path)
	}

	_, err = arrEditor.Delete(index)
	if err != nil {
		return errors.Wrapf(err, "failed to delete element at index %d in array at path %s", index, path)
	}
	return nil
}

// IncrementNumber increments a number by the given delta
func (ctx *EditContext) IncrementNumber(path string, delta float64) error {
	numEditor, err := ctx.AsNumber(path)
	if err != nil {
		return errors.Wrapf(err, "failed to get number editor for path %s", path)
	}

	_, err = numEditor.Increment(delta)
	if err != nil {
		return errors.Wrapf(err, "failed to increment number at path %s by %f", path, delta)
	}
	return nil
}

// AppendString appends text to a string
func (ctx *EditContext) AppendString(path string, text string) error {
	strEditor, err := ctx.AsString(path)
	if err != nil {
		return errors.Wrapf(err, "failed to get string editor for path %s", path)
	}

	_, err = strEditor.Append(text)
	if err != nil {
		return errors.Wrapf(err, "failed to append text to string at path %s", path)
	}
	return nil
}

// DeleteString deletes text from a string
func (ctx *EditContext) DeleteString(path string, start, end int) error {
	strEditor, err := ctx.AsString(path)
	if err != nil {
		return errors.Wrapf(err, "failed to get string editor for path %s", path)
	}

	_, err = strEditor.Delete(start, end)
	if err != nil {
		return errors.Wrapf(err, "failed to delete text from string at path %s", path)
	}
	return nil
}
