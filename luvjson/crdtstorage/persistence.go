package crdtstorage

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-redis/redis/v8"
)

// MemoryPersistence는 메모리 기반 영구 저장소 구현입니다.
type MemoryPersistence struct {
	// documents는 문서 ID에서 문서 데이터로의 맵입니다.
	documents map[string][]byte

	// documentKeyFunc는 문서 키 생성 함수입니다.
	documentKeyFunc DocumentKeyFunc

	// mutex는 문서 맵에 대한 동시 접근을 보호합니다.
	mutex sync.RWMutex
}

// MemoryPersistence 생성자는 persistence_factory.go로 이동되었습니다.

// GetDocumentKeyFunc는 문서 키 생성 함수를 반환합니다.
func (p *MemoryPersistence) GetDocumentKeyFunc() DocumentKeyFunc {
	return p.documentKeyFunc
}

// SaveDocument는 문서를 저장합니다.
func (p *MemoryPersistence) SaveDocument(ctx context.Context, doc *Document) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 문서 직렬화
	data, err := doc.serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize document: %w", err)
	}

	// 문서 데이터 복사
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	// 문서 저장
	p.documents[doc.ID] = dataCopy

	// 이 구현체에서는 문서의 기본 정보만 저장하지만,
	// 사용자가 필요한 경우 추가 인덱싱이나 메타데이터 저장 로직을 구현할 수 있습니다.

	return nil
}

// LoadDocument는 문서를 로드합니다.
func (p *MemoryPersistence) LoadDocument(ctx context.Context, key Key) ([]byte, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// 키 처리
	var documentID string
	switch k := key.(type) {
	case string:
		documentID = k
	case StringKey:
		documentID = string(k)
	case fmt.Stringer:
		documentID = k.String()
	case *CompositeKey:
		if k != nil && len(k.Parts) > 0 {
			documentID = fmt.Sprintf("%v", k.Parts[0])
		}
	case *PathKey:
		if k != nil && len(k.Path) > 0 {
			documentID = strings.Join(k.Path, "/")
		}
	default:
		documentID = fmt.Sprintf("%v", key)
	}

	// 문서 가져오기
	data, ok := p.documents[documentID]
	if !ok {
		return nil, fmt.Errorf("document not found: %s", documentID)
	}

	// 문서 데이터 복사
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	return dataCopy, nil
}

// LoadDocumentByID는 문서 ID로 문서를 로드합니다.
func (p *MemoryPersistence) LoadDocumentByID(ctx context.Context, documentID string) ([]byte, error) {
	// 문서 ID로 키 생성
	key := p.documentKeyFunc(documentID)

	// 키로 문서 로드
	return p.LoadDocument(ctx, key)
}

// QueryDocuments는 쿼리에 맞는 문서를 검색합니다.
func (p *MemoryPersistence) QueryDocuments(ctx context.Context, query interface{}) ([]string, error) {
	// 메모리 구현체에서는 간단한 쿼리만 지원
	// 문자열 쿼리인 경우 문서 ID에 포함된 문서만 반환
	if queryStr, ok := query.(string); ok && queryStr != "" {
		p.mutex.RLock()
		defer p.mutex.RUnlock()

		var result []string
		for id := range p.documents {
			if strings.Contains(id, queryStr) {
				result = append(result, id)
			}
		}
		return result, nil
	}

	// 기본적으로 모든 문서 반환
	return p.ListDocuments(ctx)
}

// ListDocuments는 모든 문서 목록을 반환합니다.
func (p *MemoryPersistence) ListDocuments(ctx context.Context) ([]string, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// 문서 ID 목록 생성
	ids := make([]string, 0, len(p.documents))
	for id := range p.documents {
		ids = append(ids, id)
	}

	return ids, nil
}

// DeleteDocument는 문서를 삭제합니다.
func (p *MemoryPersistence) DeleteDocument(ctx context.Context, key Key) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 키 처리
	var documentID string
	switch k := key.(type) {
	case string:
		documentID = k
	case StringKey:
		documentID = string(k)
	case fmt.Stringer:
		documentID = k.String()
	case *CompositeKey:
		if k != nil && len(k.Parts) > 0 {
			documentID = fmt.Sprintf("%v", k.Parts[0])
		}
	case *PathKey:
		if k != nil && len(k.Path) > 0 {
			documentID = strings.Join(k.Path, "/")
		}
	default:
		documentID = fmt.Sprintf("%v", key)
	}

	// 문서 삭제
	delete(p.documents, documentID)

	return nil
}

// DeleteDocumentByID는 문서 ID로 문서를 삭제합니다.
func (p *MemoryPersistence) DeleteDocumentByID(ctx context.Context, documentID string) error {
	// 문서 ID로 키 생성
	key := p.documentKeyFunc(documentID)

	// 키로 문서 삭제
	return p.DeleteDocument(ctx, key)
}

// Close는 영구 저장소를 닫습니다.
func (p *MemoryPersistence) Close() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 메모리 정리
	p.documents = make(map[string][]byte)

	return nil
}

// FilePersistence는 파일 기반 영구 저장소 구현입니다.
type FilePersistence struct {
	// basePath는 문서 파일이 저장될 기본 경로입니다.
	basePath string

	// documentKeyFunc는 문서 키 생성 함수입니다.
	documentKeyFunc DocumentKeyFunc

	// mutex는 파일 작업에 대한 동시 접근을 보호합니다.
	mutex sync.RWMutex
}

// FilePersistence 생성자는 persistence_factory.go로 이동되었습니다.

// GetDocumentKeyFunc는 문서 키 생성 함수를 반환합니다.
func (p *FilePersistence) GetDocumentKeyFunc() DocumentKeyFunc {
	return p.documentKeyFunc
}

// getFilePath는 문서 ID에 대한 파일 경로를 반환합니다.
func (p *FilePersistence) getFilePath(documentID string) string {
	return filepath.Join(p.basePath, fmt.Sprintf("%s.json", documentID))
}

// SaveDocument는 문서를 저장합니다.
func (p *FilePersistence) SaveDocument(ctx context.Context, doc *Document) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 문서 직렬화
	data, err := doc.serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize document: %w", err)
	}

	// 파일 경로 가져오기
	filePath := p.getFilePath(doc.ID)

	// 파일 저장
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// 메타데이터 저장 예시
	if len(doc.Metadata) > 0 {
		metadataPath := p.getFilePath(doc.ID + "-meta")
		metadataJSON, err := json.Marshal(doc.Metadata)
		if err == nil {
			ioutil.WriteFile(metadataPath, metadataJSON, 0644)
		}
	}

	// 사용자는 추가 메타데이터 파일을 생성하거나 인덱스를 관리하는 등
	// 필요에 맞게 확장할 수 있습니다.

	return nil
}

// LoadDocument는 문서를 로드합니다.
func (p *FilePersistence) LoadDocument(ctx context.Context, key Key) ([]byte, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// 키 처리
	var documentID string
	switch k := key.(type) {
	case string:
		documentID = k
	case StringKey:
		documentID = string(k)
	case fmt.Stringer:
		documentID = k.String()
	case *CompositeKey:
		if k != nil && len(k.Parts) > 0 {
			documentID = fmt.Sprintf("%v", k.Parts[0])
		}
	case *PathKey:
		if k != nil && len(k.Path) > 0 {
			documentID = strings.Join(k.Path, "/")
		}
	default:
		documentID = fmt.Sprintf("%v", key)
	}

	// 파일 경로 가져오기
	filePath := p.getFilePath(documentID)

	// 파일 존재 여부 확인
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("document not found: %s", documentID)
	}

	// 파일 읽기
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

// LoadDocumentByID는 문서 ID로 문서를 로드합니다.
func (p *FilePersistence) LoadDocumentByID(ctx context.Context, documentID string) ([]byte, error) {
	// 문서 ID로 키 생성
	key := p.documentKeyFunc(documentID)

	// 키로 문서 로드
	return p.LoadDocument(ctx, key)
}

// QueryDocuments는 쿼리에 맞는 문서를 검색합니다.
func (p *FilePersistence) QueryDocuments(ctx context.Context, query interface{}) ([]string, error) {
	// 파일 구현체에서는 간단한 쿼리만 지원
	// 문자열 쿼리인 경우 문서 ID에 포함된 문서만 반환
	if queryStr, ok := query.(string); ok && queryStr != "" {
		// 모든 문서 목록 가져오기
		allDocs, err := p.ListDocuments(ctx)
		if err != nil {
			return nil, err
		}

		// 쿼리에 맞는 문서 필터링
		var result []string
		for _, id := range allDocs {
			if strings.Contains(id, queryStr) {
				result = append(result, id)
			}
		}
		return result, nil
	}

	// 기본적으로 모든 문서 반환
	return p.ListDocuments(ctx)
}

// ListDocuments는 모든 문서 목록을 반환합니다.
func (p *FilePersistence) ListDocuments(ctx context.Context) ([]string, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// 디렉토리 읽기
	files, err := ioutil.ReadDir(p.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	// 문서 ID 목록 생성
	ids := make([]string, 0, len(files))
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".json" {
			// 확장자 제거
			id := file.Name()[:len(file.Name())-5]
			// 메타데이터 파일 제외
			if !strings.HasSuffix(id, "-meta") {
				ids = append(ids, id)
			}
		}
	}

	return ids, nil
}

// DeleteDocument는 문서를 삭제합니다.
func (p *FilePersistence) DeleteDocument(ctx context.Context, key Key) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 키 처리
	var documentID string
	switch k := key.(type) {
	case string:
		documentID = k
	case StringKey:
		documentID = string(k)
	case fmt.Stringer:
		documentID = k.String()
	case *CompositeKey:
		if k != nil && len(k.Parts) > 0 {
			documentID = fmt.Sprintf("%v", k.Parts[0])
		}
	case *PathKey:
		if k != nil && len(k.Path) > 0 {
			documentID = strings.Join(k.Path, "/")
		}
	default:
		documentID = fmt.Sprintf("%v", key)
	}

	// 파일 경로 가져오기
	filePath := p.getFilePath(documentID)

	// 파일 존재 여부 확인
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // 이미 삭제됨
	}

	// 파일 삭제
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to remove file: %w", err)
	}

	// 메타데이터 파일도 삭제
	metadataPath := p.getFilePath(documentID + "-meta")
	os.Remove(metadataPath) // 오류 무시

	return nil
}

// DeleteDocumentByID는 문서 ID로 문서를 삭제합니다.
func (p *FilePersistence) DeleteDocumentByID(ctx context.Context, documentID string) error {
	// 문서 ID로 키 생성
	key := p.documentKeyFunc(documentID)

	// 키로 문서 삭제
	return p.DeleteDocument(ctx, key)
}

// Close는 영구 저장소를 닫습니다.
func (p *FilePersistence) Close() error {
	// 파일 기반 저장소는 특별한 정리가 필요 없음
	return nil
}

// RedisPersistence는 Redis 기반 영구 저장소 구현입니다.
type RedisPersistence struct {
	// client는 Redis 클라이언트입니다.
	client *redis.Client

	// keyPrefix는 Redis 키 접두사입니다.
	keyPrefix string

	// documentKeyFunc는 문서 키 생성 함수입니다.
	documentKeyFunc DocumentKeyFunc

	// mutex는 Redis 작업에 대한 동시 접근을 보호합니다.
	mutex sync.RWMutex
}

// RedisPersistence 생성자는 persistence_factory.go로 이동되었습니다.

// GetDocumentKeyFunc는 문서 키 생성 함수를 반환합니다.
func (p *RedisPersistence) GetDocumentKeyFunc() DocumentKeyFunc {
	return p.documentKeyFunc
}

// getDocumentKey는 문서 ID에 대한 Redis 키를 반환합니다.
func (p *RedisPersistence) getDocumentKey(documentID string) string {
	return fmt.Sprintf("%s:doc:%s", p.keyPrefix, documentID)
}

// getDocumentListKey는 문서 목록에 대한 Redis 키를 반환합니다.
func (p *RedisPersistence) getDocumentListKey() string {
	return fmt.Sprintf("%s:docs", p.keyPrefix)
}

// SaveDocument는 문서를 저장합니다.
func (p *RedisPersistence) SaveDocument(ctx context.Context, doc *Document) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 문서 직렬화
	data, err := doc.serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize document: %w", err)
	}

	// 문서 키 가져오기
	docKey := p.getDocumentKey(doc.ID)

	// 문서 저장
	if err := p.client.Set(ctx, docKey, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}

	// 문서 목록에 추가
	if err := p.client.SAdd(ctx, p.getDocumentListKey(), doc.ID).Err(); err != nil {
		return fmt.Errorf("failed to add document to list: %w", err)
	}

	// 문서 메타데이터 저장
	metadataKey := fmt.Sprintf("%s:meta:%s", p.keyPrefix, doc.ID)
	metadataJSON, err := json.Marshal(doc.Metadata)
	if err == nil {
		p.client.Set(ctx, metadataKey, metadataJSON, 0)
	}

	// 마지막 수정 시간 인덱싱
	timeKey := fmt.Sprintf("%s:time:%s", p.keyPrefix, doc.ID)
	p.client.Set(ctx, timeKey, doc.LastModified.Unix(), 0)

	// 문서 내용 기반 추가 인덱싱 예시
	// Go에는 try-catch가 없으므로 오류 처리를 위해 defer-recover 패턴 사용
	func() {
		// 패닉 발생 시 복구
		defer func() {
			if r := recover(); r != nil {
				// 인덱싱 오류는 무시
			}
		}()

		// 문서 내용 가져오기
		content, err := doc.GetContent()
		if err == nil {
			// 맵으로 변환 시도
			if contentMap, ok := content.(map[string]interface{}); ok {
				// 제목이 있는 경우 제목으로 인덱싱
				if title, ok := contentMap["title"].(string); ok && title != "" {
					titleKey := fmt.Sprintf("%s:title:%s", p.keyPrefix, title)
					p.client.SAdd(ctx, titleKey, doc.ID)
				}

				// 태그가 있는 경우 태그로 인덱싱
				if tags, ok := contentMap["tags"].([]interface{}); ok {
					for _, tag := range tags {
						if tagStr, ok := tag.(string); ok && tagStr != "" {
							tagKey := fmt.Sprintf("%s:tag:%s", p.keyPrefix, tagStr)
							p.client.SAdd(ctx, tagKey, doc.ID)
						}
					}
				}
			}
		}
	}()

	// 사용자는 문서 내용을 분석하여 필요한 인덱스를 생성할 수 있습니다.
	// 예를 들어, 문서 제목이나 태그를 추출하여 검색 가능한 인덱스를 만들 수 있습니다.

	return nil
}

// LoadDocument는 문서를 로드합니다.
func (p *RedisPersistence) LoadDocument(ctx context.Context, key Key) ([]byte, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// 키 처리
	var documentID string
	switch k := key.(type) {
	case string:
		documentID = k
	case StringKey:
		documentID = string(k)
	case fmt.Stringer:
		documentID = k.String()
	case *CompositeKey:
		if k != nil && len(k.Parts) > 0 {
			documentID = fmt.Sprintf("%v", k.Parts[0])
		}
	case *PathKey:
		if k != nil && len(k.Path) > 0 {
			documentID = strings.Join(k.Path, "/")
		}
	default:
		documentID = fmt.Sprintf("%v", key)
	}

	// 문서 키 가져오기
	docKey := p.getDocumentKey(documentID)

	// 문서 가져오기
	data, err := p.client.Get(ctx, docKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("document not found: %s", documentID)
		}
		return nil, fmt.Errorf("failed to load document: %w", err)
	}

	return data, nil
}

// LoadDocumentByID는 문서 ID로 문서를 로드합니다.
func (p *RedisPersistence) LoadDocumentByID(ctx context.Context, documentID string) ([]byte, error) {
	// 문서 ID로 키 생성
	key := p.documentKeyFunc(documentID)

	// 키로 문서 로드
	return p.LoadDocument(ctx, key)
}

// ListDocuments는 모든 문서 목록을 반환합니다.
func (p *RedisPersistence) ListDocuments(ctx context.Context) ([]string, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// 문서 목록 가져오기
	ids, err := p.client.SMembers(ctx, p.getDocumentListKey()).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}

	return ids, nil
}

// DeleteDocument는 문서를 삭제합니다.
func (p *RedisPersistence) DeleteDocument(ctx context.Context, key Key) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 키 처리
	var documentID string
	switch k := key.(type) {
	case string:
		documentID = k
	case StringKey:
		documentID = string(k)
	case fmt.Stringer:
		documentID = k.String()
	case *CompositeKey:
		if k != nil && len(k.Parts) > 0 {
			documentID = fmt.Sprintf("%v", k.Parts[0])
		}
	case *PathKey:
		if k != nil && len(k.Path) > 0 {
			documentID = strings.Join(k.Path, "/")
		}
	default:
		documentID = fmt.Sprintf("%v", key)
	}

	// 문서 키 가져오기
	docKey := p.getDocumentKey(documentID)

	// 문서 삭제
	if err := p.client.Del(ctx, docKey).Err(); err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	// 문서 목록에서 제거
	if err := p.client.SRem(ctx, p.getDocumentListKey(), documentID).Err(); err != nil {
		return fmt.Errorf("failed to remove document from list: %w", err)
	}

	// 메타데이터 삭제
	metadataKey := fmt.Sprintf("%s:meta:%s", p.keyPrefix, documentID)
	p.client.Del(ctx, metadataKey)

	// 시간 인덱스 삭제
	timeKey := fmt.Sprintf("%s:time:%s", p.keyPrefix, documentID)
	p.client.Del(ctx, timeKey)

	return nil
}

// DeleteDocumentByID는 문서 ID로 문서를 삭제합니다.
func (p *RedisPersistence) DeleteDocumentByID(ctx context.Context, documentID string) error {
	// 문서 ID로 키 생성
	key := p.documentKeyFunc(documentID)

	// 키로 문서 삭제
	return p.DeleteDocument(ctx, key)
}

// QueryDocuments는 쿼리에 맞는 문서를 검색합니다.
func (p *RedisPersistence) QueryDocuments(ctx context.Context, query interface{}) ([]string, error) {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// 문자열 쿼리인 경우 문서 ID에 포함된 문서만 반환
	if queryStr, ok := query.(string); ok && queryStr != "" {
		// 모든 문서 목록 가져오기
		allDocs, err := p.ListDocuments(ctx)
		if err != nil {
			return nil, err
		}

		// 쿼리에 맞는 문서 필터링
		var result []string
		for _, id := range allDocs {
			if strings.Contains(id, queryStr) {
				result = append(result, id)
			}
		}
		return result, nil
	}

	// 맵 쿼리인 경우 태그 검색 지원
	if queryMap, ok := query.(map[string]interface{}); ok {
		// 태그 검색
		if tag, ok := queryMap["tag"].(string); ok && tag != "" {
			tagKey := fmt.Sprintf("%s:tag:%s", p.keyPrefix, tag)
			ids, err := p.client.SMembers(ctx, tagKey).Result()
			if err != nil {
				return nil, fmt.Errorf("failed to query by tag: %w", err)
			}
			return ids, nil
		}

		// 제목 검색
		if title, ok := queryMap["title"].(string); ok && title != "" {
			titleKey := fmt.Sprintf("%s:title:%s", p.keyPrefix, title)
			ids, err := p.client.SMembers(ctx, titleKey).Result()
			if err != nil {
				return nil, fmt.Errorf("failed to query by title: %w", err)
			}
			return ids, nil
		}
	}

	// 기본적으로 모든 문서 반환
	return p.ListDocuments(ctx)
}

// Close는 영구 저장소를 닫습니다.
func (p *RedisPersistence) Close() error {
	// Redis 클라이언트는 외부에서 관리되므로 여기서 닫지 않음
	return nil
}
