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
)

// FileAdapter는 파일 기반 영구 저장소 어댑터입니다.
type FileAdapter struct {
	// basePath는 문서 파일이 저장될 기본 경로입니다.
	basePath string

	// mutex는 파일 작업에 대한 동시 접근을 보호합니다.
	mutex sync.RWMutex

	// serializer는 문서 직렬화/역직렬화를 담당합니다.
	serializer DocumentSerializer
}

// NewFileAdapter는 새 파일 어댑터를 생성합니다.
func NewFileAdapter(basePath string) (*FileAdapter, error) {
	// 기본 경로가 비어 있으면 현재 디렉토리 사용
	if basePath == "" {
		basePath = "documents"
	}

	// 디렉토리 생성
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	return &FileAdapter{
		basePath:   basePath,
		serializer: NewDefaultDocumentSerializer(),
	}, nil
}

// getFilePath는 문서 ID에 대한 파일 경로를 반환합니다.
func (a *FileAdapter) getFilePath(documentID string) string {
	return filepath.Join(a.basePath, fmt.Sprintf("%s.json", documentID))
}

// SaveDocument는 문서를 파일에 저장합니다.
func (a *FileAdapter) SaveDocument(ctx context.Context, doc *Document) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 문서 직렬화
	data, err := a.serializer.Serialize(doc)
	if err != nil {
		return fmt.Errorf("failed to serialize document: %w", err)
	}

	// 파일 경로 가져오기
	filePath := a.getFilePath(doc.ID)

	// 파일 저장
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// 메타데이터 저장 예시
	if len(doc.Metadata) > 0 {
		metadataPath := a.getFilePath(doc.ID + "-meta")
		metadataJSON, err := json.Marshal(doc.Metadata)
		if err == nil {
			ioutil.WriteFile(metadataPath, metadataJSON, 0644)
		}
	}

	return nil
}

// LoadDocument는 문서를 파일에서 로드합니다.
func (a *FileAdapter) LoadDocument(ctx context.Context, documentID string) ([]byte, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 파일 경로 가져오기
	filePath := a.getFilePath(documentID)

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

// ListDocuments는 모든 문서 목록을 반환합니다.
func (a *FileAdapter) ListDocuments(ctx context.Context) ([]string, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 디렉토리 읽기
	files, err := ioutil.ReadDir(a.basePath)
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

// DeleteDocument는 문서를 파일에서 삭제합니다.
func (a *FileAdapter) DeleteDocument(ctx context.Context, documentID string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 파일 경로 가져오기
	filePath := a.getFilePath(documentID)

	// 파일 존재 여부 확인
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil // 이미 삭제됨
	}

	// 파일 삭제
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to remove file: %w", err)
	}

	// 메타데이터 파일도 삭제
	metadataPath := a.getFilePath(documentID + "-meta")
	os.Remove(metadataPath) // 오류 무시

	return nil
}

// Close는 파일 어댑터를 닫습니다.
func (a *FileAdapter) Close() error {
	// 파일 기반 저장소는 특별한 정리가 필요 없음
	return nil
}
