package crdt

import (
	"encoding/json"
	"tictactoe/luvjson/common"
)

// RGAStringNode represents a Replicated Growable Array string node.
type RGAStringNode struct {
	NodeId       common.LogicalTimestamp `json:"id"`
	NodeElements []*RGAElement           `json:"elements,omitempty"`
}

// NewRGAStringNode creates a new RGA string node.
func NewRGAStringNode(id common.LogicalTimestamp) *RGAStringNode {
	return &RGAStringNode{
		NodeId:       id,
		NodeElements: make([]*RGAElement, 0),
	}
}

// ID returns the unique identifier of the node.
func (n *RGAStringNode) ID() common.LogicalTimestamp {
	return n.NodeId
}

// Type returns the type of the node.
func (n *RGAStringNode) Type() common.NodeType {
	return common.NodeTypeStr
}

func (n *RGAStringNode) Length() int {
	return len(n.value())
}

// Value returns the value of the node.
func (n *RGAStringNode) Value() interface{} {
	return n.value()
}

func (n *RGAStringNode) String() string {
	return n.value()
}

// value returns the string value of the node.
func (n *RGAStringNode) value() string {
	var result string
	for _, elem := range n.NodeElements {
		if !elem.NodeDeleted {
			if c, ok := elem.NodeValue.(rune); ok {
				result += string(c)
			} else if s, ok := elem.NodeValue.(string); ok && len(s) == 1 {
				result += s
			}
		}
	}
	return result
}

// IsRoot returns true if this is a root node.
func (n *RGAStringNode) IsRoot() bool {
	// Check if the node has the common.RootID
	return n.NodeId.Compare(common.RootID) == 0
}

// Insert inserts a string after the specified position.
func (n *RGAStringNode) Insert(afterID common.LogicalTimestamp, id common.LogicalTimestamp, value string) bool {
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

	// Create new elements
	newElements := make([]*RGAElement, len(value))
	for i, c := range value {
		newElements[i] = &RGAElement{
			NodeId: common.LogicalTimestamp{
				SID:     id.SID,
				Counter: id.Counter + uint64(i),
			},
			NodeValue:   c,
			NodeDeleted: false,
		}
	}

	// Insert the new elements
	if pos == -1 {
		n.NodeElements = append(newElements, n.NodeElements...)
	} else {
		n.NodeElements = append(n.NodeElements[:pos+1], append(newElements, n.NodeElements[pos+1:]...)...)
	}

	return true
}

// Delete marks elements as deleted.
func (n *RGAStringNode) Delete(startID, endID common.LogicalTimestamp) bool {
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
func (n *RGAStringNode) MarshalJSON() ([]byte, error) {
	type jsonElement struct {
		ID      common.LogicalTimestamp `json:"id"`
		Value   string                  `json:"value"`
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
		var value string
		if c, ok := elem.NodeValue.(rune); ok {
			value = string(c)
		} else if s, ok := elem.NodeValue.(string); ok {
			value = s
		}

		node.Elements[i] = jsonElement{
			ID:      elem.NodeId,
			Value:   value,
			Deleted: elem.NodeDeleted,
		}
	}

	return json.Marshal(node)
}

// UnmarshalJSON parses a JSON representation of the node.
func (n *RGAStringNode) UnmarshalJSON(data []byte) error {
	type jsonElement struct {
		ID      common.LogicalTimestamp `json:"id"`
		Value   string                  `json:"value"`
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

	if node.Type != string(common.NodeTypeStr) {
		return common.ErrInvalidNodeType{Type: node.Type}
	}

	n.NodeId = node.ID
	n.NodeElements = make([]*RGAElement, len(node.Elements))

	for i, elem := range node.Elements {
		var value interface{}
		if len(elem.Value) == 1 {
			value = rune(elem.Value[0])
		} else {
			value = elem.Value
		}

		n.NodeElements[i] = &RGAElement{
			NodeId:      elem.ID,
			NodeValue:   value,
			NodeDeleted: elem.Deleted,
		}
	}

	return nil
}
