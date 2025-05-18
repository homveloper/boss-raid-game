package storage

import (
	"context"
	"fmt"

	"eventsourced/pkg/common"
	"eventsourced/pkg/event"

	"github.com/yourusername/nodestorage/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NodeStorageAdapter는 nodestorage/v2 패키지를 eventsourced에서 사용할 수 있도록 하는 어댑터입니다.
type NodeStorageAdapter struct {
	storage     *nodestorage.Storage
	eventBus    event.EventBus
	eventMapper event.EventMapper
}

// NodeStorageAdapterOptions는 NodeStorageAdapter 생성 옵션입니다.
type NodeStorageAdapterOptions struct {
	EventBus    event.EventBus
	EventMapper event.EventMapper
}

// NewNodeStorageAdapter는 새로운 NodeStorageAdapter를 생성합니다.
func NewNodeStorageAdapter(storage *nodestorage.Storage, opts *NodeStorageAdapterOptions) (*NodeStorageAdapter, error) {
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

	return &NodeStorageAdapter{
		storage:     storage,
		eventBus:    opts.EventBus,
		eventMapper: eventMapper,
	}, nil
}

// Update는 문서를 업데이트하고 이벤트를 발행합니다.
func (a *NodeStorageAdapter) Update(ctx context.Context, collection string, id string, editFunc interface{}) (*common.Diff, error) {
	// 제네릭 EditFunc를 nodestorage에 맞게 변환
	genericEditFunc := func(doc interface{}) (interface{}, error) {
		if editFunc, ok := editFunc.(func(interface{}) (interface{}, error)); ok {
			return editFunc(doc)
		}
		return nil, fmt.Errorf("invalid edit function type")
	}

	// nodestorage의 Update 호출
	diff, err := a.storage.Update(ctx, collection, id, genericEditFunc)
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

// FindOne은 단일 문서를 조회합니다.
func (a *NodeStorageAdapter) FindOne(ctx context.Context, collection string, filter interface{}) *mongo.SingleResult {
	return a.storage.FindOne(ctx, collection, filter)
}

// FindOneAndUpdate는 문서를 찾아 업데이트하고 이벤트를 발행합니다.
func (a *NodeStorageAdapter) FindOneAndUpdate(ctx context.Context, collection string, filter interface{}, update interface{}, opts ...*options.FindOneAndUpdateOptions) *mongo.SingleResult {
	// 업데이트 전 문서 ID 추출
	id, err := extractID(filter)
	if err != nil {
		// ID를 추출할 수 없는 경우 기본 동작 수행
		return a.storage.FindOneAndUpdate(ctx, collection, filter, update, opts...)
	}

	// 업데이트 전 문서 버전 조회
	oldVersion, _ := a.GetVersion(ctx, collection, id)

	// nodestorage의 FindOneAndUpdate 호출
	result := a.storage.FindOneAndUpdate(ctx, collection, filter, update, opts...)

	// 업데이트 성공 여부 확인
	var updatedDoc bson.M
	err = result.Decode(&updatedDoc)
	if err != nil {
		// 업데이트 실패 또는 문서가 없는 경우
		return result
	}

	// Diff 생성
	diff := &common.Diff{
		ID:         id,
		Collection: collection,
		IsNew:      false,
		HasChanges: true,
		Version:    oldVersion + 1,
		MergePatch: update,
	}

	// 이벤트 매핑 및 발행
	events := a.eventMapper.MapToEvents(collection, id, diff)
	for _, evt := range events {
		if err := a.eventBus.PublishEvent(ctx, evt); err != nil {
			// 이벤트 발행 실패 로깅
			fmt.Printf("Failed to publish event: %v\n", err)
		}
	}

	return result
}

// FindOneAndUpsert는 문서를 찾아 업서트하고 이벤트를 발행합니다.
func (a *NodeStorageAdapter) FindOneAndUpsert(ctx context.Context, collection string, filter interface{}, update interface{}, opts ...*options.FindOneAndUpdateOptions) *mongo.SingleResult {
	// 업데이트 전 문서 ID 추출
	id, err := extractID(filter)
	if err != nil {
		// ID를 추출할 수 없는 경우 기본 동작 수행
		return a.storage.FindOneAndUpsert(ctx, collection, filter, update, opts...)
	}

	// 문서 존재 여부 확인
	var existingDoc bson.M
	err = a.storage.FindOne(ctx, collection, filter).Decode(&existingDoc)
	isNew := err == mongo.ErrNoDocuments

	// 업데이트 전 문서 버전 조회
	oldVersion := 0
	if !isNew {
		oldVersion, _ = a.GetVersion(ctx, collection, id)
	}

	// nodestorage의 FindOneAndUpsert 호출
	result := a.storage.FindOneAndUpsert(ctx, collection, filter, update, opts...)

	// 업서트 성공 여부 확인
	var upsertedDoc bson.M
	err = result.Decode(&upsertedDoc)
	if err != nil {
		// 업서트 실패
		return result
	}

	// Diff 생성
	diff := &common.Diff{
		ID:         id,
		Collection: collection,
		IsNew:      isNew,
		HasChanges: true,
		Version:    oldVersion + 1,
		MergePatch: update,
	}

	// 이벤트 매핑 및 발행
	events := a.eventMapper.MapToEvents(collection, id, diff)
	for _, evt := range events {
		if err := a.eventBus.PublishEvent(ctx, evt); err != nil {
			// 이벤트 발행 실패 로깅
			fmt.Printf("Failed to publish event: %v\n", err)
		}
	}

	return result
}

// GetVersion은 문서의 현재 버전을 조회합니다.
func (a *NodeStorageAdapter) GetVersion(ctx context.Context, collection string, id string) (int, error) {
	var result struct {
		Version int `bson:"version"`
	}

	err := a.storage.FindOne(
		ctx,
		collection,
		bson.M{"_id": id},
		options.FindOne().SetProjection(bson.M{"version": 1}),
	).Decode(&result)

	if err != nil {
		if err == mongo.ErrNoDocuments {
			return 0, nil
		}
		return 0, err
	}

	return result.Version, nil
}

// extractID는 필터에서 ID를 추출합니다.
func extractID(filter interface{}) (string, error) {
	switch f := filter.(type) {
	case bson.M:
		if id, ok := f["_id"]; ok {
			return fmt.Sprintf("%v", id), nil
		}
	case map[string]interface{}:
		if id, ok := f["_id"]; ok {
			return fmt.Sprintf("%v", id), nil
		}
	}

	return "", fmt.Errorf("could not extract ID from filter")
}
