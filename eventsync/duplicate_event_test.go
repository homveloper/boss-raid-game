package eventsync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestStorageListener_DuplicateEventHandling은 중복 이벤트 처리를 테스트합니다.
func TestStorageListener_DuplicateEventHandling(t *testing.T) {
	// 테스트 설정
	ctx := context.Background()
	logger := zap.NewExample()
	defer logger.Sync()

	// MongoDB 설정
	client, db, cleanup := setupTestDB(t)
	defer cleanup()

	// 이벤트 스토어 설정
	eventStore, err := NewMongoEventStore(ctx, client, db.Name(), "events", logger)
	require.NoError(t, err)

	// 상태 벡터 관리자 설정
	stateVectorManager, err := NewMongoStateVectorManager(ctx, client, db.Name(), "state_vectors", eventStore, logger)
	require.NoError(t, err)

	// 동기화 서비스 설정
	syncService := NewSyncService(eventStore, stateVectorManager, logger)

	// 스토리지 설정
	collection := db.Collection("documents")
	storage, err := setupStorage(t, collection)
	require.NoError(t, err)

	// 스토리지 어댑터 설정
	adapter := NewStorageAdapter[*TestDocument](storage, logger)

	// 스토리지 리스너 설정
	listener := NewStorageListener(adapter, syncService, logger)

	// 리스너 시작
	err = listener.Start()
	require.NoError(t, err)
	defer listener.Stop()

	// 테스트 문서 생성
	doc := &TestDocument{
		Name:    "Test Document",
		Value:   100,
		Tags:    []string{"test", "duplicate"},
		Created: time.Now(),
		Updated: time.Now(),
		Version: 1,
	}

	// 문서 저장
	createdDoc, err := storage.FindOneAndUpsert(ctx, doc)
	require.NoError(t, err)

	// 이벤트 생성 확인 (약간의 지연 허용)
	time.Sleep(500 * time.Millisecond)

	// 이벤트 조회
	events, err := eventStore.GetEvents(ctx, createdDoc.ID, 0)
	require.NoError(t, err)
	require.NotEmpty(t, events)

	// 이벤트 수 확인
	assert.Len(t, events, 1, "문서 생성 후 이벤트는 1개여야 합니다")

	// 동일한 문서를 다시 저장 (중복 이벤트 발생 시도)
	_, err = storage.FindOneAndUpsert(ctx, createdDoc)
	require.NoError(t, err)

	// 이벤트 생성 확인 (약간의 지연 허용)
	time.Sleep(500 * time.Millisecond)

	// 이벤트 조회
	events, err = eventStore.GetEvents(ctx, createdDoc.ID, 0)
	require.NoError(t, err)

	// 이벤트 수 확인 (중복 이벤트가 필터링되어야 함)
	assert.Len(t, events, 1, "중복 이벤트가 필터링되어 이벤트는 1개여야 합니다")

	// 문서 업데이트
	updatedDoc, _, err := storage.FindOneAndUpdate(ctx, createdDoc.ID, func(d *TestDocument) (*TestDocument, error) {
		d.Name = "Updated Document"
		d.Value = 200
		d.Version++
		return d, nil
	})
	require.NoError(t, err)

	// 이벤트 생성 확인 (약간의 지연 허용)
	time.Sleep(500 * time.Millisecond)

	// 이벤트 조회
	events, err = eventStore.GetEvents(ctx, updatedDoc.ID, 0)
	require.NoError(t, err)

	// 이벤트 수 확인
	assert.Len(t, events, 2, "문서 업데이트 후 이벤트는 2개여야 합니다")

	// 문서 삭제
	err = storage.DeleteOne(ctx, updatedDoc.ID)
	require.NoError(t, err)

	// 이벤트 생성 확인 (약간의 지연 허용)
	time.Sleep(500 * time.Millisecond)

	// 이벤트 조회
	events, err = eventStore.GetEvents(ctx, updatedDoc.ID, 0)
	require.NoError(t, err)

	// 이벤트 수 확인
	assert.Len(t, events, 3, "문서 삭제 후 이벤트는 3개여야 합니다")
}
