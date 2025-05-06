package crdtedit

import (
	"reflect"

	"github.com/pkg/errors"

	"tictactoe/luvjson/crdt"
)

// DocumentEditor provides methods for editing a CRDT document
type DocumentEditor struct {
	doc          *crdt.Document
	pathResolver *PathResolver
	queryEngine  *QueryEngine
	modelBuilder *ModelBuilder
}

// NewDocumentEditor creates a new DocumentEditor for the given document
func NewDocumentEditor(doc *crdt.Document) *DocumentEditor {
	pathResolver := NewPathResolver(doc)

	return &DocumentEditor{
		doc:          doc,
		pathResolver: pathResolver,
		queryEngine:  NewQueryEngine(doc, pathResolver),
		modelBuilder: NewModelBuilder(doc, nil), // PatchBuilder는 필요할 때 생성
	}
}

// AsObject returns an ObjectEditor for the object at the given path
func (e *DocumentEditor) AsObject(path string) (ObjectEditor, error) {
	nodeID, err := e.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve path %s", path)
	}

	nodeType, err := e.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node type at path %s", path)
	}

	if nodeType != NodeTypeObject {
		return nil, errors.Errorf("node at path %s is not an object", path)
	}

	// 각 에디터 호출마다 새로운 PatchBuilder 생성
	patchBuilder := NewPatchBuilder(e.doc.GetSessionID())
	ctx := NewEditContext(e.doc, e.pathResolver, patchBuilder)
	return newObjectEditor(ctx, path, nodeID), nil
}

// AsArray returns an ArrayEditor for the array at the given path
func (e *DocumentEditor) AsArray(path string) (ArrayEditor, error) {
	nodeID, err := e.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve path %s", path)
	}

	nodeType, err := e.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node type at path %s", path)
	}

	if nodeType != NodeTypeArray {
		return nil, errors.Errorf("node at path %s is not an array", path)
	}

	// 각 에디터 호출마다 새로운 PatchBuilder 생성
	patchBuilder := NewPatchBuilder(e.doc.GetSessionID())
	ctx := NewEditContext(e.doc, e.pathResolver, patchBuilder)
	return newArrayEditor(ctx, path, nodeID), nil
}

// AsString returns a StringEditor for the string at the given path
func (e *DocumentEditor) AsString(path string) (StringEditor, error) {
	nodeID, err := e.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve path %s", path)
	}

	nodeType, err := e.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node type at path %s", path)
	}

	if nodeType != NodeTypeString {
		return nil, errors.Errorf("node at path %s is not a string", path)
	}

	// 각 에디터 호출마다 새로운 PatchBuilder 생성
	patchBuilder := NewPatchBuilder(e.doc.GetSessionID())
	ctx := NewEditContext(e.doc, e.pathResolver, patchBuilder)
	return newStringEditor(ctx, path, nodeID), nil
}

// AsNumber returns a NumberEditor for the number at the given path
func (e *DocumentEditor) AsNumber(path string) (NumberEditor, error) {
	nodeID, err := e.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve path %s", path)
	}

	nodeType, err := e.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node type at path %s", path)
	}

	if nodeType != NodeTypeNumber {
		return nil, errors.Errorf("node at path %s is not a number", path)
	}

	// 각 에디터 호출마다 새로운 PatchBuilder 생성
	patchBuilder := NewPatchBuilder(e.doc.GetSessionID())
	ctx := NewEditContext(e.doc, e.pathResolver, patchBuilder)
	return newNumberEditor(ctx, path, nodeID), nil
}

// AsBoolean returns a BooleanEditor for the boolean at the given path
func (e *DocumentEditor) AsBoolean(path string) (BooleanEditor, error) {
	nodeID, err := e.pathResolver.ResolveNodePath(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve path %s", path)
	}

	nodeType, err := e.pathResolver.GetNodeType(nodeID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get node type at path %s", path)
	}

	if nodeType != NodeTypeBoolean {
		return nil, errors.Errorf("node at path %s is not a boolean", path)
	}

	// 각 에디터 호출마다 새로운 PatchBuilder 생성
	patchBuilder := NewPatchBuilder(e.doc.GetSessionID())
	ctx := NewEditContext(e.doc, e.pathResolver, patchBuilder)
	return newBooleanEditor(ctx, path, nodeID), nil
}

// CreateObject creates an object at the given path
func (e *DocumentEditor) CreateObject(path string) error {
	// 각 에디터 호출마다 새로운 PatchBuilder 생성
	patchBuilder := NewPatchBuilder(e.doc.GetSessionID())
	ctx := NewEditContext(e.doc, e.pathResolver, patchBuilder)
	return ctx.CreateObject(path)
}

// CreateArray creates an array at the given path
func (e *DocumentEditor) CreateArray(path string) error {
	// 각 에디터 호출마다 새로운 PatchBuilder 생성
	patchBuilder := NewPatchBuilder(e.doc.GetSessionID())
	ctx := NewEditContext(e.doc, e.pathResolver, patchBuilder)
	return ctx.CreateArray(path)
}

// SetValue sets a value at the given path
func (e *DocumentEditor) SetValue(path string, value any) error {
	// 각 에디터 호출마다 새로운 PatchBuilder 생성
	patchBuilder := NewPatchBuilder(e.doc.GetSessionID())
	ctx := NewEditContext(e.doc, e.pathResolver, patchBuilder)
	return ctx.SetValue(path, value)
}

// Query retrieves a value from the document at the given path
func (e *DocumentEditor) Query(path string) (any, error) {
	return e.queryEngine.GetValue(path)
}

// InitFromStruct initializes the document from a Go struct
func (e *DocumentEditor) InitFromStruct(v any) error {
	if v == nil {
		return errors.New("input value cannot be nil")
	}

	if err := e.modelBuilder.BuildFromStruct(v); err != nil {
		return errors.Wrap(err, "failed to build document from struct")
	}
	return nil
}

// InitFromJSON initializes the document from JSON data
func (e *DocumentEditor) InitFromJSON(data []byte) error {
	if len(data) == 0 {
		return errors.New("input JSON cannot be empty")
	}

	if err := e.modelBuilder.BuildFromJSON(data); err != nil {
		return errors.Wrap(err, "failed to build document from JSON")
	}
	return nil
}

// GetJSON returns the document as JSON
func (e *DocumentEditor) GetJSON() ([]byte, error) {
	data, err := e.modelBuilder.ToJSON(e.doc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert document to JSON")
	}
	return data, nil
}

// GetStruct populates the provided struct with data from the document
func (e *DocumentEditor) GetStruct(v any) error {
	if v == nil {
		return errors.New("output value cannot be nil")
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errors.New("output value must be a non-nil pointer")
	}

	if err := e.modelBuilder.ToStruct(e.doc, v); err != nil {
		return errors.Wrap(err, "failed to convert document to struct")
	}
	return nil
}

// GetDocument returns the underlying CRDT document
func (e *DocumentEditor) GetDocument() *crdt.Document {
	return e.doc
}
