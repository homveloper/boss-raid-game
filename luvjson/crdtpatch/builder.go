package crdtpatch

import (
	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
)

// PatchBuilder is a helper for building JSON CRDT patches.
// It maintains a logical clock and automatically assigns IDs to operations.
type PatchBuilder struct {
	// sessionID is the session ID for the builder.
	sessionID common.SessionID

	// counter is the current counter value for the builder.
	counter uint64

	// currentPatch is the patch being built.
	currentPatch *Patch

	// pendingOperations is the list of operations to be added to the next patch.
	pendingOperations []Operation
}

// NewPatchBuilder creates a new PatchBuilder with the given session ID and initial counter.
func NewPatchBuilder(sessionID common.SessionID, initialCounter uint64) *PatchBuilder {
	return &PatchBuilder{
		sessionID:         sessionID,
		counter:           initialCounter,
		pendingOperations: make([]Operation, 0),
	}
}

// CurrentTimestamp returns the current logical timestamp.
func (b *PatchBuilder) CurrentTimestamp() common.LogicalTimestamp {
	return common.LogicalTimestamp{
		SID:     b.sessionID,
		Counter: b.counter,
	}
}

// NextTimestamp returns the next logical timestamp and increments the counter.
func (b *PatchBuilder) NextTimestamp() common.LogicalTimestamp {
	ts := b.CurrentTimestamp()
	b.counter++
	return ts
}

// NextTimestampWithSpan returns the next logical timestamp with the given span and increments the counter.
func (b *PatchBuilder) NextTimestampWithSpan(span uint64) common.LogicalTimestamp {
	ts := b.CurrentTimestamp()
	b.counter += span
	return ts
}

// AddOperation adds an operation to the pending operations list.
func (b *PatchBuilder) AddOperation(op Operation) {
	b.pendingOperations = append(b.pendingOperations, op)
}

// NewConstant creates a new constant node operation.
func (b *PatchBuilder) NewConstant(value interface{}) *NewOperation {
	op := &NewOperation{
		ID:       b.NextTimestamp(),
		NodeType: common.NodeTypeCon,
		Value:    value,
	}
	b.AddOperation(op)
	return op
}

// NewValue creates a new LWW-Value node operation.
func (b *PatchBuilder) NewValue() *NewOperation {
	op := &NewOperation{
		ID:       b.NextTimestamp(),
		NodeType: common.NodeTypeVal,
	}
	b.AddOperation(op)
	return op
}

// NewObject creates a new LWW-Object node operation.
func (b *PatchBuilder) NewObject() *NewOperation {
	op := &NewOperation{
		ID:       b.NextTimestamp(),
		NodeType: common.NodeTypeObj,
	}
	b.AddOperation(op)
	return op
}

// NewString creates a new RGA-String node operation.
func (b *PatchBuilder) NewString() *NewOperation {
	op := &NewOperation{
		ID:       b.NextTimestamp(),
		NodeType: common.NodeTypeStr,
	}
	b.AddOperation(op)
	return op
}

// InsertValue inserts a value into a LWW-Value node.
func (b *PatchBuilder) InsertValue(targetID common.LogicalTimestamp, value interface{}) *InsOperation {
	op := &InsOperation{
		ID:       b.NextTimestamp(),
		TargetID: targetID,
		Value:    value,
	}
	b.AddOperation(op)
	return op
}

// InsertObjectField inserts a field into a LWW-Object node.
func (b *PatchBuilder) InsertObjectField(targetID common.LogicalTimestamp, key string, value interface{}) *InsOperation {
	op := &InsOperation{
		ID:       b.NextTimestamp(),
		TargetID: targetID,
		Value:    map[string]interface{}{key: value},
	}
	b.AddOperation(op)
	return op
}

// InsertString inserts a string into a RGA-String node.
func (b *PatchBuilder) InsertString(targetID, refID common.LogicalTimestamp, value string) *InsOperation {
	op := &InsOperation{
		ID:       b.NextTimestamp(),
		TargetID: targetID,
		Value:    value,
	}
	b.AddOperation(op)
	return op
}

// DeleteObjectField deletes a field from a LWW-Object node.
func (b *PatchBuilder) DeleteObjectField(targetID common.LogicalTimestamp, key string) *DelOperation {
	op := &DelOperation{
		ID:       b.NextTimestamp(),
		TargetID: targetID,
		Key:      key,
	}
	b.AddOperation(op)
	return op
}

// DeleteStringRange deletes a range of characters from a RGA-String node.
func (b *PatchBuilder) DeleteStringRange(targetID, startID, endID common.LogicalTimestamp) *DelOperation {
	op := &DelOperation{
		ID:       b.NextTimestamp(),
		TargetID: targetID,
		StartID:  startID,
		EndID:    endID,
	}
	b.AddOperation(op)
	return op
}

// AddNop adds a no-op operation with the given span.
func (b *PatchBuilder) AddNop(span uint64) *NopOperation {
	op := &NopOperation{
		ID:        b.NextTimestampWithSpan(span),
		SpanValue: span,
	}
	b.AddOperation(op)
	return op
}

// Flush creates a new patch with the pending operations and clears the pending operations list.
func (b *PatchBuilder) Flush() *Patch {
	if len(b.pendingOperations) == 0 {
		return nil
	}

	// Create a new patch with the current timestamp
	patch := NewPatch(b.CurrentTimestamp())

	// Add the pending operations to the patch
	for _, op := range b.pendingOperations {
		patch.AddOperation(op)
	}

	// Clear the pending operations list
	b.pendingOperations = make([]Operation, 0)

	// Store the current patch
	b.currentPatch = patch

	return patch
}

// CurrentPatch returns the current patch.
func (b *PatchBuilder) CurrentPatch() *Patch {
	return b.currentPatch
}

// BuildFromDocument builds a patch that creates the given document.
// This is useful for initializing a document on a remote peer.
func (b *PatchBuilder) BuildFromDocument(doc *crdt.Document) *Patch {
	// Start with the root node
	rootNode := doc.Root()
	if rootNode == nil {
		return b.Flush()
	}

	// Process the root node
	b.processNode(doc, rootNode)

	return b.Flush()
}

// processNode processes a node and its children recursively.
func (b *PatchBuilder) processNode(doc *crdt.Document, node crdt.Node) {
	// Skip nil nodes
	if node == nil {
		return
	}

	// Process based on node type
	switch n := node.(type) {
	case *crdt.ConstantNode:
		b.NewConstant(n.Value())

	case *crdt.LWWValueNode:
		valueOp := b.NewValue()
		if n.NodeValue != nil {
			// Process the value node
			b.InsertValue(valueOp.ID, n.NodeValue.Value())
			b.processNode(doc, n.NodeValue)
		}

	case *crdt.LWWObjectNode:
		objOp := b.NewObject()
		// Process all fields in the object
		for _, key := range n.Keys() {
			fieldValue := n.Get(key)
			if fieldValue != nil {
				b.InsertObjectField(objOp.ID, key, fieldValue.Value())
				b.processNode(doc, fieldValue)
			}
		}

	case *crdt.RGAStringNode:
		strOp := b.NewString()
		// For simplicity, we'll just insert the entire string at once
		strValue, ok := n.Value().(string)
		if ok && strValue != "" {
			b.InsertString(strOp.ID, strOp.ID, strValue)
		}
	}
}
