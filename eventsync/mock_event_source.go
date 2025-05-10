package eventsync

import (
	"context"
	"fmt"
	"sync"
)

// MockEventSource는 테스트를 위한 EventSource 모의 구현체입니다.
type MockEventSource struct {
	eventCh     chan StorageEvent
	closed      bool
	mu          sync.RWMutex
	sendCounter int // 이벤트 전송 횟수를 추적
}

// NewMockEventSource는 새로운 MockEventSource를 생성합니다.
func NewMockEventSource() *MockEventSource {
	return &MockEventSource{
		eventCh: make(chan StorageEvent, 100),
		closed:  false,
	}
}

// Watch는 이벤트를 감시하고 이벤트 채널을 반환합니다.
func (m *MockEventSource) Watch(ctx context.Context) (<-chan StorageEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil, fmt.Errorf("event source is closed")
	}

	return m.eventCh, nil
}

// Close는 이벤트 소스를 닫습니다.
func (m *MockEventSource) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.closed {
		m.closed = true
		close(m.eventCh)
	}

	return nil
}

// SendEvent는 이벤트를 전송합니다.
func (m *MockEventSource) SendEvent(event StorageEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.closed {
		m.eventCh <- event
		m.sendCounter++
	}
}

// GetSendCounter는 이벤트 전송 횟수를 반환합니다.
func (m *MockEventSource) GetSendCounter() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.sendCounter
}
