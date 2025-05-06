package crdt

import (
	"tictactoe/luvjson/common"

	"github.com/pkg/errors"
)

// CreateNodeForValue creates a new node for a value without requiring a Document instance.
// This is a pure function that doesn't modify any state.
func CreateNodeForValue(id common.LogicalTimestamp, value any) (Node, error) {
	switch v := value.(type) {
	case nil:
		return NewConstantNode(id, nil), nil
	case bool:
		return NewConstantNode(id, v), nil
	case string:
		node := NewRGAStringNode(id)
		if v != "" {
			node.Insert(id, id, v)
		}
		return node, nil
	case float64:
		return NewConstantNode(id, v), nil
	case int:
		return NewConstantNode(id, float64(v)), nil
	case int64:
		return NewConstantNode(id, float64(v)), nil
	case map[string]any:
		objNode := NewLWWObjectNode(id)
		for k, fieldValue := range v {
			fieldID := common.LogicalTimestamp{
				SID:     id.SID,
				Counter: id.Counter + 1,
			}
			fieldNode, err := CreateNodeForValue(fieldID, fieldValue)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to create node for field %s", k)
			}
			objNode.Set(k, fieldID, fieldNode)
		}
		return objNode, nil
	case []any:
		// For now, we'll use LWWObjectNode as a placeholder for arrays
		// In a real implementation, this would be a proper array node type
		arrNode := NewLWWObjectNode(id)
		for i, elemValue := range v {
			elemID := common.LogicalTimestamp{
				SID:     id.SID,
				Counter: id.Counter + uint64(i) + 1,
			}
			elemNode, err := CreateNodeForValue(elemID, elemValue)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to create node for element at index %d", i)
			}
			arrNode.Set(string(i), elemID, elemNode)
		}
		return arrNode, nil
	default:
		return nil, errors.Errorf("unsupported value type: %T", value)
	}
}

// CreateNullNode creates a new null node.
func CreateNullNode(id common.LogicalTimestamp) Node {
	return NewConstantNode(id, nil)
}

// CreateBooleanNode creates a new boolean node.
func CreateBooleanNode(id common.LogicalTimestamp, value bool) Node {
	return NewConstantNode(id, value)
}

// CreateNumberNode creates a new number node.
func CreateNumberNode(id common.LogicalTimestamp, value float64) Node {
	return NewConstantNode(id, value)
}

// CreateStringNode creates a new string node.
func CreateStringNode(id common.LogicalTimestamp, value string) Node {
	node := NewRGAStringNode(id)
	if value != "" {
		node.Insert(id, id, value)
	}
	return node
}

// CreateObjectNode creates a new object node.
func CreateObjectNode(id common.LogicalTimestamp) Node {
	return NewLWWObjectNode(id)
}

// CreateArrayNode creates a new array node.
func CreateArrayNode(id common.LogicalTimestamp) Node {
	// For now, we'll use LWWObjectNode as a placeholder for arrays
	// In a real implementation, this would be a proper array node type
	return NewLWWObjectNode(id)
}
