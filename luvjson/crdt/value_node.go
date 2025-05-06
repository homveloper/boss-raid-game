package crdt

import (
	"encoding/json"
	"tictactoe/luvjson/common"
)

// LWWValueNode represents a Last-Write-Wins value node.
type LWWValueNode struct {
	NodeId        common.LogicalTimestamp `json:"id"`
	NodeTimestamp common.LogicalTimestamp `json:"timestamp"`
	NodeValue     Node                    `json:"value,omitempty"`
}

// NewLWWValueNode creates a new LWW value node.
func NewLWWValueNode(id common.LogicalTimestamp, timestamp common.LogicalTimestamp, value Node) *LWWValueNode {
	return &LWWValueNode{
		NodeId:        id,
		NodeTimestamp: timestamp,
		NodeValue:     value,
	}
}

// ID returns the unique identifier of the node.
func (n *LWWValueNode) ID() common.LogicalTimestamp {
	return n.NodeId
}

// Type returns the type of the node.
func (n *LWWValueNode) Type() common.NodeType {
	return common.NodeTypeVal
}

// Value returns the value of the node.
func (n *LWWValueNode) Value() interface{} {
	return n.NodeValue
}

// IsRoot returns true if this is a root node.
func (n *LWWValueNode) IsRoot() bool {
	// Check if the node has the common.RootID
	return n.NodeId.Compare(common.RootID) == 0
}

// Timestamp returns the timestamp of the last write.
func (n *LWWValueNode) Timestamp() common.LogicalTimestamp {
	return n.NodeTimestamp
}

// SetValue sets the value of the node.
func (n *LWWValueNode) SetValue(timestamp common.LogicalTimestamp, value Node) bool {
	if timestamp.Compare(n.NodeTimestamp) > 0 {
		n.NodeTimestamp = timestamp
		n.NodeValue = value
		return true
	}
	return false
}

// MarshalJSON returns a JSON representation of the node.
func (n *LWWValueNode) MarshalJSON() ([]byte, error) {
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
func (n *LWWValueNode) UnmarshalJSON(data []byte) error {
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

	if node.Type != string(common.NodeTypeVal) {
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
