package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"tictactoe/transport/cqrs/domain"
	"time"

	"github.com/go-redis/redis/v8"
)

// EventHandler는 이벤트를 처리하는 핸들러 인터페이스입니다.
type EventHandler interface {
	// HandleEvent는 이벤트를 처리합니다.
	HandleEvent(ctx context.Context, event domain.Event) error
}

// EventBus는 이벤트를 발행하고 구독하는 인터페이스입니다.
type EventBus interface {
	// PublishEvent는 이벤트를 발행합니다.
	PublishEvent(ctx context.Context, event domain.Event) error

	// Subscribe는 이벤트 타입에 대한 핸들러를 등록합니다.
	Subscribe(eventType string, handler EventHandler) error

	// SubscribeAll은 모든 이벤트에 대한 핸들러를 등록합니다.
	SubscribeAll(handler EventHandler) error

	// Start는 이벤트 버스를 시작합니다.
	Start(ctx context.Context) error

	// Stop은 이벤트 버스를 중지합니다.
	Stop() error
}

// RedisEventBus는 Redis를 사용하는 EventBus 구현체입니다.
type RedisEventBus struct {
	client       *redis.Client
	streamName   string
	consumerName string
	groupName    string
	handlers     map[string][]EventHandler
	allHandlers  []EventHandler
	eventTypes   map[string]reflect.Type
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewRedisEventBus는 새로운 RedisEventBus를 생성합니다.
func NewRedisEventBus(
	client *redis.Client,
	streamName string,
	consumerName string,
	groupName string,
) *RedisEventBus {
	ctx, cancel := context.WithCancel(context.Background())

	bus := &RedisEventBus{
		client:       client,
		streamName:   streamName,
		consumerName: consumerName,
		groupName:    groupName,
		handlers:     make(map[string][]EventHandler),
		allHandlers:  []EventHandler{},
		eventTypes:   make(map[string]reflect.Type),
		ctx:          ctx,
		cancel:       cancel,
	}

	// 이벤트 타입 등록
	bus.RegisterEventType("TransportCreated", reflect.TypeOf(domain.TransportCreatedEvent{}))
	bus.RegisterEventType("TransportStarted", reflect.TypeOf(domain.TransportStartedEvent{}))
	bus.RegisterEventType("TransportCompleted", reflect.TypeOf(domain.TransportCompletedEvent{}))
	bus.RegisterEventType("TransportRaided", reflect.TypeOf(domain.TransportRaidedEvent{}))
	bus.RegisterEventType("TransportDefended", reflect.TypeOf(domain.TransportDefendedEvent{}))
	bus.RegisterEventType("TransportParticipantAdded", reflect.TypeOf(domain.TransportParticipantAddedEvent{}))
	bus.RegisterEventType("TransportRaidCompleted", reflect.TypeOf(domain.TransportRaidCompletedEvent{}))

	bus.RegisterEventType("RaidCreated", reflect.TypeOf(domain.RaidCreatedEvent{}))
	bus.RegisterEventType("RaidStarted", reflect.TypeOf(domain.RaidStartedEvent{}))
	bus.RegisterEventType("RaidSucceeded", reflect.TypeOf(domain.RaidSucceededEvent{}))
	bus.RegisterEventType("RaidFailed", reflect.TypeOf(domain.RaidFailedEvent{}))
	bus.RegisterEventType("RaidCanceled", reflect.TypeOf(domain.RaidCanceledEvent{}))

	return bus
}

// RegisterEventType은 이벤트 타입을 등록합니다.
func (b *RedisEventBus) RegisterEventType(eventType string, eventTypeObj reflect.Type) {
	b.eventTypes[eventType] = eventTypeObj
}

// PublishEvent는 이벤트를 발행합니다.
func (b *RedisEventBus) PublishEvent(ctx context.Context, event domain.Event) error {
	// 이벤트 직렬화
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// 이벤트 발행
	values := map[string]interface{}{
		"event_id":       event.AggregateID() + "-" + fmt.Sprintf("%d", event.Version()),
		"event_type":     event.EventType(),
		"aggregate_id":   event.AggregateID(),
		"aggregate_type": event.AggregateType(),
		"version":        event.Version(),
		"timestamp":      time.Now().UnixNano(),
		"payload":        string(payload),
	}

	// 그룹 채널에도 발행 (연합/길드 ID 기반)
	if event.EventType() == "TransportCreated" ||
		event.EventType() == "TransportStarted" ||
		event.EventType() == "TransportCompleted" ||
		event.EventType() == "TransportRaided" ||
		event.EventType() == "TransportDefended" {
		// 이벤트에서 AllianceID 추출
		var allianceID string
		switch e := event.(type) {
		case *domain.TransportCreatedEvent:
			allianceID = e.AllianceID
		}

		if allianceID != "" {
			groupStreamName := fmt.Sprintf("alliance:%s", allianceID)
			if err := b.client.XAdd(ctx, &redis.XAddArgs{
				Stream: groupStreamName,
				Values: values,
			}).Err(); err != nil {
				return fmt.Errorf("failed to publish event to group stream: %w", err)
			}
		}
	}

	// 글로벌 이벤트 스트림에 발행
	if err := b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: b.streamName,
		Values: values,
	}).Err(); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	return nil
}

// Subscribe는 이벤트 타입에 대한 핸들러를 등록합니다.
func (b *RedisEventBus) Subscribe(eventType string, handler EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.handlers[eventType]; !ok {
		b.handlers[eventType] = []EventHandler{}
	}

	b.handlers[eventType] = append(b.handlers[eventType], handler)
	return nil
}

// SubscribeAll은 모든 이벤트에 대한 핸들러를 등록합니다.
func (b *RedisEventBus) SubscribeAll(handler EventHandler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.allHandlers = append(b.allHandlers, handler)
	return nil
}

// Start는 이벤트 버스를 시작합니다.
func (b *RedisEventBus) Start(ctx context.Context) error {
	// 컨슈머 그룹 생성 (이미 존재하면 무시)
	err := b.client.XGroupCreateMkStream(ctx, b.streamName, b.groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	// 이벤트 처리 고루틴 시작
	b.wg.Add(1)
	go b.processEvents()

	return nil
}

// Stop은 이벤트 버스를 중지합니다.
func (b *RedisEventBus) Stop() error {
	b.cancel()
	b.wg.Wait()
	return nil
}

// processEvents는 이벤트를 처리하는 고루틴입니다.
func (b *RedisEventBus) processEvents() {
	defer b.wg.Done()

	for {
		select {
		case <-b.ctx.Done():
			return
		default:
			// Redis 스트림에서 새 이벤트 읽기
			streams, err := b.client.XReadGroup(b.ctx, &redis.XReadGroupArgs{
				Group:    b.groupName,
				Consumer: b.consumerName,
				Streams:  []string{b.streamName, ">"},
				Count:    10,
				Block:    time.Second,
			}).Result()
			if err != nil && err != redis.Nil {

				fmt.Printf("Error reading from stream: %v\n", err)
				time.Sleep(time.Second)
				continue
			}

			// 이벤트 처리
			for _, stream := range streams {
				for _, message := range stream.Messages {
					// 이벤트 역직렬화
					eventType := message.Values["event_type"].(string)
					eventTypeObj, ok := b.eventTypes[eventType]
					if !ok {
						fmt.Printf("Unknown event type: %s\n", eventType)
						continue
					}

					// 이벤트 생성
					event := reflect.New(eventTypeObj).Interface().(domain.Event)

					// 이벤트 역직렬화
					payload := message.Values["payload"].(string)
					if err := json.Unmarshal([]byte(payload), &event); err != nil {
						fmt.Printf("Failed to unmarshal event: %v\n", err)
						continue
					}

					// 이벤트 처리
					b.mu.RLock()
					handlers, ok := b.handlers[eventType]
					allHandlers := b.allHandlers
					b.mu.RUnlock()

					// 특정 이벤트 타입 핸들러 호출
					if ok {
						for _, handler := range handlers {
							if err := handler.HandleEvent(b.ctx, event); err != nil {
								fmt.Printf("Error handling event: %v\n", err)
							}
						}
					}

					// 모든 이벤트 핸들러 호출
					for _, handler := range allHandlers {
						if err := handler.HandleEvent(b.ctx, event); err != nil {
							fmt.Printf("Error handling event: %v\n", err)
						}
					}

					// 처리된 메시지 확인
					if err := b.client.XAck(b.ctx, b.streamName, b.groupName, message.ID).Err(); err != nil {
						fmt.Printf("Error acknowledging message: %v\n", err)
					}
				}
			}
		}
	}
}
