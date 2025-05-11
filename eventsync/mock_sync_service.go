package eventsync

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MockSyncService는 테스트를 위한 SyncService 모의 구현체입니다.
type MockSyncService struct {
	events     []*Event
	mu         sync.RWMutex
	eventCount int // 이벤트 처리 횟수를 추적
}

// NewMockSyncService는 새로운 MockSyncService를 생성합니다.
func NewMockSyncService() *MockSyncService {
	return &MockSyncService{
		events: make([]*Event, 0),
	}
}

// HandleStorageEvent는 스토리지 이벤트를 처리합니다.
func (m *MockSyncService) HandleStorageEvent(ctx context.Context, eventData StorageEventData) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 이벤트 생성
	event := &Event{
		ID:          primitive.NewObjectID(),
		DocumentID:  eventData.GetID(),
		Operation:   eventData.GetOperation(),
		Diff:        eventData.GetDiff(),
		VectorClock: map[string]int64{"server": 1},
		ClientID:    "server",
		Metadata:    make(map[string]interface{}),
	}

	// 이벤트 저장
	m.events = append(m.events, event)
	m.eventCount++

	return nil
}

// GetMissingEvents는 클라이언트가 아직 수신하지 않은 이벤트를 조회합니다.
func (m *MockSyncService) GetMissingEvents(ctx context.Context, clientID string, documentID primitive.ObjectID, vectorClock map[string]int64) ([]*Event, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 문서 ID에 해당하는 이벤트만 필터링
	var result []*Event
	for _, event := range m.events {
		if event.DocumentID == documentID {
			result = append(result, event)
		}
	}

	return result, nil
}

// UpdateVectorClock은 클라이언트의 벡터 시계를 업데이트합니다.
func (m *MockSyncService) UpdateVectorClock(ctx context.Context, clientID string, documentID primitive.ObjectID, vectorClock map[string]int64) error {
	// 벡터 시계 업데이트는 테스트에서 필요하지 않으므로 빈 구현
	return nil
}

// StoreEvent는 이벤트를 저장합니다.
func (m *MockSyncService) StoreEvent(ctx context.Context, event *Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = append(m.events, event)
	return nil
}

// EventCount는 처리된 이벤트 수를 반환합니다.
func (m *MockSyncService) EventCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.eventCount
}

// GetEvents는 저장된 모든 이벤트를 반환합니다.
func (m *MockSyncService) GetEvents() []*Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Event, len(m.events))
	copy(result, m.events)
	return result
}

// RegisterClient는 새 클라이언트를 등록합니다.
func (m *MockSyncService) RegisterClient(ctx context.Context, clientID string) error {
	return nil
}

// UnregisterClient는 클라이언트를 등록 해제합니다.
func (m *MockSyncService) UnregisterClient(ctx context.Context, clientID string) error {
	return nil
}

// Close는 동기화 서비스를 닫습니다.
func (m *MockSyncService) Close() error {
	return nil
}
