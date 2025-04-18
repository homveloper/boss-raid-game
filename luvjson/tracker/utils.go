package tracker

import (
	"encoding/json"
	"fmt"
	"reflect"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
)

// GenerateJSONCRDTPatch generates a JSON CRDT patch from two structs.
// The old and new parameters must be pointers to structs.
func GenerateJSONCRDTPatch(old, new interface{}, sessionID common.SessionID) ([]byte, error) {
	// Create a temporary tracker
	doc := crdt.NewDocument(sessionID)
	tracker := NewTracker(doc, sessionID)

	// Initialize the document with the old state
	if err := tracker.InitializeDocument(old); err != nil {
		return nil, fmt.Errorf("failed to initialize document: %w", err)
	}

	// Update with the new state
	patch, err := tracker.Update(new)
	if err != nil {
		return nil, fmt.Errorf("failed to update with new state: %w", err)
	}

	// Convert the patch to JSON
	patchData, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal patch: %w", err)
	}

	return patchData, nil
}

// ApplyJSONCRDTPatch applies a JSON CRDT patch to a struct.
// The target parameter must be a pointer to a struct.
func ApplyJSONCRDTPatch(target interface{}, patchData []byte, sessionID common.SessionID) error {
	// Create a temporary document
	doc := crdt.NewDocument(sessionID)

	// Create a tracker
	tracker := NewTracker(doc, sessionID)

	// Initialize the document with the target struct
	if err := tracker.InitializeDocument(target); err != nil {
		return fmt.Errorf("failed to initialize document: %w", err)
	}

	// Parse the patch
	patch := crdtpatch.NewPatch(common.LogicalTimestamp{SID: sessionID})
	if err := json.Unmarshal(patchData, patch); err != nil {
		return fmt.Errorf("failed to parse patch: %w", err)
	}

	// Apply the patch
	if err := tracker.ApplyPatch(patch); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	// Update the target struct
	if err := tracker.ToStruct(target); err != nil {
		return fmt.Errorf("failed to update target struct: %w", err)
	}

	return nil
}

// CompareStructs compares two structs and returns true if they are equal.
// The a and b parameters must be pointers to structs.
func CompareStructs(a, b interface{}) (bool, error) {
	// Check if a and b are pointers
	aValue := reflect.ValueOf(a)
	bValue := reflect.ValueOf(b)
	if aValue.Kind() != reflect.Ptr || aValue.IsNil() || bValue.Kind() != reflect.Ptr || bValue.IsNil() {
		return false, fmt.Errorf("a and b must be non-nil pointers to structs")
	}

	// Check if a and b point to structs
	aElem := aValue.Elem()
	bElem := bValue.Elem()
	if aElem.Kind() != reflect.Struct || bElem.Kind() != reflect.Struct {
		return false, fmt.Errorf("a and b must be pointers to structs")
	}

	// Convert a and b to JSON
	aJSON, err := json.Marshal(a)
	if err != nil {
		return false, fmt.Errorf("failed to marshal a: %w", err)
	}

	bJSON, err := json.Marshal(b)
	if err != nil {
		return false, fmt.Errorf("failed to marshal b: %w", err)
	}

	// Compare the JSON representations
	return string(aJSON) == string(bJSON), nil
}

// CloneStruct creates a deep copy of a struct.
// The src parameter must be a pointer to a struct.
// The dst parameter must be a pointer to a struct of the same type.
func CloneStruct(src, dst interface{}) error {
	// Check if src and dst are pointers
	srcValue := reflect.ValueOf(src)
	dstValue := reflect.ValueOf(dst)
	if srcValue.Kind() != reflect.Ptr || srcValue.IsNil() || dstValue.Kind() != reflect.Ptr || dstValue.IsNil() {
		return fmt.Errorf("src and dst must be non-nil pointers to structs")
	}

	// Check if src and dst point to structs
	srcElem := srcValue.Elem()
	dstElem := dstValue.Elem()
	if srcElem.Kind() != reflect.Struct || dstElem.Kind() != reflect.Struct {
		return fmt.Errorf("src and dst must be pointers to structs")
	}

	// Check if src and dst are of the same type
	if srcElem.Type() != dstElem.Type() {
		return fmt.Errorf("src and dst must be pointers to structs of the same type")
	}

	// Convert src to JSON
	srcJSON, err := json.Marshal(src)
	if err != nil {
		return fmt.Errorf("failed to marshal src: %w", err)
	}

	// Unmarshal JSON to dst
	if err := json.Unmarshal(srcJSON, dst); err != nil {
		return fmt.Errorf("failed to unmarshal to dst: %w", err)
	}

	return nil
}

// TrackableStruct is a struct that can be tracked by the CRDT tracker.
type TrackableStruct struct {
	// tracker is the CRDT tracker.
	tracker *Tracker

	// data is the struct being tracked.
	data interface{}
}

// NewTrackableStruct creates a new trackable struct.
// The data parameter must be a pointer to a struct.
func NewTrackableStruct(data interface{}, sessionID common.SessionID) (*TrackableStruct, error) {
	// Create a new tracker
	doc := crdt.NewDocument(sessionID)
	tracker := NewTracker(doc, sessionID)

	// Initialize the document with the data
	if err := tracker.InitializeDocument(data); err != nil {
		return nil, fmt.Errorf("failed to initialize document: %w", err)
	}

	return &TrackableStruct{
		tracker: tracker,
		data:    data,
	}, nil
}

// Update updates the tracked struct and returns the CRDT patch.
func (ts *TrackableStruct) Update() (*crdtpatch.Patch, error) {
	return ts.tracker.Update(ts.data)
}

// GetData returns the tracked struct.
func (ts *TrackableStruct) GetData() interface{} {
	return ts.data
}

// GetTracker returns the CRDT tracker.
func (ts *TrackableStruct) GetTracker() *Tracker {
	return ts.tracker
}
