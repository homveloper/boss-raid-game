package eventsync

import (
	"context"
	"eventsync/testutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"nodestorage/v2"
	"nodestorage/v2/cache"
)

// 참고: TestDocument 구조체는 event_store_test.go에 정의되어 있습니다.

// TestStorageListener_Integration은 StorageListener와 nodestorage 통합을 테스트합니다.
func TestStorageListener_Integration(t *testing.T) {
	// 테스트 환경 설정
	client, db, cleanup := setupTestDB(t)
	defer cleanup()

	// 로거 설정
	logger := testutil.NewLogger()
	defer logger.Sync()

	// 컬렉션 설정
	ctx := context.Background()
	collection := db.Collection("documents")

	// 캐시 설정
	cacheStorage := cache.NewMemoryCache[*TestDocument](nil)

	// nodestorage 설정
	storageOptions := &nodestorage.Options{
		VersionField:      "Version",
		WatchEnabled:      true,
		WatchFullDocument: "updateLookup",
	}
	storage, err := nodestorage.NewStorage[*TestDocument](ctx, collection, cacheStorage, storageOptions)
	require.NoError(t, err)

	// 이벤트 저장소 설정
	eventStore, err := NewMongoEventStore(ctx, client, db.Name(), "events", logger)
	require.NoError(t, err)

	// 상태 벡터 관리자 설정
	stateVectorManager, err := NewMongoStateVectorManager(ctx, client, db.Name(), "state_vectors", eventStore, logger)
	require.NoError(t, err)

	// 동기화 서비스 설정
	syncService := NewSyncService(eventStore, stateVectorManager, logger)

	// 스토리지 어댑터 설정
	storageAdapter := NewStorageAdapter[*TestDocument](storage, logger)

	// 스토리지 리스너 설정 및 시작
	storageListener := NewStorageListener(storageAdapter, syncService, logger)
	err = storageListener.Start()
	require.NoError(t, err)
	defer storageListener.Stop()

	// 테스트 문서 생성
	doc := &TestDocument{
		ID:        primitive.NewObjectID(),
		Name:      "Test Document",
		Value:     100,
		Tags:      []string{"test", "integration"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
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

	// 이벤트 상세 로깅
	t.Logf("문서 생성 후 이벤트 수: %d (예상: 1개)", len(events))
	for i, event := range events {
		t.Logf("이벤트[%d]: ID=%s, 문서ID=%s, 작업=%s, 클라이언트=%s, 시퀀스=%d, 타임스탬프=%s",
			i, event.ID.Hex(), event.DocumentID.Hex(), event.Operation, event.ClientID,
			event.SequenceNum, event.Timestamp.Format(time.RFC3339))
		if event.Diff != nil {
			t.Logf("  Diff: HasChanges=%v", event.Diff.HasChanges)
		}
		// 이벤트 메타데이터 로깅
		if len(event.Metadata) > 0 {
			t.Logf("  Metadata: %+v", event.Metadata)
		}
		// 벡터 시계 로깅
		if len(event.VectorClock) > 0 {
			t.Logf("  VectorClock: %+v", event.VectorClock)
		}
	}

	// 이벤트 수 확인
	require.Len(t, events, 1, "문서 생성 후 이벤트는 1개여야 합니다")

	// 첫 번째 이벤트가 create인지 확인
	assert.Equal(t, "create", events[0].Operation)
	assert.Equal(t, "server", events[0].ClientID)

	// 문서 업데이트
	updatedDoc, diff, err := storage.FindOneAndUpdate(ctx, createdDoc.ID, func(d *TestDocument) (*TestDocument, error) {
		d.Name = "Updated Document"
		d.Value = 200
		d.Tags = append(d.Tags, "updated")
		d.UpdatedAt = time.Now()
		d.Version++
		return d, nil
	})
	require.NoError(t, err)
	require.NotNil(t, diff)
	assert.Equal(t, "Updated Document", updatedDoc.Name)
	assert.Equal(t, 200, updatedDoc.Value)
	assert.Equal(t, int64(2), updatedDoc.Version)

	// 업데이트 이벤트 확인
	time.Sleep(500 * time.Millisecond)

	// 모든 이벤트 조회 (디버깅용)
	allEvents, err := eventStore.GetEvents(ctx, updatedDoc.ID, 0)
	require.NoError(t, err)

	// 모든 이벤트 상세 로깅
	t.Logf("문서 업데이트 후 전체 이벤트 수: %d (예상: 2개)", len(allEvents))
	for i, event := range allEvents {
		t.Logf("전체 이벤트[%d]: ID=%s, 작업=%s, 클라이언트=%s, 시퀀스=%d",
			i, event.ID.Hex(), event.Operation, event.ClientID, event.SequenceNum)
		// 이벤트 메타데이터 로깅
		if len(event.Metadata) > 0 {
			t.Logf("  Metadata: %+v", event.Metadata)
		}
		// 벡터 시계 로깅
		if len(event.VectorClock) > 0 {
			t.Logf("  VectorClock: %+v", event.VectorClock)
		}
	}

	// 마지막 이벤트 이후의 이벤트 조회
	events, err = eventStore.GetEvents(ctx, updatedDoc.ID, 0) // 모든 이벤트 조회로 변경
	require.NoError(t, err)
	require.NotEmpty(t, events)

	// 이벤트 상세 로깅
	t.Logf("업데이트 이벤트 조회 결과: %d개", len(events))
	for i, event := range events {
		t.Logf("이벤트[%d]: ID=%s, 작업=%s, 클라이언트=%s, 시퀀스=%d",
			i, event.ID.Hex(), event.Operation, event.ClientID, event.SequenceNum)
	}

	// 이벤트가 2개 이상인지 확인
	require.GreaterOrEqual(t, len(events), 2, "이벤트가 2개 이상 있어야 합니다")

	// 업데이트 이벤트가 있는지 확인
	updateEventFound := false
	for _, event := range events {
		if event.Operation == "update" {
			updateEventFound = true
			assert.Equal(t, "server", event.ClientID)
			break
		}
	}
	assert.True(t, updateEventFound, "업데이트 이벤트가 존재해야 합니다")

	// 문서 삭제
	err = storage.DeleteOne(ctx, updatedDoc.ID)
	require.NoError(t, err)

	// 삭제 이벤트 확인
	time.Sleep(500 * time.Millisecond)
	events, err = eventStore.GetEvents(ctx, updatedDoc.ID, 0)
	require.NoError(t, err)

	// 이벤트 상세 로깅
	t.Logf("문서 삭제 후 이벤트 수: %d (예상: 3개)", len(events))
	for i, event := range events {
		t.Logf("이벤트[%d]: ID=%s, 작업=%s, 클라이언트=%s, 시퀀스=%d, 타임스탬프=%s",
			i, event.ID.Hex(), event.Operation, event.ClientID,
			event.SequenceNum, event.Timestamp.Format(time.RFC3339))
		if event.Diff != nil {
			t.Logf("  Diff: HasChanges=%v", event.Diff.HasChanges)
		}
		// 이벤트 메타데이터 로깅
		if len(event.Metadata) > 0 {
			t.Logf("  Metadata: %+v", event.Metadata)
		}
		// 벡터 시계 로깅
		if len(event.VectorClock) > 0 {
			t.Logf("  VectorClock: %+v", event.VectorClock)
		}
	}

	// 이벤트 수 확인 (생성, 업데이트, 삭제)
	require.Len(t, events, 3, "이벤트가 3개 있어야 합니다 (생성, 업데이트, 삭제)")

	// 세 번째 이벤트가 삭제인지 확인
	assert.Equal(t, "delete", events[2].Operation, "세 번째 이벤트는 삭제 이벤트여야 합니다")
	assert.Equal(t, "server", events[2].ClientID, "이벤트 클라이언트는 서버여야 합니다")
}

// TestGetEventsWithSnapshot_Integration은 스냅샷과 이벤트 함께 조회 기능을 통합 테스트합니다.
func TestGetEventsWithSnapshot_Integration(t *testing.T) {
	// 테스트 환경 설정
	client, db, cleanup := setupTestDB(t)
	defer cleanup()

	// 로거 설정
	logger := testutil.NewLogger()
	defer logger.Sync()

	// 이벤트 저장소 생성
	ctx := context.Background()
	eventStore, err := NewMongoEventStore(ctx, client, db.Name(), "events", logger)
	require.NoError(t, err)

	// 스냅샷 저장소 생성
	snapshotStore, err := NewMongoSnapshotStore(ctx, client, db.Name(), "snapshots", eventStore, logger)
	require.NoError(t, err)

	// 상태 벡터 관리자 설정
	stateVectorManager, err := NewMongoStateVectorManager(ctx, client, db.Name(), "state_vectors", eventStore, logger)
	require.NoError(t, err)

	// 동기화 서비스 설정
	syncService := NewSyncService(eventStore, stateVectorManager, logger)

	// 테스트 문서 ID
	docID := primitive.NewObjectID()

	// 이벤트 저장
	for i := 0; i < 10; i++ {
		// 이전 문서와 새 문서 생성
		oldDoc := &TestDocument{
			ID:        docID,
			Name:      "Test Document",
			Value:     100 + i,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Version:   int64(i + 1),
		}

		newDoc := oldDoc.Copy()
		newDoc.Value = 100 + i + 1
		newDoc.Version = int64(i + 2)

		// GenerateDiff 함수를 사용하여 Diff 생성
		diff, err := nodestorage.GenerateDiff(oldDoc, newDoc)
		require.NoError(t, err)
		require.NotNil(t, diff)
		require.True(t, diff.HasChanges)

		event := &Event{
			DocumentID:  docID,
			Operation:   "update",
			Diff:        diff,
			VectorClock: map[string]int64{"server": int64(i + 1)},
			ClientID:    "server",
		}

		err = syncService.StoreEvent(ctx, event)
		require.NoError(t, err)
	}

	// 스냅샷 생성
	state := map[string]interface{}{
		"name":  "Test Document",
		"value": int32(105),
	}
	snapshot, err := snapshotStore.CreateSnapshot(ctx, docID, state, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(10), snapshot.SequenceNum)

	// 추가 이벤트 저장
	for i := 10; i < 15; i++ {
		// 이전 문서와 새 문서 생성
		oldDoc := &TestDocument{
			ID:        docID,
			Name:      "Test Document",
			Value:     100 + i,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Version:   int64(i + 1),
		}

		newDoc := oldDoc.Copy()
		newDoc.Value = 100 + i + 1
		newDoc.Version = int64(i + 2)

		// GenerateDiff 함수를 사용하여 Diff 생성
		diff, err := nodestorage.GenerateDiff(oldDoc, newDoc)
		require.NoError(t, err)
		require.NotNil(t, diff)
		require.True(t, diff.HasChanges)

		event := &Event{
			DocumentID:  docID,
			Operation:   "update",
			Diff:        diff,
			VectorClock: map[string]int64{"server": int64(i + 1)},
			ClientID:    "server",
		}

		err = syncService.StoreEvent(ctx, event)
		require.NoError(t, err)
	}

	// 스냅샷과 이벤트 함께 조회
	retrievedSnapshot, events, err := GetEventsWithSnapshot(ctx, docID, snapshotStore, eventStore)
	require.NoError(t, err)
	require.NotNil(t, retrievedSnapshot)
	assert.Equal(t, state, retrievedSnapshot.State)
	assert.Equal(t, int64(10), retrievedSnapshot.SequenceNum)
	assert.Len(t, events, 5) // 스냅샷 이후 이벤트 5개

	// 클라이언트 상태 벡터 업데이트
	clientID := "test-client"
	vectorClock := map[string]int64{"server": 12} // 서버 이벤트 12번까지 수신
	err = syncService.UpdateVectorClock(ctx, clientID, docID, vectorClock)
	require.NoError(t, err)

	// 누락된 이벤트 조회
	missingEvents, err := syncService.GetMissingEvents(ctx, clientID, docID, vectorClock)
	require.NoError(t, err)
	assert.Len(t, missingEvents, 3) // 13, 14, 15번 이벤트 누락
}
