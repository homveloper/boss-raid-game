package nodestorage

import (
	"fmt"
	"nodestorage/v2/core"
	"reflect"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

// BsonPatchBad represents a MongoDB update document with operators like $set, $unset, etc.
// It implements the bson.Marshaler interface to be directly usable in MongoDB operations.
type BsonPatchBad struct {
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
// This allows BsonPatchBad to be directly used in MongoDB update operations.
func (p *BsonPatchBad) MarshalBSON() ([]byte, error) {
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
func (p *BsonPatchBad) GetArrayFilters() []interface{} {
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
func (p *BsonPatchBad) IsEmpty() bool {
	return len(p.Set) == 0 && len(p.Unset) == 0 && len(p.Inc) == 0 &&
		len(p.Push) == 0 && len(p.Pull) == 0 && len(p.AddToSet) == 0 &&
		len(p.PullAll) == 0
}

// NewBsonPatchBad creates a new empty BsonPatchBad.
func NewBsonPatchBad() *BsonPatchBad {
	return &BsonPatchBad{
		Set:          bson.M{},
		Unset:        bson.M{},
		Inc:          bson.M{},
		Push:         bson.M{},
		Pull:         bson.M{},
		AddToSet:     bson.M{},
		PullAll:      bson.M{},
		ArrayFilters: []bson.M{},
	}
}

// PatcherFunc is a function that generates a BsonPatchBad by comparing two values.
type PatcherFunc func(oldVal, newVal interface{}) (*BsonPatchBad, error)

// PatcherCache is a cache of PatcherFunc functions for different types.
type PatcherCache struct {
	cache map[reflect.Type]PatcherFunc
	mu    sync.RWMutex
}

// Global patcher cache
var patcherCache = &PatcherCache{
	cache: make(map[reflect.Type]PatcherFunc),
}

// GetPatcher returns a cached patcher function for a type.
// If the type is not in the cache, it generates a new patcher function.
func (c *PatcherCache) GetPatcher(t reflect.Type) (PatcherFunc, error) {
	// Check if type is already in cache
	c.mu.RLock()
	patcher, found := c.cache[t]
	c.mu.RUnlock()

	if found {
		return patcher, nil
	}

	// Type not in cache, generate a new patcher
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check again in case another goroutine added it while we were waiting
	if patcher, found = c.cache[t]; found {
		return patcher, nil
	}

	// Generate a new patcher function
	patcher, err := generatePatcher(t)
	if err != nil {
		return nil, err
	}

	// Add to cache
	c.cache[t] = patcher
	return patcher, nil
}

// generatePatcher generates a patcher function for a type.
func generatePatcher(t reflect.Type) (PatcherFunc, error) {
	// Ensure we're working with a struct type
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("cannot generate patcher for non-struct type: %s", t.String())
	}

	// Create a patcher function for the type
	return func(oldVal, newVal interface{}) (*BsonPatchBad, error) {
		// Convert to reflect.Value
		oldReflect := reflect.ValueOf(oldVal)
		newReflect := reflect.ValueOf(newVal)

		// Ensure we're working with pointers
		if oldReflect.Kind() != reflect.Ptr || newReflect.Kind() != reflect.Ptr {
			return nil, fmt.Errorf("both values must be pointers")
		}

		// Ensure pointers are not nil
		if oldReflect.IsNil() || newReflect.IsNil() {
			return nil, fmt.Errorf("pointers cannot be nil")
		}

		// Get the actual struct values
		oldReflect = oldReflect.Elem()
		newReflect = newReflect.Elem()

		// Ensure we're working with structs
		if oldReflect.Kind() != reflect.Struct || newReflect.Kind() != reflect.Struct {
			return nil, fmt.Errorf("values must be structs")
		}

		// Create a new patch
		patch := NewBsonPatchBad()

		// Compare fields
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

			// Get field values
			oldField := oldReflect.Field(i)
			newField := newReflect.Field(i)

			// Skip if both fields are zero values
			if isZeroValue(oldField) && isZeroValue(newField) {
				continue
			}

			// Process field based on its kind
			if err := processFieldBad(patch, "", tagName, oldField, newField); err != nil {
				return nil, err
			}
		}

		return patch, nil
	}, nil
}

// processFieldBad processes a field and adds appropriate operations to the patch.
func processFieldBad(patch *BsonPatchBad, prefix, fieldName string, oldVal, newVal reflect.Value) error {
	// Create the full field path
	path := fieldName
	if prefix != "" {
		path = prefix + "." + fieldName
	}

	// Handle nil pointers
	if oldVal.Kind() == reflect.Ptr && newVal.Kind() == reflect.Ptr {
		if oldVal.IsNil() && newVal.IsNil() {
			return nil // Both nil, no change
		}

		if oldVal.IsNil() {
			// Old is nil, new is not nil - set the new value
			patch.Set[path] = newVal.Interface()
			return nil
		}

		if newVal.IsNil() {
			// Old is not nil, new is nil - unset the field
			patch.Unset[path] = ""
			return nil
		}

		// Both are non-nil pointers, dereference and continue
		return processFieldBad(patch, prefix, fieldName, oldVal.Elem(), newVal.Elem())
	}

	// Handle different kinds of fields
	switch oldVal.Kind() {
	case reflect.Struct:
		return processStructBad(patch, path, oldVal, newVal)
	case reflect.Map:
		return processMapBad(patch, path, oldVal, newVal)
	case reflect.Slice, reflect.Array:
		return processArrayBad(patch, path, oldVal, newVal)
	default:
		// For primitive types, just compare values
		if !reflect.DeepEqual(oldVal.Interface(), newVal.Interface()) {
			if isZeroValue(newVal) {
				patch.Unset[path] = ""
			} else {
				patch.Set[path] = newVal.Interface()
			}
		}
		return nil
	}
}

// processStructBad processes a struct field and adds appropriate operations to the patch.
func processStructBad(patch *BsonPatchBad, prefix string, oldVal, newVal reflect.Value) error {
	// Skip if types don't match
	if oldVal.Type() != newVal.Type() {
		patch.Set[prefix] = newVal.Interface()
		return nil
	}

	// Get or generate a patcher for this struct type
	patcher, err := patcherCache.GetPatcher(oldVal.Type())
	if err != nil {
		// Fallback to direct comparison if we can't generate a patcher
		if !reflect.DeepEqual(oldVal.Interface(), newVal.Interface()) {
			patch.Set[prefix] = newVal.Interface()
		}
		return nil
	}

	// Use the patcher to generate a patch for the struct
	structPatch, err := patcher(oldVal.Addr().Interface(), newVal.Addr().Interface())
	if err != nil {
		return err
	}

	// Merge the struct patch into the main patch with prefix
	for field, value := range structPatch.Set {
		fullPath := prefix
		if field != "" {
			fullPath = prefix + "." + field
		}
		patch.Set[fullPath] = value
	}

	for field, value := range structPatch.Unset {
		fullPath := prefix
		if field != "" {
			fullPath = prefix + "." + field
		}
		patch.Unset[fullPath] = value
	}

	// Handle other operators if needed
	// (Inc, Push, Pull, etc.)

	return nil
}

// processMapBad processes a map field and adds appropriate operations to the patch.
func processMapBad(patch *BsonPatchBad, prefix string, oldVal, newVal reflect.Value) error {
	// If types don't match, replace the whole map
	if oldVal.Type() != newVal.Type() {
		patch.Set[prefix] = newVal.Interface()
		return nil
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
			// Process recursively based on value type
			if err := processFieldBad(patch, prefix, keyStr, oldMapVal, newMapVal); err != nil {
				return err
			}
		}
	}

	return nil
}

// processArrayBad processes an array or slice field and adds appropriate operations to the patch.
func processArrayBad(patch *BsonPatchBad, prefix string, oldVal, newVal reflect.Value) error {
	// If lengths are different or types don't match, replace the whole array
	if oldVal.Len() != newVal.Len() || oldVal.Type() != newVal.Type() {
		// For small arrays, just replace the whole array
		if newVal.Len() < 10 {
			patch.Set[prefix] = newVal.Interface()
			return nil
		}

		// For larger arrays, try to use array operators
		elemType := oldVal.Type().Elem()

		// If elements are primitive types, use efficient array operators
		if isPrimitiveType(elemType) {
			return processArrayOfPrimitives(patch, prefix, oldVal, newVal)
		}

		// If elements are structs, try to use struct patchers
		if elemType.Kind() == reflect.Struct ||
			(elemType.Kind() == reflect.Ptr && elemType.Elem().Kind() == reflect.Struct) {
			return processArrayOfStructs(patch, prefix, oldVal, newVal)
		}

		// Fallback to replacing the whole array
		patch.Set[prefix] = newVal.Interface()
		return nil
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
			// Create path with array index
			elemPath := fmt.Sprintf("%s.%d", prefix, i)

			// Process element based on its kind
			if err := processFieldBad(patch, "", elemPath, oldElem, newElem); err != nil {
				return err
			}
		}
	}

	return nil
}

// processArrayOfPrimitives processes an array of primitive types.
func processArrayOfPrimitives(patch *BsonPatchBad, prefix string, oldVal, newVal reflect.Value) error {
	// Create maps for quick lookup
	oldItems := make(map[interface{}]int)
	newItems := make(map[interface{}]int)

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
	var elementsToAdd []interface{}
	var elementsToRemove []interface{}

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

	return nil
}

// processArrayOfStructs processes an array of struct types.
func processArrayOfStructs(patch *BsonPatchBad, prefix string, oldVal, newVal reflect.Value) error {
	// For arrays of structs, we need a way to identify matching elements
	// This is complex and depends on the specific use case
	// For now, we'll just replace the whole array
	patch.Set[prefix] = newVal.Interface()
	return nil
}

// isPrimitiveType checks if a type is a primitive type.
func isPrimitiveType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
		reflect.String:
		return true
	default:
		return false
	}
}

// CreateBsonPatchBad creates a MongoDB update patch by comparing two documents.
// It uses runtime-generated patcher functions for efficient comparison.
func CreateBsonPatchBad[T any](oldDoc, newDoc T) (*BsonPatchBad, error) {
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

	// Get or generate a patcher for this type
	patcher, err := patcherCache.GetPatcher(oldVal.Type())
	if err != nil {
		core.Warn("Failed to get patcher for type",
			zap.String("type", oldVal.Type().String()),
			zap.Error(err))

		// Fallback to simple comparison
		if reflect.DeepEqual(oldVal.Interface(), newVal.Interface()) {
			return NewBsonPatchBad(), nil
		}

		// If documents are different, return a patch that replaces the whole document
		patch := NewBsonPatchBad()
		patch.Set[""] = newVal.Interface()
		return patch, nil
	}

	// Use the patcher to generate a patch
	return patcher(oldDoc, newDoc)
}

// RegisterPatcher manually registers a custom patcher function for a type.
// This can be used to optimize specific types with custom comparison logic.
func RegisterPatcher(t reflect.Type, patcher PatcherFunc) {
	patcherCache.mu.Lock()
	defer patcherCache.mu.Unlock()
	patcherCache.cache[t] = patcher
}

// RegisterCustomStructPatcher registers a custom patcher for a struct type.
// This is useful for complex structs that need special handling.
func RegisterCustomStructPatcher[T any](patcher func(old, new *T) (*BsonPatchBad, error)) {
	var zero T
	t := reflect.TypeOf(&zero).Elem()

	wrapper := func(oldVal, newVal interface{}) (*BsonPatchBad, error) {
		oldT, ok1 := oldVal.(*T)
		newT, ok2 := newVal.(*T)

		if !ok1 || !ok2 {
			return nil, fmt.Errorf("invalid types for custom patcher")
		}

		return patcher(oldT, newT)
	}

	RegisterPatcher(t, wrapper)
}
