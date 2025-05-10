package eventsync

import (
	"context"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"nodestorage/v2"
)

// StorageEvent는 스토리지 이벤트를 나타냅니다.
type StorageEvent struct {
	ID        primitive.ObjectID
	Operation string
	Data      interface{}
	Diff      *nodestorage.Diff
	Version   int64
}

// EventSource는 이벤트 소스 인터페이스입니다.
type EventSource interface {
	// Watch는 이벤트를 감시하고 이벤트 채널을 반환합니다.
	Watch(ctx context.Context) (<-chan StorageEvent, error)
	
	// Close는 이벤트 소스를 닫습니다.
	Close() error
}
