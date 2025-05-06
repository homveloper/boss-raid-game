package crdtedit

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
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
func (r *PathResolver) ResolveNodePath(path string) (common.LogicalTimestamp, error) {
	// 빈 경로는 루트 노드를 의미
	if path == "" {
		return common.RootID, nil
	}

	// "root"는 루트 노드를 의미
	if path == "root" {
		return common.RootID, nil
	}

	// "root." 접두사가 있는 경우 제거
	if strings.HasPrefix(path, "root.") {
		path = path[5:]
	}

	// 경로가 비어 있으면 루트 노드 반환
	if path == "" {
		return common.RootID, nil
	}

	// Start from root node
	currentID := common.RootID

	// Split path into segments
	segments := strings.Split(path, ".")

	for _, segment := range segments {
		// Check if segment contains array index
		if idx := strings.Index(segment, "["); idx >= 0 {
			if !strings.HasSuffix(segment, "]") {
				return common.NilID, errors.Errorf("invalid array index syntax: %s", segment)
			}

			// Extract key and index
			key := segment[:idx]
			indexStr := segment[idx+1 : len(segment)-1]
			index, err := strconv.Atoi(indexStr)
			if err != nil {
				// Create a zero value LogicalTimestamp
				return common.NilID, errors.Errorf("invalid array index: %s", indexStr)
			}

			// Get node
			node, err := r.doc.GetNode(currentID)
			if err != nil {
				return common.NilID, errors.Wrap(err, "failed to get node")
			}

			// 노드 타입에 따라 다르게 처리
			switch n := node.(type) {
			case *crdt.LWWObjectNode:
				// 객체 노드인 경우
				childNode := n.Get(key)
				if childNode == nil {
					return common.NilID, errors.Errorf("key not found: %s", key)
				}

				// 자식 노드가 배열인지 확인
				arrayNode, err := r.doc.GetNode(childNode.ID())
				if err != nil {
					return common.NilID, errors.Wrap(err, "failed to get array node")
				}

				// 배열 노드인지 확인
				switch arrNode := arrayNode.(type) {
				case *crdt.RGAArrayNode:
					// 배열 노드인 경우 인덱스 확인
					if index < 0 || index >= arrNode.Length() {
						return common.NilID, errors.Errorf("array index out of bounds: %d", index)
					}

					// 배열 요소 가져오기
					elementID, err := arrNode.Get(index)
					if err != nil {
						return common.NilID, errors.Wrap(err, "failed to get array element")
					}

					currentID = elementID
				default:
					return common.NilID, errors.Errorf("node is not an array at segment: %s", segment)
				}
			default:
				return common.NilID, errors.Errorf("node is not an object at segment: %s", segment)
			}
		} else {
			// Regular object key
			node, err := r.doc.GetNode(currentID)
			if err != nil {
				return common.NilID, errors.Wrap(err, "failed to get node")
			}

			objNode, ok := node.(*crdt.LWWObjectNode)
			if !ok {
				return common.NilID, errors.Errorf("node is not an object at segment: %s", segment)
			}

			childNode := objNode.Get(segment)
			if childNode == nil {
				return common.NilID, errors.Errorf("key not found: %s", segment)
			}

			// Get the child ID
			childID := childNode.ID()

			currentID = childID
		}
	}

	return currentID, nil
}

// GetNodeType returns the type of a node
func (r *PathResolver) GetNodeType(nodeID common.LogicalTimestamp) (NodeType, error) {
	node, err := r.doc.GetNode(nodeID)
	if err != nil {
		return NodeTypeUnknown, errors.Wrap(err, "failed to get node")
	}

	// Use the node's Type() method to determine its type
	nodeType := node.Type()

	// Map the node type to our NodeType enum
	switch nodeType {
	case common.NodeTypeObj:
		return NodeTypeObject, nil
	case common.NodeTypeArr, common.NodeTypeVec:
		return NodeTypeArray, nil
	case common.NodeTypeStr:
		return NodeTypeString, nil
	case common.NodeTypeCon:
		// For constant nodes, we need to check the value type
		if constNode, ok := node.(*crdt.ConstantNode); ok {
			switch constNode.Value().(type) {
			case float64, int, int64:
				return NodeTypeNumber, nil
			case bool:
				return NodeTypeBoolean, nil
			case nil:
				return NodeTypeNull, nil
			case string:
				return NodeTypeString, nil
			default:
				return NodeTypeUnknown, nil
			}
		}
		return NodeTypeUnknown, nil
	default:
		return NodeTypeUnknown, nil
	}
}

// GetParentPath returns the parent path and key for a given path
func (r *PathResolver) GetParentPath(path string) (string, string, error) {
	// 빈 경로는 루트 노드를 의미하며, 루트 노드는 부모가 없음
	if path == "" || path == "root" {
		// 루트 노드의 경우 특별한 처리를 위해 빈 문자열과 "root" 키를 반환
		return "", "root", nil
	}

	// 경로에 점(.)이 없는 경우 최상위 경로로 간주
	lastDotIndex := strings.LastIndex(path, ".")
	if lastDotIndex < 0 {
		// 경로에 점이 없으면 부모는 루트, 키는 전체 경로
		return "", path, nil
	}

	parentPath := path[:lastDotIndex]
	key := path[lastDotIndex+1:]

	// Handle array index in key
	if idx := strings.Index(key, "["); idx >= 0 {
		if !strings.HasSuffix(key, "]") {
			return "", "", errors.Errorf("invalid array index syntax: %s", key)
		}

		// 배열 인덱스가 유효한지 확인
		indexStr := key[idx+1 : len(key)-1]
		_, err := strconv.Atoi(indexStr)
		if err != nil {
			return "", "", errors.Errorf("invalid array index: %s", indexStr)
		}

		// 배열 요소의 경우 부모는 배열 자체
		if lastDotIndex < 0 {
			// 최상위 배열인 경우
			parentPath = key[:idx]
		} else {
			// 중첩된 배열인 경우
			parentPath = path[:lastDotIndex+1] + key[:idx]
		}
		key = key[:idx]
	}

	return parentPath, key, nil
}
