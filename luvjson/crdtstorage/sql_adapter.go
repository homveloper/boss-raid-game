package crdtstorage

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

// SQLAdapter는 SQL 데이터베이스 기반 영구 저장소 어댑터입니다.
type SQLAdapter struct {
	// db는 SQL 데이터베이스 연결입니다.
	db *sql.DB

	// tableName은 문서가 저장될 테이블 이름입니다.
	tableName string

	// mutex는 SQL 작업에 대한 동시 접근을 보호합니다.
	mutex sync.RWMutex

	// serializer는 문서 직렬화/역직렬화를 담당합니다.
	serializer DocumentSerializer
}

// NewSQLAdapter는 새 SQL 어댑터를 생성합니다.
func NewSQLAdapter(db *sql.DB, tableName string) (*SQLAdapter, error) {
	adapter := &SQLAdapter{
		db:         db,
		tableName:  tableName,
		serializer: NewDefaultDocumentSerializer(),
	}

	// 테이블 생성
	if err := adapter.createTable(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return adapter, nil
}

// createTable은 필요한 테이블을 생성합니다.
func (a *SQLAdapter) createTable(ctx context.Context) error {
	// 테이블 생성 쿼리
	// 데이터베이스 종류에 따라 쿼리가 달라질 수 있음
	// 여기서는 SQLite 기준으로 작성
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id TEXT PRIMARY KEY,
			data BLOB NOT NULL,
			last_modified TIMESTAMP NOT NULL,
			metadata TEXT
		)
	`, a.tableName)

	// 쿼리 실행
	_, err := a.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	return nil
}

// SaveDocument는 문서를 SQL 데이터베이스에 저장합니다.
func (a *SQLAdapter) SaveDocument(ctx context.Context, doc *Document) error {
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
	err = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT 1 FROM %s WHERE id = ?", a.tableName), doc.ID).Scan(&exists)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check document existence: %w", err)
	}

	// 문서 저장
	if err == sql.ErrNoRows {
		// 새 문서 삽입
		_, err = tx.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (id, data, last_modified) VALUES (?, ?, ?)", a.tableName),
			doc.ID, data, doc.LastModified)
	} else {
		// 기존 문서 업데이트
		_, err = tx.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET data = ?, last_modified = ? WHERE id = ?", a.tableName),
			data, doc.LastModified, doc.ID)
	}
	if err != nil {
		return fmt.Errorf("failed to save document: %w", err)
	}

	// 트랜잭션 커밋
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// LoadDocument는 문서를 SQL 데이터베이스에서 로드합니다.
func (a *SQLAdapter) LoadDocument(ctx context.Context, documentID string) ([]byte, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 문서 가져오기
	var data []byte
	err := a.db.QueryRowContext(ctx, fmt.Sprintf("SELECT data FROM %s WHERE id = ?", a.tableName), documentID).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("document not found: %s", documentID)
		}
		return nil, fmt.Errorf("failed to load document: %w", err)
	}

	return data, nil
}

// ListDocuments는 모든 문서 목록을 반환합니다.
func (a *SQLAdapter) ListDocuments(ctx context.Context) ([]string, error) {
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

// DeleteDocument는 문서를 SQL 데이터베이스에서 삭제합니다.
func (a *SQLAdapter) DeleteDocument(ctx context.Context, documentID string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 문서 삭제
	_, err := a.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE id = ?", a.tableName), documentID)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	return nil
}

// Close는 SQL 어댑터를 닫습니다.
func (a *SQLAdapter) Close() error {
	// SQL 데이터베이스 연결은 외부에서 관리하므로 여기서 닫지 않음
	return nil
}
