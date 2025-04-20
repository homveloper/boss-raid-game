package crdtsync

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdtpatch"
)

func TestMemoryPatchStore_StorePatch(t *testing.T) {
	// 패치 저장소 생성
	store := NewMemoryPatchStore()

	// 세션 ID 생성
	sid := common.NewSessionID()

	// 패치 생성
	patchID := common.LogicalTimestamp{SID: sid, Counter: 1}
	patch := crdtpatch.NewPatch(patchID)

	// 패치 저장
	err := store.StorePatch(patch)
	assert.NoError(t, err)

	// 패치 가져오기
	retrievedPatch, err := store.GetPatch(patchID)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedPatch)
	assert.Equal(t, patchID, retrievedPatch.ID())

	// 동일한 패치 다시 저장 (중복 무시)
	err = store.StorePatch(patch)
	assert.NoError(t, err)
}

func TestMemoryPatchStore_GetPatches(t *testing.T) {
	// 패치 저장소 생성
	store := NewMemoryPatchStore()

	// 세션 ID 생성
	sid1 := common.NewSessionID()
	sid2 := common.NewSessionID()

	// 패치 생성
	patch1 := crdtpatch.NewPatch(common.LogicalTimestamp{SID: sid1, Counter: 1})
	patch2 := crdtpatch.NewPatch(common.LogicalTimestamp{SID: sid1, Counter: 2})
	patch3 := crdtpatch.NewPatch(common.LogicalTimestamp{SID: sid1, Counter: 3})
	patch4 := crdtpatch.NewPatch(common.LogicalTimestamp{SID: sid2, Counter: 1})
	patch5 := crdtpatch.NewPatch(common.LogicalTimestamp{SID: sid2, Counter: 2})

	// 패치 저장
	store.StorePatch(patch1)
	store.StorePatch(patch2)
	store.StorePatch(patch3)
	store.StorePatch(patch4)
	store.StorePatch(patch5)

	// 상태 벡터 생성
	stateVector := map[string]uint64{
		sid1.String(): 1, // patch1은 이미 적용됨
		sid2.String(): 0, // 모든 패치가 누락됨
	}

	// 누락된 패치 가져오기
	patches, err := store.GetPatches(stateVector)
	assert.NoError(t, err)
	assert.Len(t, patches, 4) // patch2, patch3, patch4, patch5

	// 패치 ID 확인
	patchIDs := make([]common.LogicalTimestamp, len(patches))
	for i, p := range patches {
		patchIDs[i] = p.ID()
	}

	// 예상 패치 ID
	expectedIDs := []common.LogicalTimestamp{
		{SID: sid1, Counter: 2},
		{SID: sid1, Counter: 3},
		{SID: sid2, Counter: 1},
		{SID: sid2, Counter: 2},
	}

	// 패치 ID 확인 (순서는 중요하지 않음)
	for _, id := range expectedIDs {
		found := false
		for _, patchID := range patchIDs {
			if id.SID.Compare(patchID.SID) == 0 && id.Counter == patchID.Counter {
				found = true
				break
			}
		}
		assert.True(t, found, "패치 ID %v를 찾을 수 없습니다", id)
	}

	// 모든 패치가 적용된 상태 벡터
	fullStateVector := map[string]uint64{
		sid1.String(): 3,
		sid2.String(): 2,
	}

	// 누락된 패치 가져오기 (없어야 함)
	patches, err = store.GetPatches(fullStateVector)
	assert.NoError(t, err)
	assert.Len(t, patches, 0)
}

func TestMemoryPatchStore_GetPatch(t *testing.T) {
	// 패치 저장소 생성
	store := NewMemoryPatchStore()

	// 세션 ID 생성
	sid := common.NewSessionID()

	// 패치 생성
	patchID := common.LogicalTimestamp{SID: sid, Counter: 1}
	patch := crdtpatch.NewPatch(patchID)

	// 패치 저장
	err := store.StorePatch(patch)
	assert.NoError(t, err)

	// 패치 가져오기
	retrievedPatch, err := store.GetPatch(patchID)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedPatch)
	assert.Equal(t, patchID, retrievedPatch.ID())

	// 존재하지 않는 패치 가져오기
	nonExistentID := common.LogicalTimestamp{SID: sid, Counter: 999}
	_, err = store.GetPatch(nonExistentID)
	assert.Error(t, err)
	assert.IsType(t, common.ErrNotFound{}, err)
}

func TestMemoryPatchStore_Close(t *testing.T) {
	// 패치 저장소 생성
	store := NewMemoryPatchStore()

	// 세션 ID 생성
	sid := common.NewSessionID()

	// 패치 생성
	patchID := common.LogicalTimestamp{SID: sid, Counter: 1}
	patch := crdtpatch.NewPatch(patchID)

	// 패치 저장
	err := store.StorePatch(patch)
	assert.NoError(t, err)

	// 저장소 닫기
	err = store.Close()
	assert.NoError(t, err)

	// 패치 가져오기 (실패해야 함)
	_, err = store.GetPatch(patchID)
	assert.Error(t, err)
}
