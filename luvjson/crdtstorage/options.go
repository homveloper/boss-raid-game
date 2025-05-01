package crdtstorage

import (
	"time"
)

// StorageOptions는 저장소 옵션을 나타냅니다.
type StorageOptions struct {
	// PubSubType은 PubSub 유형입니다.
	// 지원되는 값: "memory", "redis"
	PubSubType string

	// PersistenceType은 영구 저장소 유형입니다.
	// 지원되는 값: "memory", "file", "redis"
	PersistenceType string

	// RedisAddr은 Redis 서버 주소입니다.
	RedisAddr string

	// RedisPassword는 Redis 서버 비밀번호입니다.
	RedisPassword string

	// RedisDB는 Redis 데이터베이스 번호입니다.
	RedisDB int

	// KeyPrefix는 Redis 키 접두사입니다.
	KeyPrefix string

	// PersistencePath는 파일 기반 영구 저장소의 경로입니다.
	PersistencePath string

	// AutoSave는 자동 저장 활성화 여부입니다.
	AutoSave bool

	// AutoSaveInterval은 자동 저장 간격입니다.
	AutoSaveInterval time.Duration

	// SyncMethod는 동기화 방법입니다.
	// 지원되는 값: "pubsub", "streams"
	SyncMethod string

	// MaxStreamLength는 Redis Streams의 최대 길이입니다.
	MaxStreamLength int64

	// SyncInterval은 자동 동기화 간격입니다.
	SyncInterval time.Duration

	// EnableDistributedLock은 분산 락 활성화 여부입니다.
	EnableDistributedLock bool

	// DistributedLockTimeout은 분산 락 타임아웃입니다.
	DistributedLockTimeout time.Duration

	// EnableTransactionTracking은 트랜잭션 추적 활성화 여부입니다.
	EnableTransactionTracking bool
}
