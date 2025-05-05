package crdtstorage

import (
	"context"
	"time"

	"tictactoe/luvjson/crdtpatch"
)

// DocumentOperations는 문서 작업 인터페이스입니다.
// 이 인터페이스는 문서에 대한 작업을 정의합니다.
// Document와 Storage 간의 의존성을 줄이기 위해 사용됩니다.
type DocumentOperations interface {
	// SaveDocument는 문서를 저장합니다.
	SaveDocument(ctx context.Context, doc *Document) error

	// CreateSnapshot은 문서의 스냅샷을 생성합니다.
	CreateSnapshot(ctx context.Context, doc *Document) (*DocumentSnapshot, error)

	// SaveSnapshot은 문서의 스냅샷을 저장합니다.
	SaveSnapshot(ctx context.Context, snapshot *DocumentSnapshot) error

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
}

// DocumentEventListener는 문서 이벤트 리스너 인터페이스입니다.
// 이 인터페이스는 문서 이벤트를 수신하는 메서드를 정의합니다.
type DocumentEventListener interface {
	// OnDocumentChanged는 문서가 변경되었을 때 호출됩니다.
	OnDocumentChanged(doc *Document, patch *crdtpatch.Patch)

	// OnDocumentSaved는 문서가 저장되었을 때 호출됩니다.
	OnDocumentSaved(doc *Document)

	// OnDocumentClosed는 문서가 닫혔을 때 호출됩니다.
	OnDocumentClosed(doc *Document)
}

// DocumentEvent는 문서 이벤트 유형입니다.
type DocumentEvent int

const (
	// DocumentEventChanged는 문서가 변경되었음을 나타냅니다.
	DocumentEventChanged DocumentEvent = iota

	// DocumentEventSaved는 문서가 저장되었음을 나타냅니다.
	DocumentEventSaved

	// DocumentEventClosed는 문서가 닫혔음을 나타냅니다.
	DocumentEventClosed
)

// DocumentEventData는 문서 이벤트 데이터입니다.
type DocumentEventData struct {
	// Event는 이벤트 유형입니다.
	Event DocumentEvent

	// Document는 이벤트가 발생한 문서입니다.
	Document *Document

	// Patch는 문서 변경 패치입니다.
	// DocumentEventChanged 이벤트에서만 사용됩니다.
	Patch *crdtpatch.Patch

	// Timestamp는 이벤트가 발생한 시간입니다.
	Timestamp time.Time
}
