package crdt

import (
	"encoding/json"
	"tictactoe/luvjson/common"
)

// Node represents a CRDT node in the JSON CRDT document.
type Node interface {
	// ID returns the unique identifier of the node.
	ID() common.LogicalTimestamp

	// Type returns the type of the node.
	Type() common.NodeType

	// Value returns the value of the node.
	Value() interface{}

	// MarshalJSON returns a JSON representation of the node.
	json.Marshaler

	// UnmarshalJSON parses a JSON representation of the node.
	json.Unmarshaler

	// IsRoot returns true if this is a root node.
	IsRoot() bool
}

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
	// Check if the node has the RootID
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
	// Check if the node has the RootID
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
	// Check if the node has the RootID
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

// RGAElement represents an element in a Replicated Growable Array.
type RGAElement struct {
	NodeId      common.LogicalTimestamp `json:"id"`
	NodeValue   interface{}             `json:"value"`
	NodeDeleted bool                    `json:"deleted"`
}

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

// Value returns the value of the node.
func (n *RGAStringNode) Value() interface{} {
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
	// Check if the node has the RootID
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

	// Check if afterID is not the RootID
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
