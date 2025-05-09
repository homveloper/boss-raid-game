package eventsync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// SSEClient 구조체는 SSE(Server-Sent Events) 기반 동기화 클라이언트 구현체입니다.
type SSEClient struct {
	writer     http.ResponseWriter
	flusher    http.Flusher
	clientID   string
	documentID primitive.ObjectID
	service    SyncService
	logger     *zap.Logger
	mutex      sync.Mutex
	closed     bool
	ctx        context.Context
	cancel     context.CancelFunc
	eventCh    chan *Event
}

// NewSSEClient는 새로운 SSE 클라이언트를 생성합니다.
func NewSSEClient(w http.ResponseWriter, clientID string, documentID primitive.ObjectID, service SyncService, logger *zap.Logger) (*SSEClient, error) {
	// Flusher 인터페이스 확인
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &SSEClient{
		writer:     w,
		flusher:    flusher,
		clientID:   clientID,
		documentID: documentID,
		service:    service,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		eventCh:    make(chan *Event, 100),
	}, nil
}

// Start는 클라이언트 처리를 시작합니다.
func (c *SSEClient) Start() {
	// 헤더 설정
	c.writer.Header().Set("Content-Type", "text/event-stream")
	c.writer.Header().Set("Cache-Control", "no-cache")
	c.writer.Header().Set("Connection", "keep-alive")
	c.writer.Header().Set("Access-Control-Allow-Origin", "*")

	// 클라이언트 등록
	if err := c.service.RegisterClient(c); err != nil {
		c.logger.Error("Failed to register client",
			zap.String("client_id", c.clientID),
			zap.String("document_id", c.documentID.Hex()),
			zap.Error(err))
		c.Close()
		return
	}

	// 연결 성공 메시지 전송
	connectMsg := struct {
		Type      string `json:"type"`
		ClientID  string `json:"clientId"`
		Timestamp int64  `json:"timestamp"`
	}{
		Type:      "connected",
		ClientID:  c.clientID,
		Timestamp: time.Now().UnixNano() / int64(time.Millisecond),
	}

	if err := c.sendSSEEvent("connect", connectMsg); err != nil {
		c.logger.Error("Failed to send connect message",
			zap.String("client_id", c.clientID),
			zap.String("document_id", c.documentID.Hex()),
			zap.Error(err))
		c.Close()
		return
	}

	// 이벤트 전송 루프 시작
	go c.eventLoop()
}

// eventLoop는 이벤트 전송 루프입니다.
func (c *SSEClient) eventLoop() {
	defer c.Close()

	for {
		select {
		case <-c.ctx.Done():
			return
		case event := <-c.eventCh:
			if err := c.sendSSEEvent("event", event); err != nil {
				c.logger.Error("Failed to send event",
					zap.String("client_id", c.clientID),
					zap.String("document_id", c.documentID.Hex()),
					zap.Error(err))
				return
			}
		}
	}
}

// SendEvent는 클라이언트에 이벤트를 전송합니다.
func (c *SSEClient) SendEvent(event *Event) error {
	select {
	case c.eventCh <- event:
		return nil
	default:
		return fmt.Errorf("event channel is full")
	}
}

// SendEvents는 클라이언트에 여러 이벤트를 전송합니다.
func (c *SSEClient) SendEvents(events []*Event) error {
	// 이벤트 배치 전송
	eventsMsg := struct {
		Type   string   `json:"type"`
		Events []*Event `json:"events"`
	}{
		Type:   "events",
		Events: events,
	}

	return c.sendSSEEvent("events", eventsMsg)
}

// sendSSEEvent는 SSE 이벤트를 전송합니다.
func (c *SSEClient) sendSSEEvent(eventType string, data interface{}) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	// 데이터 직렬화
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// 이벤트 전송
	fmt.Fprintf(c.writer, "event: %s\n", eventType)
	fmt.Fprintf(c.writer, "data: %s\n\n", dataBytes)
	c.flusher.Flush()

	return nil
}

// Close는 클라이언트 연결을 닫습니다.
func (c *SSEClient) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	c.cancel()

	// 클라이언트 등록 해제
	c.service.UnregisterClient(c.clientID, c.documentID)

	// 채널 닫기
	close(c.eventCh)

	return nil
}

// GetClientID는 클라이언트 ID를 반환합니다.
func (c *SSEClient) GetClientID() string {
	return c.clientID
}

// GetDocumentID는 문서 ID를 반환합니다.
func (c *SSEClient) GetDocumentID() primitive.ObjectID {
	return c.documentID
}

// SSEHandler 구조체는 SSE 핸들러입니다.
type SSEHandler struct {
	service      SyncService
	logger       *zap.Logger
	clientIDFunc func(*http.Request) string
}

// NewSSEHandler는 새로운 SSE 핸들러를 생성합니다.
func NewSSEHandler(service SyncService, logger *zap.Logger) *SSEHandler {
	return &SSEHandler{
		service: service,
		logger:  logger,
		clientIDFunc: func(r *http.Request) string {
			return r.URL.Query().Get("clientId")
		},
	}
}

// ServeHTTP는 HTTP 요청을 처리합니다.
func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 문서 ID 파싱
	documentIDStr := r.URL.Query().Get("documentId")
	if documentIDStr == "" {
		http.Error(w, "documentId is required", http.StatusBadRequest)
		return
	}

	documentID, err := primitive.ObjectIDFromHex(documentIDStr)
	if err != nil {
		http.Error(w, "invalid documentId", http.StatusBadRequest)
		return
	}

	// 클라이언트 ID 가져오기
	clientID := h.clientIDFunc(r)
	if clientID == "" {
		clientID = fmt.Sprintf("client-%d", time.Now().UnixNano())
	}

	// SSE 클라이언트 생성
	client, err := NewSSEClient(w, clientID, documentID, h.service, h.logger)
	if err != nil {
		h.logger.Error("Failed to create SSE client",
			zap.Error(err))
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// 클라이언트 시작
	client.Start()

	// 클라이언트가 연결을 종료할 때까지 대기
	<-r.Context().Done()
	client.Close()
}
