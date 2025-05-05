package crdtedit

// NodeType represents the type of a CRDT node
type NodeType string

const (
	// NodeTypeObject represents an object node
	NodeTypeObject NodeType = "object"
	// NodeTypeArray represents an array node
	NodeTypeArray NodeType = "array"
	// NodeTypeString represents a string node
	NodeTypeString NodeType = "string"
	// NodeTypeNumber represents a number node
	NodeTypeNumber NodeType = "number"
	// NodeTypeBoolean represents a boolean node
	NodeTypeBoolean NodeType = "boolean"
	// NodeTypeNull represents a null node
	NodeTypeNull NodeType = "null"
	// NodeTypeUnknown represents an unknown node type
	NodeTypeUnknown NodeType = "unknown"
)

// EditFunc is a function that performs edits on a document through an EditContext
type EditFunc func(ctx *EditContext) error

// ObjectEditor provides methods for manipulating object nodes
type ObjectEditor interface {
	// SetKey sets a key in the object to the given value
	SetKey(key string, value any) (ObjectEditor, error)
	// DeleteKey deletes a key from the object
	DeleteKey(key string) (ObjectEditor, error)
	// GetKeys returns all keys in the object
	GetKeys() ([]string, error)
	// HasKey checks if the object has the given key
	HasKey(key string) (bool, error)
	// GetValue returns the value at the given key
	GetValue(key string) (any, error)
	// GetPath returns the path to this object
	GetPath() string
}

// ArrayEditor provides methods for manipulating array nodes
type ArrayEditor interface {
	// Append adds a value to the end of the array
	Append(value any) (ArrayEditor, error)
	// Insert inserts a value at the given index
	Insert(index int, value any) (ArrayEditor, error)
	// Delete removes the element at the given index
	Delete(index int) (ArrayEditor, error)
	// GetLength returns the length of the array
	GetLength() (int, error)
	// GetElement returns the element at the given index
	GetElement(index int) (any, error)
	// GetPath returns the path to this array
	GetPath() string
}

// StringEditor provides methods for manipulating string nodes
type StringEditor interface {
	// Append adds text to the end of the string
	Append(text string) (StringEditor, error)
	// Insert inserts text at the given position
	Insert(index int, text string) (StringEditor, error)
	// Delete removes text from start to end positions
	Delete(start, end int) (StringEditor, error)
	// GetLength returns the length of the string
	GetLength() (int, error)
	// GetSubstring returns a substring from start to end
	GetSubstring(start, end int) (string, error)
	// GetValue returns the entire string value
	GetValue() (string, error)
	// GetPath returns the path to this string
	GetPath() string
}

// NumberEditor provides methods for manipulating number nodes
type NumberEditor interface {
	// Increment increases the number by the given delta
	Increment(delta float64) (NumberEditor, error)
	// SetValue sets the number to the given value
	SetValue(value float64) (NumberEditor, error)
	// GetValue returns the number value
	GetValue() (float64, error)
	// GetPath returns the path to this number
	GetPath() string
}

// BooleanEditor provides methods for manipulating boolean nodes
type BooleanEditor interface {
	// Toggle inverts the boolean value
	Toggle() (BooleanEditor, error)
	// SetValue sets the boolean to the given value
	SetValue(value bool) (BooleanEditor, error)
	// GetValue returns the boolean value
	GetValue() (bool, error)
	// GetPath returns the path to this boolean
	GetPath() string
}
