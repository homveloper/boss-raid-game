package wrapper

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// JSONPatchOperation represents a JSON Patch operation as defined in RFC6902.
type JSONPatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
	From  string      `json:"from,omitempty"`
}

// JSONPatch represents a JSON Patch document as defined in RFC6902.
type JSONPatch []JSONPatchOperation

// ChangesToJSONPatch converts a list of changes to a JSON Patch document.
func ChangesToJSONPatch(changes []Change) JSONPatch {
	patch := make(JSONPatch, 0, len(changes))
	
	for _, change := range changes {
		path := "/" + strings.ReplaceAll(change.Path, ".", "/")
		
		switch change.Type {
		case ChangeTypeCreate:
			patch = append(patch, JSONPatchOperation{
				Op:    "add",
				Path:  path,
				Value: change.NewValue,
			})
		case ChangeTypeUpdate:
			patch = append(patch, JSONPatchOperation{
				Op:    "replace",
				Path:  path,
				Value: change.NewValue,
			})
		case ChangeTypeDelete:
			patch = append(patch, JSONPatchOperation{
				Op:   "remove",
				Path: path,
			})
		}
	}
	
	return patch
}

// JSONPatchToChanges converts a JSON Patch document to a list of changes.
func JSONPatchToChanges(patch JSONPatch) ([]Change, error) {
	changes := make([]Change, 0, len(patch))
	
	for _, op := range patch {
		// Convert JSON Patch path to dot notation
		path := strings.TrimPrefix(op.Path, "/")
		path = strings.ReplaceAll(path, "/", ".")
		
		// Handle array indices
		parts := strings.Split(path, ".")
		for i, part := range parts {
			if _, err := strconv.Atoi(part); err == nil {
				// This part is an array index
				if i > 0 {
					// Combine with previous part
					parts[i-1] = parts[i-1] + "[" + part + "]"
					// Remove this part
					parts = append(parts[:i], parts[i+1:]...)
					i--
				}
			}
		}
		path = strings.Join(parts, ".")
		
		switch op.Op {
		case "add":
			changes = append(changes, Change{
				Path:     path,
				Type:     ChangeTypeCreate,
				NewValue: op.Value,
			})
		case "replace":
			changes = append(changes, Change{
				Path:     path,
				Type:     ChangeTypeUpdate,
				NewValue: op.Value,
			})
		case "remove":
			changes = append(changes, Change{
				Path: path,
				Type: ChangeTypeDelete,
			})
		case "move":
			fromPath := strings.TrimPrefix(op.From, "/")
			fromPath = strings.ReplaceAll(fromPath, "/", ".")
			
			// First remove from the source
			changes = append(changes, Change{
				Path: fromPath,
				Type: ChangeTypeDelete,
			})
			
			// Then add to the destination
			changes = append(changes, Change{
				Path:     path,
				Type:     ChangeTypeCreate,
				NewValue: op.Value,
			})
		case "copy":
			fromPath := strings.TrimPrefix(op.From, "/")
			fromPath = strings.ReplaceAll(fromPath, "/", ".")
			
			// Add to the destination
			changes = append(changes, Change{
				Path:     path,
				Type:     ChangeTypeCreate,
				NewValue: op.Value,
			})
		case "test":
			// Test operations are not converted to changes
			continue
		default:
			return nil, fmt.Errorf("unsupported JSON Patch operation: %s", op.Op)
		}
	}
	
	return changes, nil
}

// GenerateJSONPatch generates a JSON Patch document from two structs.
func GenerateJSONPatch(old, new interface{}) ([]byte, error) {
	// Use the Diff function to detect changes
	diffResult, err := Diff(old, new)
	if err != nil {
		return nil, fmt.Errorf("failed to diff structs: %w", err)
	}
	
	// Convert changes to JSON Patch
	patch := ChangesToJSONPatch(diffResult.Changes)
	
	// Marshal to JSON
	return json.Marshal(patch)
}

// ApplyJSONPatch applies a JSON Patch document to a struct.
func ApplyJSONPatch(target interface{}, patchData []byte) error {
	// Unmarshal the patch
	var patch JSONPatch
	if err := json.Unmarshal(patchData, &patch); err != nil {
		return fmt.Errorf("failed to unmarshal JSON Patch: %w", err)
	}
	
	// Convert to changes
	changes, err := JSONPatchToChanges(patch)
	if err != nil {
		return fmt.Errorf("failed to convert JSON Patch to changes: %w", err)
	}
	
	// Apply the changes
	return ApplyChanges(target, changes)
}
