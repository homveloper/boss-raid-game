package crdtedit

import (
	"encoding/json"
	"reflect"

	"github.com/pkg/errors"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
)

// ModelBuilder provides methods for building models from structs or JSON
type ModelBuilder struct {
	doc *crdt.Document
}

// NewModelBuilder creates a new ModelBuilder
func NewModelBuilder(doc *crdt.Document, _ *PatchBuilder) *ModelBuilder {
	return &ModelBuilder{
		doc: doc,
	}
}

// BuildFromStruct builds a document from a struct
func (b *ModelBuilder) BuildFromStruct(v any) error {
	if v == nil {
		return errors.New("input value cannot be nil")
	}

	// 각 호출마다 새로운 PatchBuilder 생성
	patchBuilder := NewPatchBuilder(b.doc.GetSessionID())

	// Convert struct to map using reflection
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return errors.New("input value must be a struct or pointer to struct")
	}

	// Create root object
	rootID := patchBuilder.NextID()

	// Set root ID
	if err := patchBuilder.AddSetOperation(b.doc.GetRootID(), rootID); err != nil {
		return errors.Wrap(err, "failed to set root object")
	}

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
		if err := patchBuilder.AddObjectInsertOperation(rootID, key, fieldValue.Interface()); err != nil {
			return errors.Wrapf(err, "failed to add field %s", key)
		}
	}

	// Apply operations
	patch := patchBuilder.CreatePatch(b.doc.NextTimestamp())
	if err := patch.Apply(b.doc); err != nil {
		return errors.Wrap(err, "failed to apply patch")
	}
	return nil
}

// BuildFromJSON builds a document from JSON
func (b *ModelBuilder) BuildFromJSON(data []byte) error {
	if len(data) == 0 {
		return errors.New("input JSON cannot be empty")
	}

	// 각 호출마다 새로운 PatchBuilder 생성
	patchBuilder := NewPatchBuilder(b.doc.GetSessionID())

	// Parse JSON
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return errors.Wrap(err, "failed to parse JSON")
	}

	// Create root object or array
	var rootID common.LogicalTimestamp

	switch v := value.(type) {
	case map[string]any:
		// Create root object
		rootID = patchBuilder.NextID()

		// Add fields to object
		for key, val := range v {
			if err := patchBuilder.AddObjectInsertOperation(rootID, key, val); err != nil {
				return errors.Wrapf(err, "failed to add field %s", key)
			}
		}
	case []any:
		// Create root array
		rootID = patchBuilder.NextID()

		// Add elements to array
		for i, val := range v {
			if err := patchBuilder.AddArrayInsertOperation(rootID, i, val); err != nil {
				return errors.Wrapf(err, "failed to add element at index %d", i)
			}
		}
	default:
		// For primitive values, create a value node
		rootID = patchBuilder.NextID()
		if err := patchBuilder.AddSetOperation(rootID, value); err != nil {
			return errors.Wrap(err, "failed to create root node")
		}
	}

	// Set root ID
	if err := patchBuilder.AddSetOperation(b.doc.GetRootID(), rootID); err != nil {
		return errors.Wrap(err, "failed to set root ID")
	}

	// Apply operations
	patch := patchBuilder.CreatePatch(b.doc.NextTimestamp())
	if err := patch.Apply(b.doc); err != nil {
		return errors.Wrap(err, "failed to apply patch")
	}
	return nil
}

// ToJSON converts a document to JSON
func (b *ModelBuilder) ToJSON(doc *crdt.Document) ([]byte, error) {
	// Get root node value
	rootNode := doc.Root()

	// Get root value
	rootValue, err := doc.GetNodeValue(rootNode)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get root value")
	}

	// Marshal to JSON
	data, err := json.Marshal(rootValue)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal to JSON")
	}
	return data, nil
}

// ToStruct populates a struct with data from a document
func (b *ModelBuilder) ToStruct(doc *crdt.Document, v any) error {
	if v == nil {
		return errors.New("output value cannot be nil")
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.New("output value must be a non-nil pointer")
	}

	// Get root node
	rootNode := doc.Root()

	// Get root value
	rootValue, err := doc.GetNodeValue(rootNode)
	if err != nil {
		return errors.Wrap(err, "failed to get root value")
	}

	// Convert to JSON and unmarshal to struct
	data, err := json.Marshal(rootValue)
	if err != nil {
		return errors.Wrap(err, "failed to marshal root value")
	}

	if err := json.Unmarshal(data, v); err != nil {
		return errors.Wrap(err, "failed to unmarshal to struct")
	}

	return nil
}
