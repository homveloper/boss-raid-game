package eventsync

import (
	"context"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"nodestorage/v2"
)

// EventSyncStorageOptions는 EventSyncStorage의 옵션을 정의합니다.
type EventSyncStorageOptions struct {
	// ClientID는 이벤트를 생성할 때 사용할 클라이언트 ID입니다.
	// 기본값은 "server"입니다.
	ClientID string

	// VectorClockManager는 벡터 시계를 관리하는 인터페이스입니다.
	// 제공되지 않으면 기본 구현이 사용됩니다.
	VectorClockManager VectorClockManager

	// InitialVectorClock은 초기 벡터 시계 값입니다.
	// 제공되지 않으면 빈 맵이 사용됩니다.
	InitialVectorClock map[string]int64
}

// VectorClockManager는 벡터 시계를 관리하는 인터페이스입니다.
type VectorClockManager interface {
	// GetVectorClock은 지정된 문서 ID에 대한 현재 벡터 시계를 반환합니다.
	GetVectorClock(ctx context.Context, documentID primitive.ObjectID) (map[string]int64, error)

	// UpdateVectorClock은 지정된 문서 ID에 대한 벡터 시계를 업데이트합니다.
	UpdateVectorClock(ctx context.Context, documentID primitive.ObjectID, clientID string, sequenceNum int64) error
}

// DefaultVectorClockManager는 기본 벡터 시계 관리자 구현체입니다.
type DefaultVectorClockManager struct {
	vectorClocks     map[string]map[string]int64
	vectorClockMutex sync.RWMutex
}

// NewDefaultVectorClockManager는 새로운 DefaultVectorClockManager를 생성합니다.
func NewDefaultVectorClockManager() *DefaultVectorClockManager {
	return &DefaultVectorClockManager{
		vectorClocks: make(map[string]map[string]int64),
	}
}

// GetVectorClock은 지정된 문서 ID에 대한 현재 벡터 시계를 반환합니다.
func (m *DefaultVectorClockManager) GetVectorClock(ctx context.Context, documentID primitive.ObjectID) (map[string]int64, error) {
	m.vectorClockMutex.RLock()
	defer m.vectorClockMutex.RUnlock()

	docKey := documentID.Hex()
	if vc, ok := m.vectorClocks[docKey]; ok {
		// 맵 복사본 반환
		result := make(map[string]int64)
		for k, v := range vc {
			result[k] = v
		}
		return result, nil
	}

	// 문서에 대한 벡터 시계가 없으면 빈 맵 반환
	return make(map[string]int64), nil
}

// UpdateVectorClock은 지정된 문서 ID에 대한 벡터 시계를 업데이트합니다.
func (m *DefaultVectorClockManager) UpdateVectorClock(ctx context.Context, documentID primitive.ObjectID, clientID string, sequenceNum int64) error {
	m.vectorClockMutex.Lock()
	defer m.vectorClockMutex.Unlock()

	docKey := documentID.Hex()
	if _, ok := m.vectorClocks[docKey]; !ok {
		m.vectorClocks[docKey] = make(map[string]int64)
	}

	// 현재 값보다 큰 경우에만 업데이트
	currentSeq := m.vectorClocks[docKey][clientID]
	if sequenceNum > currentSeq {
		m.vectorClocks[docKey][clientID] = sequenceNum
	}

	return nil
}

// EventSyncStorage는 nodestorage.Storage 인터페이스를 구현하여 기존 코드와의 호환성을 유지하면서
// nodestorage에서 생성된 Diff를 자동으로 이벤트 저장소에 저장하는 컴포넌트입니다.
type EventSyncStorage[T nodestorage.Cachable[T]] struct {
	storage            nodestorage.Storage[T] // 내부적으로 실제 nodestorage 인스턴스 사용
	eventStore         EventStore             // 이벤트 저장소
	logger             *zap.Logger
	clientID           string             // 이벤트를 생성할 때 사용할 클라이언트 ID
	vectorClockManager VectorClockManager // 벡터 시계 관리자
}

// NewEventSyncStorage는 새로운 EventSyncStorage 인스턴스를 생성합니다.
func NewEventSyncStorage[T nodestorage.Cachable[T]](
	storage nodestorage.Storage[T],
	eventStore EventStore,
	logger *zap.Logger,
	options ...func(*EventSyncStorageOptions),
) *EventSyncStorage[T] {
	// 기본 옵션 설정
	opts := &EventSyncStorageOptions{
		ClientID:           "server",
		VectorClockManager: NewDefaultVectorClockManager(),
		InitialVectorClock: make(map[string]int64),
	}

	// 사용자 옵션 적용
	for _, option := range options {
		option(opts)
	}

	return &EventSyncStorage[T]{
		storage:            storage,
		eventStore:         eventStore,
		logger:             logger,
		clientID:           opts.ClientID,
		vectorClockManager: opts.VectorClockManager,
	}
}

// WithClientID는 클라이언트 ID를 설정하는 옵션 함수입니다.
func WithClientID(clientID string) func(*EventSyncStorageOptions) {
	return func(opts *EventSyncStorageOptions) {
		opts.ClientID = clientID
	}
}

// WithVectorClockManager는 벡터 시계 관리자를 설정하는 옵션 함수입니다.
func WithVectorClockManager(manager VectorClockManager) func(*EventSyncStorageOptions) {
	return func(opts *EventSyncStorageOptions) {
		opts.VectorClockManager = manager
	}
}

// WithInitialVectorClock은 초기 벡터 시계를 설정하는 옵션 함수입니다.
func WithInitialVectorClock(vectorClock map[string]int64) func(*EventSyncStorageOptions) {
	return func(opts *EventSyncStorageOptions) {
		opts.InitialVectorClock = vectorClock
	}
}

// FindOne은 문서를 ID로 조회합니다.
func (s *EventSyncStorage[T]) FindOne(ctx context.Context, id primitive.ObjectID, opts ...*options.FindOneOptions) (T, error) {
	return s.storage.FindOne(ctx, id, opts...)
}

// FindMany는 필터에 맞는 여러 문서를 조회합니다.
func (s *EventSyncStorage[T]) FindMany(ctx context.Context, filter interface{}, opts ...*options.FindOptions) ([]T, error) {
	return s.storage.FindMany(ctx, filter, opts...)
}

// FindOneAndUpsert는 문서를 생성하거나 이미 존재하는 경우 반환합니다.
func (s *EventSyncStorage[T]) FindOneAndUpsert(ctx context.Context, data T) (T, error) {
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

	// 벡터 시계 가져오기
	vectorClock, err := s.vectorClockManager.GetVectorClock(ctx, v)
	if err != nil {
		s.logger.Error("Failed to get vector clock",
			zap.String("document_id", v.Hex()),
			zap.Error(err))
		vectorClock = make(map[string]int64)
	}

	// 현재 클라이언트의 시퀀스 번호 증가
	currentSeq := vectorClock[s.clientID]
	newSeq := currentSeq + 1
	vectorClock[s.clientID] = newSeq

	// 생성 이벤트 저장
	event := &Event{
		ID:          primitive.NewObjectID(),
		DocumentID:  v,
		Timestamp:   time.Now(),
		Operation:   "create",
		ClientID:    s.clientID,
		VectorClock: vectorClock,
		Metadata:    map[string]interface{}{"created_doc": doc},
	}

	if storeErr := s.eventStore.StoreEvent(ctx, event); storeErr != nil {
		s.logger.Error("Failed to store create event",
			zap.String("document_id", v.Hex()),
			zap.Error(storeErr))
	} else {
		// 이벤트 저장 성공 시 벡터 시계 업데이트
		if updateErr := s.vectorClockManager.UpdateVectorClock(ctx, v, s.clientID, newSeq); updateErr != nil {
			s.logger.Error("Failed to update vector clock",
				zap.String("document_id", v.Hex()),
				zap.Error(updateErr))
		}
	}

	return doc, nil
}

// FindOneAndUpdate는 문서를 수정하고 변경 사항을 이벤트로 저장합니다.
func (s *EventSyncStorage[T]) FindOneAndUpdate(ctx context.Context, id primitive.ObjectID, updateFn nodestorage.EditFunc[T], opts ...nodestorage.EditOption) (T, *nodestorage.Diff, error) {
	// 1. nodestorage의 FindOneAndUpdate 호출
	updatedDoc, diff, err := s.storage.FindOneAndUpdate(ctx, id, updateFn, opts...)

	// 2. 에러가 없고 변경사항이 있는 경우에만 이벤트 저장
	if err == nil && diff != nil && diff.HasChanges {
		// 벡터 시계 가져오기
		vectorClock, vcErr := s.vectorClockManager.GetVectorClock(ctx, id)
		if vcErr != nil {
			s.logger.Error("Failed to get vector clock",
				zap.String("document_id", id.Hex()),
				zap.Error(vcErr))
			vectorClock = make(map[string]int64)
		}

		// 현재 클라이언트의 시퀀스 번호 증가
		currentSeq := vectorClock[s.clientID]
		newSeq := currentSeq + 1
		vectorClock[s.clientID] = newSeq

		// 3. Diff를 이벤트로 변환
		event := &Event{
			ID:          primitive.NewObjectID(),
			DocumentID:  id,
			Timestamp:   time.Now(),
			Operation:   "update",
			Diff:        diff,
			ClientID:    s.clientID,
			VectorClock: vectorClock,
		}

		// 4. 이벤트 저장
		if storeErr := s.eventStore.StoreEvent(ctx, event); storeErr != nil {
			// 이벤트 저장 실패 로깅 (하지만 원래 작업은 성공했으므로 에러 반환하지 않음)
			s.logger.Error("Failed to store update event",
				zap.String("document_id", id.Hex()),
				zap.Error(storeErr))
		} else {
			// 이벤트 저장 성공 시 벡터 시계 업데이트
			if updateErr := s.vectorClockManager.UpdateVectorClock(ctx, id, s.clientID, newSeq); updateErr != nil {
				s.logger.Error("Failed to update vector clock",
					zap.String("document_id", id.Hex()),
					zap.Error(updateErr))
			}
		}
	}

	return updatedDoc, diff, err
}

// DeleteOne은 문서를 삭제하고 삭제 이벤트를 저장합니다.
func (s *EventSyncStorage[T]) DeleteOne(ctx context.Context, id primitive.ObjectID) error {
	// 1. 삭제 전 문서 조회 (이벤트에 포함시키기 위함)
	doc, err := s.storage.FindOne(ctx, id)
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

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

	// 벡터 시계 가져오기
	vectorClock, vcErr := s.vectorClockManager.GetVectorClock(ctx, id)
	if vcErr != nil {
		s.logger.Error("Failed to get vector clock",
			zap.String("document_id", id.Hex()),
			zap.Error(vcErr))
		vectorClock = make(map[string]int64)
	}

	// 현재 클라이언트의 시퀀스 번호 증가
	currentSeq := vectorClock[s.clientID]
	newSeq := currentSeq + 1
	vectorClock[s.clientID] = newSeq

	event := &Event{
		ID:          primitive.NewObjectID(),
		DocumentID:  id,
		Timestamp:   time.Now(),
		Operation:   "delete",
		ClientID:    s.clientID,
		VectorClock: vectorClock,
		Metadata:    metadata,
	}

	if storeErr := s.eventStore.StoreEvent(ctx, event); storeErr != nil {
		s.logger.Error("Failed to store delete event",
			zap.String("document_id", id.Hex()),
			zap.Error(storeErr))
	} else {
		// 이벤트 저장 성공 시 벡터 시계 업데이트
		if updateErr := s.vectorClockManager.UpdateVectorClock(ctx, id, s.clientID, newSeq); updateErr != nil {
			s.logger.Error("Failed to update vector clock",
				zap.String("document_id", id.Hex()),
				zap.Error(updateErr))
		}
	}

	return nil
}

// UpdateOne은 MongoDB 업데이트 연산자를 사용하여 문서를 수정합니다.
func (s *EventSyncStorage[T]) UpdateOne(ctx context.Context, id primitive.ObjectID, update bson.M, opts ...nodestorage.EditOption) (T, error) {
	// 1. 수정 전 문서 조회
	oldDoc, err := s.storage.FindOne(ctx, id)
	if err != nil {
		return oldDoc, err
	}

	// 2. 실제 수정 수행
	updatedDoc, err := s.storage.UpdateOne(ctx, id, update, opts...)
	if err != nil {
		return updatedDoc, err
	}

	// 3. 수정 이벤트 생성 및 저장
	// 벡터 시계 가져오기
	vectorClock, vcErr := s.vectorClockManager.GetVectorClock(ctx, id)
	if vcErr != nil {
		s.logger.Error("Failed to get vector clock",
			zap.String("document_id", id.Hex()),
			zap.Error(vcErr))
		vectorClock = make(map[string]int64)
	}

	// 현재 클라이언트의 시퀀스 번호 증가
	currentSeq := vectorClock[s.clientID]
	newSeq := currentSeq + 1
	vectorClock[s.clientID] = newSeq

	// 여기서는 Diff를 직접 생성하지 않고 메타데이터로 저장
	event := &Event{
		ID:          primitive.NewObjectID(),
		DocumentID:  id,
		Timestamp:   time.Now(),
		Operation:   "update",
		ClientID:    s.clientID,
		VectorClock: vectorClock,
		Metadata: map[string]interface{}{
			"update_operators": update,
			"before":           oldDoc,
			"after":            updatedDoc,
		},
	}

	if storeErr := s.eventStore.StoreEvent(ctx, event); storeErr != nil {
		s.logger.Error("Failed to store update event",
			zap.String("document_id", id.Hex()),
			zap.Error(storeErr))
	} else {
		// 이벤트 저장 성공 시 벡터 시계 업데이트
		if updateErr := s.vectorClockManager.UpdateVectorClock(ctx, id, s.clientID, newSeq); updateErr != nil {
			s.logger.Error("Failed to update vector clock",
				zap.String("document_id", id.Hex()),
				zap.Error(updateErr))
		}
	}

	return updatedDoc, err
}

// UpdateOneWithPipeline은 MongoDB 집계 파이프라인을 사용하여 문서를 수정합니다.
func (s *EventSyncStorage[T]) UpdateOneWithPipeline(ctx context.Context, id primitive.ObjectID, pipeline mongo.Pipeline, opts ...nodestorage.EditOption) (T, error) {
	// 1. 수정 전 문서 조회
	oldDoc, err := s.storage.FindOne(ctx, id)
	if err != nil {
		return oldDoc, err
	}

	// 2. 실제 수정 수행
	updatedDoc, err := s.storage.UpdateOneWithPipeline(ctx, id, pipeline, opts...)
	if err != nil {
		return updatedDoc, err
	}

	// 3. 수정 이벤트 생성 및 저장
	// 벡터 시계 가져오기
	vectorClock, vcErr := s.vectorClockManager.GetVectorClock(ctx, id)
	if vcErr != nil {
		s.logger.Error("Failed to get vector clock",
			zap.String("document_id", id.Hex()),
			zap.Error(vcErr))
		vectorClock = make(map[string]int64)
	}

	// 현재 클라이언트의 시퀀스 번호 증가
	currentSeq := vectorClock[s.clientID]
	newSeq := currentSeq + 1
	vectorClock[s.clientID] = newSeq

	event := &Event{
		ID:          primitive.NewObjectID(),
		DocumentID:  id,
		Timestamp:   time.Now(),
		Operation:   "update",
		ClientID:    s.clientID,
		VectorClock: vectorClock,
		Metadata: map[string]interface{}{
			"pipeline": pipeline,
			"before":   oldDoc,
			"after":    updatedDoc,
		},
	}

	if storeErr := s.eventStore.StoreEvent(ctx, event); storeErr != nil {
		s.logger.Error("Failed to store update event",
			zap.String("document_id", id.Hex()),
			zap.Error(storeErr))
	} else {
		// 이벤트 저장 성공 시 벡터 시계 업데이트
		if updateErr := s.vectorClockManager.UpdateVectorClock(ctx, id, s.clientID, newSeq); updateErr != nil {
			s.logger.Error("Failed to update vector clock",
				zap.String("document_id", id.Hex()),
				zap.Error(updateErr))
		}
	}

	return updatedDoc, err
}

// UpdateSection은 문서의 특정 섹션을 수정합니다.
func (s *EventSyncStorage[T]) UpdateSection(ctx context.Context, id primitive.ObjectID, sectionPath string, updateFn func(interface{}) (interface{}, error), opts ...nodestorage.EditOption) (T, error) {
	// 1. 수정 전 문서 조회
	oldDoc, err := s.storage.FindOne(ctx, id)
	if err != nil {
		return oldDoc, err
	}

	// 2. 실제 수정 수행
	updatedDoc, err := s.storage.UpdateSection(ctx, id, sectionPath, updateFn, opts...)
	if err != nil {
		return updatedDoc, err
	}

	// 3. 수정 이벤트 생성 및 저장
	// 벡터 시계 가져오기
	vectorClock, vcErr := s.vectorClockManager.GetVectorClock(ctx, id)
	if vcErr != nil {
		s.logger.Error("Failed to get vector clock",
			zap.String("document_id", id.Hex()),
			zap.Error(vcErr))
		vectorClock = make(map[string]int64)
	}

	// 현재 클라이언트의 시퀀스 번호 증가
	currentSeq := vectorClock[s.clientID]
	newSeq := currentSeq + 1
	vectorClock[s.clientID] = newSeq

	event := &Event{
		ID:          primitive.NewObjectID(),
		DocumentID:  id,
		Timestamp:   time.Now(),
		Operation:   "update_section",
		ClientID:    s.clientID,
		VectorClock: vectorClock,
		Metadata: map[string]interface{}{
			"section_path": sectionPath,
			"before":       oldDoc,
			"after":        updatedDoc,
		},
	}

	if storeErr := s.eventStore.StoreEvent(ctx, event); storeErr != nil {
		s.logger.Error("Failed to store section update event",
			zap.String("document_id", id.Hex()),
			zap.String("section_path", sectionPath),
			zap.Error(storeErr))
	} else {
		// 이벤트 저장 성공 시 벡터 시계 업데이트
		if updateErr := s.vectorClockManager.UpdateVectorClock(ctx, id, s.clientID, newSeq); updateErr != nil {
			s.logger.Error("Failed to update vector clock",
				zap.String("document_id", id.Hex()),
				zap.Error(updateErr))
		}
	}

	return updatedDoc, err
}

// WithTransaction은 MongoDB 트랜잭션 내에서 함수를 실행합니다.
func (s *EventSyncStorage[T]) WithTransaction(ctx context.Context, fn func(sessCtx mongo.SessionContext) error) error {
	return s.storage.WithTransaction(ctx, fn)
}

// Watch는 문서 변경을 감시합니다.
func (s *EventSyncStorage[T]) Watch(ctx context.Context, pipeline mongo.Pipeline, opts ...*options.ChangeStreamOptions) (<-chan nodestorage.WatchEvent[T], error) {
	return s.storage.Watch(ctx, pipeline, opts...)
}

// Collection은 기본 MongoDB 컬렉션을 반환합니다.
func (s *EventSyncStorage[T]) Collection() *mongo.Collection {
	return s.storage.Collection()
}

// Close는 스토리지를 닫습니다.
func (s *EventSyncStorage[T]) Close() error {
	return s.storage.Close()
}
