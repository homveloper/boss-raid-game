package crdtstorage

import (
	"context"
	"fmt"
)

// CreateSnapshot은 문서의 스냅샷을 생성합니다.
func (s *storageImpl) CreateSnapshot(ctx context.Context, doc *Document) (*DocumentSnapshot, error) {
	// 고급 영구 저장소 어댑터 확인
	advancedAdapter, ok := s.persistence.(interface {
		CreateSnapshot(ctx context.Context, doc *Document) (*DocumentSnapshot, error)
	})
	if !ok {
		return nil, fmt.Errorf("storage does not support snapshots")
	}

	// 스냅샷 생성
	return advancedAdapter.CreateSnapshot(ctx, doc)
}

// SaveSnapshot은 문서의 스냅샷을 저장합니다.
func (s *storageImpl) SaveSnapshot(ctx context.Context, doc *Document) error {
	// 스냅샷 생성
	snapshot, err := s.CreateSnapshot(ctx, doc)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	// 고급 영구 저장소 어댑터 확인
	advancedAdapter, ok := s.persistence.(interface {
		SaveSnapshot(ctx context.Context, snapshot *DocumentSnapshot) error
	})
	if !ok {
		return fmt.Errorf("storage does not support snapshots")
	}

	// 스냅샷 저장
	return advancedAdapter.SaveSnapshot(ctx, snapshot)
}

// ListSnapshots은 문서의 모든 스냅샷 목록을 반환합니다.
func (s *storageImpl) ListSnapshots(ctx context.Context, documentID string) ([]int64, error) {
	// 고급 영구 저장소 어댑터 확인
	advancedAdapter, ok := s.persistence.(interface {
		ListSnapshots(ctx context.Context, documentID string) ([]int64, error)
	})
	if !ok {
		return nil, fmt.Errorf("storage does not support snapshots")
	}

	// 스냅샷 목록 가져오기
	return advancedAdapter.ListSnapshots(ctx, documentID)
}

// LoadSnapshot은 문서의 스냅샷을 로드합니다.
func (s *storageImpl) LoadSnapshot(ctx context.Context, documentID string, version int64) (*DocumentSnapshot, error) {
	// 고급 영구 저장소 어댑터 확인
	advancedAdapter, ok := s.persistence.(interface {
		LoadSnapshot(ctx context.Context, documentID string, version int64) (*DocumentSnapshot, error)
	})
	if !ok {
		return nil, fmt.Errorf("storage does not support snapshots")
	}

	// 스냅샷 로드
	return advancedAdapter.LoadSnapshot(ctx, documentID, version)
}

// RestoreFromSnapshot은 스냅샷에서 문서를 복원합니다.
func (s *storageImpl) RestoreFromSnapshot(ctx context.Context, documentID string, version int64) (interface{}, error) {
	// 고급 영구 저장소 어댑터 확인
	advancedAdapter, ok := s.persistence.(interface {
		RestoreFromSnapshot(ctx context.Context, documentID string, version int64) (interface{}, error)
	})
	if !ok {
		return nil, fmt.Errorf("storage does not support snapshots")
	}

	// 스냅샷에서 복원
	return advancedAdapter.RestoreFromSnapshot(ctx, documentID, version)
}

// DeleteSnapshot은 문서의 스냅샷을 삭제합니다.
func (s *storageImpl) DeleteSnapshot(ctx context.Context, documentID string, version int64) error {
	// 고급 영구 저장소 어댑터 확인
	advancedAdapter, ok := s.persistence.(interface {
		DeleteSnapshot(ctx context.Context, documentID string, version int64) error
	})
	if !ok {
		return fmt.Errorf("storage does not support snapshots")
	}

	// 스냅샷 삭제
	return advancedAdapter.DeleteSnapshot(ctx, documentID, version)
}

// DeleteAllSnapshots은 문서의 모든 스냅샷을 삭제합니다.
func (s *storageImpl) DeleteAllSnapshots(ctx context.Context, documentID string) error {
	// 고급 영구 저장소 어댑터 확인
	advancedAdapter, ok := s.persistence.(interface {
		DeleteAllSnapshots(ctx context.Context, documentID string) error
	})
	if !ok {
		return fmt.Errorf("storage does not support snapshots")
	}

	// 모든 스냅샷 삭제
	return advancedAdapter.DeleteAllSnapshots(ctx, documentID)
}
