package guild_territory

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestSetVersion tests the setVersion function from the nodestorage/v2 package
func TestSetVersion(t *testing.T) {
	// Create a test territory
	territory := &Territory{
		ID:          primitive.NewObjectID(),
		GuildID:     primitive.NewObjectID(),
		Name:        "Test Territory",
		Level:       1,
		Size:        10,
		Buildings:   []Building{},
		Resources:   Resources{},
		UpdatedAt:   time.Now(),
		VectorClock: 0,
	}

	// Print the initial state
	t.Logf("Initial VectorClock: %d", territory.VectorClock)

	// Test setting the VectorClock directly
	territory.VectorClock = 42
	assert.Equal(t, int64(42), territory.VectorClock, "Direct field assignment should work")
	t.Logf("After direct assignment: %d", territory.VectorClock)

	// Test using the setter method
	territory.SetVectorClock(99)
	assert.Equal(t, int64(99), territory.VectorClock, "Setter method should work")
	t.Logf("After setter method: %d", territory.VectorClock)

	// Test using reflection (similar to how setVersion works)
	err := setVersionTest(territory, "VectorClock", 123)
	assert.NoError(t, err, "Setting VectorClock via reflection should work")
	assert.Equal(t, int64(123), territory.VectorClock, "Reflection should set the field")
	t.Logf("After reflection (VectorClock): %d", territory.VectorClock)

	// Test using reflection with lowercase field name (should fail)
	err = setVersionTest(territory, "vectorClock", 456)
	assert.Error(t, err, "Setting vectorClock (lowercase) via reflection should fail")
	t.Logf("After reflection attempt (vectorClock): %d", territory.VectorClock)

	// Test using reflection with bson tag name (should fail)
	err = setVersionTest(territory, "vector_clock", 789)
	assert.Error(t, err, "Setting vector_clock (bson tag) via reflection should fail")
	t.Logf("After reflection attempt (vector_clock): %d", territory.VectorClock)
}

// setVersionTest is a simplified version of the setVersion function from nodestorage/v2
func setVersionTest(doc interface{}, versionField string, version int64) error {
	// Use the same code as shown in the error message
	// setVersion sets the version value in a document
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
