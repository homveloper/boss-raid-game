package crdt

import (
	"encoding/json"
	"tictactoe/luvjson/common"
)

// RootNode represents the root node of a JSON CRDT document.
// It is a special LWWValueNode with a fixed ID of {SID: NilSessionID, Counter: 0}.
type RootNode struct {
	LWWValueNode
}

// NewRootNode creates a new root node with the given value timestamp.
// The valueTimestamp is the ID of the node that this root node points to.
func NewRootNode(valueTimestamp common.LogicalTimestamp) *RootNode {
	return &RootNode{
		LWWValueNode: LWWValueNode{
			NodeId:        common.RootID,
			NodeTimestamp: common.RootID,
			NodeValue:     NewConstantNode(valueTimestamp, nil), // Will be set separately after the node is created
		},
	}
}

// Type returns the type of the node.
func (n *RootNode) Type() common.NodeType {
	return common.NodeTypeRoot
}

// IsRoot always returns true for RootNode.
func (n *RootNode) IsRoot() bool {
	return true
}

// MarshalJSON returns a JSON representation of the node.
func (n *RootNode) MarshalJSON() ([]byte, error) {
	type jsonNode struct {
		Type      string                  `json:"type"`
		ID        common.LogicalTimestamp `json:"id"`
		Timestamp common.LogicalTimestamp `json:"timestamp"`
		Value     json.RawMessage         `json:"value,omitempty"`
	}

	node := jsonNode{
		Type:      string(n.Type()),
		ID:        n.NodeId,
		Timestamp: n.NodeTimestamp,
	}

	if n.NodeValue != nil {
		valueJSON, err := json.Marshal(n.NodeValue)
		if err != nil {
			return nil, err
		}
		node.Value = valueJSON
	}

	return json.Marshal(node)
}

// UnmarshalJSON parses a JSON representation of the node.
func (n *RootNode) UnmarshalJSON(data []byte) error {
	type jsonNode struct {
		Type      string                  `json:"type"`
		ID        common.LogicalTimestamp `json:"id"`
		Timestamp common.LogicalTimestamp `json:"timestamp"`
		Value     json.RawMessage         `json:"value,omitempty"`
	}

	var node jsonNode
	if err := json.Unmarshal(data, &node); err != nil {
		return err
	}

	if node.Type != string(common.NodeTypeRoot) {
		return common.ErrInvalidNodeType{Type: node.Type}
	}

	n.NodeId = node.ID
	n.NodeTimestamp = node.Timestamp

	// Parse the value field if it exists
	if len(node.Value) > 0 {
		// Parse the value type
		var valueType struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(node.Value, &valueType); err != nil {
			return err
		}

		// Create the appropriate node type based on the type field
		var valueNode Node
		switch common.NodeType(valueType.Type) {
		case common.NodeTypeVal:
			valueNode = &LWWValueNode{}
		case common.NodeTypeObj:
			valueNode = &LWWObjectNode{}
		case common.NodeTypeCon:
			valueNode = &ConstantNode{}
		case common.NodeTypeStr:
			valueNode = &RGAStringNode{}
		case common.NodeTypeArr:
			valueNode = &RGAArrayNode{}
		case common.NodeTypeVec:
			valueNode = &LWWVectorNode{}
		case common.NodeTypeBin:
			valueNode = &RGABinaryNode{}
		default:
			return common.ErrInvalidNodeType{Type: valueType.Type}
		}

		// Unmarshal the value
		if err := json.Unmarshal(node.Value, valueNode); err != nil {
			return err
		}

		// Set the value
		n.NodeValue = valueNode
	}

	return nil
}
