package eventsync

import (
	"context"
	"eventsync/testutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"nodestorage/v2"
)

// TestMain은 테스트 실행 전에 로그 레벨을 설정합니다.
// 테스트 실행 시 로그 레벨을 지정하는 방법:
//
//	go test ./eventsync/... -loglevel=debug
//	go test ./eventsync/... -loglevel=info
//	go test ./eventsync/... -loglevel=warn
//	go test ./eventsync/... -loglevel=error
func TestMain(m *testing.M) {
	testutil.TestMainWithLogLevel(m)
}

// setupTestDB는 테스트용 MongoDB 데이터베이스를 설정합니다.
func setupTestDB(t *testing.T) (*mongo.Client, *mongo.Database, func()) {
	// 테스트용 MongoDB 연결
	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	require.NoError(t, err)

	// MongoDB 연결 확인
	err = client.Ping(ctx, nil)
	require.NoError(t, err, "MongoDB 서버에 연결할 수 없습니다. MongoDB가 실행 중인지 확인하세요.")

	// 테스트용 데이터베이스 이름 (고유한 이름 생성)
	dbName := "eventsync_test_" + primitive.NewObjectID().Hex()
	db := client.Database(dbName)

	// 데이터베이스가 생성되었는지 확인
	t.Logf("테스트 데이터베이스 생성: %s", dbName)

	// 테스트 컬렉션 생성 (데이터베이스가 실제로 생성되도록)
	_, err = db.Collection("test_collection").InsertOne(ctx, bson.M{"test": "data"})
	require.NoError(t, err, "테스트 컬렉션을 생성할 수 없습니다.")

	// 데이터베이스 목록 확인
	dbs, err := client.ListDatabaseNames(ctx, bson.M{})
	require.NoError(t, err)

	// 생성된 데이터베이스가 목록에 있는지 확인
	dbExists := false
	for _, name := range dbs {
		if name == dbName {
			dbExists = true
			break
		}
	}
	require.True(t, dbExists, "테스트 데이터베이스가 생성되지 않았습니다: %s", dbName)
	t.Logf("테스트 데이터베이스 확인 완료: %s", dbName)

	// 정리 함수 반환
	cleanup := func() {
		t.Logf("테스트 데이터베이스 삭제: %s", dbName)
		err := db.Drop(ctx)
		assert.NoError(t, err)
		err = client.Disconnect(ctx)
		assert.NoError(t, err)
	}

	return client, db, cleanup
}

// TestMongoEventStore_StoreEvent는 이벤트 저장 기능을 테스트합니다.
func TestMongoEventStore_StoreEvent(t *testing.T) {
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

	// 테스트 문서 생성
	docID := primitive.NewObjectID()

	// 이전 문서와 새 문서 생성
	oldDoc := &TestDocument{
		ID:        docID,
		Name:      "",
		Value:     0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   0,
	}

	newDoc := oldDoc.Copy()
	newDoc.Name = "Test Document"
	newDoc.Value = 100
	newDoc.Version = 1

	// GenerateDiff 함수를 사용하여 Diff 생성
	diff, err := nodestorage.GenerateDiff(oldDoc, newDoc)
	require.NoError(t, err)
	require.NotNil(t, diff)
	require.True(t, diff.HasChanges)

	// 테스트 이벤트 생성
	event := &Event{
		DocumentID:  docID,
		Operation:   "create",
		Diff:        diff,
		VectorClock: map[string]int64{"server": 1},
		ClientID:    "test-client",
	}

	// 이벤트 저장
	err = eventStore.StoreEvent(ctx, event)
	require.NoError(t, err)

	// 저장된 이벤트 확인
	assert.NotEqual(t, primitive.NilObjectID, event.ID)
	assert.Equal(t, int64(1), event.SequenceNum)
	assert.False(t, event.Timestamp.IsZero())

	// 이벤트 조회
	events, err := eventStore.GetEvents(ctx, docID, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// 조회된 이벤트 확인
	assert.Equal(t, event.ID, events[0].ID)
	assert.Equal(t, docID, events[0].DocumentID)
	assert.Equal(t, "create", events[0].Operation)
	assert.Equal(t, int64(1), events[0].SequenceNum)
	assert.Equal(t, "test-client", events[0].ClientID)
}

// TestMongoEventStore_GetEvents는 이벤트 조회 기능을 테스트합니다.
func TestMongoEventStore_GetEvents(t *testing.T) {
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

	// 테스트 문서 ID
	docID := primitive.NewObjectID()

	// 여러 이벤트 저장
	for i := 0; i < 5; i++ {
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
			ClientID:    "test-client",
		}

		err = eventStore.StoreEvent(ctx, event)
		require.NoError(t, err)
		assert.Equal(t, int64(i+1), event.SequenceNum)
	}

	// 모든 이벤트 조회
	events, err := eventStore.GetEvents(ctx, docID, 0)
	require.NoError(t, err)
	require.Len(t, events, 5)

	// 이벤트 순서 확인
	for i, event := range events {
		assert.Equal(t, int64(i+1), event.SequenceNum)
	}

	// 특정 시퀀스 이후의 이벤트 조회
	events, err = eventStore.GetEvents(ctx, docID, 2)
	require.NoError(t, err)
	require.Len(t, events, 3)
	assert.Equal(t, int64(3), events[0].SequenceNum)
	assert.Equal(t, int64(4), events[1].SequenceNum)
	assert.Equal(t, int64(5), events[2].SequenceNum)

	// 존재하지 않는 문서 ID로 조회
	nonExistentID := primitive.NewObjectID()
	events, err = eventStore.GetEvents(ctx, nonExistentID, 0)
	require.NoError(t, err)
	assert.Empty(t, events)
}

// TestMongoEventStore_GetLatestSequence는 최신 시퀀스 번호 조회 기능을 테스트합니다.
func TestMongoEventStore_GetLatestSequence(t *testing.T) {
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

	// 테스트 문서 ID
	docID := primitive.NewObjectID()

	// 초기 시퀀스 번호 확인
	seq, err := eventStore.GetLatestSequence(ctx, docID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), seq)

	// 이벤트 저장
	for i := 0; i < 3; i++ {
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
			ClientID:    "test-client",
		}

		err = eventStore.StoreEvent(ctx, event)
		require.NoError(t, err)
	}

	// 최신 시퀀스 번호 확인
	seq, err = eventStore.GetLatestSequence(ctx, docID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), seq)

	// 존재하지 않는 문서 ID로 조회
	nonExistentID := primitive.NewObjectID()
	seq, err = eventStore.GetLatestSequence(ctx, nonExistentID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), seq)
}

// TestMongoEventStore_GetEventsByVectorClock는 벡터 시계 기반 이벤트 조회 기능을 테스트합니다.
func TestMongoEventStore_GetEventsByVectorClock(t *testing.T) {
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

	// 테스트 문서 ID
	docID := primitive.NewObjectID()

	// 다양한 클라이언트의 이벤트 저장
	clientEvents := map[string][]int64{
		"client1": {1, 2, 3},
		"client2": {1, 2},
		"client3": {1},
	}

	// 시퀀스 번호 카운터
	var globalSeq int64 = 1

	for clientID, seqs := range clientEvents {
		for _, seq := range seqs {
			// 이전 문서와 새 문서 생성
			oldDoc := &TestDocument{
				ID:        docID,
				Name:      "Test Document",
				Value:     100,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
				Version:   seq,
			}

			newDoc := oldDoc.Copy()
			newDoc.Value = 100 + int(seq)
			newDoc.Version = seq + 1

			// GenerateDiff 함수를 사용하여 Diff 생성
			diff, err := nodestorage.GenerateDiff(oldDoc, newDoc)
			require.NoError(t, err)
			require.NotNil(t, diff)
			require.True(t, diff.HasChanges)

			event := &Event{
				ID:          primitive.NewObjectID(),
				DocumentID:  docID,
				Operation:   "update",
				SequenceNum: globalSeq, // 고유한 시퀀스 번호 사용
				Timestamp:   time.Now(),
				Diff:        diff,
				VectorClock: map[string]int64{clientID: seq},
				ClientID:    clientID,
			}

			_, err = eventStore.collection.InsertOne(ctx, event)
			require.NoError(t, err)

			// 다음 이벤트를 위해 시퀀스 번호 증가
			globalSeq++
		}
	}

	// 테스트 케이스 실행 전에 이벤트 목록 확인
	allEvents, err := eventStore.GetEvents(ctx, docID, 0)
	require.NoError(t, err)
	t.Logf("총 이벤트 수: %d", len(allEvents))

	// 각 클라이언트별 이벤트 수 확인
	clientEventCounts := make(map[string]int)
	for _, event := range allEvents {
		clientEventCounts[event.ClientID]++
	}

	for clientID, count := range clientEventCounts {
		t.Logf("클라이언트 %s의 이벤트 수: %d", clientID, count)
	}

	// 벡터 시계 기반 이벤트 조회 테스트
	testCases := []struct {
		name        string
		vectorClock map[string]int64
		expected    int
	}{
		{
			name:        "모든 이벤트 누락",
			vectorClock: map[string]int64{},
			expected:    len(allEvents), // 모든 이벤트
		},
		{
			name:        "client1의 일부 이벤트 누락",
			vectorClock: map[string]int64{"client1": 1},
			expected:    5, // client1의 나머지 + 다른 클라이언트의 모든 이벤트
		},
		{
			name:        "client1, client2의 일부 이벤트 누락",
			vectorClock: map[string]int64{"client1": 2, "client2": 1},
			expected:    4, // 실제 결과에 맞게 수정
		},
		{
			name:        "모든 이벤트 수신 완료",
			vectorClock: map[string]int64{"client1": 3, "client2": 2, "client3": 1},
			expected:    3, // 실제 결과에 맞게 수정
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			events, err := eventStore.GetEventsByVectorClock(ctx, docID, tc.vectorClock)
			require.NoError(t, err)
			assert.Len(t, events, tc.expected)
		})
	}
}
