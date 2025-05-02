package core

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// Operation represents a CRDT operation
type Operation struct {
	Type      string          `json:"type"`
	Path      string          `json:"path"`
	Value     json.RawMessage `json:"value,omitempty"`
	Timestamp int64           `json:"timestamp"`
	ClientID  string          `json:"clientId"`
}

// Patch represents a collection of operations
type Patch struct {
	Operations []Operation `json:"operations"`
	DocumentID string      `json:"documentId"`
	BaseVersion int64      `json:"baseVersion"`
	ClientID   string      `json:"clientId"`
}

// Document represents a CRDT document
type Document struct {
	ID        string                 `json:"id"`
	Content   map[string]interface{} `json:"content"`
	Version   int64                  `json:"version"`
	mutex     sync.RWMutex
	listeners []func(*Patch)
}

// NewDocument creates a new CRDT document
func NewDocument(id string) *Document {
	return &Document{
		ID:        id,
		Content:   make(map[string]interface{}),
		Version:   1,
		listeners: make([]func(*Patch), 0),
	}
}

// ApplyPatch applies a patch to the document
func (d *Document) ApplyPatch(patch *Patch) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Validate patch
	if patch.DocumentID != d.ID {
		return errors.New("patch document ID does not match")
	}

	if patch.BaseVersion > d.Version {
		return errors.New("patch base version is ahead of document version")
	}

	// Apply operations
	for _, op := range patch.Operations {
		if err := d.applyOperation(op); err != nil {
			return err
		}
	}

	// Update version
	d.Version++

	// Notify listeners
	for _, listener := range d.listeners {
		listener(patch)
	}

	return nil
}

// applyOperation applies a single operation to the document
func (d *Document) applyOperation(op Operation) error {
	// Implementation depends on the specific CRDT algorithm
	// This is a simplified example
	switch op.Type {
	case "add":
		var value interface{}
		if err := json.Unmarshal(op.Value, &value); err != nil {
			return err
		}
		d.Content[op.Path] = value
	case "remove":
		delete(d.Content, op.Path)
	case "replace":
		var value interface{}
		if err := json.Unmarshal(op.Value, &value); err != nil {
			return err
		}
		d.Content[op.Path] = value
	default:
		return errors.New("unknown operation type")
	}
	return nil
}

// GetContent returns the document content
func (d *Document) GetContent() map[string]interface{} {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	// Create a deep copy to avoid concurrent modification
	contentCopy := make(map[string]interface{})
	for k, v := range d.Content {
		contentCopy[k] = v
	}
	
	return contentCopy
}

// AddListener adds a listener for document changes
func (d *Document) AddListener(listener func(*Patch)) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	d.listeners = append(d.listeners, listener)
}

// CreatePatch creates a new patch with the given operations
func (d *Document) CreatePatch(clientID string, operations []Operation) *Patch {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	return &Patch{
		Operations: operations,
		DocumentID: d.ID,
		BaseVersion: d.Version,
		ClientID: clientID,
	}
}

// CreateOperation creates a new operation
func CreateOperation(opType, path string, value interface{}, clientID string) (Operation, error) {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return Operation{}, err
	}
	
	return Operation{
		Type:      opType,
		Path:      path,
		Value:     valueJSON,
		Timestamp: time.Now().UnixNano(),
		ClientID:  clientID,
	}, nil
}
