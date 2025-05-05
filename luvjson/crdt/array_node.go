package crdt

import (
	"encoding/json"
	"tictactoe/luvjson/common"
)

// RGAArrayNode represents a Replicated Growable Array array node.
// This is similar to RGAStringNode but for arbitrary JSON values.
type RGAArrayNode struct {
	NodeId       common.LogicalTimestamp `json:"id"`
	NodeElements []*RGAElement           `json:"elements,omitempty"`
}

// NewRGAArrayNode creates a new RGA array node.
func NewRGAArrayNode(id common.LogicalTimestamp) *RGAArrayNode {
	return &RGAArrayNode{
		NodeId:       id,
		NodeElements: make([]*RGAElement, 0),
	}
}

// ID returns the unique identifier of the node.
func (n *RGAArrayNode) ID() common.LogicalTimestamp {
	return n.NodeId
}

// Type returns the type of the node.
func (n *RGAArrayNode) Type() common.NodeType {
	return common.NodeTypeArr
}

// Value returns the value of the node.
func (n *RGAArrayNode) Value() interface{} {
	var result []interface{}
	for _, elem := range n.NodeElements {
		if !elem.NodeDeleted {
			result = append(result, elem.NodeValue)
		}
	}
	return result
}

// IsRoot returns true if this is a root node.
func (n *RGAArrayNode) IsRoot() bool {
	// Check if the node has the RootID
	return n.NodeId.Compare(common.RootID) == 0
}

// Length returns the number of visible elements in the array.
func (n *RGAArrayNode) Length() int {
	count := 0
	for _, elem := range n.NodeElements {
		if !elem.NodeDeleted {
			count++
		}
	}
	return count
}

// Get returns the element at the specified index.
func (n *RGAArrayNode) Get(index int) (common.LogicalTimestamp, error) {
	if index < 0 {
		return common.LogicalTimestamp{}, common.ErrInvalidOperation{Message: "index cannot be negative"}
	}

	visibleIndex := 0
	for _, elem := range n.NodeElements {
		if !elem.NodeDeleted {
			if visibleIndex == index {
				if nodeID, ok := elem.NodeValue.(common.LogicalTimestamp); ok {
					return nodeID, nil
				}
				return common.LogicalTimestamp{}, common.ErrInvalidOperation{Message: "element is not a node ID"}
			}
			visibleIndex++
		}
	}

	return common.LogicalTimestamp{}, common.ErrInvalidOperation{Message: "index out of bounds"}
}

// Insert inserts a value after the specified position.
func (n *RGAArrayNode) Insert(afterID common.LogicalTimestamp, id common.LogicalTimestamp, value interface{}) bool {
	// Find the position to insert
	pos := -1
	for i, elem := range n.NodeElements {
		if elem.NodeId.Compare(afterID) == 0 {
			pos = i
			break
		}
	}

	// Check if afterID is not the RootID
	if pos == -1 && afterID.Compare(common.RootID) != 0 {
		return false
	}

	// Create new element
	newElement := &RGAElement{
		NodeId:      id,
		NodeValue:   value,
		NodeDeleted: false,
	}

	// Insert the new element
	if pos == -1 {
		n.NodeElements = append([]*RGAElement{newElement}, n.NodeElements...)
	} else {
		n.NodeElements = append(n.NodeElements[:pos+1], append([]*RGAElement{newElement}, n.NodeElements[pos+1:]...)...)
	}

	return true
}

// Delete marks an element as deleted.
func (n *RGAArrayNode) Delete(id common.LogicalTimestamp) bool {
	for i, elem := range n.NodeElements {
		if elem.NodeId.Compare(id) == 0 {
			n.NodeElements[i].NodeDeleted = true
			return true
		}
	}
	return false
}

// DeleteRange marks a range of elements as deleted.
func (n *RGAArrayNode) DeleteRange(startID, endID common.LogicalTimestamp) bool {
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
func (n *RGAArrayNode) MarshalJSON() ([]byte, error) {
	type jsonElement struct {
		ID      common.LogicalTimestamp `json:"id"`
		Value   json.RawMessage         `json:"value"`
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
		valueJSON, err := json.Marshal(elem.NodeValue)
		if err != nil {
			return nil, err
		}

		node.Elements[i] = jsonElement{
			ID:      elem.NodeId,
			Value:   valueJSON,
			Deleted: elem.NodeDeleted,
		}
	}

	return json.Marshal(node)
}

// UnmarshalJSON parses a JSON representation of the node.
func (n *RGAArrayNode) UnmarshalJSON(data []byte) error {
	type jsonElement struct {
		ID      common.LogicalTimestamp `json:"id"`
		Value   json.RawMessage         `json:"value"`
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

	if node.Type != string(common.NodeTypeArr) {
		return common.ErrInvalidNodeType{Type: node.Type}
	}

	n.NodeId = node.ID
	n.NodeElements = make([]*RGAElement, len(node.Elements))

	for i, elem := range node.Elements {
		// Parse the value type
		var valueType struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(elem.Value, &valueType); err == nil && valueType.Type != "" {
			// Value is a node
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
			if err := json.Unmarshal(elem.Value, valueNode); err != nil {
				return err
			}

			n.NodeElements[i] = &RGAElement{
				NodeId:      elem.ID,
				NodeValue:   valueNode,
				NodeDeleted: elem.Deleted,
			}
		} else {
			// Value is a primitive
			var value interface{}
			if err := json.Unmarshal(elem.Value, &value); err != nil {
				return err
			}

			n.NodeElements[i] = &RGAElement{
				NodeId:      elem.ID,
				NodeValue:   value,
				NodeDeleted: elem.Deleted,
			}
		}
	}

	return nil
}
