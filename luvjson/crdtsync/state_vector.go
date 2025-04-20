package crdtsync

import (
	"sync"

	"tictactoe/luvjson/common"
)

// StateVector는 CRDT 노드의 상태 벡터를 나타냅니다.
// 각 세션 ID에 대한 최신 카운터 값을 추적합니다.
type StateVector struct {
	// vector는 세션 ID 문자열에서 카운터 값으로의 맵입니다.
	vector map[string]uint64

	// mutex는 벡터에 대한 동시 접근을 보호합니다.
	mutex sync.RWMutex
}

// NewStateVector는 새 상태 벡터를 생성합니다.
func NewStateVector() *StateVector {
	return &StateVector{
		vector: make(map[string]uint64),
	}
}

// Update는 주어진 타임스탬프로 상태 벡터를 업데이트합니다.
func (sv *StateVector) Update(ts common.LogicalTimestamp) {
	sv.mutex.Lock()
	defer sv.mutex.Unlock()

	sidStr := ts.SID.String()
	if currentCounter, ok := sv.vector[sidStr]; !ok || ts.Counter > currentCounter {
		sv.vector[sidStr] = ts.Counter
	}
}

// UpdateFromMap은 맵에서 상태 벡터를 업데이트합니다.
func (sv *StateVector) UpdateFromMap(vector map[string]uint64) {
	sv.mutex.Lock()
	defer sv.mutex.Unlock()

	for sidStr, counter := range vector {
		if currentCounter, ok := sv.vector[sidStr]; !ok || counter > currentCounter {
			sv.vector[sidStr] = counter
		}
	}
}

// Get은 상태 벡터의 복사본을 반환합니다.
func (sv *StateVector) Get() map[string]uint64 {
	sv.mutex.RLock()
	defer sv.mutex.RUnlock()

	result := make(map[string]uint64, len(sv.vector))
	for sidStr, counter := range sv.vector {
		result[sidStr] = counter
	}
	return result
}

// GetCounter는 주어진 세션 ID에 대한 카운터 값을 반환합니다.
func (sv *StateVector) GetCounter(sessionID common.SessionID) uint64 {
	sv.mutex.RLock()
	defer sv.mutex.RUnlock()

	sidStr := sessionID.String()
	return sv.vector[sidStr]
}

// HasUpdates는 다른 상태 벡터와 비교하여 이 벡터에 업데이트가 있는지 확인합니다.
func (sv *StateVector) HasUpdates(other map[string]uint64) bool {
	sv.mutex.RLock()
	defer sv.mutex.RUnlock()

	for sidStr, counter := range sv.vector {
		if otherCounter, ok := other[sidStr]; !ok || counter > otherCounter {
			return true
		}
	}
	return false
}

// IsCausallyBefore는 이 벡터가 다른 벡터보다 인과적으로 이전인지 확인합니다.
func (sv *StateVector) IsCausallyBefore(other map[string]uint64) bool {
	sv.mutex.RLock()
	defer sv.mutex.RUnlock()

	// 이 벡터의 모든 카운터가 다른 벡터의 해당 카운터보다 작거나 같아야 합니다.
	for sidStr, counter := range sv.vector {
		if otherCounter, ok := other[sidStr]; !ok || counter > otherCounter {
			return false
		}
	}

	// 또한 적어도 하나의 카운터가 엄격하게 작아야 합니다.
	for sidStr, otherCounter := range other {
		if counter, ok := sv.vector[sidStr]; ok && counter < otherCounter {
			return true
		}
	}

	// 두 벡터가 동일하면 인과적으로 이전이 아닙니다.
	return false
}

// Merge는 다른 상태 벡터와 이 벡터를 병합합니다.
func (sv *StateVector) Merge(other map[string]uint64) {
	sv.mutex.Lock()
	defer sv.mutex.Unlock()

	for sidStr, counter := range other {
		if currentCounter, ok := sv.vector[sidStr]; !ok || counter > currentCounter {
			sv.vector[sidStr] = counter
		}
	}
}
