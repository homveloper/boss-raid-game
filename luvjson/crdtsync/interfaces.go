package crdtsync

import (
	"context"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
)

// Broadcaster는 노드 간 패치 전달을 위한 인터페이스입니다.
type Broadcaster interface {
	// Broadcast는 패치를 다른 노드들에게 브로드캐스트합니다.
	Broadcast(ctx context.Context, patch *crdtpatch.Patch) error

	// Next는 다음 브로드캐스트된 패치를 수신합니다.
	Next(ctx context.Context) (*crdtpatch.Patch, error)

	// Close는 브로드캐스터를 종료합니다.
	Close() error
}

// Syncer는 노드 간 상태 동기화를 위한 인터페이스입니다.
type Syncer interface {
	// Sync는 다른 노드와 상태를 동기화합니다.
	Sync(ctx context.Context, peerID string) error

	// GetStateVector는 현재 노드의 상태 벡터를 반환합니다.
	GetStateVector() map[string]uint64

	// GetMissingPatches는 주어진 상태 벡터에 따라 누락된 패치를 반환합니다.
	GetMissingPatches(ctx context.Context, stateVector map[string]uint64) ([]*crdtpatch.Patch, error)

	// Close는 싱커를 종료합니다.
	Close() error
}

// SyncManager는 CRDT 문서의 동기화를 관리합니다.
type SyncManager interface {
	// Start는 동기화 매니저를 시작합니다.
	Start(ctx context.Context) error

	// Stop은 동기화 매니저를 중지합니다.
	Stop() error

	// ApplyPatch는 패치를 적용하고 브로드캐스트합니다.
	ApplyPatch(ctx context.Context, patch *crdtpatch.Patch) error

	// SyncWithPeer는 특정 피어와 동기화합니다.
	SyncWithPeer(ctx context.Context, peerID string) error

	// GetDocument는 CRDT 문서를 반환합니다.
	GetDocument() *crdt.Document

	// GetStateVector는 현재 상태 벡터를 반환합니다.
	GetStateVector() map[string]uint64

	// GetPeerDiscovery는 피어 발견 기능을 반환합니다.
	GetPeerDiscovery() PeerDiscovery

	// SyncWithAllPeers는 모든 사용 가능한 피어와 동기화를 수행합니다.
	SyncWithAllPeers(ctx context.Context) error
}

// PeerDiscovery는 피어 발견을 위한 인터페이스입니다.
type PeerDiscovery interface {
	// DiscoverPeers는 사용 가능한 피어를 발견합니다.
	DiscoverPeers(ctx context.Context) ([]string, error)

	// RegisterPeer는 피어를 등록합니다.
	RegisterPeer(ctx context.Context, peerID string) error

	// UnregisterPeer는 피어 등록을 해제합니다.
	UnregisterPeer(ctx context.Context, peerID string) error

	// Close는 피어 발견 서비스를 종료합니다.
	Close() error
}

// PatchStore는 패치 저장소 인터페이스입니다.
type PatchStore interface {
	// StorePatch는 패치를 저장합니다.
	StorePatch(patch *crdtpatch.Patch) error

	// GetPatches는 주어진 상태 벡터 이후의 패치를 반환합니다.
	GetPatches(stateVector map[string]uint64) ([]*crdtpatch.Patch, error)

	// GetPatch는 특정 ID의 패치를 반환합니다.
	GetPatch(id common.LogicalTimestamp) (*crdtpatch.Patch, error)

	// Close는 패치 저장소를 종료합니다.
	Close() error
}
