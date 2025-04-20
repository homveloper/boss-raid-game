package api

import (
	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
)

// Model represents a JSON CRDT document with a high-level API.
type Model struct {
	// doc is the underlying CRDT document.
	doc *crdt.Document

	// api is the API for manipulating the document.
	api *ModelApi
}

// NewModel creates a new Model with the given session ID.
func NewModel(sessionID common.SessionID) *Model {
	doc := crdt.NewDocument(sessionID)
	return &Model{
		doc: doc,
		api: NewModelApi(doc),
	}
}

// NewModelWithDocument creates a new Model with the given document.
func NewModelWithDocument(doc *crdt.Document) *Model {
	return &Model{
		doc: doc,
		api: NewModelApi(doc),
	}
}

// GetDocument returns the underlying CRDT document.
func (m *Model) GetDocument() *crdt.Document {
	return m.doc
}

// GetApi returns the API for manipulating the document.
func (m *Model) GetApi() *ModelApi {
	return m.api
}

// View returns the current view of the document.
func (m *Model) View() (interface{}, error) {
	return m.doc.View()
}

// ApplyPatch applies a patch to the document.
func (m *Model) ApplyPatch(patch *crdtpatch.Patch) error {
	// Emit the before patch event
	m.api.emitBeforePatch(patch)

	// Apply the patch
	if err := patch.Apply(m.doc); err != nil {
		return err
	}

	// Emit the patch event
	m.api.emitPatch(patch)

	return nil
}

// ToBinary serializes the document to binary format.
func (m *Model) ToBinary() ([]byte, error) {
	// TODO: Implement binary serialization
	return nil, nil
}

// FromBinary deserializes the document from binary format.
func (m *Model) FromBinary(data []byte) error {
	// TODO: Implement binary deserialization
	return nil
}

// ToJSON serializes the document to JSON format.
func (m *Model) ToJSON() ([]byte, error) {
	// TODO: Implement JSON serialization
	return nil, nil
}

// FromJSON deserializes the document from JSON format.
func (m *Model) FromJSON(data []byte) error {
	// TODO: Implement JSON deserialization
	return nil
}

// String returns a string representation of the document.
func (m *Model) String() string {
	return m.doc.String()
}
