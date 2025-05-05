package crdtedit

import (
	"encoding/json"
	"fmt"
	"reflect"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
)

// ModelBuilder provides methods for building models from structs or JSON
type ModelBuilder struct {
	doc          *crdt.Document
	patchBuilder *PatchBuilder
}

// NewModelBuilder creates a new ModelBuilder
func NewModelBuilder(doc *crdt.Document, patchBuilder *PatchBuilder) *ModelBuilder {
	return &ModelBuilder{
		doc:          doc,
		patchBuilder: patchBuilder,
	}
}

// BuildFromStruct builds a document from a struct
func (b *ModelBuilder) BuildFromStruct(v any) error {
	if v == nil {
		return fmt.Errorf("input value cannot be nil")
	}

	// Convert struct to map using reflection
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("input value must be a struct or pointer to struct")
	}

	// Create root object
	rootID, err := b.doc.CreateObject()
	if err != nil {
		return fmt.Errorf("failed to create root object: %w", err)
	}

	// Set root ID
	b.doc.SetRoot(rootID)

	// Process struct fields
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fieldValue := rv.Field(i)

		// Get JSON tag or use field name
		key := field.Tag.Get("json")
		if key == "" || key == "-" {
			key = field.Name
		}

		// Skip unexported fields
		if !fieldValue.CanInterface() {
			continue
		}

		// Add field to object
		if err := b.patchBuilder.AddObjectInsertOperation(rootID, key, fieldValue.Interface()); err != nil {
			return fmt.Errorf("failed to add field %s: %w", key, err)
		}
	}

	// Apply operations
	return b.patchBuilder.Apply()
}

// BuildFromJSON builds a document from JSON
func (b *ModelBuilder) BuildFromJSON(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("input JSON cannot be empty")
	}

	// Parse JSON
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Create root object or array
	var rootID common.LogicalTimestamp
	var err error

	switch v := value.(type) {
	case map[string]any:
		rootID, err = b.doc.CreateObject()
		if err != nil {
			return fmt.Errorf("failed to create root object: %w", err)
		}

		// Add fields to object
		for key, val := range v {
			if err := b.patchBuilder.AddObjectInsertOperation(rootID, key, val); err != nil {
				return fmt.Errorf("failed to add field %s: %w", key, err)
			}
		}
	case []any:
		rootID, err = b.doc.CreateArray()
		if err != nil {
			return fmt.Errorf("failed to create root array: %w", err)
		}

		// Add elements to array
		for i, val := range v {
			if err := b.patchBuilder.AddArrayInsertOperation(rootID, i, val); err != nil {
				return fmt.Errorf("failed to add element at index %d: %w", i, err)
			}
		}
	default:
		// For primitive values, create a value node
		rootID, err = b.patchBuilder.createNodeForValue(value)
		if err != nil {
			return fmt.Errorf("failed to create root node: %w", err)
		}
	}

	// Set root ID
	b.doc.SetRoot(rootID)

	// Apply operations
	return b.patchBuilder.Apply()
}

// ToJSON converts a document to JSON
func (b *ModelBuilder) ToJSON(doc *crdt.Document) ([]byte, error) {
	// Get root node value
	rootNode := doc.Root()

	// Get root value
	rootValue, err := doc.GetNodeValue(rootNode)
	if err != nil {
		return nil, fmt.Errorf("failed to get root value: %w", err)
	}

	// Marshal to JSON
	return json.Marshal(rootValue)
}

// ToStruct populates a struct with data from a document
func (b *ModelBuilder) ToStruct(doc *crdt.Document, v any) error {
	if v == nil {
		return fmt.Errorf("output value cannot be nil")
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("output value must be a non-nil pointer")
	}

	// Get root node
	rootNode := doc.Root()

	// Get root value
	rootValue, err := doc.GetNodeValue(rootNode)
	if err != nil {
		return fmt.Errorf("failed to get root value: %w", err)
	}

	// Convert to JSON and unmarshal to struct
	data, err := json.Marshal(rootValue)
	if err != nil {
		return fmt.Errorf("failed to marshal root value: %w", err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to unmarshal to struct: %w", err)
	}

	return nil
}
