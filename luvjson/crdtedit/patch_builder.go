package crdtedit

import (
	"github.com/pkg/errors"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdtpatch"
)

// PatchBuilder provides methods for building patches
type PatchBuilder struct {
	operations []crdtpatch.Operation
	sessionID  common.SessionID
	counter    uint64
}

// NewPatchBuilder creates a new PatchBuilder
func NewPatchBuilder(sessionID common.SessionID) *PatchBuilder {
	return &PatchBuilder{
		operations: make([]crdtpatch.Operation, 0),
		sessionID:  sessionID,
		counter:    0,
	}
}

// NextID generates the next ID for operations
func (b *PatchBuilder) NextID() common.LogicalTimestamp {
	b.counter++
	return common.LogicalTimestamp{
		SID:     b.sessionID,
		Counter: b.counter,
	}
}

// AddSetOperation adds a set operation to the patch
func (b *PatchBuilder) AddSetOperation(targetID common.LogicalTimestamp, value any) error {
	// If value is already a NodeID, use it directly
	if nodeID, ok := value.(common.LogicalTimestamp); ok {
		op, err := crdtpatch.NewSetOperation(targetID, nodeID)
		if err != nil {
			return errors.Wrap(err, "failed to create set operation")
		}
		b.operations = append(b.operations, op)
		return nil
	}

	// Otherwise, create a new node for the value
	nodeID := b.NextID()

	// First, create a new node operation
	newOp := &crdtpatch.NewOperation{
		ID:       nodeID,
		NodeType: common.NodeTypeCon,
		Value:    value,
	}
	b.operations = append(b.operations, newOp)

	// Create a set operation using crdtpatch
	op, err := crdtpatch.NewSetOperation(targetID, nodeID)
	if err != nil {
		return errors.Wrap(err, "failed to create set operation")
	}

	b.operations = append(b.operations, op)
	return nil
}

// AddObjectInsertOperation adds an object insert operation to the patch
func (b *PatchBuilder) AddObjectInsertOperation(targetID common.LogicalTimestamp, key string, value any) error {
	// If value is already a NodeID, use it directly
	if nodeID, ok := value.(common.LogicalTimestamp); ok {
		op, err := crdtpatch.NewObjectInsertOperation(targetID, key, nodeID)
		if err != nil {
			return errors.Wrap(err, "failed to create object insert operation")
		}
		b.operations = append(b.operations, op)
		return nil
	}

	// Otherwise, create a new node for the value
	nodeID := b.NextID()

	// First, create a new node operation
	newOp := &crdtpatch.NewOperation{
		ID:       nodeID,
		NodeType: common.NodeTypeCon,
		Value:    value,
	}
	b.operations = append(b.operations, newOp)

	// Then, create the object insert operation
	op, err := crdtpatch.NewObjectInsertOperation(targetID, key, nodeID)
	if err != nil {
		return errors.Wrap(err, "failed to create object insert operation")
	}

	b.operations = append(b.operations, op)
	return nil
}

// AddObjectDeleteOperation adds an object delete operation to the patch
func (b *PatchBuilder) AddObjectDeleteOperation(targetID common.LogicalTimestamp, key string) error {
	op, err := crdtpatch.NewObjectDeleteOperation(targetID, key)
	if err != nil {
		return errors.Wrap(err, "failed to create object delete operation")
	}

	b.operations = append(b.operations, op)
	return nil
}

// AddArrayInsertOperation adds an array insert operation to the patch
func (b *PatchBuilder) AddArrayInsertOperation(targetID common.LogicalTimestamp, index int, value any) error {
	// If value is already a NodeID, use it directly
	if nodeID, ok := value.(common.LogicalTimestamp); ok {
		op, err := crdtpatch.NewArrayInsertOperation(targetID, index, nodeID)
		if err != nil {
			return errors.Wrap(err, "failed to create array insert operation")
		}
		b.operations = append(b.operations, op)
		return nil
	}

	// Otherwise, create a new node for the value
	nodeID := b.NextID()

	// First, create a new node operation
	newOp := &crdtpatch.NewOperation{
		ID:       nodeID,
		NodeType: common.NodeTypeCon,
		Value:    value,
	}
	b.operations = append(b.operations, newOp)

	op, err := crdtpatch.NewArrayInsertOperation(targetID, index, nodeID)
	if err != nil {
		return errors.Wrap(err, "failed to create array insert operation")
	}

	b.operations = append(b.operations, op)
	return nil
}

// AddArrayDeleteOperation adds an array delete operation to the patch
func (b *PatchBuilder) AddArrayDeleteOperation(targetID common.LogicalTimestamp, index int) error {
	op, err := crdtpatch.NewArrayDeleteOperation(targetID, index)
	if err != nil {
		return errors.Wrap(err, "failed to create array delete operation")
	}

	b.operations = append(b.operations, op)
	return nil
}

// AddStringInsertOperation adds a string insert operation to the patch
func (b *PatchBuilder) AddStringInsertOperation(targetID common.LogicalTimestamp, index int, text string) error {
	op, err := crdtpatch.NewStringInsertOperation(targetID, index, text)
	if err != nil {
		return errors.Wrap(err, "failed to create string insert operation")
	}

	b.operations = append(b.operations, op)
	return nil
}

// AddStringDeleteOperation adds a string delete operation to the patch
func (b *PatchBuilder) AddStringDeleteOperation(targetID common.LogicalTimestamp, start, end int) error {
	op, err := crdtpatch.NewStringDeleteOperation(targetID, start, end)
	if err != nil {
		return errors.Wrap(err, "failed to create string delete operation")
	}

	b.operations = append(b.operations, op)
	return nil
}

// Build returns the operations in the patch
func (b *PatchBuilder) Build() []crdtpatch.Operation {
	return b.operations
}

// CreatePatch creates a new patch from the operations
func (b *PatchBuilder) CreatePatch(id common.LogicalTimestamp) *crdtpatch.Patch {
	patch := crdtpatch.NewPatch(id)
	for _, op := range b.operations {
		patch.AddOperation(op)
	}
	return patch
}
