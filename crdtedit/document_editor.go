package crdtedit

import (
	"encoding/json"
	"errors"
	"reflect"

	"github.com/yourusername/luvjson/crdt"
)

// DocumentEditor provides methods for editing a CRDT document
type DocumentEditor struct {
	doc          *crdt.Document
	pathResolver *PathResolver
	queryEngine  *QueryEngine
	modelBuilder *ModelBuilder
	patchBuilder *PatchBuilder
}

// NewDocumentEditor creates a new DocumentEditor for the given document
func NewDocumentEditor(doc *crdt.Document) *DocumentEditor {
	pathResolver := NewPathResolver(doc)
	patchBuilder := NewPatchBuilder(doc)
	
	return &DocumentEditor{
		doc:          doc,
		pathResolver: pathResolver,
		queryEngine:  NewQueryEngine(doc, pathResolver),
		modelBuilder: NewModelBuilder(doc, patchBuilder),
		patchBuilder: patchBuilder,
	}
}

// Edit applies edits to the document using the provided edit function
func (e *DocumentEditor) Edit(fn EditFunc) error {
	if fn == nil {
		return errors.New("edit function cannot be nil")
	}

	ctx := NewEditContext(e.doc, e.pathResolver, e.patchBuilder)
	if err := fn(ctx); err != nil {
		return err
	}

	return nil
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

	return e.modelBuilder.BuildFromStruct(v)
}

// InitFromJSON initializes the document from JSON data
func (e *DocumentEditor) InitFromJSON(data []byte) error {
	if len(data) == 0 {
		return errors.New("input JSON cannot be empty")
	}

	return e.modelBuilder.BuildFromJSON(data)
}

// GetJSON returns the document as JSON
func (e *DocumentEditor) GetJSON() ([]byte, error) {
	return e.modelBuilder.ToJSON(e.doc)
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

	return e.modelBuilder.ToStruct(e.doc, v)
}

// GetDocument returns the underlying CRDT document
func (e *DocumentEditor) GetDocument() *crdt.Document {
	return e.doc
}
