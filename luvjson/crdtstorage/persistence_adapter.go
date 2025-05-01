package crdtstorage

import (
	"context"
)

// PersistenceAdapter는 영구 저장소 어댑터 인터페이스입니다.
// 이 인터페이스는 Document 객체와 저장소 간의 변환을 담당합니다.
// SQL, NoSQL 등 다양한 저장소 타입에 맞게 구현할 수 있습니다.
type PersistenceAdapter interface {
	// SaveDocument는 문서를 저장합니다.
	// ctx: 컨텍스트
	// doc: 저장할 Document 객체
	SaveDocument(ctx context.Context, doc *Document) error

	// LoadDocument는 문서를 로드합니다.
	// ctx: 컨텍스트
	// documentID: 문서 ID
	// 문서의 직렬화된 데이터를 반환합니다.
	LoadDocument(ctx context.Context, documentID string) ([]byte, error)

	// ListDocuments는 모든 문서 목록을 반환합니다.
	// ctx: 컨텍스트
	// 문서 ID 목록을 반환합니다.
	ListDocuments(ctx context.Context) ([]string, error)

	// DeleteDocument는 문서를 삭제합니다.
	// ctx: 컨텍스트
	// documentID: 문서 ID
	DeleteDocument(ctx context.Context, documentID string) error

	// Close는 어댑터를 닫습니다.
	Close() error
}

// DocumentSerializer는 문서 직렬화/역직렬화 인터페이스입니다.
// 이 인터페이스는 Document 객체와 다양한 형식 간의 변환을 담당합니다.
type DocumentSerializer interface {
	// Serialize는 문서를 바이트 배열로 직렬화합니다.
	Serialize(doc *Document) ([]byte, error)

	// Deserialize는 바이트 배열에서 문서를 역직렬화합니다.
	Deserialize(doc *Document, data []byte) error

	// ToMap은 문서를 맵으로 변환합니다.
	ToMap(doc *Document) (map[string]interface{}, error)

	// FromMap은 맵에서 문서를 생성합니다.
	FromMap(doc *Document, data map[string]interface{}) error
}

// DefaultDocumentSerializer는 기본 문서 직렬화/역직렬화 구현체입니다.
type DefaultDocumentSerializer struct{}

// NewDefaultDocumentSerializer는 새 기본 문서 직렬화/역직렬화 구현체를 생성합니다.
func NewDefaultDocumentSerializer() *DefaultDocumentSerializer {
	return &DefaultDocumentSerializer{}
}
