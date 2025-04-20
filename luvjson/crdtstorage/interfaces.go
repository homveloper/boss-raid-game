package crdtstorage

import (
	"context"
	"time"

	"tictactoe/luvjson/api"
	"tictactoe/luvjson/common"
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

	// Close는 저장소를 닫습니다.
	Close() error
}

// Document는 CRDT 문서를 나타냅니다.
// 이 구조체는 CRDT 문서와 관련된 모든 기능을 제공합니다.
type Document struct {
	// ID는 문서의 고유 식별자입니다.
	ID string

	// Model은 CRDT 문서의 API 모델입니다.
	Model *api.Model

	// SyncManager는 문서 동기화를 관리합니다.
	SyncManager crdtsync.SyncManager

	// SessionID는 이 문서 인스턴스의 세션 ID입니다.
	SessionID common.SessionID

	// LastModified는 문서가 마지막으로 수정된 시간입니다.
	LastModified time.Time

	// Metadata는 문서 메타데이터입니다.
	Metadata map[string]interface{}

	// storage는 이 문서가 속한 저장소입니다.
	storage Storage

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
}

// StorageOptions는 저장소 옵션을 나타냅니다.
type StorageOptions struct {
	// PubSubType은 사용할 PubSub 유형입니다.
	PubSubType string

	// RedisAddr은 Redis 서버 주소입니다.
	RedisAddr string

	// KeyPrefix는 Redis 키 접두사입니다.
	KeyPrefix string

	// SyncInterval은 동기화 간격입니다.
	SyncInterval time.Duration

	// AutoSave는 자동 저장 활성화 여부입니다.
	AutoSave bool

	// AutoSaveInterval은 자동 저장 간격입니다.
	AutoSaveInterval time.Duration

	// PersistenceType은 영구 저장소 유형입니다.
	PersistenceType string

	// PersistencePath는 영구 저장소 경로입니다.
	PersistencePath string
}

// DocumentOptions는 문서 옵션을 나타냅니다.
type DocumentOptions struct {
	// AutoSave는 자동 저장 활성화 여부입니다.
	AutoSave bool

	// AutoSaveInterval은 자동 저장 간격입니다.
	AutoSaveInterval time.Duration

	// Metadata는 문서 메타데이터입니다.
	Metadata map[string]interface{}
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

// EditResult는 편집 작업의 결과를 나타냅니다.
type EditResult struct {
	// Success는 편집 작업 성공 여부입니다.
	Success bool

	// Error는 편집 작업 중 발생한 오류입니다.
	Error error

	// Patch는 편집 작업으로 생성된 패치입니다.
	Patch *crdtpatch.Patch

	// Document는 편집된 문서입니다.
	Document *Document
}

// EditFunc는 문서 편집 함수 타입입니다.
type EditFunc func(*api.ModelApi) error
