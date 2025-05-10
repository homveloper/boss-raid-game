package eventsync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// SyncService 인터페이스는 동기화 서비스의 기능을 정의합니다.
type SyncService interface {
	// GetMissingEvents는 클라이언트가 누락한 이벤트를 조회합니다.
	GetMissingEvents(ctx context.Context, clientID string, documentID primitive.ObjectID, vectorClock map[string]int64) ([]*Event, error)

	// UpdateVectorClock은 클라이언트의 벡터 시계를 업데이트합니다.
	UpdateVectorClock(ctx context.Context, clientID string, documentID primitive.ObjectID, vectorClock map[string]int64) error

	// StoreEvent는 이벤트를 저장합니다.
	StoreEvent(ctx context.Context, event *Event) error

	// HandleStorageEvent는 nodestorage 이벤트를 처리합니다.
	HandleStorageEvent(ctx context.Context, event StorageEventData) error

	// Close는 동기화 서비스를 닫습니다.
	Close() error
}

// SyncServiceImpl은 동기화 서비스 구현체입니다.
type SyncServiceImpl struct {
	eventStore         EventStore
	stateVectorManager StateVectorManager
	logger             *zap.Logger
	ctx                context.Context
	cancel             context.CancelFunc
}

// NewSyncService는 새로운 동기화 서비스를 생성합니다.
func NewSyncService(eventStore EventStore, stateVectorManager StateVectorManager, logger *zap.Logger) *SyncServiceImpl {
	ctx, cancel := context.WithCancel(context.Background())
	return &SyncServiceImpl{
		eventStore:         eventStore,
		stateVectorManager: stateVectorManager,
		logger:             logger,
		ctx:                ctx,
		cancel:             cancel,
	}
}

// GetMissingEvents는 클라이언트가 누락한 이벤트를 조회합니다.
func (s *SyncServiceImpl) GetMissingEvents(ctx context.Context, clientID string, documentID primitive.ObjectID, vectorClock map[string]int64) ([]*Event, error) {
	// 누락된 이벤트 조회
	events, err := s.stateVectorManager.GetMissingEvents(ctx, clientID, documentID, vectorClock)
	if err != nil {
		return nil, fmt.Errorf("failed to get missing events: %w", err)
	}

	s.logger.Debug("Missing events retrieved",
		zap.String("client_id", clientID),
		zap.String("document_id", documentID.Hex()),
		zap.Int("event_count", len(events)))

	return events, nil
}

// UpdateVectorClock은 클라이언트의 벡터 시계를 업데이트합니다.
func (s *SyncServiceImpl) UpdateVectorClock(ctx context.Context, clientID string, documentID primitive.ObjectID, vectorClock map[string]int64) error {
	if err := s.stateVectorManager.UpdateVectorClock(ctx, clientID, documentID, vectorClock); err != nil {
		return fmt.Errorf("failed to update vector clock: %w", err)
	}

	s.logger.Debug("Vector clock updated",
		zap.String("client_id", clientID),
		zap.String("document_id", documentID.Hex()))

	return nil
}

// StoreEvent는 이벤트를 저장합니다.
func (s *SyncServiceImpl) StoreEvent(ctx context.Context, event *Event) error {
	if err := s.eventStore.StoreEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}

	s.logger.Debug("Event stored",
		zap.String("document_id", event.DocumentID.Hex()),
		zap.String("operation", event.Operation))

	return nil
}

// HandleStorageEvent는 nodestorage 이벤트를 처리합니다.
func (s *SyncServiceImpl) HandleStorageEvent(ctx context.Context, eventData StorageEventData) error {
	// 문서 ID와 작업 유형 로깅
	s.logger.Debug("Handling storage event",
		zap.String("document_id", eventData.GetID().Hex()),
		zap.String("operation", eventData.GetOperation()))

	// 이벤트 생성
	event := &Event{
		ID:          primitive.NewObjectID(),
		DocumentID:  eventData.GetID(),
		Timestamp:   time.Now(),
		Operation:   eventData.GetOperation(),
		Diff:        eventData.GetDiff(),
		VectorClock: map[string]int64{"server": 1}, // 서버 이벤트는 항상 서버 벡터 시계 사용
		ClientID:    "server",                      // 서버에서 생성된 이벤트
		Metadata:    make(map[string]interface{}),
	}

	// 메타데이터 추가
	data := eventData.GetData()
	if data != nil {
		if jsonData, err := json.Marshal(data); err == nil {
			event.Metadata["data"] = string(jsonData)
		}
	}

	// 이벤트 저장
	return s.StoreEvent(ctx, event)
}

// Close는 동기화 서비스를 닫습니다.
func (s *SyncServiceImpl) Close() error {
	s.cancel()
	return nil
}
