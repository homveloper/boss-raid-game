package crdt

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"tictactoe/luvjson/common"
)

// RGABinaryNode represents a Replicated Growable Array binary node.
// This is similar to RGAStringNode but for binary data.
type RGABinaryNode struct {
	NodeId       common.LogicalTimestamp `json:"id"`
	NodeElements []*RGABinaryElement     `json:"elements,omitempty"`
}

// RGABinaryElement represents an element in a binary RGA.
type RGABinaryElement struct {
	NodeId      common.LogicalTimestamp `json:"id"`
	NodeValue   []byte                  `json:"value"`
	NodeDeleted bool                    `json:"deleted"`
}

// NewRGABinaryNode creates a new RGA binary node.
func NewRGABinaryNode(id common.LogicalTimestamp) *RGABinaryNode {
	return &RGABinaryNode{
		NodeId:       id,
		NodeElements: make([]*RGABinaryElement, 0),
	}
}

// ID returns the unique identifier of the node.
func (n *RGABinaryNode) ID() common.LogicalTimestamp {
	return n.NodeId
}

// Type returns the type of the node.
func (n *RGABinaryNode) Type() common.NodeType {
	return common.NodeTypeBin
}

// Value returns the value of the node.
func (n *RGABinaryNode) Value() interface{} {
	var result []byte
	for _, elem := range n.NodeElements {
		if !elem.NodeDeleted {
			result = append(result, elem.NodeValue...)
		}
	}
	return result
}

// IsRoot returns true if this is a root node.
func (n *RGABinaryNode) IsRoot() bool {
	// Check if the node has the common.RootID
	return n.NodeId.Compare(common.RootID) == 0
}

// Length returns the length of the binary data in bytes.
func (n *RGABinaryNode) Length() int {
	length := 0
	for _, elem := range n.NodeElements {
		if !elem.NodeDeleted {
			length += len(elem.NodeValue)
		}
	}
	return length
}

// Insert inserts binary data after the specified position.
func (n *RGABinaryNode) Insert(afterID common.LogicalTimestamp, id common.LogicalTimestamp, data []byte) bool {
	// Find the position to insert
	pos := -1
	for i, elem := range n.NodeElements {
		if elem.NodeId.Compare(afterID) == 0 {
			pos = i
			break
		}
	}

	// Check if afterID is not the common.RootID
	if pos == -1 && afterID.Compare(common.RootID) != 0 {
		return false
	}

	// Create new element
	newElement := &RGABinaryElement{
		NodeId:      id,
		NodeValue:   data,
		NodeDeleted: false,
	}

	// Insert the new element
	if pos == -1 {
		n.NodeElements = append([]*RGABinaryElement{newElement}, n.NodeElements...)
	} else {
		n.NodeElements = append(n.NodeElements[:pos+1], append([]*RGABinaryElement{newElement}, n.NodeElements[pos+1:]...)...)
	}

	return true
}

// Delete marks elements as deleted.
func (n *RGABinaryNode) Delete(startID, endID common.LogicalTimestamp) bool {
	startPos := -1
	endPos := -1

	for i, elem := range n.NodeElements {
		if elem.NodeId.Compare(startID) == 0 {
			startPos = i
		}
		if elem.NodeId.Compare(endID) == 0 {
			endPos = i
		}
		if startPos != -1 && endPos != -1 {
			break
		}
	}

	if startPos == -1 || endPos == -1 || startPos > endPos {
		return false
	}

	for i := startPos; i <= endPos; i++ {
		n.NodeElements[i].NodeDeleted = true
	}

	return true
}

// MarshalJSON returns a JSON representation of the node.
func (n *RGABinaryNode) MarshalJSON() ([]byte, error) {
	type jsonElement struct {
		ID      common.LogicalTimestamp `json:"id"`
		Value   string                  `json:"value"` // Base64 encoded
		Deleted bool                    `json:"deleted"`
	}

	type jsonNode struct {
		Type     string                  `json:"type"`
		ID       common.LogicalTimestamp `json:"id"`
		Elements []jsonElement           `json:"elements,omitempty"`
	}

	node := jsonNode{
		Type:     string(n.Type()),
		ID:       n.NodeId,
		Elements: make([]jsonElement, len(n.NodeElements)),
	}

	for i, elem := range n.NodeElements {
		// Encode binary data as Base64
		base64Value := base64.StdEncoding.EncodeToString(elem.NodeValue)

		node.Elements[i] = jsonElement{
			ID:      elem.NodeId,
			Value:   base64Value,
			Deleted: elem.NodeDeleted,
		}
	}

	return json.Marshal(node)
}

// UnmarshalJSON parses a JSON representation of the node.
func (n *RGABinaryNode) UnmarshalJSON(data []byte) error {
	type jsonElement struct {
		ID      common.LogicalTimestamp `json:"id"`
		Value   string                  `json:"value"` // Base64 encoded
		Deleted bool                    `json:"deleted"`
	}

	type jsonNode struct {
		Type     string                  `json:"type"`
		ID       common.LogicalTimestamp `json:"id"`
		Elements []jsonElement           `json:"elements,omitempty"`
	}

	var node jsonNode
	if err := json.Unmarshal(data, &node); err != nil {
		return err
	}

	if node.Type != string(common.NodeTypeBin) {
		return common.ErrInvalidNodeType{Type: node.Type}
	}

	n.NodeId = node.ID
	n.NodeElements = make([]*RGABinaryElement, len(node.Elements))

	for i, elem := range node.Elements {
		// Decode Base64 data
		binaryData, err := base64.StdEncoding.DecodeString(elem.Value)
		if err != nil {
			return fmt.Errorf("failed to decode Base64 data: %w", err)
		}

		n.NodeElements[i] = &RGABinaryElement{
			NodeId:      elem.ID,
			NodeValue:   binaryData,
			NodeDeleted: elem.Deleted,
		}
	}

	return nil
}
