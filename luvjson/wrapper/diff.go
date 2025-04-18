package wrapper

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// ChangeType represents the type of change detected.
type ChangeType int

const (
	// ChangeTypeNone indicates no change.
	ChangeTypeNone ChangeType = iota
	// ChangeTypeCreate indicates a new field was created.
	ChangeTypeCreate
	// ChangeTypeUpdate indicates a field was updated.
	ChangeTypeUpdate
	// ChangeTypeDelete indicates a field was deleted.
	ChangeTypeDelete
)

// Change represents a change to a field.
type Change struct {
	// Path is the path to the field.
	Path string
	// Type is the type of change.
	Type ChangeType
	// OldValue is the old value of the field.
	OldValue interface{}
	// NewValue is the new value of the field.
	NewValue interface{}
}

// String returns a string representation of the change.
func (c Change) String() string {
	switch c.Type {
	case ChangeTypeCreate:
		return fmt.Sprintf("CREATE %s = %v", c.Path, c.NewValue)
	case ChangeTypeUpdate:
		return fmt.Sprintf("UPDATE %s: %v -> %v", c.Path, c.OldValue, c.NewValue)
	case ChangeTypeDelete:
		return fmt.Sprintf("DELETE %s (was %v)", c.Path, c.OldValue)
	default:
		return fmt.Sprintf("NONE %s", c.Path)
	}
}

// DiffResult represents the result of a diff operation.
type DiffResult struct {
	// Changes is the list of changes detected.
	Changes []Change
	// Timestamp is when the diff was performed.
	Timestamp time.Time
}

// Diff compares two structs and returns the differences.
func Diff(old, new interface{}) (*DiffResult, error) {
	result := &DiffResult{
		Changes:   make([]Change, 0),
		Timestamp: time.Now(),
	}

	// Convert old and new to maps
	oldMap, err := StructToMap(old)
	if err != nil {
		return nil, fmt.Errorf("failed to convert old struct to map: %w", err)
	}

	newMap, err := StructToMap(new)
	if err != nil {
		return nil, fmt.Errorf("failed to convert new struct to map: %w", err)
	}

	// Compare the maps
	diffMaps(oldMap, newMap, "", result)

	return result, nil
}

// diffMaps compares two maps and adds the differences to the result.
func diffMaps(oldMap, newMap map[string]interface{}, prefix string, result *DiffResult) {
	// Check for updated or deleted fields
	for key, oldValue := range oldMap {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		newValue, exists := newMap[key]
		if !exists {
			// Field was deleted
			result.Changes = append(result.Changes, Change{
				Path:     path,
				Type:     ChangeTypeDelete,
				OldValue: oldValue,
			})
			continue
		}

		// Field exists in both, check if it changed
		if !reflect.DeepEqual(oldValue, newValue) {
			// If both are maps, recursively compare them
			oldValueMap, oldIsMap := oldValue.(map[string]interface{})
			newValueMap, newIsMap := newValue.(map[string]interface{})
			if oldIsMap && newIsMap {
				diffMaps(oldValueMap, newValueMap, path, result)
				continue
			}

			// If both are slices, compare them element by element
			oldValueSlice, oldIsSlice := oldValue.([]interface{})
			newValueSlice, newIsSlice := newValue.([]interface{})
			if oldIsSlice && newIsSlice {
				diffSlices(oldValueSlice, newValueSlice, path, result)
				continue
			}

			// Otherwise, it's a simple value change
			result.Changes = append(result.Changes, Change{
				Path:     path,
				Type:     ChangeTypeUpdate,
				OldValue: oldValue,
				NewValue: newValue,
			})
		}
	}

	// Check for created fields
	for key, newValue := range newMap {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		_, exists := oldMap[key]
		if !exists {
			// Field was created
			result.Changes = append(result.Changes, Change{
				Path:     path,
				Type:     ChangeTypeCreate,
				NewValue: newValue,
			})
		}
	}
}

// diffSlices compares two slices and adds the differences to the result.
func diffSlices(oldSlice, newSlice []interface{}, prefix string, result *DiffResult) {
	// For simplicity, we'll just check if the slices are different lengths
	// or if any elements are different
	if len(oldSlice) != len(newSlice) {
		result.Changes = append(result.Changes, Change{
			Path:     prefix,
			Type:     ChangeTypeUpdate,
			OldValue: oldSlice,
			NewValue: newSlice,
		})
		return
	}

	// Check each element
	for i := 0; i < len(oldSlice); i++ {
		path := fmt.Sprintf("%s[%d]", prefix, i)
		oldValue := oldSlice[i]
		newValue := newSlice[i]

		if !reflect.DeepEqual(oldValue, newValue) {
			// If both are maps, recursively compare them
			oldValueMap, oldIsMap := oldValue.(map[string]interface{})
			newValueMap, newIsMap := newValue.(map[string]interface{})
			if oldIsMap && newIsMap {
				diffMaps(oldValueMap, newValueMap, path, result)
				continue
			}

			// If both are slices, recursively compare them
			oldValueSlice, oldIsSlice := oldValue.([]interface{})
			newValueSlice, newIsSlice := newValue.([]interface{})
			if oldIsSlice && newIsSlice {
				diffSlices(oldValueSlice, newValueSlice, path, result)
				continue
			}

			// Otherwise, it's a simple value change
			result.Changes = append(result.Changes, Change{
				Path:     path,
				Type:     ChangeTypeUpdate,
				OldValue: oldValue,
				NewValue: newValue,
			})
		}
	}
}

// ApplyChanges applies a list of changes to a struct.
func ApplyChanges(target interface{}, changes []Change) error {
	// Get the value of the target
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr || targetValue.IsNil() {
		return fmt.Errorf("target must be a non-nil pointer to a struct")
	}

	targetValue = targetValue.Elem()
	if targetValue.Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to a struct")
	}

	// Convert target to map
	targetMap, err := StructToMap(target)
	if err != nil {
		return fmt.Errorf("failed to convert target to map: %w", err)
	}

	// Apply each change to the map
	for _, change := range changes {
		switch change.Type {
		case ChangeTypeCreate, ChangeTypeUpdate:
			// Split the path into parts
			parts := strings.Split(change.Path, ".")
			if len(parts) == 0 {
				return fmt.Errorf("invalid path: %s", change.Path)
			}

			// Apply the change to the map
			if err := applyChangeToMap(targetMap, parts, change.NewValue); err != nil {
				return err
			}
		case ChangeTypeDelete:
			// Split the path into parts
			parts := strings.Split(change.Path, ".")
			if len(parts) == 0 {
				return fmt.Errorf("invalid path: %s", change.Path)
			}

			// Delete the field from the map
			if err := deleteFromMap(targetMap, parts); err != nil {
				return err
			}
		}
	}

	// Convert the map back to the struct
	if err := MapToStruct(targetMap, target); err != nil {
		return fmt.Errorf("failed to convert map to struct: %w", err)
	}

	return nil
}

// applyChangeToMap applies a change to a map at the specified path.
func applyChangeToMap(m map[string]interface{}, path []string, value interface{}) error {
	if len(path) == 1 {
		// We're at the leaf, set the value
		m[path[0]] = value
		return nil
	}

	// We need to navigate deeper
	key := path[0]
	restPath := path[1:]

	// Check if the key exists and is a map
	subMap, exists := m[key]
	if !exists {
		// Create a new map
		subMap = make(map[string]interface{})
		m[key] = subMap
	}

	// Convert to map[string]interface{}
	subMapTyped, ok := subMap.(map[string]interface{})
	if !ok {
		// The key exists but is not a map, replace it with a map
		subMapTyped = make(map[string]interface{})
		m[key] = subMapTyped
	}

	// Recursively apply the change
	return applyChangeToMap(subMapTyped, restPath, value)
}

// deleteFromMap deletes a field from a map at the specified path.
func deleteFromMap(m map[string]interface{}, path []string) error {
	if len(path) == 1 {
		// We're at the leaf, delete the key
		delete(m, path[0])
		return nil
	}

	// We need to navigate deeper
	key := path[0]
	restPath := path[1:]

	// Check if the key exists and is a map
	subMap, exists := m[key]
	if !exists {
		// Key doesn't exist, nothing to delete
		return nil
	}

	// Convert to map[string]interface{}
	subMapTyped, ok := subMap.(map[string]interface{})
	if !ok {
		// The key exists but is not a map, can't navigate deeper
		return fmt.Errorf("path %v leads to a non-map value", path)
	}

	// Recursively delete the field
	return deleteFromMap(subMapTyped, restPath)
}
