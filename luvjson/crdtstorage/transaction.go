package crdtstorage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"tictactoe/luvjson/crdtpatch"

	"github.com/google/uuid"
)

// TransactionStatus는 트랜잭션 상태를 나타냅니다.
type TransactionStatus string

const (
	// TransactionStatusPending은 진행 중인 트랜잭션 상태입니다.
	TransactionStatusPending TransactionStatus = "pending"

	// TransactionStatusCommitted는 커밋된 트랜잭션 상태입니다.
	TransactionStatusCommitted TransactionStatus = "committed"

	// TransactionStatusAborted는 중단된 트랜잭션 상태입니다.
	TransactionStatusAborted TransactionStatus = "aborted"
)

// TransactionMarker는 트랜잭션 마커를 나타냅니다.
// 이 구조체는 트랜잭션의 시작, 커밋, 중단 등을 표시하는 데 사용됩니다.
type TransactionMarker struct {
	// ID는 트랜잭션의 고유 식별자입니다.
	ID string `json:"id"`

	// Type은 마커 유형입니다 (start, commit, abort).
	Type string `json:"type"`

	// Timestamp는 마커가 생성된 시간입니다.
	Timestamp time.Time `json:"timestamp"`

	// DocumentID는 트랜잭션이 적용된 문서의 ID입니다.
	DocumentID string `json:"documentId"`

	// SessionID는 트랜잭션을 시작한 세션의 ID입니다.
	SessionID string `json:"sessionId"`
}

// TransactionManager는 트랜잭션 관리자 인터페이스입니다.
// 이 인터페이스는 트랜잭션을 시작, 커밋, 중단하는 기능을 제공합니다.
type TransactionManager interface {
	// BeginTransaction은 새 트랜잭션을 시작합니다.
	// 트랜잭션 ID를 반환합니다.
	BeginTransaction(ctx context.Context, documentID string, sessionID string) (string, error)

	// CommitTransaction은 트랜잭션을 커밋합니다.
	// 성공하면 true, 실패하면 false를 반환합니다.
	CommitTransaction(ctx context.Context, transactionID string) (bool, error)

	// AbortTransaction은 트랜잭션을 중단합니다.
	// 성공하면 true, 실패하면 false를 반환합니다.
	AbortTransaction(ctx context.Context, transactionID string) (bool, error)

	// GetTransactionStatus는 트랜잭션 상태를 반환합니다.
	GetTransactionStatus(ctx context.Context, transactionID string) (TransactionStatus, error)

	// Close는 트랜잭션 관리자를 닫습니다.
	Close() error
}

// RedisTransactionManager는 Redis 기반 트랜잭션 관리자 구현체입니다.
type RedisTransactionManager struct {
	// client는 Redis 클라이언트입니다.
	client RedisClient

	// lockManager는 분산 락 관리자입니다.
	lockManager DistributedLockManager

	// keyPrefix는 Redis 키 접두사입니다.
	keyPrefix string
}

// NewRedisTransactionManager는 새 Redis 트랜잭션 관리자를 생성합니다.
func NewRedisTransactionManager(client RedisClient, lockManager DistributedLockManager, keyPrefix string) *RedisTransactionManager {
	return &RedisTransactionManager{
		client:      client,
		lockManager: lockManager,
		keyPrefix:   keyPrefix,
	}
}

// getTransactionKey는 트랜잭션 키를 반환합니다.
func (m *RedisTransactionManager) getTransactionKey(transactionID string) string {
	return fmt.Sprintf("%s:tx:%s", m.keyPrefix, transactionID)
}

// BeginTransaction은 새 트랜잭션을 시작합니다.
func (m *RedisTransactionManager) BeginTransaction(ctx context.Context, documentID string, sessionID string) (string, error) {
	// 트랜잭션 ID 생성
	transactionID := uuid.New().String()

	// 트랜잭션 마커 생성
	marker := &TransactionMarker{
		ID:         transactionID,
		Type:       "start",
		Timestamp:  time.Now(),
		DocumentID: documentID,
		SessionID:  sessionID,
	}

	// 마커 직렬화
	markerBytes, err := json.Marshal(marker)
	if err != nil {
		return "", fmt.Errorf("failed to marshal transaction marker: %w", err)
	}

	// 트랜잭션 저장
	txKey := m.getTransactionKey(transactionID)
	success, err := m.client.SetNX(ctx, txKey, string(markerBytes), 30*time.Minute)
	if err != nil {
		return "", fmt.Errorf("failed to save transaction: %w", err)
	}

	if !success {
		return "", fmt.Errorf("transaction key already exists: %s", txKey)
	}

	return transactionID, nil
}

// CommitTransaction은 트랜잭션을 커밋합니다.
func (m *RedisTransactionManager) CommitTransaction(ctx context.Context, transactionID string) (bool, error) {
	// 트랜잭션 키 가져오기
	txKey := m.getTransactionKey(transactionID)

	// 트랜잭션 상태 확인
	status, err := m.GetTransactionStatus(ctx, transactionID)
	if err != nil {
		return false, fmt.Errorf("failed to get transaction status: %w", err)
	}

	// 이미 커밋되었거나 중단된 경우
	if status != TransactionStatusPending {
		return false, fmt.Errorf("transaction is not pending: %s", status)
	}

	// 트랜잭션 마커 가져오기
	markerJSON, err := m.client.Get(ctx, txKey)
	if err != nil {
		return false, fmt.Errorf("failed to get transaction marker: %w", err)
	}

	var marker TransactionMarker
	if err := json.Unmarshal([]byte(markerJSON), &marker); err != nil {
		return false, fmt.Errorf("failed to unmarshal transaction marker: %w", err)
	}

	// 커밋 마커로 업데이트
	marker.Type = "commit"
	marker.Timestamp = time.Now()

	// 마커 직렬화
	markerBytes, err := json.Marshal(marker)
	if err != nil {
		return false, fmt.Errorf("failed to marshal transaction marker: %w", err)
	}

	// 트랜잭션 업데이트
	err = m.client.Set(ctx, txKey, string(markerBytes), 30*time.Minute)
	if err != nil {
		return false, fmt.Errorf("failed to update transaction: %w", err)
	}

	return true, nil
}

// AbortTransaction은 트랜잭션을 중단합니다.
func (m *RedisTransactionManager) AbortTransaction(ctx context.Context, transactionID string) (bool, error) {
	// 트랜잭션 키 가져오기
	txKey := m.getTransactionKey(transactionID)

	// 트랜잭션 상태 확인
	status, err := m.GetTransactionStatus(ctx, transactionID)
	if err != nil {
		return false, fmt.Errorf("failed to get transaction status: %w", err)
	}

	// 이미 커밋되었거나 중단된 경우
	if status != TransactionStatusPending {
		return false, fmt.Errorf("transaction is not pending: %s", status)
	}

	// 트랜잭션 마커 가져오기
	markerJSON, err := m.client.Get(ctx, txKey)
	if err != nil {
		return false, fmt.Errorf("failed to get transaction marker: %w", err)
	}

	var marker TransactionMarker
	if err := json.Unmarshal([]byte(markerJSON), &marker); err != nil {
		return false, fmt.Errorf("failed to unmarshal transaction marker: %w", err)
	}

	// 중단 마커로 업데이트
	marker.Type = "abort"
	marker.Timestamp = time.Now()

	// 마커 직렬화
	markerBytes, err := json.Marshal(marker)
	if err != nil {
		return false, fmt.Errorf("failed to marshal transaction marker: %w", err)
	}

	// 트랜잭션 업데이트
	err = m.client.Set(ctx, txKey, string(markerBytes), 30*time.Minute)
	if err != nil {
		return false, fmt.Errorf("failed to update transaction: %w", err)
	}

	return true, nil
}

// GetTransactionStatus는 트랜잭션 상태를 반환합니다.
func (m *RedisTransactionManager) GetTransactionStatus(ctx context.Context, transactionID string) (TransactionStatus, error) {
	// 트랜잭션 키 가져오기
	txKey := m.getTransactionKey(transactionID)

	// 트랜잭션 마커 가져오기
	markerJSON, err := m.client.Get(ctx, txKey)
	if err != nil {
		return "", fmt.Errorf("failed to get transaction marker: %w", err)
	}

	var marker TransactionMarker
	if err := json.Unmarshal([]byte(markerJSON), &marker); err != nil {
		return "", fmt.Errorf("failed to unmarshal transaction marker: %w", err)
	}

	// 마커 유형에 따라 상태 반환
	switch marker.Type {
	case "start":
		return TransactionStatusPending, nil
	case "commit":
		return TransactionStatusCommitted, nil
	case "abort":
		return TransactionStatusAborted, nil
	default:
		return "", fmt.Errorf("unknown marker type: %s", marker.Type)
	}
}

// Close는 트랜잭션 관리자를 닫습니다.
func (m *RedisTransactionManager) Close() error {
	// Redis 클라이언트는 외부에서 관리되므로 여기서 닫지 않음
	return m.lockManager.Close()
}

// NoOpTransactionManager는 아무 작업도 수행하지 않는 트랜잭션 관리자 구현체입니다.
// 테스트나 단일 노드 환경에서 사용할 수 있습니다.
type NoOpTransactionManager struct{}

// NewNoOpTransactionManager는 새 NoOp 트랜잭션 관리자를 생성합니다.
func NewNoOpTransactionManager() *NoOpTransactionManager {
	return &NoOpTransactionManager{}
}

// BeginTransaction은 새 트랜잭션을 시작합니다.
func (m *NoOpTransactionManager) BeginTransaction(ctx context.Context, documentID string, sessionID string) (string, error) {
	return uuid.New().String(), nil
}

// CommitTransaction은 트랜잭션을 커밋합니다.
func (m *NoOpTransactionManager) CommitTransaction(ctx context.Context, transactionID string) (bool, error) {
	return true, nil
}

// AbortTransaction은 트랜잭션을 중단합니다.
func (m *NoOpTransactionManager) AbortTransaction(ctx context.Context, transactionID string) (bool, error) {
	return true, nil
}

// GetTransactionStatus는 트랜잭션 상태를 반환합니다.
func (m *NoOpTransactionManager) GetTransactionStatus(ctx context.Context, transactionID string) (TransactionStatus, error) {
	return TransactionStatusCommitted, nil
}

// Close는 트랜잭션 관리자를 닫습니다.
func (m *NoOpTransactionManager) Close() error {
	return nil
}

// TransactionResult는 트랜잭션 결과를 나타냅니다.
type TransactionResult struct {
	// Success는 트랜잭션 성공 여부입니다.
	Success bool

	// Error는 트랜잭션 중 발생한 오류입니다.
	Error error

	// Patch는 트랜잭션으로 생성된 패치입니다.
	Patch *crdtpatch.Patch

	// Document는 트랜잭션이 적용된 문서입니다.
	Document *Document

	// TransactionID는 트랜잭션의 고유 식별자입니다.
	TransactionID string
}

// createTransactionPatch는 트랜잭션 마커에 대한 패치를 생성합니다.
func createTransactionPatch(marker *TransactionMarker) *crdtpatch.Patch {
	// 마커 직렬화
	markerJSON, _ := json.Marshal(marker)

	// 패치 생성
	// 임시 패치 생성
	patch := &crdtpatch.Patch{}

	// 메타데이터 설정
	metadata := make(map[string]interface{})
	metadata["transactionMarker"] = string(markerJSON)
	metadata["markerType"] = marker.Type
	metadata["transactionId"] = marker.ID
	patch.SetMetadata(metadata)

	return patch
}
