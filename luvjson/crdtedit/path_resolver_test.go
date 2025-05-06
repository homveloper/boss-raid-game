package crdtedit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
)

// 테스트용 구조체 정의
type TestUser struct {
	Name     string   `json:"name"`
	Age      int      `json:"age"`
	IsActive bool     `json:"isActive"`
	Tags     []string `json:"tags"`
	Address  struct {
		City    string `json:"city"`
		Country string `json:"country"`
	} `json:"address"`
	Scores map[string]int `json:"scores"`
}

// TestPathResolver_ResolveNodePath tests the ResolveNodePath function
func TestPathResolver_ResolveNodePath(t *testing.T) {
	// 문서 생성
	sessionID := common.NewSessionID()
	doc := crdt.NewDocument(sessionID)
	editor := NewDocumentEditor(doc)

	// 테스트 데이터 생성
	user := TestUser{
		Name:     "John Doe",
		Age:      30,
		IsActive: true,
		Tags:     []string{"developer", "golang", "crdt"},
		Scores:   map[string]int{"math": 90, "science": 85},
	}
	user.Address.City = "Seoul"
	user.Address.Country = "Korea"

	// 문서 초기화
	err := editor.InitFromStruct(user)
	require.NoError(t, err, "Failed to initialize document from struct")

	// PathResolver 생성
	resolver := NewPathResolver(doc)

	// 테스트 케이스
	testCases := []struct {
		name     string
		path     string
		wantErr  bool
		checkVal func(t *testing.T, nodeID common.LogicalTimestamp)
	}{
		{
			name:    "루트 노드",
			path:    "",
			wantErr: false,
			checkVal: func(t *testing.T, nodeID common.LogicalTimestamp) {
				assert.Equal(t, common.RootID, nodeID, "Root node ID should be common.RootID")
			},
		},
		{
			name:    "루트 노드 (root 키워드)",
			path:    "root",
			wantErr: false,
			checkVal: func(t *testing.T, nodeID common.LogicalTimestamp) {
				assert.Equal(t, common.RootID, nodeID, "Root node ID should be common.RootID")
			},
		},
		{
			name:    "이름 필드",
			path:    "name",
			wantErr: false,
			checkVal: func(t *testing.T, nodeID common.LogicalTimestamp) {
				node, err := doc.GetNode(nodeID)
				require.NoError(t, err, "Failed to get node")

				constNode, ok := node.(*crdt.ConstantNode)
				require.True(t, ok, "Node should be a ConstantNode")

				assert.Equal(t, "John Doe", constNode.Value(), "Name should be 'John Doe'")
			},
		},
		{
			name:    "나이 필드",
			path:    "age",
			wantErr: false,
			checkVal: func(t *testing.T, nodeID common.LogicalTimestamp) {
				node, err := doc.GetNode(nodeID)
				require.NoError(t, err, "Failed to get node")

				constNode, ok := node.(*crdt.ConstantNode)
				require.True(t, ok, "Node should be a ConstantNode")

				// JSON 숫자는 float64로 변환됨
				assert.Equal(t, float64(30), constNode.Value(), "Age should be 30")
			},
		},
		{
			name:    "활성 상태 필드",
			path:    "isActive",
			wantErr: false,
			checkVal: func(t *testing.T, nodeID common.LogicalTimestamp) {
				node, err := doc.GetNode(nodeID)
				require.NoError(t, err, "Failed to get node")

				constNode, ok := node.(*crdt.ConstantNode)
				require.True(t, ok, "Node should be a ConstantNode")

				assert.Equal(t, true, constNode.Value(), "IsActive should be true")
			},
		},
		{
			name:    "태그 배열",
			path:    "tags",
			wantErr: false,
			checkVal: func(t *testing.T, nodeID common.LogicalTimestamp) {
				node, err := doc.GetNode(nodeID)
				require.NoError(t, err, "Failed to get node")

				// 배열 노드 확인
				_, ok := node.(*crdt.RGAArrayNode)
				require.True(t, ok, "Node should be a RGAArrayNode")
			},
		},
		{
			name:    "첫 번째 태그",
			path:    "tags[0]",
			wantErr: false,
			checkVal: func(t *testing.T, nodeID common.LogicalTimestamp) {
				node, err := doc.GetNode(nodeID)
				require.NoError(t, err, "Failed to get node")

				constNode, ok := node.(*crdt.ConstantNode)
				require.True(t, ok, "Node should be a ConstantNode")

				assert.Equal(t, "developer", constNode.Value(), "First tag should be 'developer'")
			},
		},
		{
			name:    "두 번째 태그",
			path:    "tags[1]",
			wantErr: false,
			checkVal: func(t *testing.T, nodeID common.LogicalTimestamp) {
				node, err := doc.GetNode(nodeID)
				require.NoError(t, err, "Failed to get node")

				constNode, ok := node.(*crdt.ConstantNode)
				require.True(t, ok, "Node should be a ConstantNode")

				assert.Equal(t, "golang", constNode.Value(), "Second tag should be 'golang'")
			},
		},
		{
			name:    "주소 객체",
			path:    "address",
			wantErr: false,
			checkVal: func(t *testing.T, nodeID common.LogicalTimestamp) {
				node, err := doc.GetNode(nodeID)
				require.NoError(t, err, "Failed to get node")

				// 객체 노드 확인
				_, ok := node.(*crdt.LWWObjectNode)
				require.True(t, ok, "Node should be a LWWObjectNode")
			},
		},
		{
			name:    "도시 필드",
			path:    "address.city",
			wantErr: false,
			checkVal: func(t *testing.T, nodeID common.LogicalTimestamp) {
				node, err := doc.GetNode(nodeID)
				require.NoError(t, err, "Failed to get node")

				constNode, ok := node.(*crdt.ConstantNode)
				require.True(t, ok, "Node should be a ConstantNode")

				assert.Equal(t, "Seoul", constNode.Value(), "City should be 'Seoul'")
			},
		},
		{
			name:    "국가 필드",
			path:    "address.country",
			wantErr: false,
			checkVal: func(t *testing.T, nodeID common.LogicalTimestamp) {
				node, err := doc.GetNode(nodeID)
				require.NoError(t, err, "Failed to get node")

				constNode, ok := node.(*crdt.ConstantNode)
				require.True(t, ok, "Node should be a ConstantNode")

				assert.Equal(t, "Korea", constNode.Value(), "Country should be 'Korea'")
			},
		},
		{
			name:    "점수 객체",
			path:    "scores",
			wantErr: false,
			checkVal: func(t *testing.T, nodeID common.LogicalTimestamp) {
				node, err := doc.GetNode(nodeID)
				require.NoError(t, err, "Failed to get node")

				// 객체 노드 확인
				_, ok := node.(*crdt.LWWObjectNode)
				require.True(t, ok, "Node should be a LWWObjectNode")
			},
		},
		{
			name:    "수학 점수",
			path:    "scores.math",
			wantErr: false,
			checkVal: func(t *testing.T, nodeID common.LogicalTimestamp) {
				node, err := doc.GetNode(nodeID)
				require.NoError(t, err, "Failed to get node")

				constNode, ok := node.(*crdt.ConstantNode)
				require.True(t, ok, "Node should be a ConstantNode")

				// JSON 숫자는 float64로 변환됨
				assert.Equal(t, float64(90), constNode.Value(), "Math score should be 90")
			},
		},
		{
			name:    "존재하지 않는 필드",
			path:    "nonexistent",
			wantErr: true,
		},
		{
			name:    "존재하지 않는 중첩 필드",
			path:    "address.nonexistent",
			wantErr: true,
		},
		{
			name:    "배열 인덱스 범위 초과",
			path:    "tags[10]",
			wantErr: true,
		},
		{
			name:    "잘못된 배열 인덱스 구문",
			path:    "tags[abc]",
			wantErr: true,
		},
	}

	// 테스트 실행
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nodeID, err := resolver.ResolveNodePath(tc.path)

			if tc.wantErr {
				assert.Error(t, err, "Expected error for path: %s", tc.path)
			} else {
				assert.NoError(t, err, "Failed to resolve path: %s", tc.path)
				if tc.checkVal != nil {
					tc.checkVal(t, nodeID)
				}
			}
		})
	}
}

// TestPathResolver_GetNodeType tests the GetNodeType function
func TestPathResolver_GetNodeType(t *testing.T) {
	// 문서 생성
	sessionID := common.NewSessionID()
	doc := crdt.NewDocument(sessionID)
	editor := NewDocumentEditor(doc)

	// 테스트 데이터 생성
	user := TestUser{
		Name:     "John Doe",
		Age:      30,
		IsActive: true,
		Tags:     []string{"developer", "golang", "crdt"},
		Scores:   map[string]int{"math": 90, "science": 85},
	}
	user.Address.City = "Seoul"
	user.Address.Country = "Korea"

	// 문서 초기화
	err := editor.InitFromStruct(user)
	require.NoError(t, err, "Failed to initialize document from struct")

	// PathResolver 생성
	resolver := NewPathResolver(doc)

	// 테스트 케이스
	testCases := []struct {
		name         string
		path         string
		expectedType NodeType
		wantErr      bool
	}{
		{
			name:         "문자열 필드",
			path:         "name",
			expectedType: NodeTypeString,
			wantErr:      false,
		},
		{
			name:         "숫자 필드",
			path:         "age",
			expectedType: NodeTypeNumber,
			wantErr:      false,
		},
		{
			name:         "불리언 필드",
			path:         "isActive",
			expectedType: NodeTypeBoolean,
			wantErr:      false,
		},
		{
			name:         "배열 필드",
			path:         "tags",
			expectedType: NodeTypeArray,
			wantErr:      false,
		},
		{
			name:         "객체 필드",
			path:         "address",
			expectedType: NodeTypeObject,
			wantErr:      false,
		},
		{
			name:         "중첩 문자열 필드",
			path:         "address.city",
			expectedType: NodeTypeString,
			wantErr:      false,
		},
		{
			name:         "존재하지 않는 필드",
			path:         "nonexistent",
			expectedType: NodeTypeUnknown,
			wantErr:      true,
		},
	}

	// 테스트 실행
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 경로를 노드 ID로 변환
			nodeID, err := resolver.ResolveNodePath(tc.path)
			if tc.wantErr {
				assert.Error(t, err, "Expected error for path: %s", tc.path)
				return
			}
			require.NoError(t, err, "Failed to resolve path: %s", tc.path)

			// 노드 타입 확인
			nodeType, err := resolver.GetNodeType(nodeID)
			require.NoError(t, err, "Failed to get node type for path: %s", tc.path)
			assert.Equal(t, tc.expectedType, nodeType, "Node type mismatch for path: %s", tc.path)
		})
	}
}

// TestPathResolver_GetParentPath tests the GetParentPath function
func TestPathResolver_GetParentPath(t *testing.T) {
	// PathResolver 생성
	sessionID := common.NewSessionID()
	doc := crdt.NewDocument(sessionID)
	resolver := NewPathResolver(doc)

	// 테스트 케이스
	testCases := []struct {
		name           string
		path           string
		expectedParent string
		expectedKey    string
		wantErr        bool
	}{
		{
			name:           "루트 노드",
			path:           "",
			expectedParent: "",
			expectedKey:    "root",
			wantErr:        false,
		},
		{
			name:           "루트 노드 (root 키워드)",
			path:           "root",
			expectedParent: "",
			expectedKey:    "root",
			wantErr:        false,
		},
		{
			name:           "최상위 필드",
			path:           "name",
			expectedParent: "",
			expectedKey:    "name",
			wantErr:        false,
		},
		{
			name:           "중첩 필드",
			path:           "address.city",
			expectedParent: "address",
			expectedKey:    "city",
			wantErr:        false,
		},
		{
			name:           "깊게 중첩된 필드",
			path:           "a.b.c.d",
			expectedParent: "a.b.c",
			expectedKey:    "d",
			wantErr:        false,
		},
		{
			name:           "배열 요소",
			path:           "tags[0]",
			expectedParent: "tags",
			expectedKey:    "tags",
			wantErr:        false,
		},
		{
			name:           "잘못된 배열 구문",
			path:           "tags[abc",
			expectedParent: "",
			expectedKey:    "",
			wantErr:        true,
		},
	}

	// 테스트 실행
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parent, key, err := resolver.GetParentPath(tc.path)

			if tc.wantErr {
				assert.Error(t, err, "Expected error for path: %s", tc.path)
			} else {
				assert.NoError(t, err, "Failed to get parent path for: %s", tc.path)
				assert.Equal(t, tc.expectedParent, parent, "Parent path mismatch for: %s", tc.path)
				assert.Equal(t, tc.expectedKey, key, "Key mismatch for: %s", tc.path)
			}
		})
	}
}
