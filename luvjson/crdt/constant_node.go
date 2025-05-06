package crdt

import (
	"encoding/json"
	"tictactoe/luvjson/common"
)

// ConstantNode represents a constant value node.

// ConstantNode represents a constant value node.
type ConstantNode struct {
	NodeId    common.LogicalTimestamp `json:"id"`
	NodeValue interface{}             `json:"value"`
}

// NewConstantNode creates a new constant node.
func NewConstantNode(id common.LogicalTimestamp, value interface{}) *ConstantNode {
	return &ConstantNode{
		NodeId:    id,
		NodeValue: value,
	}
}

// ID returns the unique identifier of the node.
func (n *ConstantNode) ID() common.LogicalTimestamp {
	return n.NodeId
}

// Type returns the type of the node.
func (n *ConstantNode) Type() common.NodeType {
	return common.NodeTypeCon
}

// IsRoot returns true if this is a root node.
func (n *ConstantNode) IsRoot() bool {
	// Check if the node has the common.RootID
	return n.NodeId.Compare(common.RootID) == 0
}

// Value returns the value of the node.
func (n *ConstantNode) Value() interface{} {
	return n.NodeValue
}

// MarshalJSON returns a JSON representation of the node.
func (n *ConstantNode) MarshalJSON() ([]byte, error) {
	type jsonNode struct {
		Type  string                  `json:"type"`
		ID    common.LogicalTimestamp `json:"id"`
		Value interface{}             `json:"value"`
	}

	node := jsonNode{
		Type:  string(n.Type()),
		ID:    n.NodeId,
		Value: n.NodeValue,
	}

	return json.Marshal(node)
}

// UnmarshalJSON parses a JSON representation of the node.
func (n *ConstantNode) UnmarshalJSON(data []byte) error {
	type jsonNode struct {
		Type  string                  `json:"type"`
		ID    common.LogicalTimestamp `json:"id"`
		Value interface{}             `json:"value"`
	}

	var node jsonNode
	if err := json.Unmarshal(data, &node); err != nil {
		return err
	}

	if node.Type != string(common.NodeTypeCon) {
		return common.ErrInvalidNodeType{Type: node.Type}
	}

	n.NodeId = node.ID
	n.NodeValue = node.Value

	return nil
}
