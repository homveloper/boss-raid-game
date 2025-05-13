package eventsync

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"nodestorage/v2"
)

// EventSourcedStorage는 이벤트 소싱 패턴을 구현한 저장소입니다.
// nodestorage.Storage 인터페이스와 유사한 메서드 이름을 사용하지만, 클라이언트 ID를 추가 인자로 받는 이벤트 소싱에 특화된 인터페이스를 제공합니다.
type EventSourcedStorage[T nodestorage.Cachable[T]] struct {
	storage    nodestorage.Storage[T] // 내부적으로 실제 nodestorage 인스턴스 사용
	eventStore EventStore             // 이벤트 저장소
	logger     *zap.Logger
}

// EventSourcedStorageOptions는 EventSourcedStorage의 옵션을 정의합니다.
type EventSourcedStorageOptions struct {
	// 향후 확장을 위한 옵션 필드들이 여기에 추가될 수 있습니다.
}

// NewEventSourcedStorage는 새로운 EventSourcedStorage 인스턴스를 생성합니다.
func NewEventSourcedStorage[T nodestorage.Cachable[T]](
	storage nodestorage.Storage[T],
	eventStore EventStore,
	logger *zap.Logger,
	options ...func(*EventSourcedStorageOptions),
) *EventSourcedStorage[T] {
	// 기본 옵션 설정
	opts := &EventSourcedStorageOptions{}

	// 사용자 옵션 적용
	for _, option := range options {
		option(opts)
	}

	return &EventSourcedStorage[T]{
		storage:    storage,
		eventStore: eventStore,
		logger:     logger,
	}
}

// FindOne은 문서를 ID로 조회합니다.
func (s *EventSourcedStorage[T]) FindOne(ctx context.Context, id primitive.ObjectID, opts ...*options.FindOneOptions) (T, error) {
	return s.storage.FindOne(ctx, id, opts...)
}

// FindMany는 필터에 맞는 여러 문서를 조회합니다.
func (s *EventSourcedStorage[T]) FindMany(ctx context.Context, filter interface{}, opts ...*options.FindOptions) ([]T, error) {
	return s.storage.FindMany(ctx, filter, opts...)
}

// FindOneAndUpsert는 문서를 생성하거나 이미 존재하는 경우 반환하고 이벤트를 저장합니다.
func (s *EventSourcedStorage[T]) FindOneAndUpsert(ctx context.Context, data T, clientID string) (T, error) {
	doc, err := s.storage.FindOneAndUpsert(ctx, data)
	if err != nil {
		return doc, err
	}

	// 문서 ID 추출
	v := primitive.ObjectID{}
	docValue := GetDocumentID(doc)
	if docValue.IsValid() {
		v = docValue.Interface().(primitive.ObjectID)
	}

	// 버전 필드 이름 가져오기
	versionField := s.storage.VersionField()

	// 문서의 버전 가져오기
	version, err := nodestorage.GetVersion(doc, versionField)
	if err != nil {
		s.logger.Error("Failed to get document version",
			zap.String("document_id", v.Hex()),
			zap.Error(err))
		version = 1 // 기본값 사용
	}

	// 생성 이벤트 저장
	event := &Event{
		ID:         primitive.NewObjectID(),
		DocumentID: v,
		Timestamp:  time.Now(),
		Operation:  "create",
		ClientID:   clientID,
		ServerSeq:  version,
		Metadata:   map[string]interface{}{"created_doc": doc},
	}

	if storeErr := s.eventStore.StoreEvent(ctx, event); storeErr != nil {
		s.logger.Error("Failed to store create event",
			zap.String("document_id", v.Hex()),
			zap.Error(storeErr))
		return doc, fmt.Errorf("document created but failed to store event: %w", storeErr)
	}

	return doc, nil
}

// FindOneAndUpdate는 문서를 수정하고 이벤트를 저장합니다.
func (s *EventSourcedStorage[T]) FindOneAndUpdate(ctx context.Context, id primitive.ObjectID, updateFn nodestorage.EditFunc[T], clientID string) (T, *nodestorage.Diff, error) {
	// 1. nodestorage의 FindOneAndUpdate 호출
	updatedDoc, diff, err := s.storage.FindOneAndUpdate(ctx, id, updateFn)
	if err != nil {
		return updatedDoc, diff, err
	}

	// 2. 변경사항이 있는 경우에만 이벤트 저장
	if diff != nil && diff.HasChanges {
		// 3. Diff를 이벤트로 변환 (Diff의 Version 필드 사용)
		event := &Event{
			ID:         primitive.NewObjectID(),
			DocumentID: id,
			Timestamp:  time.Now(),
			Operation:  "update",
			Diff:       diff,
			ClientID:   clientID,
			ServerSeq:  diff.Version,
		}

		// 4. 이벤트 저장
		if storeErr := s.eventStore.StoreEvent(ctx, event); storeErr != nil {
			s.logger.Error("Failed to store update event",
				zap.String("document_id", id.Hex()),
				zap.Error(storeErr))
			return updatedDoc, diff, fmt.Errorf("document updated but failed to store event: %w", storeErr)
		}
	}

	return updatedDoc, diff, nil
}

// DeleteOne은 문서를 삭제하고 이벤트를 저장합니다.
func (s *EventSourcedStorage[T]) DeleteOne(ctx context.Context, id primitive.ObjectID, clientID string) error {
	// 1. 삭제 전 문서 조회 (이벤트에 포함시키기 위함)
	doc, err := s.storage.FindOne(ctx, id)
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	// 버전 필드 이름 가져오기
	versionField := s.storage.VersionField()

	// 문서의 버전 가져오기
	var version int64 = 0
	if err != mongo.ErrNoDocuments {
		version, err = nodestorage.GetVersion(doc, versionField)
		if err != nil {
			s.logger.Error("Failed to get document version",
				zap.String("document_id", id.Hex()),
				zap.Error(err))
			// 오류가 있어도 계속 진행
		}
	}

	// 다음 버전 계산
	nextVersion := version + 1

	// 2. 실제 삭제 수행
	err = s.storage.DeleteOne(ctx, id)
	if err != nil {
		return err
	}

	// 3. 삭제 이벤트 생성 및 저장
	metadata := make(map[string]interface{})
	if err != mongo.ErrNoDocuments {
		metadata["deleted_doc"] = doc
	}

	event := &Event{
		ID:         primitive.NewObjectID(),
		DocumentID: id,
		Timestamp:  time.Now(),
		Operation:  "delete",
		ClientID:   clientID,
		ServerSeq:  nextVersion,
		Metadata:   metadata,
	}

	if storeErr := s.eventStore.StoreEvent(ctx, event); storeErr != nil {
		s.logger.Error("Failed to store delete event",
			zap.String("document_id", id.Hex()),
			zap.Error(storeErr))
		return fmt.Errorf("document deleted but failed to store event: %w", storeErr)
	}

	return nil
}

// GetEvents는 지정된 문서 ID에 대한 이벤트를 조회합니다.
func (s *EventSourcedStorage[T]) GetEvents(ctx context.Context, documentID primitive.ObjectID, afterVersion int64) ([]*Event, error) {
	return s.eventStore.GetEventsAfterVersion(ctx, documentID, afterVersion)
}

// GetMissingEvents는 클라이언트의 마지막 버전 이후의 이벤트를 조회합니다.
func (s *EventSourcedStorage[T]) GetMissingEvents(ctx context.Context, documentID primitive.ObjectID, afterVersion int64) ([]*Event, error) {
	return s.GetEvents(ctx, documentID, afterVersion)
}

// Close는 스토리지를 닫습니다.
func (s *EventSourcedStorage[T]) Close() error {
	return s.storage.Close()
}
