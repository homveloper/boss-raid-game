package storage

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/yourusername/eventsourced/pkg/event"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// EventSourcedStorage는 이벤트 소싱 기능이 추가된 Storage입니다.
type EventSourcedStorage struct {
	*Storage                // 기존 Storage 임베딩
	eventBus    event.EventBus    // 이벤트 발행을 위한 이벤트 버스
	eventMapper event.EventMapper // 문서 변경을 이벤트로 매핑하는 매퍼
}

// EventSourcedStorageOptions는 EventSourcedStorage 생성 옵션입니다.
type EventSourcedStorageOptions struct {
	StorageOptions *StorageOptions // 기존 Storage 옵션
	EventBus       event.EventBus        // 이벤트 버스
	EventMapper    event.EventMapper     // 이벤트 매퍼
}

// NewEventSourcedStorage는 새로운 EventSourcedStorage를 생성합니다.
func NewEventSourcedStorage(ctx context.Context, client *mongo.Client, dbName string, opts *EventSourcedStorageOptions) (*EventSourcedStorage, error) {
	if opts == nil {
		return nil, errors.New("options are required")
	}

	if opts.EventBus == nil {
		return nil, errors.New("event bus is required")
	}

	// 기본 Storage 생성
	storage, err := NewStorage(ctx, client, dbName, opts.StorageOptions)
	if err != nil {
		return nil, err
	}
	
	// EventMapper가 제공되지 않은 경우 기본 매퍼 사용
	eventMapper := opts.EventMapper
	if eventMapper == nil {
		eventMapper = event.NewDefaultEventMapper()
	}
	
	return &EventSourcedStorage{
		Storage:     storage,
		eventBus:    opts.EventBus,
		eventMapper: eventMapper,
	}, nil
}

// Update는 문서를 업데이트하고 이벤트를 발행합니다.
func (s *EventSourcedStorage) Update(ctx context.Context, collection string, id string, editFunc EditFunc) (*Diff, error) {
	// 기본 Storage의 Update 호출
	diff, err := s.Storage.Update(ctx, collection, id, editFunc)
	if err != nil {
		return nil, err
	}
	
	// 업데이트 성공 시 이벤트 매핑 및 발행
	events := s.eventMapper.MapToEvents(collection, id, diff)
	for _, evt := range events {
		if err := s.eventBus.PublishEvent(ctx, evt); err != nil {
			// 이벤트 발행 실패 로깅
			log.Printf("Failed to publish event: %v", err)
		}
	}
	
	return diff, nil
}

// FindOneAndUpdate는 문서를 찾아 업데이트하고 이벤트를 발행합니다.
func (s *EventSourcedStorage) FindOneAndUpdate(ctx context.Context, collection string, filter interface{}, update interface{}, opts ...*options.FindOneAndUpdateOptions) *mongo.SingleResult {
	// 업데이트 전 문서 ID 추출
	id, err := extractID(filter)
	if err != nil {
		// ID를 추출할 수 없는 경우 기본 동작 수행
		return s.Storage.FindOneAndUpdate(ctx, collection, filter, update, opts...)
	}
	
	// 업데이트 전 문서 버전 조회
	oldVersion, _ := s.GetVersion(ctx, collection, id)
	
	// 기본 Storage의 FindOneAndUpdate 호출
	result := s.Storage.FindOneAndUpdate(ctx, collection, filter, update, opts...)
	
	// 업데이트 성공 여부 확인
	var updatedDoc bson.M
	err = result.Decode(&updatedDoc)
	if err != nil {
		// 업데이트 실패 또는 문서가 없는 경우
		return result
	}
	
	// Diff 생성
	diff := &Diff{
		ID:         id,
		Collection: collection,
		IsNew:      false,
		HasChanges: true,
		Version:    oldVersion + 1,
		MergePatch: update,
	}
	
	// 이벤트 매핑 및 발행
	events := s.eventMapper.MapToEvents(collection, id, diff)
	for _, evt := range events {
		if err := s.eventBus.PublishEvent(ctx, evt); err != nil {
			// 이벤트 발행 실패 로깅
			log.Printf("Failed to publish event: %v", err)
		}
	}
	
	return result
}

// FindOneAndUpsert는 문서를 찾아 업서트하고 이벤트를 발행합니다.
func (s *EventSourcedStorage) FindOneAndUpsert(ctx context.Context, collection string, filter interface{}, update interface{}, opts ...*options.FindOneAndUpdateOptions) *mongo.SingleResult {
	// 업데이트 전 문서 ID 추출
	id, err := extractID(filter)
	if err != nil {
		// ID를 추출할 수 없는 경우 기본 동작 수행
		return s.Storage.FindOneAndUpsert(ctx, collection, filter, update, opts...)
	}
	
	// 문서 존재 여부 확인
	var existingDoc bson.M
	err = s.Storage.FindOne(ctx, collection, filter).Decode(&existingDoc)
	isNew := err == mongo.ErrNoDocuments
	
	// 업데이트 전 문서 버전 조회
	oldVersion := 0
	if !isNew {
		oldVersion, _ = s.GetVersion(ctx, collection, id)
	}
	
	// 기본 Storage의 FindOneAndUpsert 호출
	result := s.Storage.FindOneAndUpsert(ctx, collection, filter, update, opts...)
	
	// 업서트 성공 여부 확인
	var upsertedDoc bson.M
	err = result.Decode(&upsertedDoc)
	if err != nil {
		// 업서트 실패
		return result
	}
	
	// Diff 생성
	diff := &Diff{
		ID:         id,
		Collection: collection,
		IsNew:      isNew,
		HasChanges: true,
		Version:    oldVersion + 1,
		MergePatch: update,
	}
	
	// 이벤트 매핑 및 발행
	events := s.eventMapper.MapToEvents(collection, id, diff)
	for _, evt := range events {
		if err := s.eventBus.PublishEvent(ctx, evt); err != nil {
			// 이벤트 발행 실패 로깅
			log.Printf("Failed to publish event: %v", err)
		}
	}
	
	return result
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
	
	return "", errors.New("could not extract ID from filter")
}
