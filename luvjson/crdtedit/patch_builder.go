package crdtedit

import (
	"fmt"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
)

// PatchBuilder provides methods for building patches
type PatchBuilder struct {
	doc        *crdt.Document
	operations []crdtpatch.Operation
}

// NewPatchBuilder creates a new PatchBuilder
func NewPatchBuilder(doc *crdt.Document) *PatchBuilder {
	return &PatchBuilder{
		doc:        doc,
		operations: make([]crdtpatch.Operation, 0),
	}
}

// AddSetOperation adds a set operation to the patch
func (b *PatchBuilder) AddSetOperation(targetID common.LogicalTimestamp, value any) error {
	// Create a set operation using crdtpatch
	op, err := crdtpatch.NewSetOperation(targetID, value)
	if err != nil {
		return fmt.Errorf("failed to create set operation: %w", err)
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
			return fmt.Errorf("failed to create object insert operation: %w", err)
		}
		b.operations = append(b.operations, op)
		return nil
	}

	// Otherwise, create a new node for the value
	valueID, err := b.createNodeForValue(value)
	if err != nil {
		return fmt.Errorf("failed to create node for value: %w", err)
	}

	op, err := crdtpatch.NewObjectInsertOperation(targetID, key, valueID)
	if err != nil {
		return fmt.Errorf("failed to create object insert operation: %w", err)
	}

	b.operations = append(b.operations, op)
	return nil
}

// AddObjectDeleteOperation adds an object delete operation to the patch
func (b *PatchBuilder) AddObjectDeleteOperation(targetID common.LogicalTimestamp, key string) error {
	op, err := crdtpatch.NewObjectDeleteOperation(targetID, key)
	if err != nil {
		return fmt.Errorf("failed to create object delete operation: %w", err)
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
			return fmt.Errorf("failed to create array insert operation: %w", err)
		}
		b.operations = append(b.operations, op)
		return nil
	}

	// Otherwise, create a new node for the value
	valueID, err := b.createNodeForValue(value)
	if err != nil {
		return fmt.Errorf("failed to create node for value: %w", err)
	}

	op, err := crdtpatch.NewArrayInsertOperation(targetID, index, valueID)
	if err != nil {
		return fmt.Errorf("failed to create array insert operation: %w", err)
	}

	b.operations = append(b.operations, op)
	return nil
}

// AddArrayDeleteOperation adds an array delete operation to the patch
func (b *PatchBuilder) AddArrayDeleteOperation(targetID common.LogicalTimestamp, index int) error {
	op, err := crdtpatch.NewArrayDeleteOperation(targetID, index)
	if err != nil {
		return fmt.Errorf("failed to create array delete operation: %w", err)
	}

	b.operations = append(b.operations, op)
	return nil
}

// AddStringInsertOperation adds a string insert operation to the patch
func (b *PatchBuilder) AddStringInsertOperation(targetID common.LogicalTimestamp, index int, text string) error {
	op, err := crdtpatch.NewStringInsertOperation(targetID, index, text)
	if err != nil {
		return fmt.Errorf("failed to create string insert operation: %w", err)
	}

	b.operations = append(b.operations, op)
	return nil
}

// AddStringDeleteOperation adds a string delete operation to the patch
func (b *PatchBuilder) AddStringDeleteOperation(targetID common.LogicalTimestamp, start, end int) error {
	op, err := crdtpatch.NewStringDeleteOperation(targetID, start, end)
	if err != nil {
		return fmt.Errorf("failed to create string delete operation: %w", err)
	}

	b.operations = append(b.operations, op)
	return nil
}

// Build returns the operations in the patch
func (b *PatchBuilder) Build() []crdtpatch.Operation {
	return b.operations
}

// Apply applies the operations in the patch to the document
func (b *PatchBuilder) Apply() error {
	for _, op := range b.operations {
		if err := b.doc.ApplyOperation(op); err != nil {
			return fmt.Errorf("failed to apply operation: %w", err)
		}
	}
	return nil
}

// createNodeForValue creates a new node for a value
func (b *PatchBuilder) createNodeForValue(value any) (common.LogicalTimestamp, error) {
	switch v := value.(type) {
	case nil:
		return b.doc.CreateNull()
	case bool:
		return b.doc.CreateBoolean(v)
	case string:
		return b.doc.CreateString(v)
	case float64:
		return b.doc.CreateNumber(v)
	case int:
		return b.doc.CreateNumber(float64(v))
	case int64:
		return b.doc.CreateNumber(float64(v))
	case map[string]any:
		objID, err := b.doc.CreateObject()
		if err != nil {
			return common.NilID, err
		}
		for k, v := range v {
			if err := b.AddObjectInsertOperation(objID, k, v); err != nil {
				return common.NilID, err
			}
		}
		return objID, nil
	case []any:
		arrID, err := b.doc.CreateArray()
		if err != nil {
			return common.NilID, err
		}
		for i, v := range v {
			if err := b.AddArrayInsertOperation(arrID, i, v); err != nil {
				return common.NilID, err
			}
		}
		return arrID, nil
	default:
		return common.NilID, fmt.Errorf("unsupported value type: %T", value)
	}
}
