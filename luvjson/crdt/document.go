package crdt

import (
	"encoding/json"
	"fmt"

	"tictactoe/luvjson/common"
)

// Document represents a JSON CRDT document.
type Document struct {
	// root is the root node of the document.
	root Node

	// index maps node IDs to nodes.
	index map[common.LogicalTimestamp]Node

	// clock is the logical clock of the documedtnt.
	clock map[string]uint64

	// localSessionID is the session ID of the local user.
	localSessionID common.SessionID
}

// NewDocument creates a new JSON CRDT document.
func NewDocument(sessionID common.SessionID) *Document {

	doc := &Document{
		index:          make(map[common.LogicalTimestamp]Node),
		clock:          make(map[string]uint64),
		localSessionID: sessionID,
	}

	// Create the root node pointing to the constant node's ID
	rootNode := NewRootNode(common.RootID)
	doc.root = rootNode
	doc.index[common.RootID] = rootNode

	return doc
}

// Root returns the root node of the document.
func (d *Document) Root() Node {
	return d.root
}

// GetNode returns the node with the specified ID.
func (d *Document) GetNode(id common.LogicalTimestamp) (Node, error) {
	// Check if this is the root node ID
	if id.Compare(common.RootID) == 0 {
		// Return the root node directly
		return d.root, nil
	}

	// Otherwise, look up in the index
	node, ok := d.index[id]
	if !ok {
		return nil, common.ErrNodeNotFound{ID: id}
	}
	return node, nil
}

// AddNode adds a node to the document.
func (d *Document) AddNode(node Node) {
	d.index[node.ID()] = node

	// Update the clock
	sessionID := node.ID().SID
	counter := node.ID().Counter
	// Use string representation of UUID as map key
	sidStr := sessionID.String()
	if currentCounter, ok := d.clock[sidStr]; !ok || counter > currentCounter {
		d.clock[sidStr] = counter
	}
}

// NextTimestamp returns the next logical timestamp for the local session.
func (d *Document) NextTimestamp() common.LogicalTimestamp {
	// Use string representation of UUID as map key
	sidStr := d.localSessionID.String()
	counter := d.clock[sidStr] + 1
	d.clock[sidStr] = counter
	return common.LogicalTimestamp{
		SID:     d.localSessionID,
		Counter: counter,
	}
}

// GetSessionID returns the local session ID of the document.
func (d *Document) GetSessionID() common.SessionID {
	return d.localSessionID
}

// GetSessionIDString returns the string representation of the local session ID.
func (d *Document) GetSessionIDString() string {
	return d.localSessionID.String()
}

// View returns a JSON view of the document.
func (d *Document) View() (interface{}, error) {
	if d.root == nil {
		return nil, nil
	}

	// If the root is a LWWValueNode, get the value of its value
	if lwwVal, ok := d.root.(*LWWValueNode); ok {
		if lwwVal.NodeValue == nil {
			return nil, nil
		}
		return lwwVal.NodeValue.Value(), nil
	}

	return d.root.Value(), nil
}

// MarshalJSON implements the json.Marshaler interface.
// It uses the verbose format by default.
func (d *Document) MarshalJSON() ([]byte, error) {
	return d.toVerboseJSON()
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// It uses the verbose format by default.
func (d *Document) UnmarshalJSON(data []byte) error {
	return d.fromVerboseJSON(data)
}

// toVerboseJSON returns a verbose JSON representation of the document.
func (d *Document) toVerboseJSON() ([]byte, error) {
	// For now, we'll implement a simplified version
	type verboseDoc struct {
		Time map[string]uint64 `json:"time"`
		Root json.RawMessage   `json:"root"`
	}

	// Clock is already map[string]uint64
	timeMap := d.clock

	// Get root node JSON
	rootJSON, err := json.Marshal(d.root)
	if err != nil {
		return nil, err
	}

	doc := verboseDoc{
		Time: timeMap,
		Root: rootJSON,
	}

	return json.Marshal(doc)
}

// fromVerboseJSON parses a verbose JSON representation of the document.
func (d *Document) fromVerboseJSON(data []byte) error {
	type verboseDoc struct {
		Time map[string]uint64 `json:"time"`
		Root json.RawMessage   `json:"root"`
	}

	var doc verboseDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}

	// Parse the clock
	d.clock = doc.Time

	// Parse the root node
	var rootType struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(doc.Root, &rootType); err != nil {
		return err
	}

	// Create the appropriate node type based on the type field
	var root Node
	switch common.NodeType(rootType.Type) {
	case common.NodeTypeRoot:
		root = &RootNode{}
	case common.NodeTypeVal:
		root = &LWWValueNode{}
	case common.NodeTypeObj:
		root = &LWWObjectNode{}
	case common.NodeTypeCon:
		root = &ConstantNode{}
	case common.NodeTypeStr:
		root = &RGAStringNode{}
	default:
		return common.ErrInvalidNodeType{Type: rootType.Type}
	}

	// Unmarshal the JSON data into the node
	// Since each node type implements json.Unmarshaler, this will call the appropriate UnmarshalJSON method
	if err := json.Unmarshal(doc.Root, root); err != nil {
		return err
	}

	d.root = root
	d.index = make(map[common.LogicalTimestamp]Node)
	d.index[root.ID()] = root

	// Parse other nodes recursively
	// For LWWValueNode, we need to extract the value and add it to the index
	if lwwNode, ok := root.(*LWWValueNode); ok && lwwNode.NodeValue != nil {
		// Add the value to the index
		d.index[lwwNode.NodeValue.ID()] = lwwNode.NodeValue

		// Recursively parse the value node
		if err := d.parseNodeRecursively(lwwNode.NodeValue); err != nil {
			return err
		}
	}

	// For LWWObjectNode, we need to extract all fields and add them to the index
	if objNode, ok := root.(*LWWObjectNode); ok {
		// Parse all fields in the object
		for _, key := range objNode.Keys() {
			fieldValue := objNode.Get(key)
			if fieldValue != nil {
				d.index[fieldValue.ID()] = fieldValue

				// Recursively parse the field value
				if err := d.parseNodeRecursively(fieldValue); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// parseNodeRecursively parses a node and its children recursively.
func (d *Document) parseNodeRecursively(node Node) error {
	// Skip nil nodes
	if node == nil {
		return nil
	}

	// Skip root nodes (they're already in the index)
	if node.IsRoot() {
		return nil
	}

	// Process based on node type
	switch n := node.(type) {
	case *LWWValueNode:
		// If the value node has a value, add it to the index
		if n.NodeValue != nil {
			d.index[n.NodeValue.ID()] = n.NodeValue

			// Recursively parse the value node
			return d.parseNodeRecursively(n.NodeValue)
		}
	case *LWWObjectNode:
		// Parse all fields in the object
		for _, key := range n.Keys() {
			fieldValue := n.Get(key)
			if fieldValue != nil {
				d.index[fieldValue.ID()] = fieldValue

				// Recursively parse the field value
				if err := d.parseNodeRecursively(fieldValue); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// ㅖatchOperation represents a JSON CRDT patch operation.
type PatchOperation struct {
	Op        string                  `json:"op"`
	ID        common.LogicalTimestamp `json:"id"`
	TargetID  common.LogicalTimestamp `json:"target,omitempty"`
	NodeType  common.NodeType         `json:"type,omitempty"`
	Value     interface{}             `json:"value,omitempty"`
	Key       string                  `json:"key,omitempty"`
	StartID   common.LogicalTimestamp `json:"start,omitempty"`
	EndID     common.LogicalTimestamp `json:"end,omitempty"`
	SpanValue uint64                  `json:"len,omitempty"`
}

// Patch represents a JSON CRDT Patch document.
type Patch struct {
	ID         common.LogicalTimestamp `json:"id"`
	Metadata   map[string]interface{}  `json:"meta,omitempty"`
	Operations []json.RawMessage       `json:"ops"`
}

// ApplyPatch applies a JSON CRDT patch to the document.
// The patch data should be a []byte representation of a JSON CRDT patch.
// The default format is verbose.
func (d *Document) ApplyPatch(patchData []byte) error {
	// Unmarshal the patch data
	var p Patch
	if err := json.Unmarshal(patchData, &p); err != nil {
		return fmt.Errorf("failed to unmarshal patch: %w", err)
	}

	// Process each operation
	for _, opData := range p.Operations {
		// First, determine the operation type
		var opType struct {
			Op string `json:"op"`
		}
		if err := json.Unmarshal(opData, &opType); err != nil {
			return fmt.Errorf("failed to unmarshal operation type: %w", err)
		}

		// Parse the operation
		var op PatchOperation
		if err := json.Unmarshal(opData, &op); err != nil {
			return fmt.Errorf("failed to unmarshal operation: %w", err)
		}

		// Apply the operation
		if err := d.applyOperation(common.OperationType(opType.Op), &op); err != nil {
			return err
		}
	}

	return nil
}

// applyOperation applies a single operation to the document.
func (d *Document) applyOperation(opType common.OperationType, op *PatchOperation) error {
	// IDs are already LogicalTimestamp, so no conversion needed
	opID := op.ID
	targetID := op.TargetID
	startID := op.StartID
	endID := op.EndID

	// Apply the operation based on its type
	switch opType {
	case common.OperationTypeNew:
		// Create a new node
		var node Node
		switch op.NodeType {
		case common.NodeTypeCon:
			node = NewConstantNode(opID, op.Value)
		case common.NodeTypeVal:
			node = NewLWWValueNode(opID, opID, NewConstantNode(opID, nil))
		case common.NodeTypeObj:
			node = NewLWWObjectNode(opID)
		case common.NodeTypeStr:
			node = NewRGAStringNode(opID)
		case common.NodeTypeRoot:
			node = NewRootNode(opID)
		default:
			return common.ErrInvalidNodeType{Type: string(op.NodeType)}
		}
		d.AddNode(node)

	case common.OperationTypeIns:
		// Update an existing node
		target, err := d.GetNode(targetID)
		if err != nil {
			return fmt.Errorf("failed to get target node: %w", err)
		}

		switch node := target.(type) {
		case *RootNode:
			// Update the root node's value
			valueNode := NewConstantNode(opID, op.Value)
			node.NodeValue = valueNode
			d.AddNode(valueNode)
		case *LWWValueNode:
			// Update the value
			valueNode := NewConstantNode(opID, op.Value)
			node.SetValue(opID, valueNode)
			d.AddNode(valueNode)
		case *LWWObjectNode:
			// Update a field
			if obj, ok := op.Value.(map[string]interface{}); ok {
				for key, val := range obj {
					valueNode := NewConstantNode(opID, val)
					node.Set(key, opID, valueNode)
					d.AddNode(valueNode)
				}
			} else if op.Key != "" {
				valueNode := NewConstantNode(opID, op.Value)
				node.Set(op.Key, opID, valueNode)
				d.AddNode(valueNode)
			}
		case *RGAStringNode:
			// Insert a string
			if str, ok := op.Value.(string); ok {
				node.Insert(targetID, opID, str)
			}
		default:
			return common.ErrInvalidOperation{Message: "unsupported node type for 'ins' operation"}
		}

	case common.OperationTypeDel:
		// Delete contents from an existing node
		target, err := d.GetNode(targetID)
		if err != nil {
			return fmt.Errorf("failed to get target node: %w", err)
		}

		switch node := target.(type) {
		case *LWWObjectNode:
			// Delete a field
			node.Delete(op.Key, opID)
		case *RGAStringNode:
			// Delete a range of characters
			node.Delete(startID, endID)
		default:
			return common.ErrInvalidOperation{Message: "unsupported node type for 'del' operation"}
		}

	case common.OperationTypeNop:
		// No-op operation, do nothing

	default:
		return common.ErrInvalidOperationType{Type: string(opType)}
	}

	return nil
}
