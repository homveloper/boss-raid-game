package eventsync

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// WebSocketMessage 구조체는 WebSocket 메시지를 나타냅니다.
type WebSocketMessage struct {
	Type        string           `json:"type"`
	ClientID    string           `json:"clientId,omitempty"`
	DocumentID  string           `json:"documentId,omitempty"`
	VectorClock map[string]int64 `json:"vectorClock,omitempty"`
	Events      []*Event         `json:"events,omitempty"`
	Error       string           `json:"error,omitempty"`
}

// WebSocketClient 구조체는 WebSocket 기반 동기화 클라이언트 구현체입니다.
type WebSocketClient struct {
	conn       *websocket.Conn
	clientID   string
	documentID primitive.ObjectID
	service    SyncService
	logger     *zap.Logger
	mutex      sync.Mutex
	closed     bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewWebSocketClient는 새로운 WebSocket 클라이언트를 생성합니다.
func NewWebSocketClient(conn *websocket.Conn, clientID string, documentID primitive.ObjectID, service SyncService, logger *zap.Logger) *WebSocketClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &WebSocketClient{
		conn:       conn,
		clientID:   clientID,
		documentID: documentID,
		service:    service,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start는 클라이언트 처리를 시작합니다.
func (c *WebSocketClient) Start() {
	// 클라이언트 등록
	if err := c.service.RegisterClient(c); err != nil {
		c.logger.Error("Failed to register client",
			zap.String("client_id", c.clientID),
			zap.String("document_id", c.documentID.Hex()),
			zap.Error(err))
		c.Close()
		return
	}

	// 메시지 수신 루프 시작
	go c.receiveLoop()
}

// receiveLoop는 WebSocket 메시지 수신 루프입니다.
func (c *WebSocketClient) receiveLoop() {
	defer c.Close()

	for {
		// 컨텍스트 취소 확인
		select {
		case <-c.ctx.Done():
			return
		default:
			// 계속 진행
		}

		// 메시지 수신
		_, msgBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Warn("WebSocket read error",
					zap.String("client_id", c.clientID),
					zap.String("document_id", c.documentID.Hex()),
					zap.Error(err))
			}
			return
		}

		// 메시지 파싱
		var msg WebSocketMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			c.logger.Warn("Failed to parse WebSocket message",
				zap.String("client_id", c.clientID),
				zap.String("document_id", c.documentID.Hex()),
				zap.Error(err))
			continue
		}

		// 메시지 처리
		if err := c.handleMessage(&msg); err != nil {
			c.logger.Warn("Failed to handle WebSocket message",
				zap.String("client_id", c.clientID),
				zap.String("document_id", c.documentID.Hex()),
				zap.String("message_type", msg.Type),
				zap.Error(err))

			// 에러 메시지 전송
			errMsg := WebSocketMessage{
				Type:  "error",
				Error: err.Error(),
			}
			c.sendMessage(&errMsg)
		}
	}
}

// handleMessage는 WebSocket 메시지를 처리합니다.
func (c *WebSocketClient) handleMessage(msg *WebSocketMessage) error {
	switch msg.Type {
	case "sync":
		// 동기화 요청 처리
		return c.service.SyncClient(c.ctx, c.clientID, c.documentID, msg.VectorClock)

	default:
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

// SendEvent는 클라이언트에 이벤트를 전송합니다.
func (c *WebSocketClient) SendEvent(event *Event) error {
	msg := WebSocketMessage{
		Type:   "event",
		Events: []*Event{event},
	}
	return c.sendMessage(&msg)
}

// SendEvents는 클라이언트에 여러 이벤트를 전송합니다.
func (c *WebSocketClient) SendEvents(events []*Event) error {
	msg := WebSocketMessage{
		Type:   "events",
		Events: events,
	}
	return c.sendMessage(&msg)
}

// sendMessage는 WebSocket 메시지를 전송합니다.
func (c *WebSocketClient) sendMessage(msg *WebSocketMessage) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return fmt.Errorf("client is closed")
	}

	// 메시지 직렬화
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// 메시지 전송
	if err := c.conn.WriteMessage(websocket.TextMessage, msgBytes); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// Close는 클라이언트 연결을 닫습니다.
func (c *WebSocketClient) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	c.cancel()

	// 클라이언트 등록 해제
	c.service.UnregisterClient(c.clientID, c.documentID)

	// 연결 종료
	return c.conn.Close()
}

// GetClientID는 클라이언트 ID를 반환합니다.
func (c *WebSocketClient) GetClientID() string {
	return c.clientID
}

// GetDocumentID는 문서 ID를 반환합니다.
func (c *WebSocketClient) GetDocumentID() primitive.ObjectID {
	return c.documentID
}

// WebSocketHandler 구조체는 WebSocket 핸들러입니다.
type WebSocketHandler struct {
	service      SyncService
	logger       *zap.Logger
	upgrader     websocket.Upgrader
	clientIDFunc func(*http.Request) string
}

// NewWebSocketHandler는 새로운 WebSocket 핸들러를 생성합니다.
func NewWebSocketHandler(service SyncService, logger *zap.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		service: service,
		logger:  logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // 모든 오리진 허용
			},
		},
		clientIDFunc: func(r *http.Request) string {
			return r.URL.Query().Get("clientId")
		},
	}
}

// ServeHTTP는 HTTP 요청을 처리합니다.
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	// WebSocket 연결 업그레이드
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade connection",
			zap.Error(err))
		return
	}

	// 클라이언트 생성 및 시작
	client := NewWebSocketClient(conn, clientID, documentID, h.service, h.logger)
	client.Start()
}
