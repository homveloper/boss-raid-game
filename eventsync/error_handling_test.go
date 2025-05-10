package eventsync

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// MockErrorEventSource는 에러를 반환하는 EventSource 모의 구현체입니다.
type MockErrorEventSource struct {
	watchError error
	closeError error
}

// NewMockErrorEventSource는 새로운 MockErrorEventSource를 생성합니다.
func NewMockErrorEventSource(watchError, closeError error) *MockErrorEventSource {
	return &MockErrorEventSource{
		watchError: watchError,
		closeError: closeError,
	}
}

// Watch는 설정된 에러를 반환합니다.
func (m *MockErrorEventSource) Watch(ctx context.Context) (<-chan StorageEvent, error) {
	if m.watchError != nil {
		return nil, m.watchError
	}
	return make(chan StorageEvent), nil
}

// Close는 설정된 에러를 반환합니다.
func (m *MockErrorEventSource) Close() error {
	return m.closeError
}

// MockErrorSyncService는 에러를 반환하는 SyncService 모의 구현체입니다.
type MockErrorSyncService struct {
	handleError       error
	getMissingError   error
	updateVectorError error
	storeEventError   error
}

// NewMockErrorSyncService는 새로운 MockErrorSyncService를 생성합니다.
func NewMockErrorSyncService(handleError, getMissingError, updateVectorError, storeEventError error) *MockErrorSyncService {
	return &MockErrorSyncService{
		handleError:       handleError,
		getMissingError:   getMissingError,
		updateVectorError: updateVectorError,
		storeEventError:   storeEventError,
	}
}

// HandleStorageEvent는 설정된 에러를 반환합니다.
func (m *MockErrorSyncService) HandleStorageEvent(ctx context.Context, eventData StorageEventData) error {
	return m.handleError
}

// GetMissingEvents는 설정된 에러를 반환합니다.
func (m *MockErrorSyncService) GetMissingEvents(ctx context.Context, clientID string, documentID primitive.ObjectID, vectorClock map[string]int64) ([]*Event, error) {
	if m.getMissingError != nil {
		return nil, m.getMissingError
	}
	return []*Event{}, nil
}

// Close는 동기화 서비스를 닫습니다.
func (m *MockErrorSyncService) Close() error {
	return nil
}

// UpdateVectorClock은 설정된 에러를 반환합니다.
func (m *MockErrorSyncService) UpdateVectorClock(ctx context.Context, clientID string, documentID primitive.ObjectID, vectorClock map[string]int64) error {
	return m.updateVectorError
}

// StoreEvent는 설정된 에러를 반환합니다.
func (m *MockErrorSyncService) StoreEvent(ctx context.Context, event *Event) error {
	return m.storeEventError
}

func TestStorageListener_ErrorHandling(t *testing.T) {
	// 테스트 설정
	logger := zap.NewExample()
	defer logger.Sync()

	// Watch 에러 테스트
	t.Run("Watch 에러", func(t *testing.T) {
		// 에러를 반환하는 이벤트 소스 설정
		watchError := errors.New("watch error")
		mockEventSource := NewMockErrorEventSource(watchError, nil)

		// 동기화 서비스 설정
		mockSyncService := NewMockSyncService()

		// 스토리지 리스너 설정
		listener := NewStorageListener(mockEventSource, mockSyncService, logger)

		// 리스너 시작 시 에러가 발생해야 함
		err := listener.Start()
		assert.Error(t, err, "Watch 에러가 발생하면 Start 메서드가 에러를 반환해야 함")
		assert.Equal(t, watchError, err, "반환된 에러가 Watch 에러와 동일해야 함")
	})

	// HandleStorageEvent 에러 테스트
	t.Run("HandleStorageEvent 에러", func(t *testing.T) {
		// 이벤트 소스 설정
		mockEventSource := NewMockEventSource()

		// 에러를 반환하는 동기화 서비스 설정
		handleError := errors.New("handle error")
		mockSyncService := NewMockErrorSyncService(handleError, nil, nil, nil)

		// 스토리지 리스너 설정
		listener := NewStorageListener(mockEventSource, mockSyncService, logger)

		// 리스너 시작
		err := listener.Start()
		require.NoError(t, err)
		defer listener.Stop()

		// 이벤트 전송
		docID := primitive.NewObjectID()
		event := StorageEvent{
			ID:        docID,
			Operation: "create",
			Version:   1,
		}
		mockEventSource.SendEvent(event)

		// 에러가 로깅되었지만 리스너는 계속 실행 중이어야 함
		// 이 부분은 로그를 확인하는 방법이 없으므로 테스트하기 어려움
		// 대신 리스너가 종료되지 않았는지 확인
		time.Sleep(1 * time.Second)

		// 두 번째 이벤트 전송
		event2 := StorageEvent{
			ID:        docID,
			Operation: "update",
			Version:   2,
		}
		mockEventSource.SendEvent(event2)

		// 리스너가 여전히 이벤트를 처리하고 있는지 확인
		assert.Equal(t, 2, mockEventSource.GetSendCounter(), "리스너가 여전히 이벤트를 수신해야 함")
	})
}
