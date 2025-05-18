package event

import (
	"context"
	"log"
	"sync"
)

// InMemoryEventBus는 메모리 기반 이벤트 버스 구현입니다.
type InMemoryEventBus struct {
	handlers     map[string][]EventHandler
	allHandlers  []EventHandler
	mutex        sync.RWMutex
}

// NewInMemoryEventBus는 새로운 InMemoryEventBus를 생성합니다.
func NewInMemoryEventBus() *InMemoryEventBus {
	return &InMemoryEventBus{
		handlers:    make(map[string][]EventHandler),
		allHandlers: make([]EventHandler, 0),
	}
}

// PublishEvent는 이벤트를 발행합니다.
func (b *InMemoryEventBus) PublishEvent(ctx context.Context, event Event) error {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	
	// 특정 이벤트 타입에 등록된 핸들러 호출
	eventType := event.EventType()
	if handlers, ok := b.handlers[eventType]; ok {
		for _, handler := range handlers {
			if err := handler.HandleEvent(ctx, event); err != nil {
				// 오류 로깅 후 계속 진행
				log.Printf("Error handling event %s: %v", eventType, err)
			}
		}
	}
	
	// 모든 이벤트를 구독하는 핸들러 호출
	for _, handler := range b.allHandlers {
		if err := handler.HandleEvent(ctx, event); err != nil {
			// 오류 로깅 후 계속 진행
			log.Printf("Error handling event %s: %v", eventType, err)
		}
	}
	
	return nil
}

// Subscribe는 특정 이벤트 타입에 핸들러를 등록합니다.
func (b *InMemoryEventBus) Subscribe(eventType string, handler EventHandler) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	
	if _, ok := b.handlers[eventType]; !ok {
		b.handlers[eventType] = make([]EventHandler, 0)
	}
	
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// SubscribeAll은 모든 이벤트에 핸들러를 등록합니다.
func (b *InMemoryEventBus) SubscribeAll(handler EventHandler) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	
	b.allHandlers = append(b.allHandlers, handler)
}

// AsyncInMemoryEventBus는 비동기 메모리 기반 이벤트 버스 구현입니다.
type AsyncInMemoryEventBus struct {
	*InMemoryEventBus
	queue   chan Event
	workers int
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewAsyncInMemoryEventBus는 새로운 AsyncInMemoryEventBus를 생성합니다.
func NewAsyncInMemoryEventBus(workers int, queueSize int) *AsyncInMemoryEventBus {
	if workers <= 0 {
		workers = 1
	}
	
	if queueSize <= 0 {
		queueSize = 100
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	bus := &AsyncInMemoryEventBus{
		InMemoryEventBus: NewInMemoryEventBus(),
		queue:            make(chan Event, queueSize),
		workers:          workers,
		ctx:              ctx,
		cancel:           cancel,
	}
	
	// 워커 시작
	for i := 0; i < workers; i++ {
		go bus.worker()
	}
	
	return bus
}

// worker는 이벤트 처리 워커입니다.
func (b *AsyncInMemoryEventBus) worker() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case event := <-b.queue:
			// 부모 클래스의 PublishEvent 호출
			if err := b.InMemoryEventBus.PublishEvent(context.Background(), event); err != nil {
				log.Printf("Error publishing event: %v", err)
			}
		}
	}
}

// PublishEvent는 이벤트를 비동기적으로 발행합니다.
func (b *AsyncInMemoryEventBus) PublishEvent(ctx context.Context, event Event) error {
	select {
	case b.queue <- event:
		return nil
	default:
		// 큐가 가득 찬 경우 동기적으로 처리
		log.Printf("Event queue is full, processing synchronously")
		return b.InMemoryEventBus.PublishEvent(ctx, event)
	}
}

// Stop은 비동기 이벤트 버스를 중지합니다.
func (b *AsyncInMemoryEventBus) Stop() {
	b.cancel()
	close(b.queue)
}
