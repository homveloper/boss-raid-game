package crdt

import (
	"encoding/json"
	"tictactoe/luvjson/common"
)

// LWWObjectNode represents a Last-Write-Wins object node.
type LWWObjectNode struct {
	NodeId     common.LogicalTimestamp    `json:"id"`
	NodeFields map[string]*LWWObjectField `json:"fields,omitempty"`
}

// LWWObjectField represents a field in a LWW object.
type LWWObjectField struct {
	NodeTimestamp common.LogicalTimestamp `json:"timestamp"`
	NodeValue     Node                    `json:"value"`
}

// NewLWWObjectNode creates a new LWW object node.
func NewLWWObjectNode(id common.LogicalTimestamp) *LWWObjectNode {
	return &LWWObjectNode{
		NodeId:     id,
		NodeFields: make(map[string]*LWWObjectField),
	}
}

// ID returns the unique identifier of the node.
func (n *LWWObjectNode) ID() common.LogicalTimestamp {
	return n.NodeId
}

// Type returns the type of the node.
func (n *LWWObjectNode) Type() common.NodeType {
	return common.NodeTypeObj
}

// Value returns the value of the node.
func (n *LWWObjectNode) Value() interface{} {
	result := make(map[string]interface{})
	for key, field := range n.NodeFields {
		result[key] = field.NodeValue.Value()
	}
	return result
}

// IsRoot returns true if this is a root node.
func (n *LWWObjectNode) IsRoot() bool {
	// Check if the node has the common.RootID
	return n.NodeId.Compare(common.RootID) == 0
}

// Get returns the value of a field.
func (n *LWWObjectNode) Get(key string) Node {
	if field, ok := n.NodeFields[key]; ok {
		return field.NodeValue
	}
	return nil
}

// Set sets the value of a field.
func (n *LWWObjectNode) Set(key string, timestamp common.LogicalTimestamp, value Node) bool {
	field, ok := n.NodeFields[key]
	if !ok || timestamp.Compare(field.NodeTimestamp) > 0 {
		n.NodeFields[key] = &LWWObjectField{
			NodeTimestamp: timestamp,
			NodeValue:     value,
		}
		return true
	}
	return false
}

// Delete deletes a field.
func (n *LWWObjectNode) Delete(key string, timestamp common.LogicalTimestamp) bool {
	field, ok := n.NodeFields[key]
	if ok && timestamp.Compare(field.NodeTimestamp) > 0 {
		delete(n.NodeFields, key)
		return true
	}
	return false
}

// Keys returns the keys of the object.
func (n *LWWObjectNode) Keys() []string {
	keys := make([]string, 0, len(n.NodeFields))
	for key := range n.NodeFields {
		keys = append(keys, key)
	}
	return keys
}

// MarshalJSON returns a JSON representation of the node.
func (n *LWWObjectNode) MarshalJSON() ([]byte, error) {
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

	for key, field := range n.NodeFields {
		valueJSON, err := json.Marshal(field.NodeValue)
		if err != nil {
			return nil, err
		}

		node.Fields[key] = jsonField{
			Timestamp: field.NodeTimestamp,
			Value:     valueJSON,
		}
	}

	return json.Marshal(node)
}

// UnmarshalJSON parses a JSON representation of the node.
func (n *LWWObjectNode) UnmarshalJSON(data []byte) error {
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

	if node.Type != string(common.NodeTypeObj) {
		return common.ErrInvalidNodeType{Type: node.Type}
	}

	n.NodeId = node.ID
	n.NodeFields = make(map[string]*LWWObjectField)

	// Parse fields
	for key, field := range node.Fields {
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
		default:
			return common.ErrInvalidNodeType{Type: valueType.Type}
		}

		// Unmarshal the field value
		if err := json.Unmarshal(field.Value, fieldNode); err != nil {
			return err
		}

		// Create a new field
		n.NodeFields[key] = &LWWObjectField{
			NodeTimestamp: field.Timestamp,
			NodeValue:     fieldNode,
		}
	}

	return nil
}
