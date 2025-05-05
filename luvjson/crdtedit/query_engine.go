package crdtedit

import (
	"fmt"

	"tictactoe/luvjson/crdt"
)

// QueryEngine provides methods for querying a document
type QueryEngine struct {
	doc      *crdt.Document
	resolver *PathResolver
}

// NewQueryEngine creates a new QueryEngine
func NewQueryEngine(doc *crdt.Document, resolver *PathResolver) *QueryEngine {
	return &QueryEngine{
		doc:      doc,
		resolver: resolver,
	}
}

// GetValue returns the value at the given path
func (e *QueryEngine) GetValue(path string) (any, error) {
	nodeID, err := e.resolver.ResolveNodePath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
	}

	node, err := e.doc.GetNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node at path %s: %w", path, err)
	}

	return e.doc.GetNodeValue(node)
}

// GetObject returns the object at the given path
func (e *QueryEngine) GetObject(path string) (map[string]any, error) {
	nodeID, err := e.resolver.ResolveNodePath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
	}

	node, err := e.doc.GetNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node at path %s: %w", path, err)
	}

	objNode, ok := node.(*crdt.LWWObjectNode)
	if !ok {
		return nil, fmt.Errorf("node at path %s is not an object", path)
	}

	result := make(map[string]any)
	for _, key := range objNode.Keys() {
		childNode := objNode.Get(key)
		if childNode == nil {
			return nil, fmt.Errorf("key %s does not exist", key)
		}

		value, err := e.doc.GetNodeValue(childNode)
		if err != nil {
			return nil, fmt.Errorf("failed to get value for key %s: %w", key, err)
		}

		result[key] = value
	}

	return result, nil
}

// GetArray returns the array at the given path
func (e *QueryEngine) GetArray(path string) ([]any, error) {
	nodeID, err := e.resolver.ResolveNodePath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", path, err)
	}

	node, err := e.doc.GetNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to get node at path %s: %w", path, err)
	}

	arrNode, ok := node.(*crdt.RGAArrayNode)
	if !ok {
		return nil, fmt.Errorf("node at path %s is not an array", path)
	}

	length := arrNode.Length()
	result := make([]any, length)

	for i := 0; i < length; i++ {
		elemID, err := arrNode.Get(i)
		if err != nil {
			return nil, fmt.Errorf("failed to get element at index %d: %w", i, err)
		}

		elemNode, err := e.doc.GetNode(elemID)
		if err != nil {
			return nil, fmt.Errorf("failed to get node for element at index %d: %w", i, err)
		}

		value, err := e.doc.GetNodeValue(elemNode)
		if err != nil {
			return nil, fmt.Errorf("failed to get value for element at index %d: %w", i, err)
		}

		result[i] = value
	}

	return result, nil
}

// GetString returns the string at the given path
func (e *QueryEngine) GetString(path string) (string, error) {
	value, err := e.GetValue(path)
	if err != nil {
		return "", err
	}

	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("value at path %s is not a string", path)
	}

	return str, nil
}

// GetNumber returns the number at the given path
func (e *QueryEngine) GetNumber(path string) (float64, error) {
	value, err := e.GetValue(path)
	if err != nil {
		return 0, err
	}

	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("value at path %s is not a number", path)
	}
}

// GetBoolean returns the boolean at the given path
func (e *QueryEngine) GetBoolean(path string) (bool, error) {
	value, err := e.GetValue(path)
	if err != nil {
		return false, err
	}

	b, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("value at path %s is not a boolean", path)
	}

	return b, nil
}
