package api

import (
	"fmt"
	"strconv"

	"tictactoe/luvjson/crdt"
)

// PathElement represents an element in a path.
type PathElement interface {
	// String returns a string representation of the path element.
	String() string
}

// StringPathElement represents a string path element (object key).
type StringPathElement string

// String returns a string representation of the path element.
func (s StringPathElement) String() string {
	return string(s)
}

// IntPathElement represents an integer path element (array index).
type IntPathElement int

// String returns a string representation of the path element.
func (i IntPathElement) String() string {
	return strconv.Itoa(int(i))
}

// Path represents a path to a node in a CRDT document.
type Path []PathElement

// String returns a string representation of the path.
func (p Path) String() string {
	if len(p) == 0 {
		return "/"
	}

	result := ""
	for _, elem := range p {
		result += "/" + elem.String()
	}
	return result
}

// ParsePath parses a path from a string.
func ParsePath(path string) (Path, error) {
	if path == "" || path == "/" {
		return Path{}, nil
	}

	if path[0] != '/' {
		return nil, fmt.Errorf("path must start with /")
	}

	// Split the path into elements
	var result Path
	for i := 1; i < len(path); i++ {
		// Find the next /
		j := i
		for j < len(path) && path[j] != '/' {
			j++
		}

		// Extract the element
		elem := path[i:j]

		// Try to parse as an integer
		if intVal, err := strconv.Atoi(elem); err == nil {
			result = append(result, IntPathElement(intVal))
		} else {
			result = append(result, StringPathElement(elem))
		}

		// Move to the next element
		i = j
	}

	return result, nil
}

// ParsePathFromInterface parses a path from an interface{} value.
// The path can be a string, a []string, a []int, or a []interface{}.
func ParsePathFromInterface(path interface{}) (Path, error) {
	switch p := path.(type) {
	case string:
		return ParsePath(p)
	case []string:
		result := make(Path, len(p))
		for i, elem := range p {
			result[i] = StringPathElement(elem)
		}
		return result, nil
	case []int:
		result := make(Path, len(p))
		for i, elem := range p {
			result[i] = IntPathElement(elem)
		}
		return result, nil
	case []interface{}:
		result := make(Path, len(p))
		for i, elem := range p {
			switch e := elem.(type) {
			case string:
				result[i] = StringPathElement(e)
			case int:
				result[i] = IntPathElement(e)
			default:
				return nil, fmt.Errorf("unsupported path element type: %T", elem)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported path type: %T", path)
	}
}

// ResolveNode resolves a node at the given path in the document.
func ResolveNode(doc *crdt.Document, path Path) (crdt.Node, error) {
	if len(path) == 0 {
		return doc.Root(), nil
	}

	// Start with the root node
	node := doc.Root()

	// Traverse the path
	for _, elem := range path {
		// If the node is a LWWValueNode, get its value
		if lwwVal, ok := node.(*crdt.LWWValueNode); ok {
			if lwwVal.NodeValue == nil {
				return nil, fmt.Errorf("path element %s not found: value is nil", elem)
			}
			node = lwwVal.NodeValue
		}

		// Handle different node types
		switch n := node.(type) {
		case *crdt.LWWObjectNode:
			// Get the field with the given key
			key, ok := elem.(StringPathElement)
			if !ok {
				return nil, fmt.Errorf("path element %s is not a string", elem)
			}
			fieldNode := n.Get(string(key))
			if fieldNode == nil {
				return nil, fmt.Errorf("path element %s not found", elem)
			}
			node = fieldNode
		// TODO: Handle other node types (arrays, etc.)
		default:
			return nil, fmt.Errorf("cannot traverse node of type %T", node)
		}
	}

	return node, nil
}
