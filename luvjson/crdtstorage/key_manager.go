package crdtstorage

import (
	"context"
)

// KeyType은 키의 유형을 나타냅니다.
type KeyType string

const (
	// KeyTypeDocument는 문서 키 유형입니다.
	KeyTypeDocument KeyType = "document"

	// KeyTypeMetadata는 메타데이터 키 유형입니다.
	KeyTypeMetadata KeyType = "metadata"

	// KeyTypeIndex는 인덱스 키 유형입니다.
	KeyTypeIndex KeyType = "index"

	// KeyTypeCollection은 컬렉션 키 유형입니다.
	KeyTypeCollection KeyType = "collection"
)

// KeyManager는 저장소의 키 관리를 위한 인터페이스입니다.
// 각 저장소 구현체는 자신의 키 관리 방식에 맞게 이 인터페이스를 구현해야 합니다.
type KeyManager interface {
	// GetKey는 주어진 키 유형과 식별자로부터 저장소에서 사용할 키를 생성합니다.
	// 반환 값은 저장소 구현체에 따라 다를 수 있습니다(문자열, 해시, 객체 등).
	GetKey(keyType KeyType, identifiers ...string) interface{}

	// GetDocumentKey는 문서 ID로부터 문서 키를 생성합니다.
	GetDocumentKey(documentID string) interface{}

	// GetMetadataKey는 문서 ID로부터 메타데이터 키를 생성합니다.
	GetMetadataKey(documentID string) interface{}

	// GetIndexKey는 인덱스 이름과 값으로부터 인덱스 키를 생성합니다.
	GetIndexKey(indexName string, value string) interface{}

	// GetCollectionKey는 컬렉션 이름에 대한 키를 생성합니다.
	GetCollectionKey(collectionName string) interface{}
}

// StringKeyManager는 문자열 기반 키 관리자 구현체입니다.
// 간단한 저장소(파일 시스템, 메모리 등)에서 사용할 수 있습니다.
type StringKeyManager struct {
	// prefix는 모든 키에 적용될 접두사입니다.
	prefix string
}

// NewStringKeyManager는 새 문자열 키 관리자를 생성합니다.
func NewStringKeyManager(prefix string) *StringKeyManager {
	return &StringKeyManager{
		prefix: prefix,
	}
}

// GetKey는 주어진 키 유형과 식별자로부터 문자열 키를 생성합니다.
func (km *StringKeyManager) GetKey(keyType KeyType, identifiers ...string) interface{} {
	key := km.prefix
	if key != "" && key[len(key)-1] != ':' {
		key += ":"
	}
	key += string(keyType)

	for _, id := range identifiers {
		key += ":" + id
	}

	return key
}

// GetDocumentKey는 문서 ID로부터 문서 키를 생성합니다.
func (km *StringKeyManager) GetDocumentKey(documentID string) interface{} {
	return km.GetKey(KeyTypeDocument, documentID)
}

// GetMetadataKey는 문서 ID로부터 메타데이터 키를 생성합니다.
func (km *StringKeyManager) GetMetadataKey(documentID string) interface{} {
	return km.GetKey(KeyTypeMetadata, documentID)
}

// GetIndexKey는 인덱스 이름과 값으로부터 인덱스 키를 생성합니다.
func (km *StringKeyManager) GetIndexKey(indexName string, value string) interface{} {
	return km.GetKey(KeyTypeIndex, indexName, value)
}

// GetCollectionKey는 컬렉션 이름에 대한 키를 생성합니다.
func (km *StringKeyManager) GetCollectionKey(collectionName string) interface{} {
	return km.GetKey(KeyTypeCollection, collectionName)
}

// DocumentQuery는 문서 쿼리를 나타냅니다.
// 이 인터페이스는 저장소 구현체에 따라 다양한 방식으로 구현될 수 있습니다.
type DocumentQuery interface {
	// GetFilter는 쿼리 필터를 반환합니다.
	// 반환 값은 저장소 구현체에 따라 다를 수 있습니다.
	GetFilter() interface{}

	// GetSort는 정렬 기준을 반환합니다.
	// 반환 값은 저장소 구현체에 따라 다를 수 있습니다.
	GetSort() interface{}

	// GetLimit는 결과 제한 수를 반환합니다.
	GetLimit() int64

	// GetSkip은 건너뛸 결과 수를 반환합니다.
	GetSkip() int64
}

// SimpleQuery는 간단한 문서 쿼리 구현체입니다.
type SimpleQuery struct {
	// filter는 쿼리 필터입니다.
	filter map[string]interface{}

	// sort는 정렬 기준입니다.
	sort map[string]interface{}

	// limit는 결과 제한 수입니다.
	limit int64

	// skip은 건너뛸 결과 수입니다.
	skip int64
}

// NewSimpleQuery는 새 간단한 쿼리를 생성합니다.
func NewSimpleQuery() *SimpleQuery {
	return &SimpleQuery{
		filter: make(map[string]interface{}),
		sort:   make(map[string]interface{}),
		limit:  0,
		skip:   0,
	}
}

// WithFilter는 필터를 추가합니다.
func (q *SimpleQuery) WithFilter(key string, value interface{}) *SimpleQuery {
	q.filter[key] = value
	return q
}

// WithSort는 정렬 기준을 추가합니다.
func (q *SimpleQuery) WithSort(key string, ascending bool) *SimpleQuery {
	if ascending {
		q.sort[key] = 1
	} else {
		q.sort[key] = -1
	}
	return q
}

// WithLimit는 결과 제한 수를 설정합니다.
func (q *SimpleQuery) WithLimit(limit int64) *SimpleQuery {
	q.limit = limit
	return q
}

// WithSkip은 건너뛸 결과 수를 설정합니다.
func (q *SimpleQuery) WithSkip(skip int64) *SimpleQuery {
	q.skip = skip
	return q
}

// GetFilter는 쿼리 필터를 반환합니다.
func (q *SimpleQuery) GetFilter() interface{} {
	return q.filter
}

// GetSort는 정렬 기준을 반환합니다.
func (q *SimpleQuery) GetSort() interface{} {
	return q.sort
}

// GetLimit는 결과 제한 수를 반환합니다.
func (q *SimpleQuery) GetLimit() int64 {
	return q.limit
}

// GetSkip은 건너뛸 결과 수를 반환합니다.
func (q *SimpleQuery) GetSkip() int64 {
	return q.skip
}

// QueryResult는 쿼리 결과를 나타냅니다.
type QueryResult struct {
	// Documents는 쿼리 결과 문서 목록입니다.
	Documents []*Document

	// TotalCount는 필터에 일치하는 총 문서 수입니다.
	TotalCount int64

	// HasMore는 더 많은 결과가 있는지 여부입니다.
	HasMore bool
}

// NewQueryResult는 새 쿼리 결과를 생성합니다.
func NewQueryResult(documents []*Document, totalCount int64, hasMore bool) *QueryResult {
	return &QueryResult{
		Documents:  documents,
		TotalCount: totalCount,
		HasMore:    hasMore,
	}
}
