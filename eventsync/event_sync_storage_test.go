package eventsync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"nodestorage/v2"
)

// MockStorage는 테스트를 위한 nodestorage.Storage 모의 구현체입니다.
type MockStorage[T nodestorage.Cachable[T]] struct {
	mock.Mock
}

func (m *MockStorage[T]) FindOne(ctx context.Context, id primitive.ObjectID, opts ...*options.FindOneOptions) (T, error) {
	args := m.Called(ctx, id, opts)
	return args.Get(0).(T), args.Error(1)
}

func (m *MockStorage[T]) FindMany(ctx context.Context, filter interface{}, opts ...*options.FindOptions) ([]T, error) {
	args := m.Called(ctx, filter, opts)
	return args.Get(0).([]T), args.Error(1)
}

func (m *MockStorage[T]) FindOneAndUpsert(ctx context.Context, data T) (T, error) {
	args := m.Called(ctx, data)
	return args.Get(0).(T), args.Error(1)
}

func (m *MockStorage[T]) FindOneAndUpdate(ctx context.Context, id primitive.ObjectID, updateFn nodestorage.EditFunc[T], opts ...nodestorage.EditOption) (T, *nodestorage.Diff, error) {
	args := m.Called(ctx, id, updateFn, opts)
	return args.Get(0).(T), args.Get(1).(*nodestorage.Diff), args.Error(2)
}

func (m *MockStorage[T]) DeleteOne(ctx context.Context, id primitive.ObjectID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockStorage[T]) UpdateOne(ctx context.Context, id primitive.ObjectID, update bson.M, opts ...nodestorage.EditOption) (T, error) {
	args := m.Called(ctx, id, update, opts)
	return args.Get(0).(T), args.Error(1)
}

func (m *MockStorage[T]) UpdateOneWithPipeline(ctx context.Context, id primitive.ObjectID, pipeline mongo.Pipeline, opts ...nodestorage.EditOption) (T, error) {
	args := m.Called(ctx, id, pipeline, opts)
	return args.Get(0).(T), args.Error(1)
}

func (m *MockStorage[T]) UpdateSection(ctx context.Context, id primitive.ObjectID, sectionPath string, updateFn func(interface{}) (interface{}, error), opts ...nodestorage.EditOption) (T, error) {
	args := m.Called(ctx, id, sectionPath, updateFn, opts)
	return args.Get(0).(T), args.Error(1)
}

func (m *MockStorage[T]) WithTransaction(ctx context.Context, fn func(sessCtx mongo.SessionContext) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func (m *MockStorage[T]) Watch(ctx context.Context, pipeline mongo.Pipeline, opts ...*options.ChangeStreamOptions) (<-chan nodestorage.WatchEvent[T], error) {
	args := m.Called(ctx, pipeline, opts)
	return args.Get(0).(<-chan nodestorage.WatchEvent[T]), args.Error(1)
}

func (m *MockStorage[T]) Collection() *mongo.Collection {
	args := m.Called()
	return args.Get(0).(*mongo.Collection)
}

func (m *MockStorage[T]) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockEventStore는 테스트를 위한 EventStore 모의 구현체입니다.
type MockEventStore struct {
	mock.Mock
}

func (m *MockEventStore) StoreEvent(ctx context.Context, event *Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockEventStore) GetEvents(ctx context.Context, documentID primitive.ObjectID, afterSequence int64) ([]*Event, error) {
	args := m.Called(ctx, documentID, afterSequence)
	return args.Get(0).([]*Event), args.Error(1)
}

func (m *MockEventStore) GetLatestSequence(ctx context.Context, documentID primitive.ObjectID) (int64, error) {
	args := m.Called(ctx, documentID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockEventStore) GetEventsByVectorClock(ctx context.Context, documentID primitive.ObjectID, vectorClock map[string]int64) ([]*Event, error) {
	args := m.Called(ctx, documentID, vectorClock)
	return args.Get(0).([]*Event), args.Error(1)
}

func (m *MockEventStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

// TestDocument는 테스트를 위한 문서 구조체입니다.
type TestDocument struct {
	ID        primitive.ObjectID `bson:"_id" json:"id"`
	Name      string             `bson:"name" json:"name"`
	Value     int                `bson:"value" json:"value"`
	CreatedAt time.Time          `bson:"created_at" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updatedAt"`
	Tags      []string           `bson:"tags" json:"tags"`
	Version   int64              `bson:"version" json:"version"`
}

// Copy는 TestDocument의 복사본을 생성합니다.
func (d *TestDocument) Copy() *TestDocument {
	if d == nil {
		return nil
	}
	return &TestDocument{
		ID:      d.ID,
		Name:    d.Name,
		Value:   d.Value,
		Version: d.Version,
	}
}

func TestEventSyncStorage_FindOneAndUpdate(t *testing.T) {
	// 테스트 설정
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	mockStorage := new(MockStorage[*TestDocument])
	mockEventStore := new(MockEventStore)

	// EventSyncStorage 생성
	storage := NewEventSyncStorage(mockStorage, mockEventStore, logger)

	// 테스트 데이터
	id := primitive.NewObjectID()
	oldDoc := &TestDocument{
		ID:      id,
		Name:    "Test",
		Value:   10,
		Version: 1,
	}
	newDoc := &TestDocument{
		ID:      id,
		Name:    "Updated Test",
		Value:   20,
		Version: 2,
	}

	diff, err := nodestorage.GenerateDiff(oldDoc, newDoc)
	require.NoError(t, err)

	diff.Version = 2

	// 모의 동작 설정
	updateFn := func(doc *TestDocument) (*TestDocument, error) {
		doc.Name = "Updated Test"
		doc.Value = 20
		doc.Version = 2
		return doc, nil
	}

	mockStorage.On("FindOneAndUpdate", ctx, id, mock.AnythingOfType("nodestorage.EditFunc[*eventsync.TestDocument]"), mock.Anything).
		Return(newDoc, diff, nil)

	mockEventStore.On("StoreEvent", ctx, mock.MatchedBy(func(event *Event) bool {
		return event.DocumentID == id && event.Operation == "update" && event.Diff == diff
	})).Return(nil)

	// 테스트 실행
	resultDoc, resultDiff, err := storage.FindOneAndUpdate(ctx, id, updateFn)

	// 검증
	require.NoError(t, err)
	assert.Equal(t, newDoc, resultDoc)
	assert.Equal(t, diff, resultDiff)
	mockStorage.AssertExpectations(t)
	mockEventStore.AssertExpectations(t)
}

func TestEventSyncStorage_DeleteOne(t *testing.T) {
	// 테스트 설정
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	mockStorage := new(MockStorage[*TestDocument])
	mockEventStore := new(MockEventStore)

	// EventSyncStorage 생성
	storage := NewEventSyncStorage(mockStorage, mockEventStore, logger)

	// 테스트 데이터
	id := primitive.NewObjectID()
	doc := &TestDocument{
		ID:      id,
		Name:    "Test",
		Value:   10,
		Version: 1,
	}

	// 모의 동작 설정
	mockStorage.On("FindOne", ctx, id).Return(doc, nil)
	mockStorage.On("DeleteOne", ctx, id).Return(nil)
	mockEventStore.On("StoreEvent", ctx, mock.MatchedBy(func(event *Event) bool {
		return event.DocumentID == id && event.Operation == "delete"
	})).Return(nil)

	// 테스트 실행
	err := storage.DeleteOne(ctx, id)

	// 검증
	require.NoError(t, err)
	mockStorage.AssertExpectations(t)
	mockEventStore.AssertExpectations(t)
}

func TestEventSyncStorage_FindOneAndUpsert(t *testing.T) {
	// 테스트 설정
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	mockStorage := new(MockStorage[*TestDocument])
	mockEventStore := new(MockEventStore)

	// EventSyncStorage 생성
	storage := NewEventSyncStorage(mockStorage, mockEventStore, logger)

	// 테스트 데이터
	id := primitive.NewObjectID()
	doc := &TestDocument{
		ID:      id,
		Name:    "Test",
		Value:   10,
		Version: 1,
	}

	// 모의 동작 설정
	mockStorage.On("FindOneAndUpsert", ctx, doc).Return(doc, nil)
	mockEventStore.On("StoreEvent", ctx, mock.MatchedBy(func(event *Event) bool {
		return event.DocumentID == id && event.Operation == "create"
	})).Return(nil)

	// 테스트 실행
	resultDoc, err := storage.FindOneAndUpsert(ctx, doc)

	// 검증
	require.NoError(t, err)
	assert.Equal(t, doc, resultDoc)
	mockStorage.AssertExpectations(t)
	mockEventStore.AssertExpectations(t)
}
