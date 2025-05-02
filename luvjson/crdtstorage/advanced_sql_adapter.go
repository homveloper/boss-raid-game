package crdtstorage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// AdvancedSQLAdapter는 작업과 스냅샷을 분리하여 저장하는 SQL 데이터베이스 기반 영구 저장소 어댑터입니다.
type AdvancedSQLAdapter struct {
	// db는 SQL 데이터베이스 연결입니다.
	db *sql.DB

	// tableName은 문서가 저장될 테이블 이름입니다.
	tableName string

	// snapshotTableName은 스냅샷이 저장될 테이블 이름입니다.
	snapshotTableName string

	// mutex는 SQL 작업에 대한 동시 접근을 보호합니다.
	mutex sync.RWMutex

	// serializer는 문서 직렬화/역직렬화를 담당합니다.
	serializer DocumentSerializer

	// snapshotOptions는 스냅샷 옵션입니다.
	snapshotOptions *SnapshotOptions
}

// NewAdvancedSQLAdapter는 새 고급 SQL 어댑터를 생성합니다.
func NewAdvancedSQLAdapter(db *sql.DB, tableName string, snapshotTableName string) (*AdvancedSQLAdapter, error) {
	adapter := &AdvancedSQLAdapter{
		db:                db,
		tableName:         tableName,
		snapshotTableName: snapshotTableName,
		serializer:        NewDefaultDocumentSerializer(),
		snapshotOptions: &SnapshotOptions{
			Enabled:      true,
			Interval:     time.Hour,
			MaxSnapshots: 10,
		},
	}

	// 테이블 생성
	if err := adapter.createTables(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return adapter, nil
}

// createTables은 필요한 테이블을 생성합니다.
func (a *AdvancedSQLAdapter) createTables(ctx context.Context) error {
	// 문서 테이블 생성 쿼리
	docQuery := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			data BLOB NOT NULL,
			last_modified TIMESTAMP NOT NULL,
			metadata TEXT,
			version INTEGER NOT NULL DEFAULT 1
		)
	`, a.tableName)

	// 스냅샷 테이블 생성 쿼리
	snapshotQuery := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT NOT NULL,
			version INTEGER NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			data TEXT NOT NULL,
			metadata TEXT,
			PRIMARY KEY (id, version)
		)
	`, a.snapshotTableName)

	// 트랜잭션 시작
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 문서 테이블 생성
	_, err = tx.ExecContext(ctx, docQuery)
	if err != nil {
		return fmt.Errorf("failed to create document table: %w", err)
	}

	// 스냅샷 테이블 생성
	_, err = tx.ExecContext(ctx, snapshotQuery)
	if err != nil {
		return fmt.Errorf("failed to create snapshot table: %w", err)
	}

	// 트랜잭션 커밋
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SaveDocument는 문서를 SQL 데이터베이스에 저장합니다.
func (a *AdvancedSQLAdapter) SaveDocument(ctx context.Context, doc *Document) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 문서 직렬화
	data, err := a.serializer.Serialize(doc)
	if err != nil {
		return fmt.Errorf("failed to serialize document: %w", err)
	}

	// 트랜잭션 시작
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 문서 존재 여부 확인
	var exists bool
	var version int64
	err = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT 1, version FROM %s WHERE id = ?", a.tableName), doc.ID).Scan(&exists, &version)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check document existence: %w", err)
	}

	// 문서 저장
	if err == sql.ErrNoRows {
		// 새 문서 삽입
		_, err = tx.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (id, data, last_modified, version) VALUES (?, ?, ?, ?)", a.tableName),
			doc.ID, data, doc.LastModified, 1)
		version = 1
	} else {
		// 버전 증가
		version++
		// 기존 문서 업데이트
		_, err = tx.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET data = ?, last_modified = ?, version = ? WHERE id = ?", a.tableName),
			data, doc.LastModified, version, doc.ID)
	}
	if err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}

	// 문서 버전 업데이트
	doc.Version = version

	// 스냅샷 생성 (필요한 경우)
	if a.snapshotOptions.Enabled && a.snapshotOptions.SnapshotOnSave {
		snapshot, err := a.CreateSnapshot(ctx, doc)
		if err != nil {
			return fmt.Errorf("failed to create snapshot: %w", err)
		}

		// 스냅샷 저장
		err = a.saveSnapshotInTransaction(ctx, tx, snapshot)
		if err != nil {
			return fmt.Errorf("failed to save snapshot: %w", err)
		}

		// 오래된 스냅샷 정리
		if a.snapshotOptions.MaxSnapshots > 0 {
			err = a.cleanupOldSnapshotsInTransaction(ctx, tx, doc.ID)
			if err != nil {
				return fmt.Errorf("failed to cleanup old snapshots: %w", err)
			}
		}
	}

	// 트랜잭션 커밋
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// LoadDocument는 문서를 SQL 데이터베이스에서 로드합니다.
func (a *AdvancedSQLAdapter) LoadDocument(ctx context.Context, key Key) ([]byte, error) {
	documentID := key.(string)
	return a.LoadDocumentByID(ctx, documentID)
}

// LoadDocumentByID는 문서 ID로 문서를 SQL 데이터베이스에서 로드합니다.
func (a *AdvancedSQLAdapter) LoadDocumentByID(ctx context.Context, documentID string) ([]byte, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 문서 가져오기
	var data []byte
	var version int64
	err := a.db.QueryRowContext(ctx, fmt.Sprintf("SELECT data, version FROM %s WHERE id = ?", a.tableName), documentID).Scan(&data, &version)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("document not found: %s", documentID)
		}
		return nil, fmt.Errorf("failed to load document: %w", err)
	}

	return data, nil
}

// ListDocuments는 모든 문서 목록을 반환합니다.
func (a *AdvancedSQLAdapter) ListDocuments(ctx context.Context) ([]string, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 문서 ID 목록 가져오기
	rows, err := a.db.QueryContext(ctx, fmt.Sprintf("SELECT id FROM %s", a.tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to query documents: %w", err)
	}
	defer rows.Close()

	// 문서 ID 목록 생성
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan document ID: %w", err)
		}
		ids = append(ids, id)
	}

	// 오류 확인
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating document rows: %w", err)
	}

	return ids, nil
}

// QueryDocuments는 쿼리에 맞는 문서를 검색합니다.
func (a *AdvancedSQLAdapter) QueryDocuments(ctx context.Context, query interface{}) ([]string, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 쿼리 타입 확인
	switch q := query.(type) {
	case string:
		// SQL 쿼리 문자열인 경우
		rows, err := a.db.QueryContext(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}
		defer rows.Close()

		// 문서 ID 목록 생성
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, fmt.Errorf("failed to scan document ID: %w", err)
			}
			ids = append(ids, id)
		}

		// 오류 확인
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating query rows: %w", err)
		}

		return ids, nil
	case map[string]interface{}:
		// 필터 맵인 경우
		// 간단한 WHERE 절 생성
		whereClause := ""
		var args []interface{}
		for key, value := range q {
			if whereClause != "" {
				whereClause += " AND "
			}
			whereClause += fmt.Sprintf("%s = ?", key)
			args = append(args, value)
		}

		// 쿼리 실행
		query := fmt.Sprintf("SELECT id FROM %s", a.tableName)
		if whereClause != "" {
			query += " WHERE " + whereClause
		}

		rows, err := a.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}
		defer rows.Close()

		// 문서 ID 목록 생성
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, fmt.Errorf("failed to scan document ID: %w", err)
			}
			ids = append(ids, id)
		}

		// 오류 확인
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating query rows: %w", err)
		}

		return ids, nil
	default:
		return nil, fmt.Errorf("unsupported query type: %T", query)
	}
}

// DeleteDocument는 문서를 SQL 데이터베이스에서 삭제합니다.
func (a *AdvancedSQLAdapter) DeleteDocument(ctx context.Context, key Key) error {
	documentID := key.(string)
	return a.DeleteDocumentByID(ctx, documentID)
}

// DeleteDocumentByID는 문서 ID로 문서를 SQL 데이터베이스에서 삭제합니다.
func (a *AdvancedSQLAdapter) DeleteDocumentByID(ctx context.Context, documentID string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 트랜잭션 시작
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 문서 삭제
	_, err = tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ?", a.tableName), documentID)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	// 스냅샷 삭제
	_, err = tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ?", a.snapshotTableName), documentID)
	if err != nil {
		return fmt.Errorf("failed to delete snapshots: %w", err)
	}

	// 트랜잭션 커밋
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Close는 SQL 어댑터를 닫습니다.
func (a *AdvancedSQLAdapter) Close() error {
	// SQL 데이터베이스 연결은 외부에서 관리하므로 여기서 닫지 않음
	return nil
}

// GetDocumentKeyFunc는 문서 키 생성 함수를 반환합니다.
func (a *AdvancedSQLAdapter) GetDocumentKeyFunc() DocumentKeyFunc {
	return func(documentID string) Key {
		return documentID
	}
}

// SaveSnapshot은 문서 스냅샷을 저장합니다.
func (a *AdvancedSQLAdapter) SaveSnapshot(ctx context.Context, snapshot *DocumentSnapshot) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 트랜잭션 시작
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 스냅샷 저장
	err = a.saveSnapshotInTransaction(ctx, tx, snapshot)
	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	// 트랜잭션 커밋
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// saveSnapshotInTransaction은 트랜잭션 내에서 스냅샷을 저장합니다.
func (a *AdvancedSQLAdapter) saveSnapshotInTransaction(ctx context.Context, tx *sql.Tx, snapshot *DocumentSnapshot) error {
	// 스냅샷 데이터 직렬화
	dataJSON, err := json.Marshal(snapshot.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot data: %w", err)
	}

	// 메타데이터 직렬화
	metadataJSON, err := json.Marshal(snapshot.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot metadata: %w", err)
	}

	// 스냅샷 저장
	_, err = tx.ExecContext(ctx, fmt.Sprintf("INSERT OR REPLACE INTO %s (id, version, timestamp, data, metadata) VALUES (?, ?, ?, ?, ?)", a.snapshotTableName),
		snapshot.DocumentID, snapshot.Version, snapshot.Timestamp, string(dataJSON), string(metadataJSON))
	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	return nil
}

// LoadSnapshot은 문서 스냅샷을 로드합니다.
func (a *AdvancedSQLAdapter) LoadSnapshot(ctx context.Context, documentID string, version int64) (*DocumentSnapshot, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 쿼리 생성
	query := fmt.Sprintf("SELECT version, timestamp, data, metadata FROM %s WHERE id = ?", a.snapshotTableName)
	args := []interface{}{documentID}

	// 특정 버전 요청인 경우
	if version > 0 {
		query += " AND version = ?"
		args = append(args, version)
	} else {
		// 최신 버전 요청인 경우
		query += " ORDER BY version DESC LIMIT 1"
	}

	// 스냅샷 가져오기
	var snapshotVersion int64
	var timestamp time.Time
	var dataJSON string
	var metadataJSON string
	err := a.db.QueryRowContext(ctx, query, args...).Scan(&snapshotVersion, &timestamp, &dataJSON, &metadataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("snapshot not found for document: %s, version: %d", documentID, version)
		}
		return nil, fmt.Errorf("failed to load snapshot: %w", err)
	}

	// 데이터 역직렬화
	var data interface{}
	if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot data: %w", err)
	}

	// 메타데이터 역직렬화
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot metadata: %w", err)
	}

	// 스냅샷 생성
	snapshot := &DocumentSnapshot{
		DocumentID: documentID,
		Version:    snapshotVersion,
		Timestamp:  timestamp,
		Data:       data,
		Metadata:   metadata,
	}

	return snapshot, nil
}

// ListSnapshots은 문서의 모든 스냅샷 목록을 반환합니다.
func (a *AdvancedSQLAdapter) ListSnapshots(ctx context.Context, documentID string) ([]int64, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 스냅샷 버전 목록 가져오기
	rows, err := a.db.QueryContext(ctx, fmt.Sprintf("SELECT version FROM %s WHERE id = ? ORDER BY version DESC", a.snapshotTableName), documentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	// 스냅샷 버전 목록 생성
	var versions []int64
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan snapshot version: %w", err)
		}
		versions = append(versions, version)
	}

	// 오류 확인
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating snapshot rows: %w", err)
	}

	return versions, nil
}

// DeleteSnapshot은 문서 스냅샷을 삭제합니다.
func (a *AdvancedSQLAdapter) DeleteSnapshot(ctx context.Context, documentID string, version int64) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 스냅샷 삭제
	_, err := a.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ? AND version = ?", a.snapshotTableName), documentID, version)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	return nil
}

// DeleteAllSnapshots은 문서의 모든 스냅샷을 삭제합니다.
func (a *AdvancedSQLAdapter) DeleteAllSnapshots(ctx context.Context, documentID string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 모든 스냅샷 삭제
	_, err := a.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ?", a.snapshotTableName), documentID)
	if err != nil {
		return fmt.Errorf("failed to delete all snapshots: %w", err)
	}

	return nil
}

// cleanupOldSnapshotsInTransaction은 트랜잭션 내에서 오래된 스냅샷을 정리합니다.
func (a *AdvancedSQLAdapter) cleanupOldSnapshotsInTransaction(ctx context.Context, tx *sql.Tx, documentID string) error {
	// 현재 스냅샷 수 확인
	var count int
	err := tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id = ?", a.snapshotTableName), documentID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count snapshots: %w", err)
	}

	// 최대 스냅샷 수를 초과하는 경우 오래된 스냅샷 삭제
	if count > a.snapshotOptions.MaxSnapshots {
		// 삭제할 스냅샷 수
		deleteCount := count - a.snapshotOptions.MaxSnapshots

		// 오래된 스냅샷 삭제
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			DELETE FROM %s
			WHERE id = ? AND version IN (
				SELECT version FROM %s
				WHERE id = ?
				ORDER BY version ASC
				LIMIT ?
			)
		`, a.snapshotTableName, a.snapshotTableName), documentID, documentID, deleteCount)
		if err != nil {
			return fmt.Errorf("failed to delete old snapshots: %w", err)
		}
	}

	return nil
}

// GetSnapshotOptions는 스냅샷 옵션을 반환합니다.
func (a *AdvancedSQLAdapter) GetSnapshotOptions() *SnapshotOptions {
	return a.snapshotOptions
}

// SetSnapshotOptions는 스냅샷 옵션을 설정합니다.
func (a *AdvancedSQLAdapter) SetSnapshotOptions(options *SnapshotOptions) {
	a.snapshotOptions = options
}

// CreateSnapshot은 문서의 스냅샷을 생성합니다.
func (a *AdvancedSQLAdapter) CreateSnapshot(ctx context.Context, doc *Document) (*DocumentSnapshot, error) {
	// 문서 내용 가져오기
	content, err := doc.GetContent()
	if err != nil {
		return nil, fmt.Errorf("failed to get document content: %w", err)
	}

	// 스냅샷 생성
	snapshot := &DocumentSnapshot{
		DocumentID: doc.ID,
		Version:    doc.Version,
		Timestamp:  time.Now(),
		Data:       content,
		Metadata:   doc.Metadata,
	}

	return snapshot, nil
}

// RestoreFromSnapshot은 스냅샷에서 문서를 복원합니다.
func (a *AdvancedSQLAdapter) RestoreFromSnapshot(ctx context.Context, documentID string, version int64) (interface{}, error) {
	// 스냅샷 로드
	snapshot, err := a.LoadSnapshot(ctx, documentID, version)
	if err != nil {
		return nil, fmt.Errorf("failed to load snapshot: %w", err)
	}

	return snapshot.Data, nil
}
