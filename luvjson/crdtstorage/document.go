package crdtstorage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"tictactoe/luvjson/crdtpatch"
)

// DocumentData는 문서 직렬화 데이터를 나타냅니다.
type DocumentData struct {
	// ID는 문서의 고유 식별자입니다.
	ID string `json:"id"`

	// Content는 문서 내용의 JSON 표현입니다.
	Content json.RawMessage `json:"content"`

	// LastModified는 문서가 마지막으로 수정된 시간입니다.
	LastModified time.Time `json:"last_modified"`

	// Metadata는 문서 메타데이터입니다.
	Metadata map[string]interface{} `json:"metadata"`

	// Version은 문서 버전입니다.
	Version int `json:"version"`
}

// serialize는 문서를 바이트 배열로 직렬화합니다.
func (d *Document) serialize() ([]byte, error) {
	// 문서 내용 가져오기
	content, err := d.Model.View()
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
		ID:           d.ID,
		Content:      contentJSON,
		LastModified: d.LastModified,
		Metadata:     d.Metadata,
		Version:      1, // 현재는 항상 버전 1
	}

	// 문서 데이터를 JSON으로 마샬링
	return json.Marshal(data)
}

// deserialize는 바이트 배열에서 문서를 역직렬화합니다.
func (d *Document) deserialize(data []byte) error {
	// 문서 데이터 언마샬링
	var docData DocumentData
	if err := json.Unmarshal(data, &docData); err != nil {
		return fmt.Errorf("failed to unmarshal document data: %w", err)
	}

	// 문서 필드 설정
	d.ID = docData.ID
	d.LastModified = docData.LastModified
	d.Metadata = docData.Metadata

	// 내용 언마샬링
	var content interface{}
	if err := json.Unmarshal(docData.Content, &content); err != nil {
		return fmt.Errorf("failed to unmarshal content: %w", err)
	}

	// 문서 내용 설정
	d.Model.GetApi().Root(content)

	return nil
}

// Save는 문서를 저장합니다.
func (d *Document) Save(ctx context.Context) error {
	// 마지막 수정 시간 업데이트
	d.LastModified = time.Now()

	// 저장소에 저장
	// storageImpl의 saveDocument 메서드를 직접 호출하는 대신 저장소에 저장 기능 구현

	// 저장소에 저장
	// 이 구현은 임시적으로 이전 인터페이스와의 호환성을 유지하기 위한 것입니다.
	// 실제 애플리케이션에서는 저장소 구현체에 맞는 저장 메서드를 사용해야 합니다.
	if storageImpl, ok := d.storage.(*storageImpl); ok {
		return storageImpl.persistence.SaveDocument(ctx, d)
	}

	return fmt.Errorf("unsupported storage implementation")
}

// startAutoSave는 자동 저장을 시작합니다.
func (d *Document) startAutoSave() {
	ticker := time.NewTicker(d.autoSaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			if err := d.Save(d.ctx); err != nil {
				fmt.Printf("Error auto-saving document %s: %v\n", d.ID, err)
			}
		}
	}
}

// Edit는 문서를 편집합니다.
func (d *Document) Edit(ctx context.Context, editFunc EditFunc) *EditResult {
	result := &EditResult{
		Success:  false,
		Document: d,
	}

	// 편집 함수 실행
	if err := editFunc(d.Model.GetApi()); err != nil {
		result.Error = fmt.Errorf("edit function failed: %w", err)
		return result
	}

	// 변경사항 플러시
	patch := d.Model.GetApi().Flush()

	// 패치 적용 및 브로드캐스트
	if err := d.SyncManager.ApplyPatch(ctx, patch); err != nil {
		result.Error = fmt.Errorf("failed to apply patch: %w", err)
		return result
	}

	// 마지막 수정 시간 업데이트
	d.LastModified = time.Now()

	// 변경 콜백 호출
	for _, callback := range d.onChangeCallbacks {
		callback(d, patch)
	}

	// 결과 설정
	result.Success = true
	result.Patch = patch

	return result
}

// OnChange는 문서 변경 시 호출될 콜백 함수를 등록합니다.
func (d *Document) OnChange(callback func(*Document, *crdtpatch.Patch)) {
	d.onChangeCallbacks = append(d.onChangeCallbacks, callback)
}

// GetContent는 문서 내용을 반환합니다.
func (d *Document) GetContent() (interface{}, error) {
	return d.Model.View()
}

// GetContentAs는 문서 내용을 지정된 타입으로 변환하여 반환합니다.
func (d *Document) GetContentAs(target interface{}) error {
	// 문서 내용 가져오기
	content, err := d.Model.View()
	if err != nil {
		return fmt.Errorf("failed to get document view: %w", err)
	}

	// 내용을 JSON으로 마샬링
	contentJSON, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal content: %w", err)
	}

	// JSON을 타겟 타입으로 언마샬링
	if err := json.Unmarshal(contentJSON, target); err != nil {
		return fmt.Errorf("failed to unmarshal content to target type: %w", err)
	}

	return nil
}

// SetMetadata는 문서 메타데이터를 설정합니다.
func (d *Document) SetMetadata(key string, value interface{}) {
	d.Metadata[key] = value
}

// GetMetadata는 문서 메타데이터를 반환합니다.
func (d *Document) GetMetadata(key string) (interface{}, bool) {
	value, ok := d.Metadata[key]
	return value, ok
}

// Sync는 원하는 시점에 수동으로 동기화를 수행합니다.
// 선택적으로 특정 피어와만 동기화하거나, 모든 피어와 동기화할 수 있습니다.
func (d *Document) Sync(ctx context.Context, peerID string) error {
	// 동기화 매니저가 없는 경우 오류 반환
	if d.SyncManager == nil {
		return fmt.Errorf("sync manager is not initialized")
	}

	// 특정 피어가 지정된 경우 해당 피어와만 동기화
	if peerID != "" {
		return d.SyncManager.SyncWithPeer(ctx, peerID)
	}

	// 모든 피어와 동기화하기 위해 피어 발견 및 동기화 수행
	// 이 기능은 SyncManager의 구현에 따라 다를 수 있음
	if syncAllFunc, ok := d.SyncManager.(interface{ SyncWithAllPeers(context.Context) error }); ok {
		return syncAllFunc.SyncWithAllPeers(ctx)
	}

	// SyncWithAllPeers 기능이 없는 경우, 기본 동기화 로직 사용
	// 이 경우 피어 발견 및 각 피어와의 동기화를 수동으로 처리해야 함
	return fmt.Errorf("sync with all peers is not supported by the current sync manager implementation")
}

// SyncWithAllPeers는 모든 사용 가능한 피어와 동기화를 수행합니다.
func (d *Document) SyncWithAllPeers(ctx context.Context) error {
	// 동기화 매니저가 없는 경우 오류 반환
	if d.SyncManager == nil {
		return fmt.Errorf("sync manager is not initialized")
	}

	// 피어 발견 기능을 가진 인터페이스로 형변환 시도
	if syncManager, ok := d.SyncManager.(interface {
		GetPeerDiscovery() interface {
			DiscoverPeers(context.Context) ([]string, error)
		}
	}); ok {
		// 피어 발견 기능 가져오기
		peerDiscovery := syncManager.GetPeerDiscovery()
		if peerDiscovery != nil {
			// 사용 가능한 피어 발견
			peers, err := peerDiscovery.DiscoverPeers(ctx)
			if err != nil {
				return fmt.Errorf("failed to discover peers: %w", err)
			}

			// 각 피어와 동기화
			for _, peer := range peers {
				if err := d.SyncManager.SyncWithPeer(ctx, peer); err != nil {
					// 하나의 피어 동기화 실패를 전체 실패로 처리하지 않고 계속 진행
					fmt.Printf("Warning: Failed to sync with peer %s: %v\n", peer, err)
				}
			}
			return nil
		}
	}

	return fmt.Errorf("peer discovery is not supported by the current sync manager implementation")
}

// Close는 문서를 닫습니다.
func (d *Document) Close() error {
	// 컨텍스트 취소
	d.cancel()

	// 동기화 매니저 중지
	if d.SyncManager != nil {
		if err := d.SyncManager.Stop(); err != nil {
			return fmt.Errorf("failed to stop sync manager: %w", err)
		}
	}

	return nil
}

// SetAutoSave는 자동 저장 설정을 변경합니다.
func (d *Document) SetAutoSave(autoSave bool, interval time.Duration) {
	// 이전 자동 저장 중지
	if d.autoSave && !autoSave {
		d.autoSave = false
		// 기존 고루틴은 컨텍스트 취소 시 종료됨
	}

	// 새 자동 저장 설정
	d.autoSave = autoSave
	if interval > 0 {
		d.autoSaveInterval = interval
	}

	// 자동 저장 시작
	if d.autoSave && d.ctx.Err() != nil {
		go d.startAutoSave()
	}
}
