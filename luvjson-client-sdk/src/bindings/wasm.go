package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/totomo/luvjson-client-sdk/src/core"
)

var (
	documents = make(map[string]*core.Document)
)

// registerCallbacks registers JavaScript callbacks
func registerCallbacks() {
	js.Global().Set("createDocument", js.FuncOf(createDocument))
	js.Global().Set("applyPatch", js.FuncOf(applyPatch))
	js.Global().Set("getDocumentContent", js.FuncOf(getDocumentContent))
	js.Global().Set("createOperation", js.FuncOf(createOperation))
	js.Global().Set("createPatch", js.FuncOf(createPatch))
}

// createDocument creates a new CRDT document
func createDocument(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Document ID is required",
		})
	}

	documentID := args[0].String()
	doc := core.NewDocument(documentID)
	documents[documentID] = doc

	return js.ValueOf(map[string]interface{}{
		"success": true,
		"id":      documentID,
	})
}

// applyPatch applies a patch to a document
func applyPatch(this js.Value, args []js.Value) interface{} {
	if len(args) < 2 {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Document ID and patch are required",
		})
	}

	documentID := args[0].String()
	patchJSON := args[1].String()

	doc, ok := documents[documentID]
	if !ok {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Document not found",
		})
	}

	var patch core.Patch
	if err := json.Unmarshal([]byte(patchJSON), &patch); err != nil {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Invalid patch format: " + err.Error(),
		})
	}

	if err := doc.ApplyPatch(&patch); err != nil {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Failed to apply patch: " + err.Error(),
		})
	}

	return js.ValueOf(map[string]interface{}{
		"success": true,
		"version": doc.Version,
	})
}

// getDocumentContent returns the content of a document
func getDocumentContent(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Document ID is required",
		})
	}

	documentID := args[0].String()
	doc, ok := documents[documentID]
	if !ok {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Document not found",
		})
	}

	content := doc.GetContent()
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Failed to serialize content: " + err.Error(),
		})
	}

	return js.ValueOf(map[string]interface{}{
		"success": true,
		"content": string(contentJSON),
		"version": doc.Version,
	})
}

// createOperation creates a new operation
func createOperation(this js.Value, args []js.Value) interface{} {
	if len(args) < 4 {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Operation type, path, value, and client ID are required",
		})
	}

	opType := args[0].String()
	path := args[1].String()
	valueJSON := args[2].String()
	clientID := args[3].String()

	var value interface{}
	if err := json.Unmarshal([]byte(valueJSON), &value); err != nil {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Invalid value format: " + err.Error(),
		})
	}

	op, err := core.CreateOperation(opType, path, value, clientID)
	if err != nil {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Failed to create operation: " + err.Error(),
		})
	}

	opJSON, err := json.Marshal(op)
	if err != nil {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Failed to serialize operation: " + err.Error(),
		})
	}

	return js.ValueOf(map[string]interface{}{
		"success":   true,
		"operation": string(opJSON),
	})
}

// createPatch creates a new patch
func createPatch(this js.Value, args []js.Value) interface{} {
	if len(args) < 3 {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Document ID, operations, and client ID are required",
		})
	}

	documentID := args[0].String()
	opsJSON := args[1].String()
	clientID := args[2].String()

	doc, ok := documents[documentID]
	if !ok {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Document not found",
		})
	}

	var operations []core.Operation
	if err := json.Unmarshal([]byte(opsJSON), &operations); err != nil {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Invalid operations format: " + err.Error(),
		})
	}

	patch := doc.CreatePatch(clientID, operations)
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return js.ValueOf(map[string]interface{}{
			"success": false,
			"error":   "Failed to serialize patch: " + err.Error(),
		})
	}

	return js.ValueOf(map[string]interface{}{
		"success": true,
		"patch":   string(patchJSON),
	})
}

func main() {
	c := make(chan struct{}, 0)
	registerCallbacks()
	<-c
}
