package wrapper

import (
	"fmt"
	"reflect"
	"strings"
)

// StructTags represents the tags for a struct field.
type StructTags struct {
	// FieldName is the name of the field in the CRDT document.
	FieldName string
	
	// Ignore indicates whether the field should be ignored.
	Ignore bool
	
	// ReadOnly indicates whether the field is read-only.
	ReadOnly bool
}

// ParseStructTags parses the tags for a struct field.
func ParseStructTags(field reflect.StructField) StructTags {
	tags := StructTags{
		FieldName: field.Name,
	}
	
	// Get the crdt tag
	tag := field.Tag.Get("crdt")
	if tag == "" {
		// If no crdt tag, use the json tag
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" {
			parts := strings.Split(jsonTag, ",")
			if parts[0] != "" {
				tags.FieldName = parts[0]
			}
			for _, part := range parts[1:] {
				if part == "omitempty" {
					// Ignore empty fields
				}
			}
		}
		return tags
	}
	
	// Parse the crdt tag
	parts := strings.Split(tag, ",")
	if parts[0] != "" {
		tags.FieldName = parts[0]
	}
	
	for _, part := range parts[1:] {
		switch part {
		case "ignore":
			tags.Ignore = true
		case "readonly":
			tags.ReadOnly = true
		}
	}
	
	return tags
}

// StructToMap converts a struct to a map, respecting CRDT tags.
func StructToMap(data interface{}) (map[string]interface{}, error) {
	// Get the value and type of the struct
	value := reflect.ValueOf(data)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	
	if value.Kind() != reflect.Struct {
		return nil, fmt.Errorf("data is not a struct")
	}
	
	// Create a map to hold the result
	result := make(map[string]interface{})
	
	// Iterate over the struct fields
	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := typ.Field(i)
		
		// Parse the tags
		tags := ParseStructTags(fieldType)
		if tags.Ignore {
			continue
		}
		
		// Get the field value
		fieldValue := field.Interface()
		
		// If the field is a struct, recursively convert it
		if field.Kind() == reflect.Struct {
			nestedMap, err := StructToMap(fieldValue)
			if err != nil {
				return nil, err
			}
			result[tags.FieldName] = nestedMap
		} else if field.Kind() == reflect.Slice || field.Kind() == reflect.Array {
			// If the field is a slice or array, convert each element
			sliceLen := field.Len()
			sliceResult := make([]interface{}, sliceLen)
			for j := 0; j < sliceLen; j++ {
				elem := field.Index(j)
				if elem.Kind() == reflect.Struct {
					nestedMap, err := StructToMap(elem.Interface())
					if err != nil {
						return nil, err
					}
					sliceResult[j] = nestedMap
				} else {
					sliceResult[j] = elem.Interface()
				}
			}
			result[tags.FieldName] = sliceResult
		} else {
			// Otherwise, just add the field value
			result[tags.FieldName] = fieldValue
		}
	}
	
	return result, nil
}

// MapToStruct converts a map to a struct, respecting CRDT tags.
func MapToStruct(data map[string]interface{}, result interface{}) error {
	// Get the value and type of the struct
	value := reflect.ValueOf(result)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return fmt.Errorf("result must be a non-nil pointer to a struct")
	}
	
	value = value.Elem()
	if value.Kind() != reflect.Struct {
		return fmt.Errorf("result must be a pointer to a struct")
	}
	
	// Iterate over the struct fields
	typ := value.Type()
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := typ.Field(i)
		
		// Parse the tags
		tags := ParseStructTags(fieldType)
		if tags.Ignore {
			continue
		}
		
		// Get the field value from the map
		fieldValue, ok := data[tags.FieldName]
		if !ok {
			continue
		}
		
		// Set the field value
		fieldValueReflect := reflect.ValueOf(fieldValue)
		if field.Kind() == reflect.Struct && fieldValueReflect.Kind() == reflect.Map {
			// If the field is a struct and the value is a map, recursively convert it
			mapValue, ok := fieldValue.(map[string]interface{})
			if !ok {
				return fmt.Errorf("field %s: expected map[string]interface{}, got %T", tags.FieldName, fieldValue)
			}
			
			// Create a new instance of the struct
			newStruct := reflect.New(field.Type()).Elem()
			
			// Recursively convert the map to the struct
			if err := MapToStruct(mapValue, newStruct.Addr().Interface()); err != nil {
				return err
			}
			
			// Set the field value
			field.Set(newStruct)
		} else if (field.Kind() == reflect.Slice || field.Kind() == reflect.Array) && fieldValueReflect.Kind() == reflect.Slice {
			// If the field is a slice or array and the value is a slice, convert each element
			sliceValue, ok := fieldValue.([]interface{})
			if !ok {
				return fmt.Errorf("field %s: expected []interface{}, got %T", tags.FieldName, fieldValue)
			}
			
			// Create a new slice
			newSlice := reflect.MakeSlice(field.Type(), len(sliceValue), len(sliceValue))
			
			// Convert each element
			for j, elem := range sliceValue {
				elemReflect := reflect.ValueOf(elem)
				if newSlice.Index(j).Kind() == reflect.Struct && elemReflect.Kind() == reflect.Map {
					// If the element is a struct and the value is a map, recursively convert it
					mapValue, ok := elem.(map[string]interface{})
					if !ok {
						return fmt.Errorf("field %s[%d]: expected map[string]interface{}, got %T", tags.FieldName, j, elem)
					}
					
					// Create a new instance of the struct
					newStruct := reflect.New(newSlice.Index(j).Type()).Elem()
					
					// Recursively convert the map to the struct
					if err := MapToStruct(mapValue, newStruct.Addr().Interface()); err != nil {
						return err
					}
					
					// Set the element value
					newSlice.Index(j).Set(newStruct)
				} else {
					// Otherwise, just set the element value
					newSlice.Index(j).Set(elemReflect)
				}
			}
			
			// Set the field value
			field.Set(newSlice)
		} else {
			// Otherwise, just set the field value
			field.Set(fieldValueReflect)
		}
	}
	
	return nil
}
