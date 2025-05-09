package eventsync

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"

	"nodestorage/v2"
)

// SyncClient 인터페이스는 동기화 클라이언트의 기능을 정의합니다.
type SyncClient interface {
	// SendEvent는 클라이언트에 이벤트를 전송합니다.
	SendEvent(event *Event) error

	// SendEvents는 클라이언트에 여러 이벤트를 전송합니다.
	SendEvents(events []*Event) error

	// Close는 클라이언트 연결을 닫습니다.
	Close() error

	// GetClientID는 클라이언트 ID를 반환합니다.
	GetClientID() string

	// GetDocumentID는 문서 ID를 반환합니다.
	GetDocumentID() primitive.ObjectID
}

// SyncService 인터페이스는 동기화 서비스의 기능을 정의합니다.
type SyncService interface {
	// RegisterClient는 클라이언트를 등록합니다.
	RegisterClient(client SyncClient) error

	// UnregisterClient는 클라이언트 등록을 해제합니다.
	UnregisterClient(clientID string, documentID primitive.ObjectID) error

	// SyncClient는 클라이언트를 동기화합니다.
	SyncClient(ctx context.Context, clientID string, documentID primitive.ObjectID, vectorClock map[string]int64) error

	// BroadcastEvent는 이벤트를 모든 클라이언트에 브로드캐스트합니다.
	BroadcastEvent(ctx context.Context, event *Event) error

	// HandleStorageEvent는 nodestorage 이벤트를 처리합니다.
	HandleStorageEvent(ctx context.Context, event nodestorage.WatchEvent[any]) error

	// Close는 동기화 서비스를 닫습니다.
	Close() error
}

// SyncServiceImpl은 동기화 서비스 구현체입니다.
type SyncServiceImpl struct {
	eventStore        EventStore
	stateVectorManager StateVectorManager
	clients           map[string]map[string]SyncClient // documentID -> clientID -> client
	clientsMutex      sync.RWMutex
	logger            *zap.Logger
	ctx               context.Context
	cancel            context.CancelFunc
}

// NewSyncService는 새로운 동기화 서비스를 생성합니다.
func NewSyncService(eventStore EventStore, stateVectorManager StateVectorManager, logger *zap.Logger) *SyncServiceImpl {
	ctx, cancel := context.WithCancel(context.Background())
	return &SyncServiceImpl{
		eventStore:        eventStore,
		stateVectorManager: stateVectorManager,
		clients:           make(map[string]map[string]SyncClient),
		logger:            logger,
		ctx:               ctx,
		cancel:            cancel,
	}
}

// RegisterClient는 클라이언트를 등록합니다.
func (s *SyncServiceImpl) RegisterClient(client SyncClient) error {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	clientID := client.GetClientID()
	documentID := client.GetDocumentID()
	documentIDStr := documentID.Hex()

	// 문서별 클라이언트 맵 초기화
	if _, ok := s.clients[documentIDStr]; !ok {
		s.clients[documentIDStr] = make(map[string]SyncClient)
	}

	// 클라이언트 등록
	s.clients[documentIDStr][clientID] = client

	s.logger.Info("Client registered",
		zap.String("client_id", clientID),
		zap.String("document_id", documentIDStr))

	return nil
}

// UnregisterClient는 클라이언트 등록을 해제합니다.
func (s *SyncServiceImpl) UnregisterClient(clientID string, documentID primitive.ObjectID) error {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	documentIDStr := documentID.Hex()

	// 문서별 클라이언트 맵 확인
	if clients, ok := s.clients[documentIDStr]; ok {
		// 클라이언트 등록 해제
		if client, ok := clients[clientID]; ok {
			client.Close()
			delete(clients, clientID)
			s.logger.Info("Client unregistered",
				zap.String("client_id", clientID),
				zap.String("document_id", documentIDStr))
		}

		// 문서에 클라이언트가 없으면 맵에서 제거
		if len(clients) == 0 {
			delete(s.clients, documentIDStr)
		}
	}

	return nil
}

// SyncClient는 클라이언트를 동기화합니다.
func (s *SyncServiceImpl) SyncClient(ctx context.Context, clientID string, documentID primitive.ObjectID, vectorClock map[string]int64) error {
	// 누락된 이벤트 조회
	events, err := s.stateVectorManager.GetMissingEvents(ctx, clientID, documentID, vectorClock)
	if err != nil {
		return fmt.Errorf("failed to get missing events: %w", err)
	}

	// 클라이언트 조회
	s.clientsMutex.RLock()
	client, ok := s.clients[documentID.Hex()][clientID]
	s.clientsMutex.RUnlock()

	if !ok {
		return fmt.Errorf("client not found: %s", clientID)
	}

	// 이벤트 전송
	if len(events) > 0 {
		if err := client.SendEvents(events); err != nil {
			return fmt.Errorf("failed to send events: %w", err)
		}

		s.logger.Debug("Events sent to client",
			zap.String("client_id", clientID),
			zap.String("document_id", documentID.Hex()),
			zap.Int("event_count", len(events)))
	}

	// 벡터 시계 업데이트
	newVectorClock := make(map[string]int64)
	for _, event := range events {
		for clientID, seq := range event.VectorClock {
			if currentSeq, ok := newVectorClock[clientID]; !ok || seq > currentSeq {
				newVectorClock[clientID] = seq
			}
		}
	}

	if len(newVectorClock) > 0 {
		if err := s.stateVectorManager.UpdateVectorClock(ctx, clientID, documentID, newVectorClock); err != nil {
			s.logger.Warn("Failed to update vector clock",
				zap.String("client_id", clientID),
				zap.String("document_id", documentID.Hex()),
				zap.Error(err))
		}
	}

	return nil
}

// BroadcastEvent는 이벤트를 모든 클라이언트에 브로드캐스트합니다.
func (s *SyncServiceImpl) BroadcastEvent(ctx context.Context, event *Event) error {
	documentIDStr := event.DocumentID.Hex()

	// 이벤트 저장
	if err := s.eventStore.StoreEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}

	// 클라이언트 조회
	s.clientsMutex.RLock()
	clients, ok := s.clients[documentIDStr]
	s.clientsMutex.RUnlock()

	if !ok {
		// 클라이언트가 없으면 브로드캐스트 건너뛰기
		return nil
	}

	// 모든 클라이언트에 이벤트 전송
	for clientID, client := range clients {
		// 이벤트를 생성한 클라이언트는 제외
		if clientID == event.ClientID {
			continue
		}

		if err := client.SendEvent(event); err != nil {
			s.logger.Warn("Failed to send event to client",
				zap.String("client_id", clientID),
				zap.String("document_id", documentIDStr),
				zap.Error(err))
		}
	}

	return nil
}

// HandleStorageEvent는 nodestorage 이벤트를 처리합니다.
func (s *SyncServiceImpl) HandleStorageEvent(ctx context.Context, storageEvent nodestorage.WatchEvent[any]) error {
	// 이벤트 변환
	event := &Event{
		DocumentID:  storageEvent.ID,
		Timestamp:   time.Now(),
		Operation:   storageEvent.Operation,
		Diff:        storageEvent.Diff,
		VectorClock: make(map[string]int64),
		ClientID:    "server", // 서버에서 생성된 이벤트
		Metadata:    make(map[string]interface{}),
	}

	// 메타데이터 추가
	if data, err := json.Marshal(storageEvent.Data); err == nil {
		event.Metadata["data"] = string(data)
	}

	// 이벤트 브로드캐스트
	return s.BroadcastEvent(ctx, event)
}

// Close는 동기화 서비스를 닫습니다.
func (s *SyncServiceImpl) Close() error {
	s.cancel()

	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	// 모든 클라이언트 연결 종료
	for _, clients := range s.clients {
		for _, client := range clients {
			client.Close()
		}
	}

	// 맵 초기화
	s.clients = make(map[string]map[string]SyncClient)

	return nil
}
