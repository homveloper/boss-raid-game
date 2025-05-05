package crdt

import (
	"encoding/json"
	"fmt"
	"tictactoe/luvjson/common"
)

// LWWVectorNode represents a Last-Write-Wins vector node.
// This is similar to LWWObjectNode but with integer indices instead of string keys.
type LWWVectorNode struct {
	NodeId     common.LogicalTimestamp    `json:"id"`
	NodeFields map[int]*LWWVectorField    `json:"fields,omitempty"`
}

// LWWVectorField represents a field in a LWW vector.
type LWWVectorField struct {
	NodeTimestamp common.LogicalTimestamp `json:"timestamp"`
	NodeValue     Node                    `json:"value"`
}

// NewLWWVectorNode creates a new LWW vector node.
func NewLWWVectorNode(id common.LogicalTimestamp) *LWWVectorNode {
	return &LWWVectorNode{
		NodeId:     id,
		NodeFields: make(map[int]*LWWVectorField),
	}
}

// ID returns the unique identifier of the node.
func (n *LWWVectorNode) ID() common.LogicalTimestamp {
	return n.NodeId
}

// Type returns the type of the node.
func (n *LWWVectorNode) Type() common.NodeType {
	return common.NodeTypeVec
}

// Value returns the value of the node.
func (n *LWWVectorNode) Value() interface{} {
	// Find the highest index to determine the array length
	maxIndex := -1
	for index := range n.NodeFields {
		if index > maxIndex {
			maxIndex = index
		}
	}

	// Create an array of the appropriate length
	result := make([]interface{}, maxIndex+1)

	// Fill in the values
	for index, field := range n.NodeFields {
		if index >= 0 && index < len(result) {
			result[index] = field.NodeValue.Value()
		}
	}

	return result
}

// IsRoot returns true if this is a root node.
func (n *LWWVectorNode) IsRoot() bool {
	// Check if the node has the RootID
	return n.NodeId.Compare(common.RootID) == 0
}

// Get returns the value at the specified index.
func (n *LWWVectorNode) Get(index int) Node {
	if field, ok := n.NodeFields[index]; ok {
		return field.NodeValue
	}
	return nil
}

// Set sets the value at the specified index.
func (n *LWWVectorNode) Set(index int, timestamp common.LogicalTimestamp, value Node) bool {
	field, ok := n.NodeFields[index]
	if !ok || timestamp.Compare(field.NodeTimestamp) > 0 {
		n.NodeFields[index] = &LWWVectorField{
			NodeTimestamp: timestamp,
			NodeValue:     value,
		}
		return true
	}
	return false
}

// Delete deletes the value at the specified index.
func (n *LWWVectorNode) Delete(index int, timestamp common.LogicalTimestamp) bool {
	field, ok := n.NodeFields[index]
	if ok && timestamp.Compare(field.NodeTimestamp) > 0 {
		delete(n.NodeFields, index)
		return true
	}
	return false
}

// Length returns the length of the vector.
func (n *LWWVectorNode) Length() int {
	// Find the highest index to determine the array length
	maxIndex := -1
	for index := range n.NodeFields {
		if index > maxIndex {
			maxIndex = index
		}
	}
	return maxIndex + 1
}

// Indices returns the indices of the vector.
func (n *LWWVectorNode) Indices() []int {
	indices := make([]int, 0, len(n.NodeFields))
	for index := range n.NodeFields {
		indices = append(indices, index)
	}
	return indices
}

// MarshalJSON returns a JSON representation of the node.
func (n *LWWVectorNode) MarshalJSON() ([]byte, error) {
	type jsonField struct {
		Timestamp common.LogicalTimestamp `json:"timestamp"`
		Value     json.RawMessage         `json:"value"`
	}

	type jsonNode struct {
		Type   string                  `json:"type"`
		ID     common.LogicalTimestamp `json:"id"`
		Fields map[string]jsonField    `json:"fields,omitempty"`
	}

	node := jsonNode{
		Type:   string(n.Type()),
		ID:     n.NodeId,
		Fields: make(map[string]jsonField),
	}

	for index, field := range n.NodeFields {
		valueJSON, err := json.Marshal(field.NodeValue)
		if err != nil {
			return nil, err
		}

		// Convert index to string for JSON
		indexStr := fmt.Sprintf("%d", index)
		node.Fields[indexStr] = jsonField{
			Timestamp: field.NodeTimestamp,
			Value:     valueJSON,
		}
	}

	return json.Marshal(node)
}

// UnmarshalJSON parses a JSON representation of the node.
func (n *LWWVectorNode) UnmarshalJSON(data []byte) error {
	type jsonField struct {
		Timestamp common.LogicalTimestamp `json:"timestamp"`
		Value     json.RawMessage         `json:"value"`
	}

	type jsonNode struct {
		Type   string                  `json:"type"`
		ID     common.LogicalTimestamp `json:"id"`
		Fields map[string]jsonField    `json:"fields,omitempty"`
	}

	var node jsonNode
	if err := json.Unmarshal(data, &node); err != nil {
		return err
	}

	if node.Type != string(common.NodeTypeVec) {
		return common.ErrInvalidNodeType{Type: node.Type}
	}

	n.NodeId = node.ID
	n.NodeFields = make(map[int]*LWWVectorField)

	// Parse fields
	for indexStr, field := range node.Fields {
		// Convert string index to int
		var index int
		if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
			return fmt.Errorf("invalid index: %s", indexStr)
		}

		// Parse the field value
		var valueType struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(field.Value, &valueType); err != nil {
			return err
		}

		// Create the appropriate node type based on the type field
		var fieldNode Node
		switch common.NodeType(valueType.Type) {
		case common.NodeTypeVal:
			fieldNode = &LWWValueNode{}
		case common.NodeTypeObj:
			fieldNode = &LWWObjectNode{}
		case common.NodeTypeCon:
			fieldNode = &ConstantNode{}
		case common.NodeTypeStr:
			fieldNode = &RGAStringNode{}
		case common.NodeTypeArr:
			fieldNode = &RGAArrayNode{}
		case common.NodeTypeVec:
			fieldNode = &LWWVectorNode{}
		case common.NodeTypeBin:
			fieldNode = &RGABinaryNode{}
		default:
			return common.ErrInvalidNodeType{Type: valueType.Type}
		}

		// Unmarshal the field value
		if err := json.Unmarshal(field.Value, fieldNode); err != nil {
			return err
		}

		// Create a new field
		n.NodeFields[index] = &LWWVectorField{
			NodeTimestamp: field.Timestamp,
			NodeValue:     fieldNode,
		}
	}

	return nil
}
