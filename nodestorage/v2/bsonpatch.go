package v2

import (
	"fmt"
	"nodestorage/v2/core"
	"reflect"
	"sync"

	"github.com/jinzhu/copier"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

// BsonPatch represents a MongoDB update document with operators like $set, $unset, etc.
// It implements the bson.Marshaler interface to be directly usable in MongoDB operations.
type BsonPatch struct {
	// Set contains fields to be set with $set operator
	Set bson.M `json:"set,omitempty"`
	// Unset contains fields to be unset with $unset operator
	Unset bson.M `json:"unset,omitempty"`
	// Inc contains fields to be incremented with $inc operator
	Inc bson.M `json:"inc,omitempty"`
	// Push contains fields to be pushed to arrays with $push operator
	Push bson.M `json:"push,omitempty"`
	// Pull contains fields to be pulled from arrays with $pull operator
	Pull bson.M `json:"pull,omitempty"`
	// AddToSet contains fields to be added to arrays with $addToSet operator
	AddToSet bson.M `json:"addToSet,omitempty"`
	// PullAll contains fields to be pulled from arrays with $pullAll operator
	PullAll bson.M `json:"pullAll,omitempty"`
	// ArrayFilters contains array filters for positional updates
	ArrayFilters []bson.M `json:"arrayFilters,omitempty"`
}

// MarshalBSON implements the bson.Marshaler interface.
// This allows BsonPatch to be directly used in MongoDB update operations.
func (p *BsonPatch) MarshalBSON() ([]byte, error) {
	update := bson.M{}

	if len(p.Set) > 0 {
		update["$set"] = p.Set
	}

	if len(p.Unset) > 0 {
		update["$unset"] = p.Unset
	}

	if len(p.Inc) > 0 {
		update["$inc"] = p.Inc
	}

	if len(p.Push) > 0 {
		update["$push"] = p.Push
	}

	if len(p.Pull) > 0 {
		update["$pull"] = p.Pull
	}

	if len(p.AddToSet) > 0 {
		update["$addToSet"] = p.AddToSet
	}

	if len(p.PullAll) > 0 {
		update["$pullAll"] = p.PullAll
	}

	return bson.Marshal(update)
}

// GetArrayFilters returns array filters for use in MongoDB update operations.
// This is used for positional updates with filtered array elements.
func (p *BsonPatch) GetArrayFilters() []interface{} {
	if len(p.ArrayFilters) == 0 {
		return nil
	}

	filters := make([]interface{}, len(p.ArrayFilters))
	for i, filter := range p.ArrayFilters {
		filters[i] = filter
	}
	return filters
}

// IsEmpty returns true if the patch contains no operations.
func (p *BsonPatch) IsEmpty() bool {
	return len(p.Set) == 0 && len(p.Unset) == 0 && len(p.Inc) == 0 &&
		len(p.Push) == 0 && len(p.Pull) == 0 && len(p.AddToSet) == 0 &&
		len(p.PullAll) == 0
}

// StructFieldInfo contains cached information about a struct field.
type StructFieldInfo struct {
	Name    string // Original field name
	BSONTag string // BSON tag name
	Type    reflect.Type
	Index   []int // Field index for reflect.Value.FieldByIndex
}

// StructTypeInfo contains cached information about a struct type.
type StructTypeInfo struct {
	Fields map[string]*StructFieldInfo // Map of field name to field info
}

// TypeInfoCache caches struct type information to avoid repeated reflection.
type TypeInfoCache struct {
	cache map[reflect.Type]*StructTypeInfo
	mu    sync.RWMutex
}

// Global type info cache
var typeInfoCache = &TypeInfoCache{
	cache: make(map[reflect.Type]*StructTypeInfo),
}

// GetTypeInfo returns cached type information for a struct type.
// If the type is not in the cache, it analyzes the type and adds it to the cache.
func (c *TypeInfoCache) GetTypeInfo(t reflect.Type) *StructTypeInfo {
	// Check if type is already in cache
	c.mu.RLock()
	info, found := c.cache[t]
	c.mu.RUnlock()

	if found {
		return info
	}

	// Type not in cache, analyze it
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check again in case another goroutine added it while we were waiting
	if info, found = c.cache[t]; found {
		return info
	}

	// Analyze type and add to cache
	info = analyzeStructType(t)
	c.cache[t] = info
	return info
}

// analyzeStructType analyzes a struct type and returns its field information.
func analyzeStructType(t reflect.Type) *StructTypeInfo {
	// Ensure we're working with a struct type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		core.Error("Type is not a struct", zap.String("type", t.String()))
		return &StructTypeInfo{Fields: make(map[string]*StructFieldInfo)}
	}

	info := &StructTypeInfo{
		Fields: make(map[string]*StructFieldInfo),
	}

	// Analyze all fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Get BSON tag
		bsonTag := field.Tag.Get("bson")
		if bsonTag == "" || bsonTag == "-" {
			continue
		}

		// Extract the tag name (before any options like omitempty)
		tagName := bsonTag
		if comma := len(bsonTag); comma > 0 {
			tagName = bsonTag[:comma]
		}

		// Store field info
		info.Fields[field.Name] = &StructFieldInfo{
			Name:    field.Name,
			BSONTag: tagName,
			Type:    field.Type,
			Index:   field.Index,
		}
	}

	return info
}

// CreateBsonPatch creates a MongoDB update patch by comparing two documents.
// It uses reflection to analyze the documents and create a patch with MongoDB update operators.
// This function efficiently handles nested structures and arrays.
func CreateBsonPatch[T any](oldDoc, newDoc T) (*BsonPatch, error) {
	// Get reflect values
	oldVal := reflect.ValueOf(oldDoc)
	newVal := reflect.ValueOf(newDoc)

	// Ensure we're working with pointers
	if oldVal.Kind() != reflect.Ptr || newVal.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("both documents must be pointers")
	}

	// Ensure pointers are not nil
	if oldVal.IsNil() || newVal.IsNil() {
		return nil, fmt.Errorf("document pointers cannot be nil")
	}

	// Get the actual struct values
	oldVal = oldVal.Elem()
	newVal = newVal.Elem()

	// Ensure we're working with structs
	if oldVal.Kind() != reflect.Struct || newVal.Kind() != reflect.Struct {
		return nil, fmt.Errorf("documents must be structs")
	}

	// Ensure both documents are of the same type
	if oldVal.Type() != newVal.Type() {
		return nil, fmt.Errorf("documents must be of the same type")
	}

	// Create patch
	patch := &BsonPatch{
		Set:          bson.M{},
		Unset:        bson.M{},
		Inc:          bson.M{},
		Push:         bson.M{},
		Pull:         bson.M{},
		AddToSet:     bson.M{},
		PullAll:      bson.M{},
		ArrayFilters: []bson.M{},
	}

	// Get cached type info
	typeInfo := typeInfoCache.GetTypeInfo(oldVal.Type())

	// Compare fields and build patch
	for _, fieldInfo := range typeInfo.Fields {
		oldField := oldVal.FieldByIndex(fieldInfo.Index)
		newField := newVal.FieldByIndex(fieldInfo.Index)

		// Skip if both fields are zero values
		if isZeroValue(oldField) && isZeroValue(newField) {
			continue
		}

		// Process field based on its kind
		processField(patch, "", fieldInfo.BSONTag, oldField, newField)
	}

	return patch, nil
}

// processField processes a field based on its kind and adds appropriate operations to the patch.
// It handles nested structures and arrays recursively.
func processField(patch *BsonPatch, prefix, fieldName string, oldVal, newVal reflect.Value) {
	// Create the full field path
	path := fieldName
	if prefix != "" {
		path = prefix + "." + fieldName
	}

	// Handle nil pointers
	if oldVal.Kind() == reflect.Ptr && newVal.Kind() == reflect.Ptr {
		if oldVal.IsNil() && newVal.IsNil() {
			return // Both nil, no change
		}

		if oldVal.IsNil() {
			// Old is nil, new is not nil - set the new value
			// 포인터 값을 복사하여 외부 수정으로부터 보호
			newValCopy := deepCopyPointerValue(newVal)
			patch.Set[path] = newValCopy
			return
		}

		if newVal.IsNil() {
			// Old is not nil, new is nil - unset the field
			patch.Unset[path] = ""
			return
		}

		// Both are non-nil pointers, dereference and continue
		processField(patch, prefix, fieldName, oldVal.Elem(), newVal.Elem())
		return
	}

	// Handle different kinds of fields
	switch oldVal.Kind() {
	case reflect.Struct:
		processStruct(patch, path, oldVal, newVal)
	case reflect.Map:
		processMap(patch, path, oldVal, newVal)
	case reflect.Slice, reflect.Array:
		processArray(patch, path, oldVal, newVal)
	default:
		// For primitive types, just compare values
		if !reflect.DeepEqual(oldVal.Interface(), newVal.Interface()) {
			// 특별한 처리:
			// 1. bool 타입은 false가 zero value이지만 $unset 하지 않고 $set으로 처리
			// 2. 숫자 타입(int, float 등)은 0이 zero value이지만 $unset 하지 않고 $set으로 처리
			if newVal.Kind() == reflect.Bool ||
				newVal.Kind() == reflect.Int || newVal.Kind() == reflect.Int8 ||
				newVal.Kind() == reflect.Int16 || newVal.Kind() == reflect.Int32 ||
				newVal.Kind() == reflect.Int64 || newVal.Kind() == reflect.Uint ||
				newVal.Kind() == reflect.Uint8 || newVal.Kind() == reflect.Uint16 ||
				newVal.Kind() == reflect.Uint32 || newVal.Kind() == reflect.Uint64 ||
				newVal.Kind() == reflect.Float32 || newVal.Kind() == reflect.Float64 {
				// 숫자 타입과 bool 타입은 항상 $set으로 처리
				patch.Set[path] = newVal.Interface()
			} else if isZeroValue(newVal) {
				// 그 외 타입의 zero value는 $unset으로 처리
				patch.Unset[path] = ""
			} else {
				// 그 외 타입의 non-zero value는 $set으로 처리
				patch.Set[path] = newVal.Interface()
			}
		}
	}
}

// processStruct processes a struct field and adds appropriate operations to the patch.
func processStruct(patch *BsonPatch, prefix string, oldVal, newVal reflect.Value) {
	// Skip if types don't match
	if oldVal.Type() != newVal.Type() {
		patch.Set[prefix] = newVal.Interface()
		return
	}

	// Get cached type info
	typeInfo := typeInfoCache.GetTypeInfo(oldVal.Type())

	// Compare struct fields
	for _, fieldInfo := range typeInfo.Fields {
		oldField := oldVal.FieldByIndex(fieldInfo.Index)
		newField := newVal.FieldByIndex(fieldInfo.Index)

		// Skip if both fields are zero values
		if isZeroValue(oldField) && isZeroValue(newField) {
			continue
		}

		// Process field recursively
		processField(patch, prefix, fieldInfo.BSONTag, oldField, newField)
	}
}

// processMap processes a map field and adds appropriate operations to the patch.
func processMap(patch *BsonPatch, prefix string, oldVal, newVal reflect.Value) {
	// If types don't match, replace the whole map
	if oldVal.Type() != newVal.Type() {
		patch.Set[prefix] = newVal.Interface()
		return
	}

	// Get all keys from both maps
	keys := make(map[interface{}]bool)
	for _, key := range oldVal.MapKeys() {
		keys[key.Interface()] = true
	}
	for _, key := range newVal.MapKeys() {
		keys[key.Interface()] = true
	}

	// Compare map entries
	for key := range keys {
		keyVal := reflect.ValueOf(key)

		oldMapVal := reflect.Value{}
		if oldVal.MapIndex(keyVal).IsValid() {
			oldMapVal = oldVal.MapIndex(keyVal)
		}

		newMapVal := reflect.Value{}
		if newVal.MapIndex(keyVal).IsValid() {
			newMapVal = newVal.MapIndex(keyVal)
		}

		// Convert key to string for MongoDB path
		keyStr := fmt.Sprintf("%v", key)
		mapPath := prefix + "." + keyStr

		// Key exists in old but not in new - unset it
		if oldMapVal.IsValid() && !newMapVal.IsValid() {
			patch.Unset[mapPath] = ""
			continue
		}

		// Key exists in new but not in old - set it
		if !oldMapVal.IsValid() && newMapVal.IsValid() {
			patch.Set[mapPath] = newMapVal.Interface()
			continue
		}

		// Key exists in both - compare values
		if !reflect.DeepEqual(oldMapVal.Interface(), newMapVal.Interface()) {
			// 특별한 처리: bool 타입과 숫자 타입은 항상 $set으로 처리
			if newMapVal.Kind() == reflect.Bool ||
				newMapVal.Kind() == reflect.Int || newMapVal.Kind() == reflect.Int8 ||
				newMapVal.Kind() == reflect.Int16 || newMapVal.Kind() == reflect.Int32 ||
				newMapVal.Kind() == reflect.Int64 || newMapVal.Kind() == reflect.Uint ||
				newMapVal.Kind() == reflect.Uint8 || newMapVal.Kind() == reflect.Uint16 ||
				newMapVal.Kind() == reflect.Uint32 || newMapVal.Kind() == reflect.Uint64 ||
				newMapVal.Kind() == reflect.Float32 || newMapVal.Kind() == reflect.Float64 {
				// 숫자 타입과 bool 타입은 항상 $set으로 처리
				patch.Set[mapPath] = newMapVal.Interface()
			} else {
				// 그 외 타입은 processField로 처리
				processField(patch, prefix, keyStr, oldMapVal, newMapVal)
			}
		}
	}
}

// processArray processes an array or slice field and adds appropriate operations to the patch.
func processArray(patch *BsonPatch, prefix string, oldVal, newVal reflect.Value) {
	// If lengths are different or types don't match, replace the whole array
	if oldVal.Len() != newVal.Len() || oldVal.Type() != newVal.Type() {
		// For small arrays, just replace the whole array
		if newVal.Len() < 10 {
			patch.Set[prefix] = newVal.Interface()
			return
		}

		// For larger arrays, try to use array operators

		// Find elements to add (in new but not in old)
		var elementsToAdd []interface{}
		var elementsToRemove []interface{}

		// Create maps for quick lookup
		oldItems := make(map[interface{}]int)
		newItems := make(map[interface{}]int)

		// Try to use hash-based comparison for comparable types
		canUseHash := true

		// Check if array elements are comparable
		if oldVal.Len() > 0 {
			elemType := oldVal.Index(0).Type()
			canUseHash = isComparableType(elemType)
		}

		if canUseHash {
			// Build maps of items for quick lookup
			for i := 0; i < oldVal.Len(); i++ {
				item := oldVal.Index(i).Interface()
				oldItems[item]++
			}

			for i := 0; i < newVal.Len(); i++ {
				item := newVal.Index(i).Interface()
				newItems[item]++
			}

			// Find items to add and remove
			for item, count := range newItems {
				oldCount := oldItems[item]
				if count > oldCount {
					// Add this item (count - oldCount) times
					for i := 0; i < count-oldCount; i++ {
						elementsToAdd = append(elementsToAdd, item)
					}
				}
			}

			for item, count := range oldItems {
				newCount := newItems[item]
				if count > newCount {
					// Remove this item (count - newCount) times
					for i := 0; i < count-newCount; i++ {
						elementsToRemove = append(elementsToRemove, item)
					}
				}
			}

			// Apply array operations
			if len(elementsToAdd) > 0 {
				// Use $push with $each for multiple elements
				if len(elementsToAdd) == 1 {
					patch.Push[prefix] = elementsToAdd[0]
				} else {
					patch.Push[prefix] = bson.M{"$each": elementsToAdd}
				}
			}

			if len(elementsToRemove) > 0 {
				// Use $pullAll for multiple elements
				if len(elementsToRemove) == 1 {
					patch.Pull[prefix] = elementsToRemove[0]
				} else {
					patch.PullAll[prefix] = elementsToRemove
				}
			}

			return
		}

		// If we can't use hash-based comparison, just replace the whole array
		patch.Set[prefix] = newVal.Interface()
		return
	}

	// If arrays have the same length, compare elements
	for i := 0; i < oldVal.Len(); i++ {
		oldElem := oldVal.Index(i)
		newElem := newVal.Index(i)

		// Skip if both elements are zero values
		if isZeroValue(oldElem) && isZeroValue(newElem) {
			continue
		}

		// Compare elements
		if !reflect.DeepEqual(oldElem.Interface(), newElem.Interface()) {
			// Check if we should use array filters for this element
			if shouldUseArrayFilter(oldElem, newElem) {
				// Create array filter for this element
				processArrayWithFilter(patch, prefix, i, oldElem, newElem)
			} else {
				// Create path with array index
				elemPath := fmt.Sprintf("%s.%d", prefix, i)

				// Process element based on its kind
				if oldElem.Kind() == reflect.Struct {
					processStruct(patch, elemPath, oldElem, newElem)
				} else if oldElem.Kind() == reflect.Map {
					processMap(patch, elemPath, oldElem, newElem)
				} else if oldElem.Kind() == reflect.Slice || oldElem.Kind() == reflect.Array {
					// For nested arrays, just replace the whole element
					patch.Set[elemPath] = newElem.Interface()
				} else {
					// For primitive types, just set the new value
					patch.Set[elemPath] = newElem.Interface()
				}
			}
		}
	}
}

// shouldUseArrayFilter determines if we should use array filters for this element.
// This is typically useful for struct elements with identifiable fields.
func shouldUseArrayFilter(oldElem, newElem reflect.Value) bool {
	// Only use array filters for structs
	if oldElem.Kind() != reflect.Struct || newElem.Kind() != reflect.Struct {
		return false
	}

	// Check if the struct has an ID field or similar that can be used for identification
	typeInfo := typeInfoCache.GetTypeInfo(oldElem.Type())

	// Look for common identifier fields
	for _, fieldName := range []string{"ID", "Id", "_id", "id"} {
		if fieldInfo, ok := typeInfo.Fields[fieldName]; ok {
			// If the field exists and has the same value in both elements,
			// we can use it for identification in array filters
			oldID := oldElem.FieldByIndex(fieldInfo.Index)
			newID := newElem.FieldByIndex(fieldInfo.Index)

			if oldID.IsValid() && newID.IsValid() &&
				!isZeroValue(oldID) && !isZeroValue(newID) &&
				reflect.DeepEqual(oldID.Interface(), newID.Interface()) {
				return true
			}
		}
	}

	return false
}

// processArrayWithFilter processes an array element using array filters.
func processArrayWithFilter(patch *BsonPatch, prefix string, index int, oldElem, newElem reflect.Value) {
	// Get type info for the struct
	typeInfo := typeInfoCache.GetTypeInfo(oldElem.Type())

	// Find an identifier field
	var identifierField *StructFieldInfo
	var identifierValue interface{}

	for _, fieldName := range []string{"ID", "Id", "_id", "id"} {
		if fieldInfo, ok := typeInfo.Fields[fieldName]; ok {
			oldID := oldElem.FieldByIndex(fieldInfo.Index)
			if oldID.IsValid() && !isZeroValue(oldID) {
				identifierField = fieldInfo
				identifierValue = oldID.Interface()
				break
			}
		}
	}

	if identifierField == nil {
		// Fallback to regular index-based update if no identifier found
		elemPath := fmt.Sprintf("%s.%d", prefix, index)
		patch.Set[elemPath] = newElem.Interface()
		return
	}

	// Create a unique identifier for this array filter
	filterID := fmt.Sprintf("elem%d", len(patch.ArrayFilters))

	// Create the array filter condition
	filterCondition := bson.M{
		fmt.Sprintf("%s.%s", filterID, identifierField.BSONTag): identifierValue,
	}

	// Add the filter to the patch
	patch.ArrayFilters = append(patch.ArrayFilters, filterCondition)

	// Create the path with positional filtered operator
	positionPath := fmt.Sprintf("%s.$[%s]", prefix, filterID)

	// Compare fields and create updates
	for _, fieldInfo := range typeInfo.Fields {
		// Skip the identifier field
		if fieldInfo.Name == identifierField.Name {
			continue
		}

		oldField := oldElem.FieldByIndex(fieldInfo.Index)
		newField := newElem.FieldByIndex(fieldInfo.Index)

		// Skip if both fields are zero values
		if isZeroValue(oldField) && isZeroValue(newField) {
			continue
		}

		// Compare field values
		if !reflect.DeepEqual(oldField.Interface(), newField.Interface()) {
			fieldPath := fmt.Sprintf("%s.%s", positionPath, fieldInfo.BSONTag)

			if isZeroValue(newField) {
				patch.Unset[fieldPath] = ""
			} else {
				patch.Set[fieldPath] = newField.Interface()
			}
		}
	}
}

// isComparableType checks if a type can be used as a map key (is comparable).
func isComparableType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
		reflect.String:
		return true
	case reflect.Array:
		return isComparableType(t.Elem())
	case reflect.Struct:
		// A struct is comparable if all its fields are comparable
		for i := 0; i < t.NumField(); i++ {
			if !isComparableType(t.Field(i).Type) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// deepCopyPointerValue creates a deep copy of a pointer value to protect it from external modifications.
func deepCopyPointerValue(v reflect.Value) interface{} {
	if v.IsNil() {
		return nil
	}

	// Create a new value of the same type as the original pointer
	newValue := reflect.New(v.Type().Elem())

	// Copy the value pointed to by the original pointer to the new value
	if err := copier.CopyWithOption(newValue.Interface(), v.Elem().Interface(), copier.Option{DeepCopy: true}); err != nil {
		// if copy failed use the original value
		return v.Interface()
	}

	return newValue.Interface()
}

// isZeroValue checks if a reflect.Value is the zero value for its type.
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}
