package api

import (
	"tictactoe/luvjson/crdt"
)

// NodeApi is the base interface for all node APIs.
type NodeApi interface {
	// View returns the current view of the node.
	View() (interface{}, error)
}

// ValApi provides an API for working with LWWValueNode.
type ValApi struct {
	node *crdt.LWWValueNode
	api  *ModelApi
}

// View returns the current view of the node.
func (v *ValApi) View() (interface{}, error) {
	if v.node == nil {
		return nil, nil
	}
	return v.node.Value(), nil
}

// Get returns the value of the node.
func (v *ValApi) Get() (interface{}, error) {
	return v.View()
}

// Set sets the value of the node.
func (v *ValApi) Set(value interface{}) *ValApi {
	if v.node == nil {
		return v
	}

	// Create a constant node for the value
	valueID := v.api.doc.NextTimestamp()
	valueOp := &crdt.ConstantNode{
		NodeId:    valueID,
		NodeValue: value,
	}
	v.api.doc.AddNode(valueOp)

	// Set the value
	v.node.SetValue(valueID, valueOp)

	return v
}

// ObjApi provides an API for working with LWWObjectNode.
type ObjApi struct {
	node *crdt.LWWObjectNode
	api  *ModelApi
}

// View returns the current view of the node.
func (o *ObjApi) View() (interface{}, error) {
	if o.node == nil {
		return nil, nil
	}
	return o.node.Value(), nil
}

// Get returns the value of the field with the given key.
func (o *ObjApi) Get(key string) (interface{}, error) {
	if o.node == nil {
		return nil, nil
	}
	fieldNode := o.node.Get(key)
	if fieldNode == nil {
		return nil, nil
	}
	return fieldNode.Value(), nil
}

// Set sets the values of the fields in the object.
func (o *ObjApi) Set(values map[string]interface{}) *ObjApi {
	if o.node == nil {
		return o
	}

	for key, value := range values {
		// Create a constant node for the value
		valueID := o.api.doc.NextTimestamp()
		valueNode := crdt.NewConstantNode(valueID, value)
		o.api.doc.AddNode(valueNode)

		// Set the field
		o.node.Set(key, valueID, valueNode)
	}

	return o
}

// Del deletes the field with the given key.
func (o *ObjApi) Del(key string) *ObjApi {
	if o.node == nil {
		return o
	}

	o.node.Delete(key)

	return o
}

// StrApi provides an API for working with RGAStringNode.
type StrApi struct {
	node *crdt.RGAStringNode
	api  *ModelApi
}

// View returns the current view of the node.
func (s *StrApi) View() (interface{}, error) {
	if s.node == nil {
		return "", nil
	}
	return s.node.Value(), nil
}

// Get returns the string value of the node.
func (s *StrApi) Get() (string, error) {
	if s.node == nil {
		return "", nil
	}
	value := s.node.Value()
	if str, ok := value.(string); ok {
		return str, nil
	}
	return "", nil
}

// Ins inserts a string at the given position.
func (s *StrApi) Ins(pos int, value string) *StrApi {
	if s.node == nil {
		return s
	}

	// Get the target ID
	targetID := s.node.GetPositionID(pos)

	// Insert the string
	insertID := s.api.doc.NextTimestamp()
	s.node.Insert(targetID, insertID, value)

	return s
}

// Del deletes a substring from the given position with the given length.
func (s *StrApi) Del(pos int, length int) *StrApi {
	if s.node == nil {
		return s
	}

	// Get the start and end IDs
	startID := s.node.GetPositionID(pos)
	endID := s.node.GetPositionID(pos + length)

	// Delete the substring
	s.node.Delete(startID, endID)

	return s
}

// ConApi provides an API for working with ConstantNode.
type ConApi struct {
	node *crdt.ConstantNode
	api  *ModelApi
}

// View returns the current view of the node.
func (c *ConApi) View() (interface{}, error) {
	if c.node == nil {
		return nil, nil
	}
	return c.node.Value(), nil
}

// Get returns the value of the node.
func (c *ConApi) Get() (interface{}, error) {
	return c.View()
}
