package crdtstorage

import (
	"encoding/json"
	"fmt"
	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdtpatch"
	"time"
)

// DocumentData는 문서 직렬화 데이터를 나타냅니다.
type DocumentData struct {
	// ID는 문서의 고유 식별자입니다.
	ID string `json:"id" bson:"_id"`

	// Content는 문서 내용의 JSON 표현입니다.
	Content json.RawMessage `json:"content" bson:"content"`

	// LastModified는 문서가 마지막으로 수정된 시간입니다.
	LastModified time.Time `json:"last_modified" bson:"lastModified"`

	// Metadata는 문서 메타데이터입니다.
	Metadata map[string]interface{} `json:"metadata" bson:"metadata"`

	// Version은 문서 버전입니다.
	Version int64 `json:"version" bson:"version"`
}

// Serialize는 문서를 바이트 배열로 직렬화합니다.
func (s *DefaultDocumentSerializer) Serialize(doc *Document) ([]byte, error) {
	// 문서 내용 가져오기
	content, err := doc.CRDTDoc.View()
	if err != nil {
		return nil, fmt.Errorf("failed to get document view: %w", err)
	}

	// 내용을 JSON으로 마샬링
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal content: %w", err)
	}

	// 문서 데이터 생성
	data := DocumentData{
		ID:           doc.ID,
		Content:      contentJSON,
		LastModified: doc.LastModified,
		Metadata:     doc.Metadata,
		Version:      doc.Version,
	}

	// 문서 데이터를 JSON으로 마샬링
	return json.Marshal(data)
}

// Deserialize는 바이트 배열에서 문서를 역직렬화합니다.
func (s *DefaultDocumentSerializer) Deserialize(doc *Document, data []byte) error {
	// 문서 데이터 언마샬링
	var docData DocumentData
	if err := json.Unmarshal(data, &docData); err != nil {
		return fmt.Errorf("failed to unmarshal document data: %w", err)
	}

	// 문서 필드 설정
	doc.ID = docData.ID
	doc.LastModified = docData.LastModified
	doc.Metadata = docData.Metadata
	doc.Version = docData.Version

	// 내용 언마샬링
	var content interface{}
	if err := json.Unmarshal(docData.Content, &content); err != nil {
		return fmt.Errorf("failed to unmarshal content: %w", err)
	}

	// 문서 내용 설정
	// 루트 노드 생성
	rootID := doc.CRDTDoc.NextTimestamp()
	rootOp := &crdtpatch.NewOperation{
		ID:       rootID,
		NodeType: common.NodeTypeCon,
		Value:    content,
	}

	// 패치 생성 및 적용
	patchID := doc.CRDTDoc.NextTimestamp()
	patch := crdtpatch.NewPatch(patchID)
	patch.AddOperation(rootOp)

	// 루트 설정 작업 추가
	rootSetOp := &crdtpatch.InsOperation{
		ID:       doc.CRDTDoc.NextTimestamp(),
		TargetID: common.RootID,
		Value:    rootID,
	}
	patch.AddOperation(rootSetOp)

	// 패치 적용
	if err := patch.Apply(doc.CRDTDoc); err != nil {
		return fmt.Errorf("failed to apply root patch: %w", err)
	}

	// 패치 빌더 재설정
	doc.PatchBuilder = crdtpatch.NewPatchBuilder(doc.SessionID, doc.CRDTDoc.NextTimestamp().Counter)

	return nil
}

// ToMap은 문서를 맵으로 변환합니다.
func (s *DefaultDocumentSerializer) ToMap(doc *Document) (map[string]interface{}, error) {
	// 문서 내용 가져오기
	content, err := doc.CRDTDoc.View()
	if err != nil {
		return nil, fmt.Errorf("failed to get document view: %w", err)
	}

	// 맵 생성
	return map[string]interface{}{
		"id":           doc.ID,
		"content":      content,
		"lastModified": doc.LastModified,
		"metadata":     doc.Metadata,
		"version":      doc.Version,
	}, nil
}

// FromMap은 맵에서 문서를 생성합니다.
func (s *DefaultDocumentSerializer) FromMap(doc *Document, data map[string]interface{}) error {
	// ID 설정
	if id, ok := data["id"].(string); ok {
		doc.ID = id
	} else if id, ok := data["_id"].(string); ok {
		doc.ID = id
	}

	// 마지막 수정 시간 설정
	if lastModified, ok := data["lastModified"].(time.Time); ok {
		doc.LastModified = lastModified
	} else if lastModified, ok := data["last_modified"].(time.Time); ok {
		doc.LastModified = lastModified
	}

	// 메타데이터 설정
	if metadata, ok := data["metadata"].(map[string]interface{}); ok {
		doc.Metadata = metadata
	}

	// 버전 설정
	if version, ok := data["version"].(int64); ok {
		doc.Version = version
	}

	// 내용 설정
	if content, ok := data["content"]; ok {
		// 루트 노드 생성
		rootID := doc.CRDTDoc.NextTimestamp()
		rootOp := &crdtpatch.NewOperation{
			ID:       rootID,
			NodeType: common.NodeTypeCon,
			Value:    content,
		}

		// 패치 생성 및 적용
		patchID := doc.CRDTDoc.NextTimestamp()
		patch := crdtpatch.NewPatch(patchID)
		patch.AddOperation(rootOp)

		// 루트 설정 작업 추가
		rootSetOp := &crdtpatch.InsOperation{
			ID:       doc.CRDTDoc.NextTimestamp(),
			TargetID: common.RootID,
			Value:    rootID,
		}
		patch.AddOperation(rootSetOp)

		// 패치 적용
		if err := patch.Apply(doc.CRDTDoc); err != nil {
			return fmt.Errorf("failed to apply root patch: %w", err)
		}

		// 패치 빌더 재설정
		doc.PatchBuilder = crdtpatch.NewPatchBuilder(doc.SessionID, doc.CRDTDoc.NextTimestamp().Counter)
	}

	return nil
}
