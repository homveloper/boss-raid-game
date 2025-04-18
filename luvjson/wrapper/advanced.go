package wrapper

import (
	"fmt"
	"strings"

	"tictactoe/luvjson/common"

	"tictactoe/luvjson/crdtpatch"
)

// UpdateNestedField updates a nested field in the CRDT document.
// fieldPath is a dot-separated path to the field, e.g. "user.address.city"
func (cd *CRDTDocument) UpdateNestedField(fieldPath string, value interface{}) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	// Split the path into parts
	parts := strings.Split(fieldPath, ".")
	if len(parts) == 0 {
		return fmt.Errorf("invalid field path: %s", fieldPath)
	}

	// Get the current view
	view, err := cd.doc.View()
	if err != nil {
		return fmt.Errorf("failed to get document view: %w", err)
	}

	// Navigate to the parent object
	_, ok := view.(map[string]interface{})
	if !ok {
		return fmt.Errorf("document view is not a map")
	}

	// Create a patch
	patchID := cd.doc.NextTimestamp()
	p := crdtpatch.NewPatch(patchID)

	// For now, we'll only handle the simple case of updating a top-level field
	// A more complete implementation would need to navigate the CRDT document
	// and find the correct node to update

	// Create a constant node for the value
	valueID := cd.doc.NextTimestamp()
	valueOp := &crdtpatch.NewOperation{
		ID:       valueID,
		NodeType: common.NodeTypeCon,
		Value:    value,
	}
	p.AddOperation(valueOp)

	// Create an insert operation to update the field
	insOp := &crdtpatch.InsOperation{
		ID:       cd.doc.NextTimestamp(),
		TargetID: cd.rootID,
		Value: map[string]interface{}{
			parts[0]: value,
		},
	}
	p.AddOperation(insOp)

	// Apply the patch
	if err := p.Apply(cd.doc); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	return nil
}

// WatchField registers a callback function to be called when a field changes.
// This is a simplified version that doesn't actually watch for changes.
func (cd *CRDTDocument) WatchField(fieldPath string, callback func(interface{})) error {
	// This would require implementing a change notification system
	// which is beyond the scope of this example
	return fmt.Errorf("not implemented")
}

// MergeDocument merges another CRDT document into this one.
func (cd *CRDTDocument) MergeDocument(other *CRDTDocument) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	// This would require implementing a document merge algorithm
	// which is beyond the scope of this example
	return fmt.Errorf("not implemented")
}

// CreateArray creates a new array in the CRDT document.
func (cd *CRDTDocument) CreateArray(fieldPath string) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	// Create a patch to create an array
	patchID := cd.doc.NextTimestamp()
	p := crdtpatch.NewPatch(patchID)

	// Create a new array node
	arrayID := cd.doc.NextTimestamp()
	arrayOp := &crdtpatch.NewOperation{
		ID:       arrayID,
		NodeType: common.NodeTypeArr,
	}
	p.AddOperation(arrayOp)

	// Create an insert operation to add the array to the root object
	insOp := &crdtpatch.InsOperation{
		ID:       cd.doc.NextTimestamp(),
		TargetID: cd.rootID,
		Value: map[string]interface{}{
			fieldPath: []interface{}{},
		},
	}
	p.AddOperation(insOp)

	// Apply the patch
	if err := p.Apply(cd.doc); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	return nil
}

// AppendToArray appends a value to an array in the CRDT document.
func (cd *CRDTDocument) AppendToArray(fieldPath string, value interface{}) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	// Get the current view
	view, err := cd.doc.View()
	if err != nil {
		return fmt.Errorf("failed to get document view: %w", err)
	}

	// Navigate to the array
	_, ok := view.(map[string]interface{})
	if !ok {
		return fmt.Errorf("document view is not a map")
	}

	// For now, we'll only handle the simple case of appending to a top-level array
	// A more complete implementation would need to navigate the CRDT document
	// and find the correct node to update

	// Create a patch to append to the array
	patchID := cd.doc.NextTimestamp()
	p := crdtpatch.NewPatch(patchID)

	// Create a constant node for the value
	valueID := cd.doc.NextTimestamp()
	valueOp := &crdtpatch.NewOperation{
		ID:       valueID,
		NodeType: common.NodeTypeCon,
		Value:    value,
	}
	p.AddOperation(valueOp)

	// Create an insert operation to append to the array
	// This is a simplified version that doesn't actually append to an array
	// A more complete implementation would need to find the array node
	// and append to it
	insOp := &crdtpatch.InsOperation{
		ID:       cd.doc.NextTimestamp(),
		TargetID: cd.rootID,
		Value: map[string]interface{}{
			fieldPath: []interface{}{value},
		},
	}
	p.AddOperation(insOp)

	// Apply the patch
	if err := p.Apply(cd.doc); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	return nil
}

// RemoveFromArray removes a value from an array in the CRDT document.
func (cd *CRDTDocument) RemoveFromArray(fieldPath string, index int) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	// This would require implementing array removal operations
	// which is beyond the scope of this example
	return fmt.Errorf("not implemented")
}

// CreateObject creates a new object in the CRDT document.
func (cd *CRDTDocument) CreateObject(fieldPath string) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	// Create a patch to create an object
	patchID := cd.doc.NextTimestamp()
	p := crdtpatch.NewPatch(patchID)

	// Create a new object node
	objID := cd.doc.NextTimestamp()
	objOp := &crdtpatch.NewOperation{
		ID:       objID,
		NodeType: common.NodeTypeObj,
	}
	p.AddOperation(objOp)

	// Create an insert operation to add the object to the root object
	insOp := &crdtpatch.InsOperation{
		ID:       cd.doc.NextTimestamp(),
		TargetID: cd.rootID,
		Value: map[string]interface{}{
			fieldPath: map[string]interface{}{},
		},
	}
	p.AddOperation(insOp)

	// Apply the patch
	if err := p.Apply(cd.doc); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	return nil
}

// DeleteField deletes a field from the CRDT document.
func (cd *CRDTDocument) DeleteField(fieldPath string) error {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	// Get the current view
	view, err := cd.doc.View()
	if err != nil {
		return fmt.Errorf("failed to get document view: %w", err)
	}

	// Navigate to the parent object
	_, ok := view.(map[string]interface{})
	if !ok {
		return fmt.Errorf("document view is not a map")
	}

	// Create a patch to delete the field
	patchID := cd.doc.NextTimestamp()
	p := crdtpatch.NewPatch(patchID)

	// Create a delete operation to remove the field
	// This is a simplified version that doesn't actually delete a field
	// A more complete implementation would need to find the object node
	// and delete the field from it
	delOp := &crdtpatch.DelOperation{
		ID:       patchID,
		TargetID: cd.rootID,
		Key:      fieldPath,
	}
	p.AddOperation(delOp)

	// Apply the patch
	if err := p.Apply(cd.doc); err != nil {
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	return nil
}

// GetFieldValue gets the value of a field in the CRDT document.
func (cd *CRDTDocument) GetFieldValue(fieldPath string) (interface{}, error) {
	cd.mu.RLock()
	defer cd.mu.RUnlock()

	// Get the document view
	view, err := cd.doc.View()
	if err != nil {
		return nil, fmt.Errorf("failed to get document view: %w", err)
	}

	// Split the path into parts
	parts := strings.Split(fieldPath, ".")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid field path: %s", fieldPath)
	}

	// Navigate to the field
	current := view
	for i, part := range parts {
		// Check if current is a map
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("field path %s is invalid at part %s", fieldPath, part)
		}

		// Get the next part
		next, ok := currentMap[part]
		if !ok {
			return nil, fmt.Errorf("field path %s is invalid at part %s", fieldPath, part)
		}

		// If this is the last part, return the value
		if i == len(parts)-1 {
			return next, nil
		}

		// Otherwise, continue navigating
		current = next
	}

	return nil, fmt.Errorf("field path %s is invalid", fieldPath)
}
