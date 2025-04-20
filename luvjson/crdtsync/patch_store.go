package crdtsync

import (
	"sync"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdtpatch"
)

// MemoryPatchStore는 메모리 기반 패치 저장소 구현입니다.
type MemoryPatchStore struct {
	// patches는 패치 ID에서 패치로의 맵입니다.
	patches map[string]*crdtpatch.Patch

	// patchesBySession은 세션 ID에서 패치 ID 목록으로의 맵입니다.
	patchesBySession map[string][]common.LogicalTimestamp

	// mutex는 저장소에 대한 동시 접근을 보호합니다.
	mutex sync.RWMutex
}

// NewMemoryPatchStore는 새 메모리 패치 저장소를 생성합니다.
func NewMemoryPatchStore() *MemoryPatchStore {
	return &MemoryPatchStore{
		patches:         make(map[string]*crdtpatch.Patch),
		patchesBySession: make(map[string][]common.LogicalTimestamp),
	}
}

// StorePatch는 패치를 저장합니다.
func (ps *MemoryPatchStore) StorePatch(patch *crdtpatch.Patch) error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// 패치 ID 가져오기
	patchID := patch.ID()
	patchIDStr := patchID.String()

	// 패치가 이미 존재하는지 확인
	if _, exists := ps.patches[patchIDStr]; exists {
		return nil // 이미 저장됨
	}

	// 패치 저장
	ps.patches[patchIDStr] = patch.Clone()

	// 세션별 패치 목록 업데이트
	sidStr := patchID.SID.String()
	ps.patchesBySession[sidStr] = append(ps.patchesBySession[sidStr], patchID)

	return nil
}

// GetPatches는 주어진 상태 벡터 이후의 패치를 반환합니다.
func (ps *MemoryPatchStore) GetPatches(stateVector map[string]uint64) ([]*crdtpatch.Patch, error) {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	var result []*crdtpatch.Patch

	// 각 세션에 대해 누락된 패치 찾기
	for sidStr, patches := range ps.patchesBySession {
		// 상태 벡터에서 세션의 카운터 가져오기
		counter, ok := stateVector[sidStr]
		if !ok {
			counter = 0
		}

		// 카운터보다 큰 패치 추가
		for _, patchID := range patches {
			if patchID.Counter > counter {
				patchIDStr := patchID.String()
				if patch, exists := ps.patches[patchIDStr]; exists {
					result = append(result, patch.Clone())
				}
			}
		}
	}

	return result, nil
}

// GetPatch는 특정 ID의 패치를 반환합니다.
func (ps *MemoryPatchStore) GetPatch(id common.LogicalTimestamp) (*crdtpatch.Patch, error) {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	idStr := id.String()
	if patch, exists := ps.patches[idStr]; exists {
		return patch.Clone(), nil
	}

	return nil, common.ErrNotFound{Message: "patch not found"}
}

// Close는 패치 저장소를 종료합니다.
func (ps *MemoryPatchStore) Close() error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	// 메모리 정리
	ps.patches = make(map[string]*crdtpatch.Patch)
	ps.patchesBySession = make(map[string][]common.LogicalTimestamp)

	return nil
}
