package nodestorage

import (
	"fmt"
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
)

// GetVersion retrieves the version value from a document
func GetVersion(doc interface{}, versionField string) (int64, error) {
	// Use reflection to get the version field
	v := reflect.ValueOf(doc)

	// Check if it's a pointer and not nil
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		// Get the element the pointer points to
		elem := v.Elem()

		// Check if it's a struct
		if elem.Kind() == reflect.Struct {
			// Try to find the version field
			field := elem.FieldByName(versionField)
			if field.IsValid() && field.CanInt() {
				return field.Int(), nil
			}

			// If the field doesn't exist or is not an integer, try to use bson marshaling
			docBytes, err := bson.Marshal(doc)
			if err != nil {
				return 0, fmt.Errorf("failed to marshal document: %w", err)
			}

			var docMap bson.M
			if err := bson.Unmarshal(docBytes, &docMap); err != nil {
				return 0, fmt.Errorf("failed to unmarshal document: %w", err)
			}

			// Try to get the version field from the map
			if versionValue, ok := docMap[versionField]; ok {
				switch v := versionValue.(type) {
				case int64:
					return v, nil
				case int32:
					return int64(v), nil
				case int:
					return int64(v), nil
				case float64:
					return int64(v), nil
				default:
					return 0, fmt.Errorf("version field is not a number: %T", versionValue)
				}
			}

			// If the field doesn't exist, return 0 (new document)
			return 0, nil
		}
	}

	return 0, fmt.Errorf("invalid document: not a pointer to struct")
}

// setVersion sets the version value in a document
func setVersion(doc interface{}, versionField string, version int64) error {
	// Use reflection to set the version field
	v := reflect.ValueOf(doc)

	// Check if it's a pointer and not nil
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		// Get the element the pointer points to
		elem := v.Elem()

		// Check if it's a struct
		if elem.Kind() == reflect.Struct {
			// Try to find the version field
			field := elem.FieldByName(versionField)
			if field.IsValid() && field.CanSet() && field.CanInt() {
				field.SetInt(version)
				return nil
			}

			// If the field doesn't exist or can't be set, return an error
			return fmt.Errorf("version field %s not found or cannot be set", versionField)
		}
	}

	return fmt.Errorf("invalid document: not a pointer to struct")
}

// incrementVersion increments the version value in a document
func incrementVersion(doc interface{}, versionField string) (int64, error) {
	// Get current version
	currentVersion, err := GetVersion(doc, versionField)
	if err != nil {
		return 0, err
	}

	// Increment version
	newVersion := currentVersion + 1

	// Set new version
	if err := setVersion(doc, versionField, newVersion); err != nil {
		return 0, err
	}

	return newVersion, nil
}

// createVersionUpdate creates a MongoDB update to increment the version field
func createVersionUpdate(versionField string, currentVersion int64) bson.M {
	return bson.M{
		"$set": bson.M{
			versionField: currentVersion + 1,
		},
	}
}

// createVersionFilter creates a MongoDB filter to match a specific version
func createVersionFilter(id interface{}, versionField string, currentVersion int64) bson.M {
	return bson.M{
		"_id":        id,
		versionField: currentVersion,
	}
}

// createVersionIncrement creates a MongoDB update to increment the version field
func createVersionIncrement(versionField string) bson.M {
	return bson.M{
		"$inc": bson.M{
			versionField: 1,
		},
	}
}

// mergeUpdates merges multiple MongoDB updates into one
func mergeUpdates(updates ...bson.M) bson.M {
	result := bson.M{}

	for _, update := range updates {
		for op, fields := range update {
			if existingFields, ok := result[op]; ok {
				// Merge fields for the same operator
				if existingMap, ok := existingFields.(bson.M); ok {
					if fieldsMap, ok := fields.(bson.M); ok {
						for k, v := range fieldsMap {
							existingMap[k] = v
						}
					}
				}
			} else {
				// Add new operator
				result[op] = fields
			}
		}
	}

	return result
}
