package eventsync

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// StorageListener 구조체는 스토리지 이벤트를 수신하고 처리합니다.
type StorageListener struct {
	eventSource EventSource
	syncService SyncService
	logger      *zap.Logger
	ctx         context.Context
	cancel      context.CancelFunc
	watchCh     <-chan StorageEvent
	wg          sync.WaitGroup

	// 이벤트 중복 처리 방지를 위한 필드
	processedEvents     sync.Map // 처리된 이벤트 ID를 저장하는 맵
	processedEventsMu   sync.Mutex
	lastProcessedEvents map[string]int64 // 문서별 마지막으로 처리된 이벤트 버전
}

// NewStorageListener는 새로운 스토리지 리스너를 생성합니다.
func NewStorageListener(eventSource EventSource, syncService SyncService, logger *zap.Logger) *StorageListener {
	ctx, cancel := context.WithCancel(context.Background())
	return &StorageListener{
		eventSource:         eventSource,
		syncService:         syncService,
		logger:              logger,
		ctx:                 ctx,
		cancel:              cancel,
		lastProcessedEvents: make(map[string]int64),
	}
}

// Start는 스토리지 리스너를 시작합니다.
func (l *StorageListener) Start() error {
	// 이벤트 소스에서 이벤트 채널 가져오기
	watchCh, err := l.eventSource.Watch(l.ctx)
	if err != nil {
		return fmt.Errorf("failed to watch event source: %w", err)
	}

	l.watchCh = watchCh

	// 이벤트 처리 고루틴 시작
	l.wg.Add(1)
	go l.processEvents()

	l.logger.Info("Storage listener started")
	return nil
}

// processEvents는 스토리지 이벤트를 처리합니다.
func (l *StorageListener) processEvents() {
	defer l.wg.Done()

	for {
		select {
		case <-l.ctx.Done():
			return
		case event, ok := <-l.watchCh:
			if !ok {
				l.logger.Warn("Watch channel closed")
				return
			}

			// 이벤트 처리
			if err := l.handleEvent(event); err != nil {
				l.logger.Error("Failed to handle storage event",
					zap.String("document_id", event.ID.Hex()),
					zap.String("operation", event.Operation),
					zap.Error(err))
			}
		}
	}
}

// handleEvent는 스토리지 이벤트를 처리합니다.
func (l *StorageListener) handleEvent(event StorageEvent) error {
	// 이벤트 중복 처리 방지
	// 이벤트 고유 식별자 생성 (문서 ID + 작업 + 버전)
	version := event.Version

	// 이벤트 식별자 생성
	eventKey := fmt.Sprintf("%s:%s:%d", event.ID.Hex(), event.Operation, version)

	// 이미 처리된 이벤트인지 확인
	if _, loaded := l.processedEvents.LoadOrStore(eventKey, true); loaded {
		l.logger.Debug("Skipping duplicate event",
			zap.String("document_id", event.ID.Hex()),
			zap.String("operation", event.Operation),
			zap.Int64("version", version),
			zap.String("event_key", eventKey))
		return nil
	}

	l.logger.Debug("Processing event",
		zap.String("document_id", event.ID.Hex()),
		zap.String("operation", event.Operation),
		zap.Int64("version", version),
		zap.String("event_key", eventKey))

	// 동기화 서비스에 이벤트 전달
	// StorageEventData 인터페이스를 구현하는 CustomEvent 구조체 생성
	customEvent := &CustomEvent{
		ID:        event.ID,
		Operation: event.Operation,
		Data:      event.Data,
		Diff:      event.Diff,
	}

	return l.syncService.HandleStorageEvent(l.ctx, customEvent)
}

// Stop은 스토리지 리스너를 중지합니다.
func (l *StorageListener) Stop() {
	l.cancel()
	l.wg.Wait()
	l.logger.Info("Storage listener stopped")
}
