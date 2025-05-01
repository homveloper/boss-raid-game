package crdtstorage

import (
	"context"
	"fmt"
	"sync"
)

// MemoryAdapter는 메모리 기반 영구 저장소 어댑터입니다.
type MemoryAdapter struct {
	// documents는 문서 ID에서 문서 데이터로의 맵입니다.
	documents map[string][]byte

	// mutex는 문서 맵에 대한 동시 접근을 보호합니다.
	mutex sync.RWMutex

	// serializer는 문서 직렬화/역직렬화를 담당합니다.
	serializer DocumentSerializer
}

// NewMemoryAdapter는 새 메모리 어댑터를 생성합니다.
func NewMemoryAdapter() *MemoryAdapter {
	return &MemoryAdapter{
		documents:  make(map[string][]byte),
		serializer: NewDefaultDocumentSerializer(),
	}
}

// SaveDocument는 문서를 메모리에 저장합니다.
func (a *MemoryAdapter) SaveDocument(ctx context.Context, doc *Document) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 문서 직렬화
	data, err := a.serializer.Serialize(doc)
	if err != nil {
		return fmt.Errorf("failed to serialize document: %w", err)
	}

	// 문서 데이터 복사
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	// 문서 저장
	a.documents[doc.ID] = dataCopy

	return nil
}

// LoadDocument는 문서를 메모리에서 로드합니다.
func (a *MemoryAdapter) LoadDocument(ctx context.Context, documentID string) ([]byte, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 문서 가져오기
	data, ok := a.documents[documentID]
	if !ok {
		return nil, fmt.Errorf("document not found: %s", documentID)
	}

	// 문서 데이터 복사
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	return dataCopy, nil
}

// ListDocuments는 모든 문서 목록을 반환합니다.
func (a *MemoryAdapter) ListDocuments(ctx context.Context) ([]string, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 문서 ID 목록 생성
	ids := make([]string, 0, len(a.documents))
	for id := range a.documents {
		ids = append(ids, id)
	}

	return ids, nil
}

// DeleteDocument는 문서를 메모리에서 삭제합니다.
func (a *MemoryAdapter) DeleteDocument(ctx context.Context, documentID string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 문서 삭제
	delete(a.documents, documentID)

	return nil
}

// Close는 메모리 어댑터를 닫습니다.
func (a *MemoryAdapter) Close() error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 메모리 정리
	a.documents = make(map[string][]byte)

	return nil
}
