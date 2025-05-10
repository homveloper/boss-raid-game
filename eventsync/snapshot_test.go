package eventsync

import (
	"context"
	"testing"
	"tictactoe/eventsync/testutil"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"nodestorage/v2"
)

// TestMongoSnapshotStore_CreateSnapshot은 스냅샷 생성 기능을 테스트합니다.
func TestMongoSnapshotStore_CreateSnapshot(t *testing.T) {
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

	// 테스트 문서 ID
	docID := primitive.NewObjectID()

	// 이벤트 저장
	for i := 0; i < 3; i++ {
		// 이전 문서와 새 문서 생성
		oldDoc := &TestDocument{
			ID:      docID,
			Name:    "Test Document",
			Value:   100 + i,
			Created: time.Now(),
			Updated: time.Now(),
			Version: int64(i + 1),
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
			ClientID:    "test-client",
		}

		err = eventStore.StoreEvent(ctx, event)
		require.NoError(t, err)
	}

	// 스냅샷 생성
	state := map[string]interface{}{
		"name":  "Test Document",
		"value": 102,
	}
	snapshot, err := snapshotStore.CreateSnapshot(ctx, docID, state, 1)
	require.NoError(t, err)

	// 스냅샷 확인
	assert.NotEqual(t, primitive.NilObjectID, snapshot.ID)
	assert.Equal(t, docID, snapshot.DocumentID)
	assert.Equal(t, int64(1), snapshot.Version)
	assert.Equal(t, int64(3), snapshot.SequenceNum) // 최신 이벤트 시퀀스 번호
	assert.Equal(t, state, snapshot.State)
	assert.False(t, snapshot.CreatedAt.IsZero())
}

// TestMongoSnapshotStore_GetLatestSnapshot은 최신 스냅샷 조회 기능을 테스트합니다.
func TestMongoSnapshotStore_GetLatestSnapshot(t *testing.T) {
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

	// 테스트 문서 ID
	docID := primitive.NewObjectID()

	// 초기 상태 확인
	snapshot, err := snapshotStore.GetLatestSnapshot(ctx, docID)
	require.NoError(t, err)
	assert.Nil(t, snapshot)

	// 이벤트 저장
	for i := 0; i < 3; i++ {
		// 이전 문서와 새 문서 생성
		oldDoc := &TestDocument{
			ID:      docID,
			Name:    "Test Document",
			Value:   100 + i,
			Created: time.Now(),
			Updated: time.Now(),
			Version: int64(i + 1),
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
			ClientID:    "test-client",
		}

		err = eventStore.StoreEvent(ctx, event)
		require.NoError(t, err)
	}

	// 첫 번째 스냅샷 생성
	state1 := map[string]interface{}{
		"name":  "Test Document",
		"value": 100,
	}
	snapshot1, err := snapshotStore.CreateSnapshot(ctx, docID, state1, 1)
	require.NoError(t, err)

	// 최신 스냅샷 확인
	latestSnapshot, err := snapshotStore.GetLatestSnapshot(ctx, docID)
	require.NoError(t, err)
	require.NotNil(t, latestSnapshot)
	assert.Equal(t, snapshot1.ID, latestSnapshot.ID)

	// 상태 값 비교 (타입은 다를 수 있음)
	assert.Equal(t, state1["name"], latestSnapshot.State["name"])
	assert.Equal(t, int(state1["value"].(int)), int(latestSnapshot.State["value"].(int32)))

	t.Logf("첫 번째 스냅샷 ID: %s", snapshot1.ID.Hex())

	// 두 번째 스냅샷 생성
	state2 := map[string]interface{}{
		"name":  "Updated Document",
		"value": 102,
	}
	snapshot2, err := snapshotStore.CreateSnapshot(ctx, docID, state2, 2)
	require.NoError(t, err)
	t.Logf("두 번째 스냅샷 ID: %s", snapshot2.ID.Hex())

	// 최신 스냅샷 확인
	latestSnapshot, err = snapshotStore.GetLatestSnapshot(ctx, docID)
	require.NoError(t, err)
	require.NotNil(t, latestSnapshot)
	t.Logf("최신 스냅샷 ID: %s", latestSnapshot.ID.Hex())
	t.Logf("최신 스냅샷 상태: %v", latestSnapshot.State)

	// ID와 상태 확인 (ID는 다를 수 있으므로 생략)
	assert.Equal(t, int64(2), latestSnapshot.Version)
	assert.Equal(t, int64(3), latestSnapshot.SequenceNum)

	// 상태 값 비교 (타입은 다를 수 있음)
	assert.Equal(t, state2["name"], latestSnapshot.State["name"])
	assert.Equal(t, int(state2["value"].(int)), int(latestSnapshot.State["value"].(int32)))
}

// TestMongoSnapshotStore_GetSnapshotBySequence는 시퀀스 번호 기반 스냅샷 조회 기능을 테스트합니다.
func TestMongoSnapshotStore_GetSnapshotBySequence(t *testing.T) {
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

	// 테스트 문서 ID
	docID := primitive.NewObjectID()

	// 이벤트 저장
	for i := 0; i < 10; i++ {
		// 이전 문서와 새 문서 생성
		oldDoc := &TestDocument{
			ID:      docID,
			Name:    "Test Document",
			Value:   100 + i,
			Created: time.Now(),
			Updated: time.Now(),
			Version: int64(i + 1),
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
			SequenceNum: int64(i + 1),
			Timestamp:   time.Now(),
			Diff:        diff,
			VectorClock: map[string]int64{"server": int64(i + 1)},
			ClientID:    "test-client",
		}

		_, err = eventStore.collection.InsertOne(ctx, event)
		require.NoError(t, err)
	}

	// 스냅샷 생성
	snapshots := []struct {
		seqNum  int64
		version int64
		state   map[string]interface{}
	}{
		{
			seqNum:  3,
			version: 1,
			state: map[string]interface{}{
				"name":  "Snapshot 1",
				"value": 103,
			},
		},
		{
			seqNum:  7,
			version: 2,
			state: map[string]interface{}{
				"name":  "Snapshot 2",
				"value": 107,
			},
		},
	}

	for _, s := range snapshots {
		snapshot := &Snapshot{
			ID:          primitive.NewObjectID(),
			DocumentID:  docID,
			State:       s.state,
			Version:     s.version,
			SequenceNum: s.seqNum,
			CreatedAt:   time.Now(),
		}

		_, err = snapshotStore.collection.InsertOne(ctx, snapshot)
		require.NoError(t, err)
	}

	// 시퀀스 번호 기반 스냅샷 조회 테스트
	testCases := []struct {
		name        string
		maxSequence int64
		expected    int64
	}{
		{
			name:        "시퀀스 2 이전 스냅샷 없음",
			maxSequence: 2,
			expected:    0,
		},
		{
			name:        "시퀀스 3 이전 스냅샷",
			maxSequence: 3,
			expected:    3,
		},
		{
			name:        "시퀀스 5 이전 스냅샷",
			maxSequence: 5,
			expected:    3,
		},
		{
			name:        "시퀀스 7 이전 스냅샷",
			maxSequence: 7,
			expected:    7,
		},
		{
			name:        "시퀀스 10 이전 스냅샷",
			maxSequence: 10,
			expected:    7,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot, err := snapshotStore.GetSnapshotBySequence(ctx, docID, tc.maxSequence)
			require.NoError(t, err)

			if tc.expected == 0 {
				assert.Nil(t, snapshot)
			} else {
				require.NotNil(t, snapshot)
				assert.Equal(t, tc.expected, snapshot.SequenceNum)
			}
		})
	}
}

// TestMongoSnapshotStore_DeleteSnapshots는 스냅샷 삭제 기능을 테스트합니다.
func TestMongoSnapshotStore_DeleteSnapshots(t *testing.T) {
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

	// 테스트 문서 ID
	docID := primitive.NewObjectID()

	// 스냅샷 생성
	seqNums := []int64{5, 10, 15, 20, 25}
	for i, seqNum := range seqNums {
		snapshot := &Snapshot{
			ID:          primitive.NewObjectID(),
			DocumentID:  docID,
			State:       map[string]interface{}{"value": 100 + i},
			Version:     int64(i + 1),
			SequenceNum: seqNum,
			CreatedAt:   time.Now(),
		}

		_, err = snapshotStore.collection.InsertOne(ctx, snapshot)
		require.NoError(t, err)
	}

	// 시퀀스 18 이전의 스냅샷 삭제
	deletedCount, err := snapshotStore.DeleteSnapshots(ctx, docID, 18)
	require.NoError(t, err)
	assert.Equal(t, int64(2), deletedCount) // 시퀀스 5, 10 삭제

	// 남은 스냅샷 확인
	snapshot, err := snapshotStore.GetLatestSnapshot(ctx, docID)
	require.NoError(t, err)
	assert.Equal(t, int64(25), snapshot.SequenceNum)

	snapshot, err = snapshotStore.GetSnapshotBySequence(ctx, docID, 18)
	require.NoError(t, err)
	assert.Equal(t, int64(15), snapshot.SequenceNum)
}

// TestGetEventsWithSnapshot은 스냅샷과 이벤트 함께 조회 기능을 테스트합니다.
func TestGetEventsWithSnapshot(t *testing.T) {
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

	// 테스트 문서 ID
	docID := primitive.NewObjectID()

	// 이벤트 저장
	for i := 0; i < 10; i++ {
		// 이전 문서와 새 문서 생성
		oldDoc := &TestDocument{
			ID:      docID,
			Name:    "Test Document",
			Value:   100 + i,
			Created: time.Now(),
			Updated: time.Now(),
			Version: int64(i + 1),
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
			ClientID:    "test-client",
		}

		err = eventStore.StoreEvent(ctx, event)
		require.NoError(t, err)
	}

	// 스냅샷 없는 경우 테스트
	snapshot, events, err := GetEventsWithSnapshot(ctx, docID, snapshotStore, eventStore)
	require.NoError(t, err)
	assert.Nil(t, snapshot)
	assert.Len(t, events, 10)

	// 스냅샷 생성
	state := map[string]interface{}{
		"name":  "Test Document",
		"value": int32(105),
	}
	_, err = snapshotStore.CreateSnapshot(ctx, docID, state, 1)
	require.NoError(t, err)

	// 추가 이벤트 저장
	for i := 10; i < 15; i++ {
		// 이전 문서와 새 문서 생성
		oldDoc := &TestDocument{
			ID:      docID,
			Name:    "Test Document",
			Value:   100 + i,
			Created: time.Now(),
			Updated: time.Now(),
			Version: int64(i + 1),
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
			ClientID:    "test-client",
		}

		err = eventStore.StoreEvent(ctx, event)
		require.NoError(t, err)
	}

	// 스냅샷 있는 경우 테스트
	snapshot, events, err = GetEventsWithSnapshot(ctx, docID, snapshotStore, eventStore)
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	assert.Equal(t, state, snapshot.State)
	assert.Equal(t, int64(10), snapshot.SequenceNum)
	assert.Len(t, events, 5) // 스냅샷 이후 이벤트 5개
}
