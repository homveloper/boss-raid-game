package eventsync

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"nodestorage/v2"
)

// StorageEventData는 스토리지 이벤트 데이터를 위한 인터페이스입니다.
type StorageEventData interface {
	// GetID는 문서의 ID를 반환합니다.
	GetID() primitive.ObjectID
	
	// GetOperation은 수행된 작업(create, update, delete 등)을 반환합니다.
	GetOperation() string
	
	// GetDiff는 변경 사항을 반환합니다.
	GetDiff() *nodestorage.Diff
	
	// GetData는 문서 데이터를 반환합니다.
	GetData() interface{}
}

// CustomEvent는 스토리지 이벤트를 위한 구조체입니다.
type CustomEvent struct {
	ID        primitive.ObjectID `json:"id"`
	Operation string             `json:"operation"`
	Data      any                `json:"data"`
	Diff      *nodestorage.Diff  `json:"diff"`
}

// GetID는 문서의 ID를 반환합니다.
func (e *CustomEvent) GetID() primitive.ObjectID {
	return e.ID
}

// GetOperation은 수행된 작업을 반환합니다.
func (e *CustomEvent) GetOperation() string {
	return e.Operation
}

// GetDiff는 변경 사항을 반환합니다.
func (e *CustomEvent) GetDiff() *nodestorage.Diff {
	return e.Diff
}

// GetData는 문서 데이터를 반환합니다.
func (e *CustomEvent) GetData() interface{} {
	return e.Data
}
