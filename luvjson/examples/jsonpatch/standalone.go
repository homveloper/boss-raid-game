package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"
)

// Person is a user-defined struct.
type Person struct {
	Name    string   `json:"name"`
	Age     int      `json:"age"`
	Email   string   `json:"email"`
	Tags    []string `json:"tags"`
	Address Address  `json:"address"`
}

// Address is a nested struct.
type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	Country string `json:"country"`
}

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

// Diff compares two structs and returns the differences.
func Diff(old, new interface{}) (*DiffResult, error) {
	result := &DiffResult{
		Changes:   make([]Change, 0),
		Timestamp: time.Now(),
	}

	// Convert old and new to maps
	oldMap, err := structToMap(old)
	if err != nil {
		return nil, fmt.Errorf("failed to convert old struct to map: %w", err)
	}

	newMap, err := structToMap(new)
	if err != nil {
		return nil, fmt.Errorf("failed to convert new struct to map: %w", err)
	}

	// Compare the maps
	diffMaps(oldMap, newMap, "", result)

	return result, nil
}

// structToMap converts a struct to a map.
func structToMap(data interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
	}

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

	// Convert target to map
	targetMap, err := structToMap(target)
	if err != nil {
		return fmt.Errorf("failed to convert target to map: %w", err)
	}

	// Apply each operation in the patch
	for _, op := range patch {
		// Convert JSON Patch path to dot notation
		path := strings.TrimPrefix(op.Path, "/")
		path = strings.ReplaceAll(path, "/", ".")

		// Split the path into parts
		parts := strings.Split(path, ".")

		switch op.Op {
		case "add", "replace":
			// Apply the change to the map
			if err := applyChangeToMap(targetMap, parts, op.Value); err != nil {
				return err
			}
		case "remove":
			// Delete the field from the map
			if err := deleteFromMap(targetMap, parts); err != nil {
				return err
			}
		case "move":
			// Not implemented
			return fmt.Errorf("move operation not implemented")
		case "copy":
			// Not implemented
			return fmt.Errorf("copy operation not implemented")
		case "test":
			// Not implemented
			return fmt.Errorf("test operation not implemented")
		default:
			return fmt.Errorf("unsupported JSON Patch operation: %s", op.Op)
		}
	}

	// Convert the map back to the struct
	jsonData, err := json.Marshal(targetMap)
	if err != nil {
		return fmt.Errorf("failed to marshal map: %w", err)
	}

	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to unmarshal to struct: %w", err)
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

func main() {
	// Create two Person structs with some differences
	person1 := Person{
		Name:  "John Doe",
		Age:   30,
		Email: "john@example.com",
		Tags:  []string{"developer", "golang"},
		Address: Address{
			Street:  "123 Main St",
			City:    "New York",
			Country: "USA",
		},
	}

	person2 := Person{
		Name:  "John Doe",                              // Same
		Age:   31,                                      // Changed
		Email: "john.doe@example.com",                  // Changed
		Tags:  []string{"developer", "golang", "crdt"}, // Added an element
		Address: Address{
			Street:  "123 Main St", // Same
			City:    "Boston",      // Changed
			Country: "USA",         // Same
		},
	}

	// Generate a JSON Patch from person1 to person2
	jsonPatch, err := GenerateJSONPatch(&person1, &person2)
	if err != nil {
		log.Fatalf("Failed to generate JSON Patch: %v", err)
	}

	// Print the JSON Patch
	fmt.Println("JSON Patch (RFC6902):")
	var prettyPatch bytes.Buffer
	if err := json.Indent(&prettyPatch, jsonPatch, "", "  "); err != nil {
		log.Fatalf("Failed to indent JSON Patch: %v", err)
	}
	fmt.Println(prettyPatch.String())

	// Apply the JSON Patch to person1
	var person1Copy Person = person1
	if err := ApplyJSONPatch(&person1Copy, jsonPatch); err != nil {
		log.Fatalf("Failed to apply JSON Patch: %v", err)
	}

	// Print the result
	fmt.Println("\nAfter applying JSON Patch:")
	fmt.Printf("Name: %s\n", person1Copy.Name)
	fmt.Printf("Age: %d\n", person1Copy.Age)
	fmt.Printf("Email: %s\n", person1Copy.Email)
	fmt.Printf("Tags: %v\n", person1Copy.Tags)
	fmt.Printf("Address: %s, %s, %s\n", person1Copy.Address.Street, person1Copy.Address.City, person1Copy.Address.Country)

	// Use the Diff function to detect changes
	diffResult, err := Diff(&person1, &person2)
	if err != nil {
		log.Fatalf("Failed to diff structs: %v", err)
	}

	// Print the changes
	fmt.Println("\nChanges detected:")
	for i, change := range diffResult.Changes {
		fmt.Printf("%d. %s\n", i+1, change)
	}
}
