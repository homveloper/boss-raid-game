package crdt

import (
	"fmt"

	"tictactoe/luvjson/common"
)

// CreateObject creates a new object node and adds it to the document.
func (d *Document) CreateObject() (common.LogicalTimestamp, error) {
	// Generate a new ID for the object
	id := d.NextTimestamp()

	// Create a new object node
	node := NewLWWObjectNode(id)

	// Add the node to the document
	d.AddNode(node)

	return id, nil
}

// CreateArray creates a new array node and adds it to the document.
func (d *Document) CreateArray() (common.LogicalTimestamp, error) {
	// Generate a new ID for the array
	id := d.NextTimestamp()

	// Create a new array node (using LWWObjectNode as a placeholder)
	// In a real implementation, this would be a proper array node type
	node := NewLWWObjectNode(id)

	// Add the node to the document
	d.AddNode(node)

	return id, nil
}

// CreateString creates a new string node and adds it to the document.
func (d *Document) CreateString(value string) (common.LogicalTimestamp, error) {
	// Generate a new ID for the string
	id := d.NextTimestamp()

	// Create a new string node
	node := NewRGAStringNode(id)

	// If there's an initial value, insert it
	if value != "" {
		node.Insert(id, id, value)
	}

	// Add the node to the document
	d.AddNode(node)

	return id, nil
}

// CreateNumber creates a new number node and adds it to the document.
func (d *Document) CreateNumber(value float64) (common.LogicalTimestamp, error) {
	// Generate a new ID for the number
	id := d.NextTimestamp()

	// Create a new constant node with the number value
	node := NewConstantNode(id, value)

	// Add the node to the document
	d.AddNode(node)

	return id, nil
}

// CreateBoolean creates a new boolean node and adds it to the document.
func (d *Document) CreateBoolean(value bool) (common.LogicalTimestamp, error) {
	// Generate a new ID for the boolean
	id := d.NextTimestamp()

	// Create a new constant node with the boolean value
	node := NewConstantNode(id, value)

	// Add the node to the document
	d.AddNode(node)

	return id, nil
}

// CreateNull creates a new null node and adds it to the document.
func (d *Document) CreateNull() (common.LogicalTimestamp, error) {
	// Generate a new ID for the null value
	id := d.NextTimestamp()

	// Create a new constant node with nil value
	node := NewConstantNode(id, nil)

	// Add the node to the document
	d.AddNode(node)

	return id, nil
}

// GetRootID returns the root ID of the document.
func (d *Document) GetRootID() common.LogicalTimestamp {
	// Get the root ID (assuming it's a constant)
	return common.RootID
}

// SetRoot sets the root node of the document.
func (d *Document) SetRoot(nodeID common.LogicalTimestamp) error {
	// Get the root node
	rootNode := d.Root()
	if rootNode == nil {
		return fmt.Errorf("root node not found")
	}

	// Get the target node
	targetNode, err := d.GetNode(nodeID)
	if err != nil {
		return fmt.Errorf("target node not found: %w", err)
	}

	// Set the root value to the target node
	if rootLWW, ok := rootNode.(*RootNode); ok {
		rootLWW.NodeValue = targetNode
	} else if rootLWW, ok := rootNode.(*LWWValueNode); ok {
		rootLWW.SetValue(nodeID, targetNode)
	} else {
		return fmt.Errorf("unexpected root node type: %T", rootNode)
	}

	return nil
}

// GetNodeValue returns the value of a node.
func (d *Document) GetNodeValue(node Node) (any, error) {
	if node == nil {
		return nil, fmt.Errorf("node cannot be nil")
	}

	return node.Value(), nil
}

// ApplyOperation applies an operation to the document.
func (d *Document) ApplyOperation(op interface{}) error {
	// This is a placeholder implementation
	// In a real implementation, this would handle different operation types
	return fmt.Errorf("operation application not implemented")
}
