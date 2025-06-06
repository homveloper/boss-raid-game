package crdtstorage

import (
	"context"
	"sync"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtpubsub"
	"tictactoe/luvjson/crdtsync"
)

// Storage는 CRDT 문서 저장소 인터페이스입니다.
// 이 인터페이스는 CRDT 문서의 생성, 로드, 저장, 동기화 등의 기능을 제공합니다.
type Storage interface {
	// CreateDocument는 새 문서를 생성합니다.
	CreateDocument(ctx context.Context, documentID string) (*Document, error)

	// GetDocument는 문서 ID로 문서를 가져옵니다.
	GetDocument(ctx context.Context, documentID string) (*Document, error)

	// ListDocuments는 모든 문서 목록을 반환합니다.
	ListDocuments(ctx context.Context) ([]string, error)

	// DeleteDocument는 문서를 삭제합니다.
	DeleteDocument(ctx context.Context, documentID string) error

	// SyncDocument는 특정 문서를 동기화합니다.
	// peerID가 비어 있으면 모든 피어와 동기화합니다.
	SyncDocument(ctx context.Context, documentID string, peerID string) error

	// SyncAllDocuments는 모든 문서를 동기화합니다.
	// peerID가 비어 있으면 모든 피어와 동기화합니다.
	SyncAllDocuments(ctx context.Context, peerID string) error

	// SaveDocument는 문서를 저장합니다.
	SaveDocument(ctx context.Context, doc *Document) error

	// CreateSnapshot은 문서의 스냅샷을 생성합니다.
	CreateSnapshot(ctx context.Context, doc *Document) (*DocumentSnapshot, error)

	// SaveSnapshot은 문서의 스냅샷을 저장합니다.
	SaveSnapshot(ctx context.Context, doc *Document) error

	// ListSnapshots은 문서의 모든 스냅샷 목록을 반환합니다.
	ListSnapshots(ctx context.Context, documentID string) ([]int64, error)

	// LoadSnapshot은 문서의 스냅샷을 로드합니다.
	LoadSnapshot(ctx context.Context, documentID string, version int64) (*DocumentSnapshot, error)

	// RestoreFromSnapshot은 스냅샷에서 문서를 복원합니다.
	RestoreFromSnapshot(ctx context.Context, documentID string, version int64) (interface{}, error)

	// DeleteSnapshot은 문서의 스냅샷을 삭제합니다.
	DeleteSnapshot(ctx context.Context, documentID string, version int64) error

	// DeleteAllSnapshots은 문서의 모든 스냅샷을 삭제합니다.
	DeleteAllSnapshots(ctx context.Context, documentID string) error

	// Close는 저장소를 닫습니다.
	Close() error
}

// Document는 CRDT 문서를 나타냅니다.
// 이 구조체는 CRDT 문서와 관련된 모든 기능을 제공합니다.
type Document struct {
	// ID는 문서의 고유 식별자입니다.
	ID string

	// CRDTDoc은 기본 CRDT 문서입니다.
	CRDTDoc *crdt.Document

	// PatchBuilder는 패치 생성을 위한 빌더입니다.
	PatchBuilder *crdtpatch.PatchBuilder

	// SyncManager는 문서 동기화를 관리합니다.
	SyncManager crdtsync.SyncManager

	// SessionID는 이 문서 인스턴스의 세션 ID입니다.
	SessionID common.SessionID

	// LastModified는 문서가 마지막으로 수정된 시간입니다.
	LastModified time.Time

	// Metadata는 문서 메타데이터입니다.
	Metadata map[string]interface{}

	// 저장소 의존성 제거

	// ctx는 문서의 컨텍스트입니다.
	ctx context.Context

	// cancel은 컨텍스트 취소 함수입니다.
	cancel context.CancelFunc

	// autoSave는 자동 저장 활성화 여부입니다.
	autoSave bool

	// autoSaveInterval은 자동 저장 간격입니다.
	autoSaveInterval time.Duration

	// onChangeCallbacks는 문서 변경 시 호출되는 콜백 함수 목록입니다.
	onChangeCallbacks []func(*Document, *crdtpatch.Patch)

	// mutex는 문서 편집 작업에 대한 동시 접근을 보호합니다.
	// 이를 통해 트랜잭션 일관성을 보장합니다.
	mutex sync.Mutex

	// lockManager는 문서에 대한 분산 락을 관리합니다.
	// 분산 환경에서 트랜잭션을 보장하는 데 사용됩니다.
	lockManager DistributedLockManager

	// transactionManager는 문서에 대한 트랜잭션을 관리합니다.
	// 분산 환경에서 트랜잭션을 추적하고 관리하는 데 사용됩니다.
	transactionManager TransactionManager

	// Version은 문서의 버전 번호입니다.
	// 낙관적 동시성 제어에 사용됩니다.
	Version int64

	// activeTransaction은 현재 진행 중인 트랜잭션 ID입니다.
	// 하나의 문서에 대해 한 번에 하나의 트랜잭션만 허용됩니다.
	activeTransaction string
}

// DocumentOptions는 문서 옵션을 나타냅니다.
type DocumentOptions struct {
	// AutoSave는 자동 저장 활성화 여부입니다.
	AutoSave bool

	// AutoSaveInterval은 자동 저장 간격입니다.
	AutoSaveInterval time.Duration

	// Metadata는 문서 메타데이터입니다.
	Metadata map[string]interface{}

	// OptimisticConcurrency는 낙관적 동시성 제어 활성화 여부입니다.
	// 분산 환경에서 충돌을 감지하고 해결하는 데 사용됩니다.
	OptimisticConcurrency bool

	// MaxTransactionRetries는 트랜잭션 재시도 횟수입니다.
	// 낙관적 동시성 제어를 활성화한 경우에만 사용됩니다.
	MaxTransactionRetries int

	// RequireDistributedLock은 분산 락 요구 여부입니다.
	// true인 경우 EditTransaction에서 분산 락을 사용합니다.
	RequireDistributedLock bool
}

// PubSubFactory는 PubSub 인스턴스를 생성하는 팩토리 함수입니다.
type PubSubFactory func(ctx context.Context, options *StorageOptions) (crdtpubsub.PubSub, error)

// PersistenceProvider는 영구 저장소 인터페이스입니다.
type PersistenceProvider interface {
	// GetDocumentKeyFunc는 문서 키 생성 함수를 반환합니다.
	GetDocumentKeyFunc() DocumentKeyFunc

	// SaveDocument는 문서를 저장합니다.
	// ctx: 컨텍스트
	// doc: 저장할 Document 객체
	// 사용자는 Document 객체의 모든 정보에 접근하여 필요에 맞게 데이터를 인덱싱하고 저장 쿼리를 작성할 수 있습니다.
	SaveDocument(ctx context.Context, doc *Document) error

	// LoadDocument는 문서를 로드합니다.
	// ctx: 컨텍스트
	// key: 문서 키
	// 문서의 직렬화된 데이터를 반환합니다.
	LoadDocument(ctx context.Context, key Key) ([]byte, error)

	// LoadDocumentByID는 문서 ID로 문서를 로드합니다.
	// ctx: 컨텍스트
	// documentID: 문서 ID
	// 문서의 직렬화된 데이터를 반환합니다.
	LoadDocumentByID(ctx context.Context, documentID string) ([]byte, error)

	// QueryDocuments는 쿼리에 맞는 문서를 검색합니다.
	// ctx: 컨텍스트
	// query: 쿼리 매개변수 (구현체에 따라 해석 방식이 다를 수 있음)
	// 문서 ID 목록을 반환합니다.
	QueryDocuments(ctx context.Context, query interface{}) ([]string, error)

	// ListDocuments는 모든 문서 목록을 반환합니다.
	// ctx: 컨텍스트
	// 문서 ID 목록을 반환합니다.
	ListDocuments(ctx context.Context) ([]string, error)

	// DeleteDocument는 문서를 삭제합니다.
	// ctx: 컨텍스트
	// key: 문서 키
	DeleteDocument(ctx context.Context, key Key) error

	// DeleteDocumentByID는 문서 ID로 문서를 삭제합니다.
	// ctx: 컨텍스트
	// documentID: 문서 ID
	DeleteDocumentByID(ctx context.Context, documentID string) error

	// Close는 영구 저장소를 닫습니다.
	Close() error
}

// EditFunc는 문서 편집 함수 타입입니다.
type EditFunc func(*crdt.Document, *crdtpatch.PatchBuilder) error

// SnapshotOptions는 스냅샷 옵션을 나타냅니다.
type SnapshotOptions struct {
	// Enabled는 스냅샷 활성화 여부입니다.
	Enabled bool

	// Interval은 스냅샷 생성 간격입니다.
	// 이 시간이 지나면 새로운 스냅샷이 생성됩니다.
	Interval time.Duration

	// MaxSnapshots는 유지할 최대 스냅샷 수입니다.
	// 이 값을 초과하면 가장 오래된 스냅샷이 삭제됩니다.
	MaxSnapshots int

	// SnapshotOnSave는 저장 시 스냅샷 생성 여부입니다.
	// true인 경우 문서가 저장될 때마다 스냅샷이 생성됩니다.
	SnapshotOnSave bool
}

// DocumentSnapshot은 문서 스냅샷을 나타냅니다.
type DocumentSnapshot struct {
	// DocumentID는 문서의 고유 식별자입니다.
	DocumentID string

	// Version은 스냅샷의 버전 번호입니다.
	Version int64

	// Timestamp는 스냅샷이 생성된 시간입니다.
	Timestamp time.Time

	// Data는 스냅샷 데이터입니다.
	// 이는 문서의 View() 메서드로 얻은 데이터입니다.
	Data interface{}

	// Metadata는 스냅샷 메타데이터입니다.
	Metadata map[string]interface{}
}

// SnapshotPersistenceProvider는 스냅샷 저장소 인터페이스입니다.
type SnapshotPersistenceProvider interface {
	// SaveSnapshot은 문서 스냅샷을 저장합니다.
	// ctx: 컨텍스트
	// snapshot: 저장할 스냅샷
	SaveSnapshot(ctx context.Context, snapshot *DocumentSnapshot) error

	// LoadSnapshot은 문서 스냅샷을 로드합니다.
	// ctx: 컨텍스트
	// documentID: 문서 ID
	// version: 스냅샷 버전 (0이면 최신 버전)
	// 스냅샷을 반환합니다.
	LoadSnapshot(ctx context.Context, documentID string, version int64) (*DocumentSnapshot, error)

	// ListSnapshots은 문서의 모든 스냅샷 목록을 반환합니다.
	// ctx: 컨텍스트
	// documentID: 문서 ID
	// 스냅샷 버전 목록을 반환합니다.
	ListSnapshots(ctx context.Context, documentID string) ([]int64, error)

	// DeleteSnapshot은 문서 스냅샷을 삭제합니다.
	// ctx: 컨텍스트
	// documentID: 문서 ID
	// version: 스냅샷 버전
	DeleteSnapshot(ctx context.Context, documentID string, version int64) error

	// DeleteAllSnapshots은 문서의 모든 스냅샷을 삭제합니다.
	// ctx: 컨텍스트
	// documentID: 문서 ID
	DeleteAllSnapshots(ctx context.Context, documentID string) error
}

// AdvancedPersistenceProvider는 작업과 스냅샷을 분리하여 저장하는 영구 저장소 인터페이스입니다.
type AdvancedPersistenceProvider interface {
	PersistenceProvider
	SnapshotPersistenceProvider

	// GetSnapshotOptions는 스냅샷 옵션을 반환합니다.
	GetSnapshotOptions() *SnapshotOptions

	// SetSnapshotOptions는 스냅샷 옵션을 설정합니다.
	SetSnapshotOptions(options *SnapshotOptions)

	// CreateSnapshot은 문서의 스냅샷을 생성합니다.
	// ctx: 컨텍스트
	// doc: 스냅샷을 생성할 문서
	// 생성된 스냅샷을 반환합니다.
	CreateSnapshot(ctx context.Context, doc *Document) (*DocumentSnapshot, error)

	// RestoreFromSnapshot은 스냅샷에서 문서를 복원합니다.
	// ctx: 컨텍스트
	// documentID: 문서 ID
	// version: 스냅샷 버전 (0이면 최신 버전)
	// 복원된 문서 데이터를 반환합니다.
	RestoreFromSnapshot(ctx context.Context, documentID string, version int64) (interface{}, error)
}
