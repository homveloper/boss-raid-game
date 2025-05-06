package crdtpatch

import (
	"encoding/json"

	"github.com/pkg/errors"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
)

// Patch represents a JSON CRDT Patch document.
type Patch struct {
	// id is the ID of the patch.
	id common.LogicalTimestamp

	// metadata is optional custom metadata.
	metadata map[string]interface{}

	// operations is the list of operations in the patch.
	operations []Operation
}

// NewPatch creates a new JSON CRDT Patch document.
func NewPatch(id common.LogicalTimestamp) *Patch {
	return &Patch{
		id:         id,
		metadata:   make(map[string]interface{}),
		operations: make([]Operation, 0),
	}
}

// ID returns the ID of the patch.
func (p *Patch) ID() common.LogicalTimestamp {
	return p.id
}

// Metadata returns the metadata of the patch.
func (p *Patch) Metadata() map[string]interface{} {
	return p.metadata
}

// SetMetadata sets the metadata of the patch.
func (p *Patch) SetMetadata(metadata map[string]interface{}) {
	p.metadata = metadata
}

// Operations returns the operations in the patch.
func (p *Patch) Operations() []Operation {
	return p.operations
}

// AddOperation adds an operation to the patch.
func (p *Patch) AddOperation(op Operation) {
	p.operations = append(p.operations, op)
}

// Apply applies the patch to the document.
func (p *Patch) Apply(doc *crdt.Document) error {
	for _, op := range p.operations {
		if err := op.Apply(doc); err != nil {
			return errors.Wrap(err, "failed to apply operation")
		}
	}
	return nil
}

// MarshalJSON implements the json.Marshaler interface.
// It uses the verbose format by default.
func (p *Patch) MarshalJSON() ([]byte, error) {
	return p.toVerboseJSON()
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// It uses the verbose format by default.
func (p *Patch) UnmarshalJSON(data []byte) error {
	return p.fromVerboseJSON(data)
}

// toVerboseJSON returns a verbose JSON representation of the patch.
func (p *Patch) toVerboseJSON() ([]byte, error) {
	// Updated structure to match the new format
	type verbosePatch struct {
		ID       common.LogicalTimestamp `json:"id"`
		Metadata map[string]interface{}  `json:"meta,omitempty"`
		Ops      []json.RawMessage       `json:"ops"`
	}

	ops := make([]json.RawMessage, len(p.operations))
	for i, op := range p.operations {
		opJSON, err := json.Marshal(op)
		if err != nil {
			return nil, err
		}
		ops[i] = opJSON
	}

	patch := verbosePatch{
		ID:       p.id,
		Metadata: p.metadata,
		Ops:      ops,
	}

	return json.Marshal(patch)
}

// fromVerboseJSON parses a verbose JSON representation of the patch.
func (p *Patch) fromVerboseJSON(data []byte) error {
	// Updated structure to match the new format
	var patch struct {
		ID       common.LogicalTimestamp `json:"id"`
		Metadata map[string]interface{}  `json:"meta,omitempty"`
		Ops      []json.RawMessage       `json:"ops"`
	}

	if err := json.Unmarshal(data, &patch); err != nil {
		return err
	}

	// Set the patch ID
	p.id = patch.ID
	p.metadata = patch.Metadata

	p.operations = make([]Operation, len(patch.Ops))
	for i, opJSON := range patch.Ops {
		var opmeta struct {
			Op string                  `json:"op"`
			ID common.LogicalTimestamp `json:"id"`
		}
		if err := json.Unmarshal(opJSON, &opmeta); err != nil {
			return err
		}

		opStr := opmeta.Op

		var opType common.OperationType
		if opStr == "nop" {
			opType = common.OperationTypeNop
		} else if opStr == "ins" {
			opType = common.OperationTypeIns
		} else if opStr == "del" {
			opType = common.OperationTypeDel
		} else if len(opStr) >= 4 && opStr[:4] == "new_" {
			opType = common.OperationTypeNew
		} else {
			return common.ErrInvalidOperation{Message: "invalid operation type: " + opStr}
		}

		opID := opmeta.ID

		op := MakeOperation(opType, opID)
		if op == nil {
			return common.ErrInvalidOperationType{Type: string(opType)}
		}

		if err := json.Unmarshal(opJSON, op); err != nil {
			return err
		}

		p.operations[i] = op
	}

	return nil
}

// toCompactJSON returns a compact JSON representation of the patch.
func (p *Patch) toCompactJSON() ([]byte, error) {
	// For now, we'll use the verbose format as a base
	verboseJSON, err := p.toVerboseJSON()
	if err != nil {
		return nil, err
	}

	// In a real implementation, this would compress the verbose format
	// For now, we'll just return the verbose format
	return verboseJSON, nil
}

// fromCompactJSON parses a compact JSON representation of the patch.
func (p *Patch) fromCompactJSON(data []byte) error {
	// For now, we'll assume the compact format is the same as the verbose format
	return p.fromVerboseJSON(data)
}

// toBinaryJSON returns a binary JSON representation of the patch.
func (p *Patch) toBinaryJSON() ([]byte, error) {
	// For now, we'll use the verbose format as a base
	verboseJSON, err := p.toVerboseJSON()
	if err != nil {
		return nil, err
	}

	// In a real implementation, this would convert to a binary format
	// For now, we'll just return the verbose format
	return verboseJSON, nil
}

// fromBinaryJSON parses a binary JSON representation of the patch.
func (p *Patch) fromBinaryJSON(data []byte) error {
	// For now, we'll assume the binary format is the same as the verbose format
	return p.fromVerboseJSON(data)
}

// RewriteTime creates a new patch with the same operations but with a new timestamp.
// The new timestamp is used as the base for all operations in the patch.
func (p *Patch) RewriteTime(newID common.LogicalTimestamp) *Patch {
	// Create a new patch with the new ID
	newPatch := NewPatch(newID)

	// Copy metadata
	for k, v := range p.metadata {
		newPatch.metadata[k] = v
	}

	// Calculate the offset between the old and new IDs
	offsetCounter := int64(newID.Counter) - int64(p.id.Counter)

	// Copy operations with adjusted IDs
	for _, op := range p.operations {
		// Create a new operation of the same type
		newOp := MakeOperation(op.Type(), common.LogicalTimestamp{})

		// Copy the operation data
		switch origOp := op.(type) {
		case *NewOperation:
			newNewOp := newOp.(*NewOperation)
			newNewOp.NodeType = origOp.NodeType
			newNewOp.Value = origOp.Value

			// Adjust the ID
			newID := common.LogicalTimestamp{
				SID:     newID.SID,
				Counter: uint64(int64(origOp.ID.Counter) + offsetCounter),
			}
			newNewOp.ID = newID

		case *InsOperation:
			newInsOp := newOp.(*InsOperation)
			newInsOp.Value = origOp.Value

			// Adjust the ID
			newID := common.LogicalTimestamp{
				SID:     newID.SID,
				Counter: uint64(int64(origOp.ID.Counter) + offsetCounter),
			}
			newInsOp.ID = newID

			// If the target ID is from the same session, adjust it too
			if origOp.TargetID.SID.Compare(p.id.SID) == 0 {
				newTargetID := common.LogicalTimestamp{
					SID:     newID.SID,
					Counter: uint64(int64(origOp.TargetID.Counter) + offsetCounter),
				}
				newInsOp.TargetID = newTargetID
			} else {
				// Otherwise, keep the original target ID
				newInsOp.TargetID = origOp.TargetID
			}

		case *DelOperation:
			newDelOp := newOp.(*DelOperation)
			newDelOp.Key = origOp.Key

			// Adjust the ID
			newID := common.LogicalTimestamp{
				SID:     newID.SID,
				Counter: uint64(int64(origOp.ID.Counter) + offsetCounter),
			}
			newDelOp.ID = newID

			// If the target ID is from the same session, adjust it too
			if origOp.TargetID.SID.Compare(p.id.SID) == 0 {
				newTargetID := common.LogicalTimestamp{
					SID:     newID.SID,
					Counter: uint64(int64(origOp.TargetID.Counter) + offsetCounter),
				}
				newDelOp.TargetID = newTargetID
			} else {
				// Otherwise, keep the original target ID
				newDelOp.TargetID = origOp.TargetID
			}

			// Adjust StartID and EndID if they are from the same session
			if origOp.StartID.SID.Compare(p.id.SID) == 0 {
				newStartID := common.LogicalTimestamp{
					SID:     newID.SID,
					Counter: uint64(int64(origOp.StartID.Counter) + offsetCounter),
				}
				newDelOp.StartID = newStartID
			} else {
				// Otherwise, keep the original StartID
				newDelOp.StartID = origOp.StartID
			}

			if origOp.EndID.SID.Compare(p.id.SID) == 0 {
				newEndID := common.LogicalTimestamp{
					SID:     newID.SID,
					Counter: uint64(int64(origOp.EndID.Counter) + offsetCounter),
				}
				newDelOp.EndID = newEndID
			} else {
				// Otherwise, keep the original EndID
				newDelOp.EndID = origOp.EndID
			}

		case *NopOperation:
			newNopOp := newOp.(*NopOperation)
			newNopOp.SpanValue = origOp.SpanValue

			// Adjust the ID
			newID := common.LogicalTimestamp{
				SID:     newID.SID,
				Counter: uint64(int64(origOp.ID.Counter) + offsetCounter),
			}
			newNopOp.ID = newID
		}

		// Add the new operation to the new patch
		newPatch.AddOperation(newOp)
	}

	return newPatch
}

// Clone creates a copy of the patch with the same ID and operations.
func (p *Patch) Clone() *Patch {
	// Use RewriteTime with the current patch ID to create a clone
	return p.RewriteTime(p.id)
}
