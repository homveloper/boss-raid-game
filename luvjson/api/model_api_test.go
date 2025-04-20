package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tictactoe/luvjson/common"
)

func TestModelApi_Root(t *testing.T) {
	// Create a new model
	model := NewModel(common.NewSessionID())

	// Set the root value to an object
	model.GetApi().Root(map[string]interface{}{
		"counter": 0,
		"text":    "Hello",
	})

	// Get the view
	view, err := model.View()
	assert.NoError(t, err)
	assert.NotNil(t, view)

	// Check the view
	viewMap, ok := view.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(0), viewMap["counter"])
	assert.Equal(t, "Hello", viewMap["text"])
}

func TestModelApi_Obj(t *testing.T) {
	// Create a new model
	model := NewModel(common.NewSessionID())

	// Set the root value to an object
	model.GetApi().Root(map[string]interface{}{
		"counter": 0,
		"text":    "Hello",
	})

	// Update the counter field
	model.GetApi().Obj([]interface{}{}).Set(map[string]interface{}{
		"counter": 25,
	})

	// Get the view
	view, err := model.View()
	assert.NoError(t, err)
	assert.NotNil(t, view)

	// Check the view
	viewMap, ok := view.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(25), viewMap["counter"])
	assert.Equal(t, "Hello", viewMap["text"])
}

func TestModelApi_Str(t *testing.T) {
	// Create a new model
	model := NewModel(common.NewSessionID())

	// Set the root value to an object
	model.GetApi().Root(map[string]interface{}{
		"counter": 0,
		"text":    "Hello",
	})

	// Get the text field
	textNode, err := ResolveNode(model.GetDocument(), Path{StringPathElement("text")})
	assert.NoError(t, err)
	assert.NotNil(t, textNode)

	// Create a string API for the text field
	strApi := model.GetApi().Wrap(textNode).(*StrApi)

	// Insert text
	strApi.Ins(5, " world!")

	// Get the view
	view, err := model.View()
	assert.NoError(t, err)
	assert.NotNil(t, view)

	// Check the view
	viewMap, ok := view.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(0), viewMap["counter"])
	assert.Equal(t, "Hello world!", viewMap["text"])
}

func TestModelApi_Flush(t *testing.T) {
	// Create a new model
	model := NewModel(common.NewSessionID())

	// Set the root value to an object
	model.GetApi().Root(map[string]interface{}{
		"counter": 0,
		"text":    "Hello",
	})

	// Update the counter field
	model.GetApi().Obj([]interface{}{}).Set(map[string]interface{}{
		"counter": 25,
	})

	// Flush the changes
	patch := model.GetApi().Flush()
	assert.NotNil(t, patch)

	// Check that the patch has operations
	assert.Greater(t, len(patch.GetOperations()), 0)
}

func TestPath_ParsePath(t *testing.T) {
	// Test empty path
	path, err := ParsePath("")
	assert.NoError(t, err)
	assert.Equal(t, Path{}, path)

	// Test root path
	path, err = ParsePath("/")
	assert.NoError(t, err)
	assert.Equal(t, Path{}, path)

	// Test simple path
	path, err = ParsePath("/foo")
	assert.NoError(t, err)
	assert.Equal(t, Path{StringPathElement("foo")}, path)

	// Test nested path
	path, err = ParsePath("/foo/bar")
	assert.NoError(t, err)
	assert.Equal(t, Path{StringPathElement("foo"), StringPathElement("bar")}, path)

	// Test path with array index
	path, err = ParsePath("/foo/0/bar")
	assert.NoError(t, err)
	assert.Equal(t, Path{StringPathElement("foo"), IntPathElement(0), StringPathElement("bar")}, path)
}

func TestPath_ParsePathFromInterface(t *testing.T) {
	// Test string path
	path, err := ParsePathFromInterface("/foo/bar")
	assert.NoError(t, err)
	assert.Equal(t, Path{StringPathElement("foo"), StringPathElement("bar")}, path)

	// Test string slice
	path, err = ParsePathFromInterface([]string{"foo", "bar"})
	assert.NoError(t, err)
	assert.Equal(t, Path{StringPathElement("foo"), StringPathElement("bar")}, path)

	// Test int slice
	path, err = ParsePathFromInterface([]int{0, 1})
	assert.NoError(t, err)
	assert.Equal(t, Path{IntPathElement(0), IntPathElement(1)}, path)

	// Test interface slice
	path, err = ParsePathFromInterface([]interface{}{"foo", 0, "bar"})
	assert.NoError(t, err)
	assert.Equal(t, Path{StringPathElement("foo"), IntPathElement(0), StringPathElement("bar")}, path)
}
