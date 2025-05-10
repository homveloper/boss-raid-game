package eventsync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// TestSyncService_VectorClockSynchronization은 벡터 시계 동기화를 테스트합니다.
func TestSyncService_VectorClockSynchronization(t *testing.T) {
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

	// 문서 ID 생성
	docID := primitive.NewObjectID()

	// 클라이언트 ID 설정
	client1ID := "client1"
	client2ID := "client2"

	// 클라이언트 1의 이벤트 생성
	event1 := &Event{
		ID:          primitive.NewObjectID(),
		DocumentID:  docID,
		Timestamp:   time.Now(),
		Operation:   "create",
		ClientID:    client1ID,
		VectorClock: map[string]int64{client1ID: 1},
		Metadata:    make(map[string]interface{}),
	}

	// 이벤트 저장
	err = syncService.StoreEvent(ctx, event1)
	require.NoError(t, err)

	// 클라이언트 2의 이벤트 생성
	event2 := &Event{
		ID:          primitive.NewObjectID(),
		DocumentID:  docID,
		Timestamp:   time.Now(),
		Operation:   "update",
		ClientID:    client2ID,
		VectorClock: map[string]int64{client2ID: 1},
		Metadata:    make(map[string]interface{}),
	}

	// 이벤트 저장
	err = syncService.StoreEvent(ctx, event2)
	require.NoError(t, err)

	// 클라이언트 1의 벡터 시계 업데이트
	err = syncService.UpdateVectorClock(ctx, client1ID, docID, map[string]int64{client2ID: 1})
	require.NoError(t, err)

	// 클라이언트 1의 상태 벡터 조회
	stateVector, err := stateVectorManager.GetStateVector(ctx, client1ID, docID)
	require.NoError(t, err)

	// 상태 벡터 검증
	assert.Equal(t, int64(1), stateVector.VectorClock[client1ID], "클라이언트 1의 벡터 시계 값이 1이어야 함")
	assert.Equal(t, int64(1), stateVector.VectorClock[client2ID], "클라이언트 2의 벡터 시계 값이 1이어야 함")

	// 클라이언트 1의 누락된 이벤트 조회
	events, err := syncService.GetMissingEvents(ctx, client1ID, docID, stateVector.VectorClock)
	require.NoError(t, err)

	// 누락된 이벤트가 없어야 함
	assert.Empty(t, events, "클라이언트 1은 모든 이벤트를 수신했으므로 누락된 이벤트가 없어야 함")

	// 클라이언트 3의 누락된 이벤트 조회
	client3ID := "client3"
	events, err = syncService.GetMissingEvents(ctx, client3ID, docID, map[string]int64{})
	require.NoError(t, err)

	// 모든 이벤트가 누락되어야 함
	assert.Len(t, events, 2, "클라이언트 3은 어떤 이벤트도 수신하지 않았으므로 모든 이벤트가 누락되어야 함")

	// 클라이언트 1의 새 이벤트 생성
	event3 := &Event{
		ID:          primitive.NewObjectID(),
		DocumentID:  docID,
		Timestamp:   time.Now(),
		Operation:   "update",
		ClientID:    client1ID,
		VectorClock: map[string]int64{client1ID: 2, client2ID: 1},
		Metadata:    make(map[string]interface{}),
	}

	// 이벤트 저장
	err = syncService.StoreEvent(ctx, event3)
	require.NoError(t, err)

	// 클라이언트 2의 누락된 이벤트 조회
	events, err = syncService.GetMissingEvents(ctx, client2ID, docID, map[string]int64{client1ID: 1, client2ID: 1})
	require.NoError(t, err)

	// 클라이언트 1의 새 이벤트만 누락되어야 함
	assert.Len(t, events, 1, "클라이언트 2는 클라이언트 1의 새 이벤트만 누락되어야 함")
	if len(events) > 0 {
		assert.Equal(t, client1ID, events[0].ClientID, "누락된 이벤트는 클라이언트 1의 이벤트여야 함")
		assert.Equal(t, int64(2), events[0].VectorClock[client1ID], "누락된 이벤트의 벡터 시계 값이 2여야 함")
	}
}
