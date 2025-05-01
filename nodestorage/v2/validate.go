package nodestorage

import (
	"fmt"
	"reflect"
	"strings"
)

// validateVersionField validates that the version field exists in the struct and returns
// the struct field name and its corresponding BSON tag name.
func validateVersionField[T any](doc T, versionFieldName string) (string, string, error) {
	// Get the type of the document
	docType := reflect.TypeOf(doc)

	// If it's a nil interface, create a new instance to get the type
	if docType == nil {
		// Create a new instance of T
		var newDoc T
		docType = reflect.TypeOf(newDoc)
	}

	// Handle pointer types
	if docType.Kind() == reflect.Ptr {
		// Get the element type
		docType = docType.Elem()
	}

	// Ensure it's a struct
	if docType.Kind() != reflect.Struct {
		return "", "", fmt.Errorf("document type must be a struct or pointer to struct")
	}

	// Try to find the version field by name
	field, found := docType.FieldByName(versionFieldName)
	if !found {
		return "", "", fmt.Errorf("version field %s not found in document type", versionFieldName)
	}

	// Ensure the field is of type int64
	if field.Type.Kind() != reflect.Int64 {
		return "", "", fmt.Errorf("version field %s must be of type int64, got %s", versionFieldName, field.Type.String())
	}

	// Get the BSON tag
	bsonTag := field.Tag.Get("bson")
	if bsonTag == "" {
		// If no BSON tag is specified, use the lowercase field name
		bsonTag = strings.ToLower(versionFieldName)
	} else {
		// Extract the field name part from the BSON tag (ignoring options like "omitempty")
		parts := strings.Split(bsonTag, ",")
		bsonTag = parts[0]
	}

	return versionFieldName, bsonTag, nil
}
