package eventsync

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// TestClientManagement는 클라이언트 등록 및 해제 기능을 테스트합니다.
func TestClientManagement(t *testing.T) {
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

	// 테스트 클라이언트 ID
	clientID := "test_client"

	// 클라이언트 등록
	err = syncService.RegisterClient(ctx, clientID)
	require.NoError(t, err)

	// 문서 ID 생성
	docID := primitive.NewObjectID()

	// 벡터 시계 업데이트 (상태 벡터 생성)
	err = syncService.UpdateVectorClock(ctx, clientID, docID, map[string]int64{"server": 1})
	require.NoError(t, err)

	// 상태 벡터 조회
	stateVector, err := stateVectorManager.GetStateVector(ctx, clientID, docID)
	require.NoError(t, err)
	assert.Equal(t, clientID, stateVector.ClientID)
	assert.Equal(t, docID, stateVector.DocumentID)
	assert.Equal(t, int64(1), stateVector.VectorClock["server"])

	// 다른 문서 ID 생성
	docID2 := primitive.NewObjectID()

	// 두 번째 문서에 대한 벡터 시계 업데이트
	err = syncService.UpdateVectorClock(ctx, clientID, docID2, map[string]int64{"server": 1})
	require.NoError(t, err)

	// 상태 벡터 컬렉션에서 클라이언트의 상태 벡터 수 확인
	collection := db.Collection("state_vectors")
	count, err := collection.CountDocuments(ctx, bson.M{"client_id": clientID})
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "클라이언트는 두 개의 상태 벡터를 가져야 함")

	// 클라이언트 등록 해제
	err = syncService.UnregisterClient(ctx, clientID)
	require.NoError(t, err)

	// 상태 벡터 컬렉션에서 클라이언트의 상태 벡터 수 확인
	count, err = collection.CountDocuments(ctx, bson.M{"client_id": clientID})
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "클라이언트 등록 해제 후 상태 벡터가 없어야 함")

	// 등록 해제된 클라이언트의 상태 벡터 조회 시도
	stateVector, err = stateVectorManager.GetStateVector(ctx, clientID, docID)
	require.NoError(t, err) // 에러는 발생하지 않지만 새로운 상태 벡터가 생성됨
	assert.Equal(t, clientID, stateVector.ClientID)
	assert.Equal(t, docID, stateVector.DocumentID)
	assert.Empty(t, stateVector.VectorClock, "새로 생성된 상태 벡터의 벡터 시계는 비어 있어야 함")

	// 다시 상태 벡터 컬렉션에서 클라이언트의 상태 벡터 수 확인
	count, err = collection.CountDocuments(ctx, bson.M{"client_id": clientID})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "GetStateVector 호출 후 상태 벡터가 하나 생성되어야 함")

	// 다시 클라이언트 등록 해제
	err = syncService.UnregisterClient(ctx, clientID)
	require.NoError(t, err)

	// 상태 벡터 컬렉션에서 클라이언트의 상태 벡터 수 확인
	count, err = collection.CountDocuments(ctx, bson.M{"client_id": clientID})
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "클라이언트 등록 해제 후 상태 벡터가 없어야 함")
}
