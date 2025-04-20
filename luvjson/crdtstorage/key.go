package crdtstorage

import (
	"fmt"
	"strings"
)

// Key는 저장소에서 사용하는 키를 나타내는 인터페이스입니다.
// 각 저장소 구현체는 자신에게 맞는 Key 구현체를 사용할 수 있습니다.
type Key interface {
	// String은 키의 문자열 표현을 반환합니다.
	// 이는 디버깅이나 로깅 목적으로 사용됩니다.
	String() string
}

// StringKey는 문자열 기반 키 구현체입니다.
// 간단한 저장소(파일 시스템, 메모리 등)에서 사용할 수 있습니다.
type StringKey string

// String은 키의 문자열 표현을 반환합니다.
func (k StringKey) String() string {
	return string(k)
}

// DocumentKey는 문서 키를 생성하는 함수 타입입니다.
// 각 저장소 구현체는 자신에게 맞는 방식으로 문서 ID를 키로 변환할 수 있습니다.
type DocumentKeyFunc func(documentID string) Key

// DefaultDocumentKeyFunc는 기본 문서 키 생성 함수입니다.
// 문서 ID를 그대로 StringKey로 변환합니다.
func DefaultDocumentKeyFunc(documentID string) Key {
	return StringKey(documentID)
}

// PrefixedDocumentKeyFunc는 접두사가 있는 문서 키 생성 함수를 반환합니다.
func PrefixedDocumentKeyFunc(prefix string) DocumentKeyFunc {
	return func(documentID string) Key {
		if prefix == "" {
			return StringKey(documentID)
		}
		return StringKey(prefix + ":" + documentID)
	}
}

// CompositeKey는 여러 부분으로 구성된 복합 키 구현체입니다.
// MongoDB나 다른 NoSQL 데이터베이스에서 복합 키를 표현할 때 유용합니다.
type CompositeKey struct {
	// Parts는 키의 각 부분입니다.
	Parts []interface{}
}

// NewCompositeKey는 새 복합 키를 생성합니다.
func NewCompositeKey(parts ...interface{}) *CompositeKey {
	return &CompositeKey{
		Parts: parts,
	}
}

// String은 키의 문자열 표현을 반환합니다.
func (k *CompositeKey) String() string {
	if k == nil || len(k.Parts) == 0 {
		return ""
	}

	result := fmt.Sprintf("%v", k.Parts[0])
	for i := 1; i < len(k.Parts); i++ {
		result += fmt.Sprintf(":%v", k.Parts[i])
	}
	return result
}

// PathKey는 경로 기반 키 구현체입니다.
// 계층적 구조를 가진 저장소(파일 시스템 등)에서 유용합니다.
type PathKey struct {
	// Path는 키의 경로 부분입니다.
	Path []string
}

// NewPathKey는 새 경로 키를 생성합니다.
func NewPathKey(path ...string) *PathKey {
	return &PathKey{
		Path: path,
	}
}

// String은 키의 문자열 표현을 반환합니다.
func (k *PathKey) String() string {
	if k == nil || len(k.Path) == 0 {
		return ""
	}

	return strings.Join(k.Path, "/")
}
