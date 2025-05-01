package crdtsync

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"

	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpubsub"
	redispubsub "tictactoe/luvjson/crdtpubsub"
	"tictactoe/luvjson/crdtpubsub/memory"
)

// SyncType은 동기화 유형을 나타냅니다.
type SyncType string

const (
	// SyncTypeMemory는 메모리 기반 동기화를 나타냅니다.
	SyncTypeMemory SyncType = "memory"

	// SyncTypeRedisPubSub은 Redis PubSub 기반 동기화를 나타냅니다.
	SyncTypeRedisPubSub SyncType = "redis_pubsub"

	// SyncTypeRedisStreams은 Redis Streams 기반 동기화를 나타냅니다.
	SyncTypeRedisStreams SyncType = "redis_streams"

	// SyncTypeKafka는 Kafka 기반 동기화를 나타냅니다.
	SyncTypeKafka SyncType = "kafka"

	// SyncTypeCustom은 사용자 정의 동기화를 나타냅니다.
	SyncTypeCustom SyncType = "custom"
)

// SyncOptions는 동기화 옵션을 나타냅니다.
type SyncOptions struct {
	// SyncType은 동기화 유형입니다.
	SyncType SyncType

	// RedisAddr은 Redis 서버 주소입니다.
	RedisAddr string

	// RedisPassword는 Redis 서버 비밀번호입니다.
	RedisPassword string

	// RedisDB는 Redis 데이터베이스 번호입니다.
	RedisDB int

	// KafkaBrokers는 Kafka 브로커 주소 목록입니다.
	KafkaBrokers []string

	// EncodingFormat은 패치 인코딩 형식입니다.
	EncodingFormat crdtpubsub.EncodingFormat

	// MaxStreamLength는 Redis Streams의 최대 길이입니다.
	MaxStreamLength int64

	// CustomBroadcaster는 사용자 정의 브로드캐스터입니다.
	CustomBroadcaster Broadcaster

	// CustomPatchStore는 사용자 정의 패치 저장소입니다.
	CustomPatchStore PatchStore

	// CustomPeerDiscovery는 사용자 정의 피어 발견입니다.
	CustomPeerDiscovery PeerDiscovery
}

// DefaultSyncOptions는 기본 동기화 옵션을 반환합니다.
func DefaultSyncOptions() *SyncOptions {
	return &SyncOptions{
		SyncType:        SyncTypeMemory,
		RedisAddr:       "localhost:6379",
		RedisPassword:   "",
		RedisDB:         0,
		KafkaBrokers:    []string{"localhost:9092"},
		EncodingFormat:  crdtpubsub.EncodingFormatJSON,
		MaxStreamLength: 10000,
	}
}

// CreateSyncManager는 동기화 옵션에 따라 동기화 매니저를 생성합니다.
func CreateSyncManager(
	ctx context.Context,
	doc *crdt.Document,
	documentID string,
	options *SyncOptions,
) (SyncManager, error) {
	if options == nil {
		options = DefaultSyncOptions()
	}

	// 브로드캐스터, 패치 저장소, 피어 발견 생성
	var (
		broadcaster   Broadcaster
		patchStore    PatchStore
		peerDiscovery PeerDiscovery
		err           error
	)

	// 동기화 유형에 따라 구성 요소 생성
	switch options.SyncType {
	case SyncTypeMemory:
		// 메모리 PubSub 생성
		pubsub, err := memory.NewPubSub()
		if err != nil {
			return nil, fmt.Errorf("failed to create memory PubSub: %w", err)
		}

		// 브로드캐스터 생성
		broadcaster = NewPubSubBroadcaster(
			pubsub,
			fmt.Sprintf("%s-patches", documentID),
			options.EncodingFormat,
			doc.GetSessionID(),
		)

		// 패치 저장소 생성
		patchStore = NewMemoryPatchStore()

		// 피어 발견 생성 (메모리 모드에서는 더미 구현 사용)
		peerDiscovery = &dummyPeerDiscovery{}

	case SyncTypeRedisPubSub:
		// Redis 클라이언트 생성
		redisClient := redis.NewClient(&redis.Options{
			Addr:     options.RedisAddr,
			Password: options.RedisPassword,
			DB:       options.RedisDB,
		})

		// Redis 연결 테스트
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("failed to connect to Redis: %w", err)
		}

		// Redis PubSub 생성
		pubsubOptions := crdtpubsub.NewOptions()
		pubsubOptions.DefaultFormat = options.EncodingFormat
		pubsub, err := redispubsub.NewRedisPubSub(redisClient, pubsubOptions)
		if err != nil {
			redisClient.Close()
			return nil, fmt.Errorf("failed to create Redis PubSub: %w", err)
		}

		// 브로드캐스터 생성
		broadcaster = NewPubSubBroadcaster(
			pubsub,
			fmt.Sprintf("%s-patches", documentID),
			options.EncodingFormat,
			doc.GetSessionID(),
		)

		// 패치 저장소 생성 (메모리 구현 사용)
		patchStore = NewMemoryPatchStore()

		// 피어 발견 생성
		peerDiscovery = NewRedisPeerDiscovery(redisClient, documentID, doc.GetSessionID().String()[:8])
		if err := peerDiscovery.(*RedisPeerDiscovery).Start(ctx); err != nil {
			pubsub.Close()
			redisClient.Close()
			return nil, fmt.Errorf("failed to start peer discovery: %w", err)
		}

	case SyncTypeRedisStreams:
		// Redis 클라이언트 생성
		redisClient := redis.NewClient(&redis.Options{
			Addr:     options.RedisAddr,
			Password: options.RedisPassword,
			DB:       options.RedisDB,
		})

		// Redis 연결 테스트
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("failed to connect to Redis: %w", err)
		}

		// 브로드캐스터 생성
		broadcaster, err = NewRedisStreamsBroadcaster(
			redisClient,
			fmt.Sprintf("%s-patches-stream", documentID),
			options.EncodingFormat,
			doc.GetSessionID(),
		)
		if err != nil {
			redisClient.Close()
			return nil, fmt.Errorf("failed to create Redis Streams broadcaster: %w", err)
		}

		// 패치 저장소 생성
		patchStore, err = NewRedisStreamsPatchStore(
			redisClient,
			fmt.Sprintf("%s-patches-store", documentID),
			options.EncodingFormat,
		)
		if err != nil {
			broadcaster.Close()
			redisClient.Close()
			return nil, fmt.Errorf("failed to create Redis Streams patch store: %w", err)
		}

		// 최대 스트림 길이 설정
		if options.MaxStreamLength > 0 {
			patchStore.(*RedisStreamsPatchStore).SetMaxLen(options.MaxStreamLength)
		}

		// 피어 발견 생성
		peerDiscovery = NewRedisPeerDiscovery(redisClient, documentID, doc.GetSessionID().String()[:8])
		if err := peerDiscovery.(*RedisPeerDiscovery).Start(ctx); err != nil {
			broadcaster.Close()
			patchStore.Close()
			redisClient.Close()
			return nil, fmt.Errorf("failed to start peer discovery: %w", err)
		}

	case SyncTypeKafka:
		// TODO: Kafka 구현 추가
		return nil, fmt.Errorf("Kafka sync type not implemented yet")

	case SyncTypeCustom:
		// 사용자 정의 구성 요소 사용
		if options.CustomBroadcaster == nil {
			return nil, fmt.Errorf("custom broadcaster is required for custom sync type")
		}
		if options.CustomPatchStore == nil {
			return nil, fmt.Errorf("custom patch store is required for custom sync type")
		}
		if options.CustomPeerDiscovery == nil {
			return nil, fmt.Errorf("custom peer discovery is required for custom sync type")
		}

		broadcaster = options.CustomBroadcaster
		patchStore = options.CustomPatchStore
		peerDiscovery = options.CustomPeerDiscovery

	default:
		return nil, fmt.Errorf("unsupported sync type: %s", options.SyncType)
	}

	// 상태 벡터 생성
	stateVector := NewStateVector()

	// 싱커 생성
	syncer := NewPubSubSyncer(
		nil, // PubSub은 사용하지 않음 (향후 개선 필요)
		fmt.Sprintf("%s-sync", documentID),
		doc.GetSessionID().String()[:8],
		stateVector,
		patchStore,
		options.EncodingFormat,
	)

	// 동기화 매니저 생성
	syncManager := NewSyncManager(doc, broadcaster, syncer, peerDiscovery, patchStore)

	return syncManager, nil
}

// dummyPeerDiscovery는 더미 피어 발견 구현입니다.
type dummyPeerDiscovery struct{}

func (d *dummyPeerDiscovery) DiscoverPeers(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (d *dummyPeerDiscovery) RegisterPeer(ctx context.Context, peerID string) error {
	return nil
}

func (d *dummyPeerDiscovery) UnregisterPeer(ctx context.Context, peerID string) error {
	return nil
}

func (d *dummyPeerDiscovery) Close() error {
	return nil
}
