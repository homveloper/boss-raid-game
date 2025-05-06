package crdtedit

import (
	"github.com/pkg/errors"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
)

// objectEditor implements the ObjectEditor interface
type objectEditor struct {
	ctx    *EditContext
	path   string
	nodeID common.LogicalTimestamp
}

// newObjectEditor creates a new objectEditor
func newObjectEditor(ctx *EditContext, path string, nodeID common.LogicalTimestamp) *objectEditor {
	return &objectEditor{
		ctx:    ctx,
		path:   path,
		nodeID: nodeID,
	}
}

// SetKey implements ObjectEditor.SetKey
func (e *objectEditor) SetKey(key string, value any) (ObjectEditor, error) {
	err := e.ctx.patchBuilder.AddObjectInsertOperation(e.nodeID, key, value)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to set key %s", key)
	}

	// 패치 생성 및 즉시 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := e.ctx.patchBuilder.CreatePatch(patchID)
	if err := patch.Apply(e.ctx.doc); err != nil {
		return nil, errors.Wrapf(err, "failed to apply patch for key %s", key)
	}

	return e, nil
}

// DeleteKey implements ObjectEditor.DeleteKey
func (e *objectEditor) DeleteKey(key string) (ObjectEditor, error) {
	err := e.ctx.patchBuilder.AddObjectDeleteOperation(e.nodeID, key)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to delete key %s", key)
	}

	// 패치 생성 및 즉시 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := e.ctx.patchBuilder.CreatePatch(patchID)
	if err := patch.Apply(e.ctx.doc); err != nil {
		return nil, errors.Wrapf(err, "failed to apply patch for deleting key %s", key)
	}

	return e, nil
}

// GetKeys implements ObjectEditor.GetKeys
func (e *objectEditor) GetKeys() ([]string, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node")
	}

	objNode, ok := node.(*crdt.LWWObjectNode)
	if !ok {
		return nil, errors.New("node is not an object")
	}

	return objNode.Keys(), nil
}

// HasKey implements ObjectEditor.HasKey
func (e *objectEditor) HasKey(key string) (bool, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return false, errors.Wrap(err, "failed to get node")
	}

	objNode, ok := node.(*crdt.LWWObjectNode)
	if !ok {
		return false, errors.New("node is not an object")
	}

	childNode := objNode.Get(key)
	return childNode != nil, nil
}

// GetValue implements ObjectEditor.GetValue
func (e *objectEditor) GetValue(key string) (any, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node")
	}

	objNode, ok := node.(*crdt.LWWObjectNode)
	if !ok {
		return nil, errors.New("node is not an object")
	}

	childNode := objNode.Get(key)
	if childNode == nil {
		return nil, errors.Errorf("key %s does not exist", key)
	}

	value, err := e.ctx.doc.GetNodeValue(childNode)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get value for key %s", key)
	}
	return value, nil
}

// GetPath implements ObjectEditor.GetPath
func (e *objectEditor) GetPath() string {
	return e.path
}

// arrayEditor implements the ArrayEditor interface
type arrayEditor struct {
	ctx    *EditContext
	path   string
	nodeID common.LogicalTimestamp
}

// newArrayEditor creates a new arrayEditor
func newArrayEditor(ctx *EditContext, path string, nodeID common.LogicalTimestamp) *arrayEditor {
	return &arrayEditor{
		ctx:    ctx,
		path:   path,
		nodeID: nodeID,
	}
}

// Append implements ArrayEditor.Append
func (e *arrayEditor) Append(value any) (ArrayEditor, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node")
	}

	arrNode, ok := node.(*crdt.RGAArrayNode)
	if !ok {
		return nil, errors.New("node is not an array")
	}

	index := arrNode.Length()
	err = e.ctx.patchBuilder.AddArrayInsertOperation(e.nodeID, index, value)
	if err != nil {
		return nil, errors.Wrap(err, "failed to append value to array")
	}

	// 패치 생성 및 즉시 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := e.ctx.patchBuilder.CreatePatch(patchID)
	if err := patch.Apply(e.ctx.doc); err != nil {
		return nil, errors.Wrap(err, "failed to apply patch for append operation")
	}

	return e, nil
}

// Insert implements ArrayEditor.Insert
func (e *arrayEditor) Insert(index int, value any) (ArrayEditor, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node")
	}

	arrNode, ok := node.(*crdt.RGAArrayNode)
	if !ok {
		return nil, errors.New("node is not an array")
	}

	if index < 0 || index > arrNode.Length() {
		return nil, errors.Errorf("index out of bounds: %d", index)
	}

	err = e.ctx.patchBuilder.AddArrayInsertOperation(e.nodeID, index, value)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to insert value at index %d", index)
	}

	// 패치 생성 및 즉시 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := e.ctx.patchBuilder.CreatePatch(patchID)
	if err := patch.Apply(e.ctx.doc); err != nil {
		return nil, errors.Wrapf(err, "failed to apply patch for insert at index %d", index)
	}

	return e, nil
}

// Delete implements ArrayEditor.Delete
func (e *arrayEditor) Delete(index int) (ArrayEditor, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node")
	}

	arrNode, ok := node.(*crdt.RGAArrayNode)
	if !ok {
		return nil, errors.New("node is not an array")
	}

	if index < 0 || index >= arrNode.Length() {
		return nil, errors.Errorf("index out of bounds: %d", index)
	}

	err = e.ctx.patchBuilder.AddArrayDeleteOperation(e.nodeID, index)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to delete element at index %d", index)
	}

	// 패치 생성 및 즉시 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := e.ctx.patchBuilder.CreatePatch(patchID)
	if err := patch.Apply(e.ctx.doc); err != nil {
		return nil, errors.Wrapf(err, "failed to apply patch for delete at index %d", index)
	}

	return e, nil
}

// GetLength implements ArrayEditor.GetLength
func (e *arrayEditor) GetLength() (int, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get node")
	}

	arrNode, ok := node.(*crdt.RGAArrayNode)
	if !ok {
		return 0, errors.New("node is not an array")
	}

	return arrNode.Length(), nil
}

// GetElement implements ArrayEditor.GetElement
func (e *arrayEditor) GetElement(index int) (any, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node")
	}

	arrNode, ok := node.(*crdt.RGAArrayNode)
	if !ok {
		return nil, errors.New("node is not an array")
	}

	if index < 0 || index >= arrNode.Length() {
		return nil, errors.Errorf("index out of bounds: %d", index)
	}

	childID, err := arrNode.Get(index)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get element at index %d", index)
	}

	childNode, err := e.ctx.doc.GetNode(childID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get child node")
	}

	value, err := e.ctx.doc.GetNodeValue(childNode)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get value for element at index %d", index)
	}
	return value, nil
}

// GetPath implements ArrayEditor.GetPath
func (e *arrayEditor) GetPath() string {
	return e.path
}

// stringEditor implements the StringEditor interface
type stringEditor struct {
	ctx    *EditContext
	path   string
	nodeID common.LogicalTimestamp
}

// newStringEditor creates a new stringEditor
func newStringEditor(ctx *EditContext, path string, nodeID common.LogicalTimestamp) *stringEditor {
	return &stringEditor{
		ctx:    ctx,
		path:   path,
		nodeID: nodeID,
	}
}

// Append implements StringEditor.Append
func (e *stringEditor) Append(text string) (StringEditor, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node")
	}

	strNode, ok := node.(*crdt.RGAStringNode)
	if !ok {
		return nil, errors.New("node is not a string")
	}

	value := strNode.Value().(string)
	length := len(value)
	err = e.ctx.patchBuilder.AddStringInsertOperation(e.nodeID, length, text)
	if err != nil {
		return nil, errors.Wrap(err, "failed to append text to string")
	}

	// 패치 생성 및 즉시 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := e.ctx.patchBuilder.CreatePatch(patchID)
	if err := patch.Apply(e.ctx.doc); err != nil {
		return nil, errors.Wrap(err, "failed to apply patch for append text")
	}

	return e, nil
}

// Insert implements StringEditor.Insert
func (e *stringEditor) Insert(index int, text string) (StringEditor, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node")
	}

	strNode, ok := node.(*crdt.RGAStringNode)
	if !ok {
		return nil, errors.New("node is not a string")
	}

	value := strNode.Value().(string)
	length := len(value)
	if index < 0 || index > length {
		return nil, errors.Errorf("index out of bounds: %d", index)
	}

	err = e.ctx.patchBuilder.AddStringInsertOperation(e.nodeID, index, text)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to insert text at index %d", index)
	}

	// 패치 생성 및 즉시 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := e.ctx.patchBuilder.CreatePatch(patchID)
	if err := patch.Apply(e.ctx.doc); err != nil {
		return nil, errors.Wrapf(err, "failed to apply patch for insert text at index %d", index)
	}

	return e, nil
}

// Delete implements StringEditor.Delete
func (e *stringEditor) Delete(start, end int) (StringEditor, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node")
	}

	strNode, ok := node.(*crdt.RGAStringNode)
	if !ok {
		return nil, errors.New("node is not a string")
	}

	value := strNode.Value().(string)
	length := len(value)
	if start < 0 || start >= length || end <= start || end > length {
		return nil, errors.Errorf("invalid range: [%d, %d)", start, end)
	}

	err = e.ctx.patchBuilder.AddStringDeleteOperation(e.nodeID, start, end)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to delete text from range [%d, %d)", start, end)
	}

	// 패치 생성 및 즉시 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := e.ctx.patchBuilder.CreatePatch(patchID)
	if err := patch.Apply(e.ctx.doc); err != nil {
		return nil, errors.Wrapf(err, "failed to apply patch for delete text from range [%d, %d)", start, end)
	}

	return e, nil
}

// GetLength implements StringEditor.GetLength
func (e *stringEditor) GetLength() (int, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get node")
	}

	strNode, ok := node.(*crdt.RGAStringNode)
	if !ok {
		return 0, errors.New("node is not a string")
	}

	value := strNode.Value().(string)
	return len(value), nil
}

// GetSubstring implements StringEditor.GetSubstring
func (e *stringEditor) GetSubstring(start, end int) (string, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return "", errors.Wrap(err, "failed to get node")
	}

	strNode, ok := node.(*crdt.RGAStringNode)
	if !ok {
		return "", errors.New("node is not a string")
	}

	value := strNode.Value().(string)
	length := len(value)
	if start < 0 || start >= length || end <= start || end > length {
		return "", errors.Errorf("invalid range: [%d, %d)", start, end)
	}

	return value[start:end], nil
}

// GetValue implements StringEditor.GetValue
func (e *stringEditor) GetValue() (string, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return "", errors.Wrap(err, "failed to get node")
	}

	strNode, ok := node.(*crdt.RGAStringNode)
	if !ok {
		return "", errors.New("node is not a string")
	}

	return strNode.Value().(string), nil
}

// GetPath implements StringEditor.GetPath
func (e *stringEditor) GetPath() string {
	return e.path
}

// numberEditor implements the NumberEditor interface
type numberEditor struct {
	ctx    *EditContext
	path   string
	nodeID common.LogicalTimestamp
}

// newNumberEditor creates a new numberEditor
func newNumberEditor(ctx *EditContext, path string, nodeID common.LogicalTimestamp) *numberEditor {
	return &numberEditor{
		ctx:    ctx,
		path:   path,
		nodeID: nodeID,
	}
}

// Increment implements NumberEditor.Increment
func (e *numberEditor) Increment(delta float64) (NumberEditor, error) {
	currentValue, err := e.GetValue()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current value")
	}

	newValue := currentValue + delta
	return e.SetValue(newValue)
}

// SetValue implements NumberEditor.SetValue
func (e *numberEditor) SetValue(value float64) (NumberEditor, error) {
	err := e.ctx.patchBuilder.AddSetOperation(e.nodeID, value)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set number value")
	}

	// 패치 생성 및 즉시 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := e.ctx.patchBuilder.CreatePatch(patchID)
	if err := patch.Apply(e.ctx.doc); err != nil {
		return nil, errors.Wrap(err, "failed to apply patch for set number value")
	}

	return e, nil
}

// GetValue implements NumberEditor.GetValue
func (e *numberEditor) GetValue() (float64, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get node")
	}

	numNode, ok := node.(*crdt.ConstantNode)
	if !ok {
		return 0, errors.New("node is not a number")
	}

	// Try to convert the value to a float64
	value := numNode.Value()
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, errors.Errorf("node value is not a number: %T", value)
	}
}

// GetPath implements NumberEditor.GetPath
func (e *numberEditor) GetPath() string {
	return e.path
}

// booleanEditor implements the BooleanEditor interface
type booleanEditor struct {
	ctx    *EditContext
	path   string
	nodeID common.LogicalTimestamp
}

// newBooleanEditor creates a new booleanEditor
func newBooleanEditor(ctx *EditContext, path string, nodeID common.LogicalTimestamp) *booleanEditor {
	return &booleanEditor{
		ctx:    ctx,
		path:   path,
		nodeID: nodeID,
	}
}

// Toggle implements BooleanEditor.Toggle
func (e *booleanEditor) Toggle() (BooleanEditor, error) {
	currentValue, err := e.GetValue()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current value")
	}

	return e.SetValue(!currentValue)
}

// SetValue implements BooleanEditor.SetValue
func (e *booleanEditor) SetValue(value bool) (BooleanEditor, error) {
	err := e.ctx.patchBuilder.AddSetOperation(e.nodeID, value)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set boolean value")
	}

	// 패치 생성 및 즉시 적용
	patchID := common.LogicalTimestamp{
		SID:     common.NewSessionID(),
		Counter: 1,
	}
	patch := e.ctx.patchBuilder.CreatePatch(patchID)
	if err := patch.Apply(e.ctx.doc); err != nil {
		return nil, errors.Wrap(err, "failed to apply patch for set boolean value")
	}

	return e, nil
}

// GetValue implements BooleanEditor.GetValue
func (e *booleanEditor) GetValue() (bool, error) {
	node, err := e.ctx.doc.GetNode(e.nodeID)
	if err != nil {
		return false, errors.Wrap(err, "failed to get node")
	}

	boolNode, ok := node.(*crdt.ConstantNode)
	if !ok {
		return false, errors.New("node is not a boolean")
	}

	// Try to convert the value to a bool
	value := boolNode.Value()
	if boolValue, ok := value.(bool); ok {
		return boolValue, nil
	}

	return false, errors.Errorf("node value is not a boolean: %T", value)
}

// GetPath implements BooleanEditor.GetPath
func (e *booleanEditor) GetPath() string {
	return e.path
}
