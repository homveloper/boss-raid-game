package eventsync

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"nodestorage/v2"
)

// StorageAdapter는 nodestorage.Storage를 EventSource로 변환하는 어댑터입니다.
type StorageAdapter[T nodestorage.Cachable[T]] struct {
	storage nodestorage.Storage[T]
	logger  *zap.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewStorageAdapter는 새로운 StorageAdapter를 생성합니다.
func NewStorageAdapter[T nodestorage.Cachable[T]](storage nodestorage.Storage[T], logger *zap.Logger) *StorageAdapter[T] {
	ctx, cancel := context.WithCancel(context.Background())
	return &StorageAdapter[T]{
		storage: storage,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Watch는 이벤트를 감시하고 이벤트 채널을 반환합니다.
func (a *StorageAdapter[T]) Watch(ctx context.Context) (<-chan StorageEvent, error) {
	// 변경 감시 시작
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "operationType", Value: bson.D{{Key: "$in", Value: bson.A{"insert", "update", "delete"}}}}}}},
	}
	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)

	// nodestorage의 Watch 메서드 호출
	watchCh, err := a.storage.Watch(ctx, pipeline, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to watch storage: %w", err)
	}

	// StorageEvent 채널 생성
	eventCh := make(chan StorageEvent, 100)

	// 이벤트 변환 고루틴 시작
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer close(eventCh)

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watchCh:
				if !ok {
					a.logger.Warn("Watch channel closed")
					return
				}

				// 버전 추출
				var version int64
				v := reflect.ValueOf(event.Data)
				if !v.IsValid() {
					// Data가 nil인 경우 (삭제 이벤트 등)
					version = 0
				} else if v.Kind() == reflect.Ptr && !v.IsNil() {
					v = v.Elem()

					if v.Kind() == reflect.Struct {
						versionField := v.FieldByName("Version")
						if versionField.IsValid() && versionField.CanInt() {
							version = versionField.Int()
						}
					}
				}

				// StorageEvent 생성
				storageEvent := StorageEvent{
					ID:        event.ID,
					Operation: event.Operation,
					Data:      event.Data,
					Diff:      event.Diff,
					Version:   version,
				}

				// 이벤트 전송
				select {
				case eventCh <- storageEvent:
					// 이벤트 전송 성공
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return eventCh, nil
}

// Close는 이벤트 소스를 닫습니다.
func (a *StorageAdapter[T]) Close() error {
	a.cancel()
	a.wg.Wait()
	return nil
}
