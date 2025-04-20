package crdtsync

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"tictactoe/luvjson/common"
)

func TestStateVector_Update(t *testing.T) {
	// 상태 벡터 생성
	sv := NewStateVector()

	// 세션 ID 생성
	sid1 := common.NewSessionID()
	sid2 := common.NewSessionID()

	// 타임스탬프 생성
	ts1 := common.LogicalTimestamp{SID: sid1, Counter: 1}
	ts2 := common.LogicalTimestamp{SID: sid1, Counter: 2}
	ts3 := common.LogicalTimestamp{SID: sid2, Counter: 1}

	// 타임스탬프 업데이트
	sv.Update(ts1)
	assert.Equal(t, uint64(1), sv.GetCounter(sid1))
	assert.Equal(t, uint64(0), sv.GetCounter(sid2))

	// 더 높은 카운터로 업데이트
	sv.Update(ts2)
	assert.Equal(t, uint64(2), sv.GetCounter(sid1))

	// 다른 세션 ID로 업데이트
	sv.Update(ts3)
	assert.Equal(t, uint64(2), sv.GetCounter(sid1))
	assert.Equal(t, uint64(1), sv.GetCounter(sid2))

	// 더 낮은 카운터로 업데이트 (무시되어야 함)
	sv.Update(ts1)
	assert.Equal(t, uint64(2), sv.GetCounter(sid1))
}

func TestStateVector_UpdateFromMap(t *testing.T) {
	// 상태 벡터 생성
	sv := NewStateVector()

	// 세션 ID 생성
	sid1 := common.NewSessionID()
	sid2 := common.NewSessionID()

	// 맵 생성
	m := map[string]uint64{
		sid1.String(): 5,
		sid2.String(): 3,
	}

	// 맵에서 업데이트
	sv.UpdateFromMap(m)
	assert.Equal(t, uint64(5), sv.GetCounter(sid1))
	assert.Equal(t, uint64(3), sv.GetCounter(sid2))

	// 일부만 더 높은 값으로 업데이트
	m2 := map[string]uint64{
		sid1.String(): 4, // 더 낮음, 무시되어야 함
		sid2.String(): 7, // 더 높음, 업데이트되어야 함
	}
	sv.UpdateFromMap(m2)
	assert.Equal(t, uint64(5), sv.GetCounter(sid1))
	assert.Equal(t, uint64(7), sv.GetCounter(sid2))
}

func TestStateVector_Get(t *testing.T) {
	// 상태 벡터 생성
	sv := NewStateVector()

	// 세션 ID 생성
	sid1 := common.NewSessionID()
	sid2 := common.NewSessionID()

	// 타임스탬프 생성
	ts1 := common.LogicalTimestamp{SID: sid1, Counter: 1}
	ts2 := common.LogicalTimestamp{SID: sid2, Counter: 2}

	// 타임스탬프 업데이트
	sv.Update(ts1)
	sv.Update(ts2)

	// 상태 벡터 가져오기
	vector := sv.Get()
	assert.Equal(t, uint64(1), vector[sid1.String()])
	assert.Equal(t, uint64(2), vector[sid2.String()])
}

func TestStateVector_HasUpdates(t *testing.T) {
	// 상태 벡터 생성
	sv := NewStateVector()

	// 세션 ID 생성
	sid1 := common.NewSessionID()
	sid2 := common.NewSessionID()

	// 타임스탬프 생성
	ts1 := common.LogicalTimestamp{SID: sid1, Counter: 5}
	ts2 := common.LogicalTimestamp{SID: sid2, Counter: 3}

	// 타임스탬프 업데이트
	sv.Update(ts1)
	sv.Update(ts2)

	// 동일한 상태 벡터
	sameVector := map[string]uint64{
		sid1.String(): 5,
		sid2.String(): 3,
	}
	assert.False(t, sv.HasUpdates(sameVector))

	// 일부 카운터가 더 높은 상태 벡터
	higherVector := map[string]uint64{
		sid1.String(): 6,
		sid2.String(): 3,
	}
	assert.False(t, sv.HasUpdates(higherVector))

	// 일부 카운터가 더 낮은 상태 벡터
	lowerVector := map[string]uint64{
		sid1.String(): 4,
		sid2.String(): 3,
	}
	assert.True(t, sv.HasUpdates(lowerVector))

	// 일부 세션이 없는 상태 벡터
	missingVector := map[string]uint64{
		sid1.String(): 5,
	}
	assert.True(t, sv.HasUpdates(missingVector))
}

func TestStateVector_IsCausallyBefore(t *testing.T) {
	// 상태 벡터 생성
	sv := NewStateVector()

	// 세션 ID 생성
	sid1 := common.NewSessionID()
	sid2 := common.NewSessionID()

	// 타임스탬프 생성
	ts1 := common.LogicalTimestamp{SID: sid1, Counter: 5}
	ts2 := common.LogicalTimestamp{SID: sid2, Counter: 3}

	// 타임스탬프 업데이트
	sv.Update(ts1)
	sv.Update(ts2)

	// 동일한 상태 벡터
	sameVector := map[string]uint64{
		sid1.String(): 5,
		sid2.String(): 3,
	}
	assert.False(t, sv.IsCausallyBefore(sameVector))

	// 일부 카운터가 더 높은 상태 벡터
	higherVector := map[string]uint64{
		sid1.String(): 6,
		sid2.String(): 3,
	}
	assert.True(t, sv.IsCausallyBefore(higherVector))

	// 일부 카운터가 더 낮은 상태 벡터
	lowerVector := map[string]uint64{
		sid1.String(): 4,
		sid2.String(): 3,
	}
	assert.False(t, sv.IsCausallyBefore(lowerVector))

	// 일부 세션이 없는 상태 벡터
	missingVector := map[string]uint64{
		sid1.String(): 5,
	}
	assert.False(t, sv.IsCausallyBefore(missingVector))

	// 추가 세션이 있는 상태 벡터
	extraVector := map[string]uint64{
		sid1.String(): 5,
		sid2.String(): 3,
		common.NewSessionID().String(): 1,
	}
	assert.True(t, sv.IsCausallyBefore(extraVector))
}

func TestStateVector_Merge(t *testing.T) {
	// 상태 벡터 생성
	sv := NewStateVector()

	// 세션 ID 생성
	sid1 := common.NewSessionID()
	sid2 := common.NewSessionID()
	sid3 := common.NewSessionID()

	// 타임스탬프 생성
	ts1 := common.LogicalTimestamp{SID: sid1, Counter: 5}
	ts2 := common.LogicalTimestamp{SID: sid2, Counter: 3}

	// 타임스탬프 업데이트
	sv.Update(ts1)
	sv.Update(ts2)

	// 병합할 상태 벡터
	mergeVector := map[string]uint64{
		sid1.String(): 4, // 더 낮음, 무시되어야 함
		sid2.String(): 7, // 더 높음, 업데이트되어야 함
		sid3.String(): 2, // 새로운 세션, 추가되어야 함
	}

	// 병합
	sv.Merge(mergeVector)

	// 결과 확인
	result := sv.Get()
	assert.Equal(t, uint64(5), result[sid1.String()])
	assert.Equal(t, uint64(7), result[sid2.String()])
	assert.Equal(t, uint64(2), result[sid3.String()])
}
