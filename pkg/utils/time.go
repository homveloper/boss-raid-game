package utils

import (
	"math/rand"
	"sync"
	"time"
)

var (
	// 시간 함수 (테스트를 위해 오버라이드 가능)
	timeNow = time.Now

	// 랜덤 생성기
	randomGenerator     = rand.New(rand.NewSource(time.Now().UnixNano()))
	randomGeneratorLock sync.Mutex
)

// GetTimeNow는 현재 시간을 반환합니다.
func GetTimeNow() time.Time {
	return timeNow()
}

// SetTimeNow는 시간 함수를 설정합니다. (테스트용)
func SetTimeNow(fn func() time.Time) func() time.Time {
	old := timeNow
	timeNow = fn
	return old
}

// GetRandom은 랜덤 생성기를 반환합니다.
func GetRandom() *rand.Rand {
	randomGeneratorLock.Lock()
	defer randomGeneratorLock.Unlock()
	return randomGenerator
}

// SetRandomSeed는 랜덤 생성기의 시드를 설정합니다.
func SetRandomSeed(seed int64) {
	randomGeneratorLock.Lock()
	defer randomGeneratorLock.Unlock()
	randomGenerator = rand.New(rand.NewSource(seed))
}
