package tracker

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
)

// TrackedState represents a tracked state of a struct.
type TrackedState struct {
	// Data is the JSON representation of the struct.
	Data []byte

	// Timestamp is when the state was captured.
	Timestamp time.Time
}

// Tracker tracks changes to a struct and generates CRDT patches.
type Tracker struct {
	// doc is the CRDT document being tracked.
	doc *crdt.Document

	// sessionID is the session ID for the tracker.
	sessionID common.SessionID

	// states maps struct types to their previous states.
	states map[reflect.Type]TrackedState

	// builder is the patch builder used to generate patches.
	builder *crdtpatch.PatchBuilder

	// snapshotManager manages snapshots of the document.
	snapshotManager *SnapshotManager

	// patches is a list of all patches applied to the document.
	patches []*crdtpatch.Patch
}

// NewTracker creates a new CRDT tracker for the given document.
func NewTracker(doc *crdt.Document, sessionID common.SessionID) *Tracker {
	return &Tracker{
		doc:             doc,
		sessionID:       sessionID,
		states:          make(map[reflect.Type]TrackedState),
		builder:         crdtpatch.NewPatchBuilder(sessionID, doc.NextTimestamp().Counter),
		snapshotManager: NewSnapshotManager(),
		patches:         make([]*crdtpatch.Patch, 0),
	}
}

// NewTrackerFromDocument creates a new CRDT tracker from an existing document
// and initializes tracking for the given struct type.
// The data parameter must be a pointer to a struct that will be filled with the document's data.
func NewTrackerFromDocument(doc *crdt.Document, sessionID common.SessionID, data interface{}) (*Tracker, error) {
	// Create a new tracker
	tracker := NewTracker(doc, sessionID)

	// Fill the struct with the document's data
	if err := tracker.ToStruct(data); err != nil {
		return nil, fmt.Errorf("failed to convert document to struct: %w", err)
	}

	// Start tracking the struct
	if err := tracker.Track(data); err != nil {
		return nil, fmt.Errorf("failed to track struct: %w", err)
	}

	return tracker, nil
}

// Track starts tracking the given struct.
// The data parameter must be a pointer to a struct.
func (t *Tracker) Track(data interface{}) error {
	// Check if data is a pointer
	dataValue := reflect.ValueOf(data)
	if dataValue.Kind() != reflect.Ptr || dataValue.IsNil() {
		return fmt.Errorf("data must be a non-nil pointer to a struct")
	}

	// Check if data points to a struct
	dataElem := dataValue.Elem()
	if dataElem.Kind() != reflect.Struct {
		return fmt.Errorf("data must be a pointer to a struct, got pointer to %v", dataElem.Kind())
	}

	// Convert struct to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal struct: %w", err)
	}

	// Store the state
	t.states[dataElem.Type()] = TrackedState{
		Data:      jsonData,
		Timestamp: time.Now(),
	}

	return nil
}

// Update updates the tracked struct and returns the CRDT patch.
// The data parameter must be a pointer to a struct.
func (t *Tracker) Update(data interface{}) (*crdtpatch.Patch, error) {
	// Check if data is a pointer
	dataValue := reflect.ValueOf(data)
	if dataValue.Kind() != reflect.Ptr || dataValue.IsNil() {
		return nil, fmt.Errorf("data must be a non-nil pointer to a struct")
	}

	// Check if data points to a struct
	dataElem := dataValue.Elem()
	if dataElem.Kind() != reflect.Struct {
		return nil, fmt.Errorf("data must be a pointer to a struct, got pointer to %v", dataElem.Kind())
	}

	// Check if we're tracking this struct type
	prevState, ok := t.states[dataElem.Type()]
	if !ok {
		// If not, start tracking it
		if err := t.Track(data); err != nil {
			return nil, err
		}
		// Return an empty patch
		return crdtpatch.NewPatch(common.LogicalTimestamp{}), nil
	}

	// Get the changes
	changes, err := t.GetChanges(prevState.Data, data)
	if err != nil {
		return nil, fmt.Errorf("failed to get changes: %w", err)
	}

	// If there are no changes, return an empty patch
	if len(changes) == 0 {
		return crdtpatch.NewPatch(common.LogicalTimestamp{}), nil
	}

	// Generate a patch from the changes
	patch, err := t.GeneratePatch(changes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate patch: %w", err)
	}

	// Update the tracked state
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct: %w", err)
	}

	t.states[dataElem.Type()] = TrackedState{
		Data:      jsonData,
		Timestamp: time.Now(),
	}

	return patch, nil
}

// GetChanges returns the changes between the previous and current state.
func (t *Tracker) GetChanges(prevData []byte, currentData interface{}) ([]Change, error) {
	// Create a temporary struct to hold the previous state
	// For simplicity, we'll use a map[string]interface{}
	var prevMap map[string]interface{}
	if err := json.Unmarshal(prevData, &prevMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal previous state: %w", err)
	}

	// Convert current data to map
	currentJSON, err := json.Marshal(currentData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal current data: %w", err)
	}

	var currentMap map[string]interface{}
	if err := json.Unmarshal(currentJSON, &currentMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal current data: %w", err)
	}

	// Create a diff result
	result := &DiffResult{
		Changes:   make([]Change, 0),
		Timestamp: time.Now(),
	}

	// Compare the maps
	diffMaps(prevMap, currentMap, "", result)

	return result.Changes, nil
}

// GeneratePatch generates a CRDT patch from the changes.
func (t *Tracker) GeneratePatch(changes []Change) (*crdtpatch.Patch, error) {
	// Create a new patch
	patchID := t.builder.NextTimestamp()

	patch := crdtpatch.NewPatch(patchID)

	// Get the root node
	rootNode := t.doc.Root()

	// Get the root object
	var rootObj *crdt.LWWObjectNode
	if lwwVal, ok := rootNode.(*crdt.LWWValueNode); ok {
		if lwwVal.NodeValue != nil {
			if obj, ok := lwwVal.NodeValue.(*crdt.LWWObjectNode); ok {
				rootObj = obj
			}
		}
	}

	// If the root object doesn't exist, create it
	if rootObj == nil {
		// Create a new object node
		rootID := t.doc.NextTimestamp()
		rootObj = crdt.NewLWWObjectNode(rootID)
		t.doc.AddNode(rootObj)

		// Set the root value
		if lwwVal, ok := rootNode.(*crdt.LWWValueNode); ok {
			lwwVal.SetValue(rootID, rootObj)
		}
	}

	// Add operations for each change
	for _, change := range changes {
		switch change.Type {
		case ChangeTypeCreate, ChangeTypeUpdate:
			// Create a constant node for the value
			valueID := t.doc.NextTimestamp()
			valueOp := &crdtpatch.NewOperation{
				ID:       valueID,
				NodeType: common.NodeTypeCon,
				Value:    change.NewValue,
			}
			patch.AddOperation(valueOp)

			// Create an insert operation to update the field
			insID := t.doc.NextTimestamp()

			// Handle nested paths
			pathParts := strings.Split(change.Path, ".")
			if len(pathParts) > 1 {
				// This is a nested path, we need to get the actual object
				// Get the root object from the document view
				view, err := t.doc.View()
				if err != nil {
					return nil, fmt.Errorf("failed to get document view: %w", err)
				}

				// Convert view to map
				viewMap, ok := view.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("document view is not a map")
				}

				// Navigate to the parent object
				parentPath := pathParts[0]
				parentObj, exists := viewMap[parentPath]
				if !exists {
					// Parent object doesn't exist, create it
					parentObj = make(map[string]interface{})
					viewMap[parentPath] = parentObj
				}

				// Convert to map
				parentMap, ok := parentObj.(map[string]interface{})
				if !ok {
					// Parent exists but is not a map, replace it
					parentMap = make(map[string]interface{})
					viewMap[parentPath] = parentMap
				}

				// Update the field in the parent object
				parentMap[pathParts[1]] = change.NewValue

				// Create an insert operation for the parent object
				insOp := &crdtpatch.InsOperation{
					ID:       insID,
					TargetID: rootObj.ID(),
					Value:    map[string]interface{}{pathParts[0]: parentMap},
				}
				patch.AddOperation(insOp)
			} else {
				// This is a top-level field
				insOp := &crdtpatch.InsOperation{
					ID:       insID,
					TargetID: rootObj.ID(),
					Value:    map[string]interface{}{change.Path: change.NewValue},
				}
				patch.AddOperation(insOp)
			}

		case ChangeTypeDelete:
			// Create a delete operation
			delID := t.doc.NextTimestamp()

			// Handle nested paths
			pathParts := strings.Split(change.Path, ".")
			if len(pathParts) > 1 {
				// This is a nested path, we need to handle it differently
				// Get the root object from the document view
				view, err := t.doc.View()
				if err != nil {
					return nil, fmt.Errorf("failed to get document view: %w", err)
				}

				// Convert view to map
				viewMap, ok := view.(map[string]interface{})
				if !ok {
					return nil, fmt.Errorf("document view is not a map")
				}

				// Navigate to the parent object
				parentPath := pathParts[0]
				parentObj, exists := viewMap[parentPath]
				if !exists {
					// Parent object doesn't exist, nothing to delete
					continue
				}

				// Convert to map
				parentMap, ok := parentObj.(map[string]interface{})
				if !ok {
					// Parent exists but is not a map, nothing to delete
					continue
				}

				// Delete the field from the parent object
				delete(parentMap, pathParts[1])

				// Create an insert operation for the updated parent object
				insID := t.doc.NextTimestamp()
				insOp := &crdtpatch.InsOperation{
					ID:       insID,
					TargetID: rootObj.ID(),
					Value:    map[string]interface{}{pathParts[0]: parentMap},
				}
				patch.AddOperation(insOp)
			} else {
				// This is a top-level field
				delOp := &crdtpatch.DelOperation{
					ID:       delID,
					TargetID: rootObj.ID(),
					Key:      change.Path,
				}
				patch.AddOperation(delOp)
			}
		}
	}

	return patch, nil
}

// ApplyPatch applies a CRDT patch to the document.
func (t *Tracker) ApplyPatch(patch *crdtpatch.Patch) error {
	// Apply the patch to the document
	if err := patch.Apply(t.doc); err != nil {
		return err
	}

	// Add the patch to the list of applied patches
	t.patches = append(t.patches, patch.Clone())

	return nil
}

// GetDocument returns the CRDT document being tracked.
func (t *Tracker) GetDocument() *crdt.Document {
	return t.doc
}

// GetSessionID returns the session ID of the tracker.
func (t *Tracker) GetSessionID() common.SessionID {
	return t.sessionID
}

// GetPreviousState returns the previous state of the given struct type.
func (t *Tracker) GetPreviousState(dataType reflect.Type) (TrackedState, bool) {
	state, ok := t.states[dataType]
	return state, ok
}

// ClearState clears the tracked state for the given struct type.
func (t *Tracker) ClearState(dataType reflect.Type) {
	delete(t.states, dataType)
}

// ClearAllStates clears all tracked states.
func (t *Tracker) ClearAllStates() {
	t.states = make(map[reflect.Type]TrackedState)
}

// GetPatchBuilder returns the patch builder used by the tracker.
func (t *Tracker) GetPatchBuilder() *crdtpatch.PatchBuilder {
	return t.builder
}

// ResetPatchBuilder resets the patch builder's counter.
func (t *Tracker) ResetPatchBuilder() {
	t.builder = crdtpatch.NewPatchBuilder(t.sessionID, 0)
}

// InitializeDocument initializes the document with the given struct.
func (t *Tracker) InitializeDocument(data interface{}) error {
	// Check if data is a pointer
	dataValue := reflect.ValueOf(data)
	if dataValue.Kind() != reflect.Ptr || dataValue.IsNil() {
		return fmt.Errorf("data must be a non-nil pointer to a struct")
	}

	// Check if data points to a struct
	dataElem := dataValue.Elem()
	if dataElem.Kind() != reflect.Struct {
		return fmt.Errorf("data must be a pointer to a struct, got pointer to %v", dataElem.Kind())
	}

	// Convert struct to map
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal struct: %w", err)
	}

	var dataMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &dataMap); err != nil {
		return fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	// Get the root node
	zeroSID := common.SessionID{}
	rootNode, err := t.doc.GetNode(common.LogicalTimestamp{SID: zeroSID, Counter: 0})
	if err != nil {
		return fmt.Errorf("failed to get root node: %w", err)
	}

	// Create a new object node
	rootID := t.doc.NextTimestamp()
	rootObj := crdt.NewLWWObjectNode(rootID)
	t.doc.AddNode(rootObj)

	// Set the root value
	if lwwVal, ok := rootNode.(*crdt.LWWValueNode); ok {
		lwwVal.SetValue(rootID, rootObj)
	}

	// Add fields to the object
	for key, value := range dataMap {
		// Check if the value is a map (nested object)
		if nestedMap, isMap := value.(map[string]interface{}); isMap {
			// Create a new object node for the nested object
			nestedID := t.doc.NextTimestamp()
			nestedObj := crdt.NewLWWObjectNode(nestedID)
			t.doc.AddNode(nestedObj)

			// Add the nested object to the root object
			rootObj.Set(key, nestedID, nestedObj)

			// Add fields to the nested object
			for nestedKey, nestedValue := range nestedMap {
				// Create a constant node for the nested value
				nestedValueID := t.doc.NextTimestamp()
				nestedValueNode := crdt.NewConstantNode(nestedValueID, nestedValue)
				t.doc.AddNode(nestedValueNode)

				// Add the field to the nested object
				nestedObj.Set(nestedKey, nestedValueID, nestedValueNode)
			}
		} else {
			// Create a constant node for the value
			valueID := t.doc.NextTimestamp()
			valueNode := crdt.NewConstantNode(valueID, value)
			t.doc.AddNode(valueNode)

			// Add the field to the object
			rootObj.Set(key, valueID, valueNode)
		}
	}

	// Track the struct
	return t.Track(data)
}

// GetView returns the current view of the document.
func (t *Tracker) GetView() (interface{}, error) {
	return t.doc.View()
}

// CreateSnapshot creates a snapshot of the current document state.
func (t *Tracker) CreateSnapshot(id string) (*Snapshot, error) {
	return t.snapshotManager.CreateSnapshot(t.doc, id)
}

// GetSnapshot returns the snapshot with the given ID.
func (t *Tracker) GetSnapshot(id string) (*Snapshot, error) {
	return t.snapshotManager.GetSnapshot(id)
}

// ListSnapshots returns a list of all snapshots.
func (t *Tracker) ListSnapshots() []*Snapshot {
	return t.snapshotManager.ListSnapshots()
}

// DeleteSnapshot deletes the snapshot with the given ID.
func (t *Tracker) DeleteSnapshot(id string) error {
	return t.snapshotManager.DeleteSnapshot(id)
}

// TimeTravel creates a new document from the snapshot with the given ID.
// This does not modify the current document, but returns a new document.
func (t *Tracker) TimeTravel(id string) (*crdt.Document, error) {
	return t.snapshotManager.TimeTravel(id)
}

// TimeTravelToTime creates a new document from the snapshot closest to the given time.
// This does not modify the current document, but returns a new document.
func (t *Tracker) TimeTravelToTime(timestamp time.Time) (*crdt.Document, error) {
	return t.snapshotManager.TimeTravelToTime(timestamp)
}

// RevertToSnapshot reverts the current document to the state in the snapshot with the given ID.
func (t *Tracker) RevertToSnapshot(id string) error {
	// Get the snapshot
	snapshot, err := t.snapshotManager.GetSnapshot(id)
	if err != nil {
		return err
	}

	// Restore the document
	doc, err := snapshot.RestoreDocument()
	if err != nil {
		return err
	}

	// Replace the current document with the restored one
	*t.doc = *doc

	// Clear the patches list
	t.patches = make([]*crdtpatch.Patch, 0)

	// Clear the states
	t.states = make(map[reflect.Type]TrackedState)

	return nil
}

// RevertToTime reverts the current document to the state in the snapshot closest to the given time.
func (t *Tracker) RevertToTime(timestamp time.Time) error {
	// Get the snapshot
	snapshot, err := t.snapshotManager.GetSnapshotByTime(timestamp)
	if err != nil {
		return err
	}

	// Restore the document
	doc, err := snapshot.RestoreDocument()
	if err != nil {
		return err
	}

	// Replace the current document with the restored one
	*t.doc = *doc

	// Clear the patches list
	t.patches = make([]*crdtpatch.Patch, 0)

	// Clear the states
	t.states = make(map[reflect.Type]TrackedState)

	return nil
}

// GetPatches returns all patches applied to the document.
func (t *Tracker) GetPatches() []*crdtpatch.Patch {
	return t.patches
}

// ToStruct converts the document view to a struct.
func (t *Tracker) ToStruct(result interface{}) error {
	// Check if result is a pointer
	resultValue := reflect.ValueOf(result)
	if resultValue.Kind() != reflect.Ptr || resultValue.IsNil() {
		return fmt.Errorf("result must be a non-nil pointer to a struct")
	}

	// Check if result points to a struct
	resultElem := resultValue.Elem()
	if resultElem.Kind() != reflect.Struct {
		return fmt.Errorf("result must be a pointer to a struct, got pointer to %v", resultElem.Kind())
	}

	// Get the document view
	view, err := t.doc.View()
	if err != nil {
		return fmt.Errorf("failed to get document view: %w", err)
	}

	// Convert view to JSON
	jsonData, err := json.Marshal(view)
	if err != nil {
		return fmt.Errorf("failed to marshal view: %w", err)
	}

	// Unmarshal JSON to struct
	if err := json.Unmarshal(jsonData, result); err != nil {
		return fmt.Errorf("failed to unmarshal to struct: %w", err)
	}

	return nil
}

// TrackFromDocument fills the given struct with the document's data and starts tracking it.
// The data parameter must be a pointer to a struct.
func (t *Tracker) TrackFromDocument(data interface{}) error {
	// Fill the struct with the document's data
	if err := t.ToStruct(data); err != nil {
		return fmt.Errorf("failed to convert document to struct: %w", err)
	}

	// Start tracking the struct
	if err := t.Track(data); err != nil {
		return fmt.Errorf("failed to track struct: %w", err)
	}

	return nil
}
