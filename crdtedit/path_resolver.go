package crdtedit

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/yourusername/luvjson/crdt"
)

// PathResolver resolves paths to node IDs
type PathResolver struct {
	doc *crdt.Document
}

// NewPathResolver creates a new PathResolver
func NewPathResolver(doc *crdt.Document) *PathResolver {
	return &PathResolver{
		doc: doc,
	}
}

// ResolveNodePath resolves a path to a node ID
func (r *PathResolver) ResolveNodePath(path string) (*crdt.NodeID, error) {
	if path == "" {
		return nil, errors.New("path cannot be empty")
	}

	// Special case for root
	if path == "root" {
		return r.doc.GetRootID(), nil
	}

	// Check if path starts with root
	if !strings.HasPrefix(path, "root.") {
		return nil, fmt.Errorf("path must start with 'root': %s", path)
	}

	// Remove 'root.' prefix
	path = path[5:]
	if path == "" {
		return r.doc.GetRootID(), nil
	}

	// Start from root node
	currentID := r.doc.GetRootID()

	// Split path into segments
	segments := strings.Split(path, ".")

	for _, segment := range segments {
		// Check if segment contains array index
		if idx := strings.Index(segment, "["); idx >= 0 {
			if !strings.HasSuffix(segment, "]") {
				return nil, fmt.Errorf("invalid array index syntax: %s", segment)
			}

			// Extract key and index
			key := segment[:idx]
			indexStr := segment[idx+1 : len(segment)-1]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				return nil, fmt.Errorf("invalid array index: %s", indexStr)
			}

			// Get object node
			node, err := r.doc.GetNode(currentID)
			if err != nil {
				return nil, fmt.Errorf("failed to get node: %w", err)
			}

			objNode, ok := node.(*crdt.ObjectNode)
			if !ok {
				return nil, fmt.Errorf("node is not an object at segment: %s", segment)
			}

			// Get array node
			arrayID, exists := objNode.Get(key)
			if !exists {
				return nil, fmt.Errorf("key not found: %s", key)
			}

			arrayNode, err := r.doc.GetNode(arrayID)
			if err != nil {
				return nil, fmt.Errorf("failed to get array node: %w", err)
			}

			arrNode, ok := arrayNode.(*crdt.ArrayNode)
			if !ok {
				return nil, fmt.Errorf("node is not an array at key: %s", key)
			}

			// Get element at index
			if index < 0 || index >= arrNode.Length() {
				return nil, fmt.Errorf("array index out of bounds: %d", index)
			}

			elementID, err := arrNode.Get(index)
			if err != nil {
				return nil, fmt.Errorf("failed to get array element: %w", err)
			}

			currentID = elementID
		} else {
			// Regular object key
			node, err := r.doc.GetNode(currentID)
			if err != nil {
				return nil, fmt.Errorf("failed to get node: %w", err)
			}

			objNode, ok := node.(*crdt.ObjectNode)
			if !ok {
				return nil, fmt.Errorf("node is not an object at segment: %s", segment)
			}

			childID, exists := objNode.Get(segment)
			if !exists {
				return nil, fmt.Errorf("key not found: %s", segment)
			}

			currentID = childID
		}
	}

	return currentID, nil
}

// GetNodeType returns the type of a node
func (r *PathResolver) GetNodeType(nodeID *crdt.NodeID) (NodeType, error) {
	node, err := r.doc.GetNode(nodeID)
	if err != nil {
		return NodeTypeUnknown, fmt.Errorf("failed to get node: %w", err)
	}

	switch node.(type) {
	case *crdt.ObjectNode:
		return NodeTypeObject, nil
	case *crdt.ArrayNode:
		return NodeTypeArray, nil
	case *crdt.StringNode:
		return NodeTypeString, nil
	case *crdt.NumberNode:
		return NodeTypeNumber, nil
	case *crdt.BooleanNode:
		return NodeTypeBoolean, nil
	case *crdt.NullNode:
		return NodeTypeNull, nil
	default:
		return NodeTypeUnknown, nil
	}
}

// GetParentPath returns the parent path and key for a given path
func (r *PathResolver) GetParentPath(path string) (string, string, error) {
	if path == "" {
		return "", "", errors.New("path cannot be empty")
	}

	if path == "root" {
		return "", "", errors.New("root has no parent")
	}

	lastDotIndex := strings.LastIndex(path, ".")
	if lastDotIndex < 0 {
		return "", "", fmt.Errorf("invalid path format: %s", path)
	}

	parentPath := path[:lastDotIndex]
	key := path[lastDotIndex+1:]

	// Handle array index in key
	if idx := strings.Index(key, "["); idx >= 0 {
		if !strings.HasSuffix(key, "]") {
			return "", "", fmt.Errorf("invalid array index syntax: %s", key)
		}
		key = key[:idx]
	}

	return parentPath, key, nil
}
