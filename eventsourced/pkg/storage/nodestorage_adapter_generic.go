package storage

import (
	"context"
	"fmt"
	"reflect"

	"eventsourced/pkg/common"
	"eventsourced/pkg/event"

	"github.com/yourusername/nodestorage/v2"
	"go.mongodb.org/mongo-driver/bson"
)

// NodeStorageGenericAdapter는 nodestorage/v2 패키지의 제네릭 기능을 활용하는 어댑터입니다.
type NodeStorageGenericAdapter[T any] struct {
	storage     *nodestorage.Storage
	eventBus    event.EventBus
	eventMapper event.EventMapper
}

// NewNodeStorageGenericAdapter는 새로운 NodeStorageGenericAdapter를 생성합니다.
func NewNodeStorageGenericAdapter[T any](storage *nodestorage.Storage, opts *NodeStorageAdapterOptions) (*NodeStorageGenericAdapter[T], error) {
	if storage == nil {
		return nil, fmt.Errorf("storage cannot be nil")
	}

	if opts == nil {
		return nil, fmt.Errorf("options cannot be nil")
	}

	if opts.EventBus == nil {
		return nil, fmt.Errorf("event bus cannot be nil")
	}

	eventMapper := opts.EventMapper
	if eventMapper == nil {
		eventMapper = event.NewDefaultEventMapper()
	}

	return &NodeStorageGenericAdapter[T]{
		storage:     storage,
		eventBus:    opts.EventBus,
		eventMapper: eventMapper,
	}, nil
}

// Update는 문서를 업데이트하고 이벤트를 발행합니다.
func (a *NodeStorageGenericAdapter[T]) Update(ctx context.Context, collection string, id string, editFunc func(doc T) (T, error)) (*common.Diff, error) {
	// nodestorage의 Update 호출
	diff, err := a.storage.Update(ctx, collection, id, editFunc)
	if err != nil {
		return nil, err
	}

	// nodestorage의 Diff를 common.Diff로 변환
	commonDiff := &common.Diff{
		ID:         diff.ID,
		Collection: collection,
		IsNew:      diff.IsNew,
		HasChanges: diff.HasChanges,
		Version:    diff.Version,
		MergePatch: diff.MergePatch,
		BsonPatch:  diff.BsonPatch,
	}

	// 이벤트 매핑 및 발행
	events := a.eventMapper.MapToEvents(collection, id, commonDiff)
	for _, evt := range events {
		if err := a.eventBus.PublishEvent(ctx, evt); err != nil {
			// 이벤트 발행 실패 로깅
			fmt.Printf("Failed to publish event: %v\n", err)
		}
	}

	return commonDiff, nil
}

// CreateDocument는 새 문서를 생성하고 이벤트를 발행합니다.
func (a *NodeStorageGenericAdapter[T]) CreateDocument(ctx context.Context, collection string, id string, doc T) (*common.Diff, error) {
	// 문서 ID 설정
	setValue(doc, "_id", id)

	// 버전 초기화
	setValue(doc, "version", 1)

	// 문서 저장
	_, err := a.storage.Collection(collection).InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}

	// Diff 생성
	diff := &common.Diff{
		ID:         id,
		Collection: collection,
		IsNew:      true,
		HasChanges: true,
		Version:    1,
		MergePatch: doc,
	}

	// 이벤트 매핑 및 발행
	events := a.eventMapper.MapToEvents(collection, id, diff)
	for _, evt := range events {
		if err := a.eventBus.PublishEvent(ctx, evt); err != nil {
			// 이벤트 발행 실패 로깅
			fmt.Printf("Failed to publish event: %v\n", err)
		}
	}

	return diff, nil
}

// GetDocument는 문서를 조회합니다.
func (a *NodeStorageGenericAdapter[T]) GetDocument(ctx context.Context, collection string, id string) (T, error) {
	var result T
	err := a.storage.FindOne(ctx, collection, bson.M{"_id": id}).Decode(&result)
	return result, err
}

// setValue는 리플렉션을 사용하여 구조체 필드 값을 설정합니다.
func setValue(obj interface{}, fieldName string, value interface{}) {
	// 포인터인 경우 역참조
	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// 구조체가 아닌 경우 처리
	if val.Kind() != reflect.Struct {
		// 맵인 경우 직접 설정
		if val.Kind() == reflect.Map {
			val.SetMapIndex(reflect.ValueOf(fieldName), reflect.ValueOf(value))
		}
		return
	}

	// 필드 찾기
	field := val.FieldByName(fieldName)
	if !field.IsValid() {
		// 필드를 찾을 수 없는 경우 (대소문자 구분)
		for i := 0; i < val.NumField(); i++ {
			typeField := val.Type().Field(i)
			// bson 태그 확인
			tag := typeField.Tag.Get("bson")
			if tag == fieldName {
				field = val.Field(i)
				break
			}
		}
	}

	// 필드를 찾았고 설정 가능한 경우
	if field.IsValid() && field.CanSet() {
		field.Set(reflect.ValueOf(value))
	}
}
