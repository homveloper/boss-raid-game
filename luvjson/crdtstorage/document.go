package crdtstorage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"tictactoe/luvjson/crdtpatch"
)

// serialize는 문서를 바이트 배열로 직렬화합니다.
// 이 메서드는 하위 호환성을 위해 유지됩니다.
// 새 코드에서는 DocumentSerializer 인터페이스를 사용하세요.
func (d *Document) serialize() ([]byte, error) {
	serializer := NewDefaultDocumentSerializer()
	return serializer.Serialize(d)
}

// deserialize는 바이트 배열에서 문서를 역직렬화합니다.
// 이 메서드는 하위 호환성을 위해 유지됩니다.
// 새 코드에서는 DocumentSerializer 인터페이스를 사용하세요.
func (d *Document) deserialize(data []byte) error {
	serializer := NewDefaultDocumentSerializer()
	return serializer.Deserialize(d, data)
}

// Save는 문서를 저장합니다.
// 이 메서드는 하위 호환성을 위해 유지되지만, 실제 저장은 Storage 인터페이스를 통해 수행해야 합니다.
// 예: storage.SaveDocument(ctx, doc)
func (d *Document) Save(ctx context.Context) error {
	// 마지막 수정 시간 업데이트
	d.LastModified = time.Now()

	// 이 메서드는 더 이상 직접 저장을 수행하지 않습니다.
	// 대신 Storage 인터페이스를 통해 저장해야 합니다.
	return fmt.Errorf("direct save is no longer supported, use Storage.SaveDocument instead")
}

// startAutoSave는 자동 저장을 시작합니다.
// 이 메서드는 하위 호환성을 위해 유지되지만, 실제 자동 저장은 Storage에서 처리해야 합니다.
func (d *Document) startAutoSave() {
	ticker := time.NewTicker(d.autoSaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			// 자동 저장은 더 이상 Document에서 직접 처리하지 않습니다.
			// Storage에서 처리해야 합니다.
			fmt.Printf("Auto-save for document %s is no longer handled by Document, use Storage instead\n", d.ID)
		}
	}
}

// OnChange는 문서 변경 시 호출될 콜백 함수를 등록합니다.
func (d *Document) OnChange(callback func(*Document, *crdtpatch.Patch)) {
	d.onChangeCallbacks = append(d.onChangeCallbacks, callback)
}

// GetContent는 문서 내용을 반환합니다.
func (d *Document) GetContent() (interface{}, error) {
	return d.CRDTDoc.View()
}

// GetContentAs는 문서 내용을 지정된 타입으로 변환하여 반환합니다.
func (d *Document) GetContentAs(target interface{}) error {
	// 문서 내용 가져오기
	content, err := d.CRDTDoc.View()
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

	// SyncWithAllPeers 메서드가 있는지 확인
	if syncAllPeers, ok := d.SyncManager.(interface{ SyncWithAllPeers(context.Context) error }); ok {
		return syncAllPeers.SyncWithAllPeers(ctx)
	}

	// 기존 방식으로 시도 (하위 호환성)
	peerDiscovery := d.SyncManager.GetPeerDiscovery()
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

// CreateSnapshot은 문서의 현재 상태 스냅샷을 생성합니다.
// 이 메서드는 하위 호환성을 위해 유지되지만, 실제 스냅샷 생성은 Storage 인터페이스를 통해 수행해야 합니다.
// 예: storage.CreateSnapshot(ctx, doc)
func (d *Document) CreateSnapshot(ctx context.Context) (*DocumentSnapshot, error) {
	return nil, fmt.Errorf("direct snapshot creation is no longer supported, use Storage.CreateSnapshot instead")
}

// SaveSnapshot은 문서의 스냅샷을 저장합니다.
// 이 메서드는 하위 호환성을 위해 유지되지만, 실제 스냅샷 저장은 Storage 인터페이스를 통해 수행해야 합니다.
// 예: storage.SaveSnapshot(ctx, doc)
func (d *Document) SaveSnapshot(ctx context.Context) error {
	return fmt.Errorf("direct snapshot saving is no longer supported, use Storage.SaveSnapshot instead")
}

// ListSnapshots은 문서의 모든 스냅샷 목록을 반환합니다.
// 이 메서드는 하위 호환성을 위해 유지되지만, 실제 스냅샷 목록 조회는 Storage 인터페이스를 통해 수행해야 합니다.
// 예: storage.ListSnapshots(ctx, documentID)
func (d *Document) ListSnapshots(ctx context.Context) ([]int64, error) {
	return nil, fmt.Errorf("direct snapshot listing is no longer supported, use Storage.ListSnapshots instead")
}

// LoadSnapshot은 문서의 스냅샷을 로드합니다.
// 이 메서드는 하위 호환성을 위해 유지되지만, 실제 스냅샷 로드는 Storage 인터페이스를 통해 수행해야 합니다.
// 예: storage.LoadSnapshot(ctx, documentID, version)
func (d *Document) LoadSnapshot(ctx context.Context, version int64) (*DocumentSnapshot, error) {
	return nil, fmt.Errorf("direct snapshot loading is no longer supported, use Storage.LoadSnapshot instead")
}

// RestoreFromSnapshot은 스냅샷에서 문서를 복원합니다.
// 이 메서드는 하위 호환성을 위해 유지되지만, 실제 스냅샷 복원은 Storage 인터페이스를 통해 수행해야 합니다.
// 예: storage.RestoreFromSnapshot(ctx, documentID, version)
func (d *Document) RestoreFromSnapshot(ctx context.Context, version int64) error {
	return fmt.Errorf("direct snapshot restoration is no longer supported, use Storage.RestoreFromSnapshot instead")
}

// DeleteSnapshot은 문서의 스냅샷을 삭제합니다.
// 이 메서드는 하위 호환성을 위해 유지되지만, 실제 스냅샷 삭제는 Storage 인터페이스를 통해 수행해야 합니다.
// 예: storage.DeleteSnapshot(ctx, documentID, version)
func (d *Document) DeleteSnapshot(ctx context.Context, version int64) error {
	return fmt.Errorf("direct snapshot deletion is no longer supported, use Storage.DeleteSnapshot instead")
}

// DeleteAllSnapshots은 문서의 모든 스냅샷을 삭제합니다.
// 이 메서드는 하위 호환성을 위해 유지되지만, 실제 스냅샷 삭제는 Storage 인터페이스를 통해 수행해야 합니다.
// 예: storage.DeleteAllSnapshots(ctx, documentID)
func (d *Document) DeleteAllSnapshots(ctx context.Context) error {
	return fmt.Errorf("direct snapshot deletion is no longer supported, use Storage.DeleteAllSnapshots instead")
}
