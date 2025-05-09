package eventsync

import (
	"context"
	"fmt"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"nodestorage/v2"
)

// StorageListener 구조체는 nodestorage 이벤트를 수신하고 처리합니다.
type StorageListener[T any] struct {
	storage     nodestorage.Storage[T]
	syncService SyncService
	logger      *zap.Logger
	ctx         context.Context
	cancel      context.CancelFunc
	watchCh     <-chan nodestorage.WatchEvent[T]
	wg          sync.WaitGroup
}

// NewStorageListener는 새로운 스토리지 리스너를 생성합니다.
func NewStorageListener[T any](storage nodestorage.Storage[T], syncService SyncService, logger *zap.Logger) *StorageListener[T] {
	ctx, cancel := context.WithCancel(context.Background())
	return &StorageListener[T]{
		storage:     storage,
		syncService: syncService,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start는 스토리지 리스너를 시작합니다.
func (l *StorageListener[T]) Start() error {
	// 변경 감시 시작
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "operationType", Value: bson.D{{Key: "$in", Value: bson.A{"insert", "update", "delete"}}}}}}},
	}
	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)

	watchCh, err := l.storage.Watch(l.ctx, pipeline, opts)
	if err != nil {
		return fmt.Errorf("failed to watch storage: %w", err)
	}

	l.watchCh = watchCh

	// 이벤트 처리 고루틴 시작
	l.wg.Add(1)
	go l.processEvents()

	l.logger.Info("Storage listener started")
	return nil
}

// processEvents는 스토리지 이벤트를 처리합니다.
func (l *StorageListener[T]) processEvents() {
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
func (l *StorageListener[T]) handleEvent(event nodestorage.WatchEvent[T]) error {
	// 이벤트를 any 타입으로 변환
	anyEvent := nodestorage.WatchEvent[any]{
		ID:        event.ID,
		Operation: event.Operation,
		Data:      event.Data,
		Diff:      event.Diff,
	}

	// 동기화 서비스에 이벤트 전달
	return l.syncService.HandleStorageEvent(l.ctx, anyEvent)
}

// Stop은 스토리지 리스너를 중지합니다.
func (l *StorageListener[T]) Stop() {
	l.cancel()
	l.wg.Wait()
	l.logger.Info("Storage listener stopped")
}
