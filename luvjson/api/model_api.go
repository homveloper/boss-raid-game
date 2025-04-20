package api

import (
	"sync"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
)

// ModelApi provides a high-level API for working with CRDT documents.
// It is the main entry point for executing local user actions on a CRDT document.
type ModelApi struct {
	// doc is the CRDT document being manipulated.
	doc *crdt.Document

	// builder is the patch builder for local changes.
	builder *crdtpatch.PatchBuilder

	// next is the index of the next operation in builder's patch to be committed locally.
	next int

	// mutex protects access to the document and builder.
	mutex sync.RWMutex

	// callbacks for various events
	onBeforeReset   []func()
	onReset         []func()
	onBeforePatch   []func(*crdtpatch.Patch)
	onPatch         []func(*crdtpatch.Patch)
	onBeforeChange  []func(int)
	onChange        []func(int)
	onFlush         []func(*crdtpatch.Patch)
}

// NewModelApi creates a new ModelApi for the given document.
func NewModelApi(doc *crdt.Document) *ModelApi {
	return &ModelApi{
		doc:     doc,
		builder: crdtpatch.NewPatchBuilder(doc.GetSessionID(), doc.NextTimestamp().Counter),
		next:    0,
	}
}

// OnBeforeReset registers a callback to be called before the document is reset.
func (m *ModelApi) OnBeforeReset(callback func()) {
	m.onBeforeReset = append(m.onBeforeReset, callback)
}

// OnReset registers a callback to be called after the document is reset.
func (m *ModelApi) OnReset(callback func()) {
	m.onReset = append(m.onReset, callback)
}

// OnBeforePatch registers a callback to be called before a patch is applied.
func (m *ModelApi) OnBeforePatch(callback func(*crdtpatch.Patch)) {
	m.onBeforePatch = append(m.onBeforePatch, callback)
}

// OnPatch registers a callback to be called after a patch is applied.
func (m *ModelApi) OnPatch(callback func(*crdtpatch.Patch)) {
	m.onPatch = append(m.onPatch, callback)
}

// OnBeforeChange registers a callback to be called before local changes are applied.
func (m *ModelApi) OnBeforeChange(callback func(int)) {
	m.onBeforeChange = append(m.onBeforeChange, callback)
}

// OnChange registers a callback to be called after local changes are applied.
func (m *ModelApi) OnChange(callback func(int)) {
	m.onChange = append(m.onChange, callback)
}

// OnFlush registers a callback to be called when the builder's change buffer is flushed.
func (m *ModelApi) OnFlush(callback func(*crdtpatch.Patch)) {
	m.onFlush = append(m.onFlush, callback)
}

// emitBeforeReset emits the onBeforeReset event.
func (m *ModelApi) emitBeforeReset() {
	for _, callback := range m.onBeforeReset {
		callback()
	}
}

// emitReset emits the onReset event.
func (m *ModelApi) emitReset() {
	for _, callback := range m.onReset {
		callback()
	}
}

// emitBeforePatch emits the onBeforePatch event.
func (m *ModelApi) emitBeforePatch(patch *crdtpatch.Patch) {
	for _, callback := range m.onBeforePatch {
		callback(patch)
	}
}

// emitPatch emits the onPatch event.
func (m *ModelApi) emitPatch(patch *crdtpatch.Patch) {
	for _, callback := range m.onPatch {
		callback(patch)
	}
}

// emitBeforeChange emits the onBeforeChange event.
func (m *ModelApi) emitBeforeChange(from int) {
	for _, callback := range m.onBeforeChange {
		callback(from)
	}
}

// emitChange emits the onChange event.
func (m *ModelApi) emitChange(from int) {
	for _, callback := range m.onChange {
		callback(from)
	}
}

// emitFlush emits the onFlush event.
func (m *ModelApi) emitFlush(patch *crdtpatch.Patch) {
	for _, callback := range m.onFlush {
		callback(patch)
	}
}

// GetDocument returns the CRDT document being manipulated.
func (m *ModelApi) GetDocument() *crdt.Document {
	return m.doc
}

// Apply applies the pending operations in the builder to the document.
func (m *ModelApi) Apply() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get the operations to apply
	ops := m.builder.GetPatch().GetOperations()[m.next:]
	if len(ops) == 0 {
		return
	}

	// Emit the before change event
	m.emitBeforeChange(m.next)

	// Apply each operation
	for _, op := range ops {
		if err := op.Apply(m.doc); err != nil {
			// TODO: Handle error
			continue
		}
	}

	// Update the next index
	m.next = len(m.builder.GetPatch().GetOperations())

	// Emit the change event
	m.emitChange(m.next)
}

// Flush flushes the builder's change buffer and returns the patch.
func (m *ModelApi) Flush() *crdtpatch.Patch {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Apply any pending operations
	m.Apply()

	// Get the patch
	patch := m.builder.GetPatch()

	// Reset the builder
	m.builder = crdtpatch.NewPatchBuilder(m.doc.GetSessionID(), m.doc.NextTimestamp().Counter)
	m.next = 0

	// Emit the flush event
	m.emitFlush(patch)

	return patch
}

// View returns the current view of the document.
func (m *ModelApi) View() (interface{}, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.doc.View()
}

// Root sets the root value of the document.
func (m *ModelApi) Root(value interface{}) *ModelApi {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Create a new patch
	patchID := m.doc.NextTimestamp()
	patch := crdtpatch.NewPatch(patchID)

	// Create a constant node for the value
	valueID := m.doc.NextTimestamp()
	valueOp := &crdtpatch.NewOperation{
		ID:       valueID,
		NodeType: common.NodeTypeCon,
		Value:    value,
	}
	patch.AddOperation(valueOp)

	// Create a new object node if the value is a map
	if mapValue, ok := value.(map[string]interface{}); ok {
		objID := m.doc.NextTimestamp()
		objOp := &crdtpatch.NewOperation{
			ID:       objID,
			NodeType: common.NodeTypeObj,
		}
		patch.AddOperation(objOp)

		// Add each field to the object
		for key, val := range mapValue {
			// Create a constant node for the field value
			fieldValueID := m.doc.NextTimestamp()
			fieldValueOp := &crdtpatch.NewOperation{
				ID:       fieldValueID,
				NodeType: common.NodeTypeCon,
				Value:    val,
			}
			patch.AddOperation(fieldValueOp)

			// Add the field to the object
			insOp := &crdtpatch.InsOperation{
				ID:       m.doc.NextTimestamp(),
				TargetID: objID,
				Key:      key,
				Value:    fieldValueID,
			}
			patch.AddOperation(insOp)
		}

		// Set the root value to the object
		rootOp := &crdtpatch.InsOperation{
			ID:       m.doc.NextTimestamp(),
			TargetID: common.LogicalTimestamp{SID: common.SessionID{}, Counter: 0},
			Value:    objID,
		}
		patch.AddOperation(rootOp)
	} else {
		// Set the root value directly
		rootOp := &crdtpatch.InsOperation{
			ID:       m.doc.NextTimestamp(),
			TargetID: common.LogicalTimestamp{SID: common.SessionID{}, Counter: 0},
			Value:    valueID,
		}
		patch.AddOperation(rootOp)
	}

	// Apply the patch
	if err := patch.Apply(m.doc); err != nil {
		// TODO: Handle error
		return m
	}

	return m
}

// Wrap returns a node API for the given node.
func (m *ModelApi) Wrap(node crdt.Node) NodeApi {
	switch n := node.(type) {
	case *crdt.LWWValueNode:
		return &ValApi{node: n, api: m}
	case *crdt.LWWObjectNode:
		return &ObjApi{node: n, api: m}
	case *crdt.RGAStringNode:
		return &StrApi{node: n, api: m}
	case *crdt.ConstantNode:
		return &ConApi{node: n, api: m}
	default:
		// TODO: Handle other node types
		return nil
	}
}

// R returns the root node API.
func (m *ModelApi) R() *ValApi {
	return &ValApi{node: m.doc.Root().(*crdt.LWWValueNode), api: m}
}

// Obj returns an object node API for the node at the given path.
func (m *ModelApi) Obj(path []interface{}) *ObjApi {
	// Get the root node
	rootNode := m.doc.Root()

	// If the path is empty, return the root node
	if len(path) == 0 {
		// If the root is a LWWValueNode, get its value
		if lwwVal, ok := rootNode.(*crdt.LWWValueNode); ok {
			if lwwVal.NodeValue == nil {
				return nil
			}
			// If the value is an object node, return it
			if objNode, ok := lwwVal.NodeValue.(*crdt.LWWObjectNode); ok {
				return &ObjApi{node: objNode, api: m}
			}
		}
		return nil
	}

	// TODO: Implement path traversal
	return nil
}

// Str returns a string node API for the node at the given path.
func (m *ModelApi) Str(path []interface{}) *StrApi {
	// TODO: Implement path traversal
	return nil
}

// Val returns a value node API for the node at the given path.
func (m *ModelApi) Val(path []interface{}) *ValApi {
	// TODO: Implement path traversal
	return nil
}

// Con returns a constant node API for the node at the given path.
func (m *ModelApi) Con(path []interface{}) *ConApi {
	// TODO: Implement path traversal
	return nil
}
