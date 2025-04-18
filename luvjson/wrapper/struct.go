package wrapper

// // StructToMap converts a struct to a map[string]interface{}.
// // The data parameter must be a pointer to a struct.
// func StructToMap(data interface{}) (map[string]interface{}, error) {
// 	// Check if data is a pointer
// 	dataValue := reflect.ValueOf(data)
// 	if dataValue.Kind() != reflect.Ptr || dataValue.IsNil() {
// 		return nil, fmt.Errorf("data must be a non-nil pointer to a struct")
// 	}

// 	// Check if data points to a struct
// 	dataElem := dataValue.Elem()
// 	if dataElem.Kind() != reflect.Struct {
// 		return nil, fmt.Errorf("data must be a pointer to a struct, got pointer to %v", dataElem.Kind())
// 	}

// 	// Convert struct to map
// 	jsonData, err := json.Marshal(data)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to marshal struct: %w", err)
// 	}

// 	var dataMap map[string]interface{}
// 	if err := json.Unmarshal(jsonData, &dataMap); err != nil {
// 		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
// 	}

// 	return dataMap, nil
// }

// // MapToStruct converts a map[string]interface{} to a struct.
// // The result parameter must be a pointer to a struct.
// func MapToStruct(dataMap map[string]interface{}, result interface{}) error {
// 	// Check if result is a pointer
// 	resultValue := reflect.ValueOf(result)
// 	if resultValue.Kind() != reflect.Ptr || resultValue.IsNil() {
// 		return fmt.Errorf("result must be a non-nil pointer to a struct")
// 	}

// 	// Check if result points to a struct
// 	resultElem := resultValue.Elem()
// 	if resultElem.Kind() != reflect.Struct {
// 		return fmt.Errorf("result must be a pointer to a struct, got pointer to %v", resultElem.Kind())
// 	}

// 	// Convert map to JSON
// 	jsonData, err := json.Marshal(dataMap)
// 	if err != nil {
// 		return fmt.Errorf("failed to marshal map: %w", err)
// 	}

// 	// Unmarshal JSON to struct
// 	if err := json.Unmarshal(jsonData, result); err != nil {
// 		return fmt.Errorf("failed to unmarshal to struct: %w", err)
// 	}

// 	return nil
// }

// DiffMaps compares two maps and adds the differences to the result.
// This is an exported version of diffMaps for use by other packages.
func DiffMaps(oldMap, newMap map[string]interface{}, prefix string, result *DiffResult) {
	diffMaps(oldMap, newMap, prefix, result)
}

// DiffSlices compares two slices and adds the differences to the result.
// This is an exported version of diffSlices for use by other packages.
func DiffSlices(oldSlice, newSlice []interface{}, prefix string, result *DiffResult) {
	diffSlices(oldSlice, newSlice, prefix, result)
}
