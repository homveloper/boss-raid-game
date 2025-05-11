package server

import (
	"encoding/json"
	"fmt"
	"idledungeon/pkg/utils"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// SSEClient represents a connected SSE client
type SSEClient struct {
	ID        string
	GameID    string
	Writer    http.ResponseWriter
	Flusher   http.Flusher
	CreatedAt time.Time
}

// handleSSE handles the /events endpoint for Server-Sent Events
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	// 패닉 복구
	defer func() {
		if err := recover(); err != nil {
			stack := string(debug.Stack())
			if s.logger != nil {
				s.logger.Error("SSE handler panic",
					zap.Any("error", err),
					zap.String("stack", stack),
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.String("remoteAddr", r.RemoteAddr),
					zap.String("userAgent", r.UserAgent()),
				)
			}

			// 콘솔에도 출력
			fmt.Printf("SSE handler panic: %v\nStack: %s\n", err, stack)
		}
	}()

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		r := utils.WithErrorAndCodeAndMessage(r, fmt.Errorf("flusher not available"), http.StatusInternalServerError, "Flusher not available")
		utils.WriteError(w, r)
		return
	}

	// Get client info from query params
	clientID := r.URL.Query().Get("clientId")
	gameID := r.URL.Query().Get("gameId")

	if clientID == "" {
		http.Error(w, "Client ID is required", http.StatusBadRequest)
		return
	}

	// 로그 추가
	if s.logger != nil {
		s.logger.Debug("SSE connection established",
			zap.String("clientId", clientID),
			zap.String("gameId", gameID),
			zap.String("remoteAddr", r.RemoteAddr),
		)
	}

	// Create client
	client := &SSEClient{
		ID:        clientID,
		GameID:    gameID,
		Writer:    w,
		Flusher:   flusher,
		CreatedAt: time.Now(),
	}

	// 클라이언트를 SSE 매니저에 추가 (nil 체크)
	if s.sseManager != nil {
		s.sseManager.AddClient(client)

		// 함수 종료 시 클라이언트 제거
		defer s.sseManager.RemoveClient(clientID)
	} else {
		if s.logger != nil {
			s.logger.Error("SSE manager is nil")
		}
		fmt.Println("SSE manager is nil")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// 안전하게 이벤트 전송 시도
	err := s.trySendSSEEvent(client, "connected", map[string]interface{}{
		"message":  "Connected to event stream",
		"clientId": clientID,
		"gameId":   gameID,
		"time":     time.Now().Format(time.RFC3339),
	})

	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to send initial SSE event", zap.Error(err))
		}
		fmt.Printf("Failed to send initial SSE event: %v\n", err)
		return
	}

	// 게임 ID가 있으면 게임 상태 전송
	if gameID != "" && s.storage != nil {
		// 게임 ID를 ObjectID로 변환
		gameObjID, err := primitive.ObjectIDFromHex(gameID)
		if err == nil {
			// 게임 상태 조회
			game, err := s.storage.GetGame(r.Context(), gameObjID)
			if err == nil {
				// 게임 상태 전송
				err = s.trySendSSEEvent(client, "game_update", map[string]interface{}{
					"game": game,
				})
				if err != nil && s.logger != nil {
					s.logger.Error("Failed to send game state", zap.Error(err))
				}
			} else if s.logger != nil {
				s.logger.Error("Failed to get game", zap.Error(err), zap.String("gameId", gameID))
			}
		} else if s.logger != nil {
			s.logger.Error("Invalid game ID", zap.Error(err), zap.String("gameId", gameID))
		}
	}

	// Create a channel to notify the handler when the client disconnects
	clientGone := r.Context().Done()

	// 하트비트 타이머 생성
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	// Keep the connection open
	for {
		select {
		case <-clientGone:
			// Client disconnected
			if s.logger != nil {
				s.logger.Debug("Client disconnected", zap.String("clientId", clientID))
			}
			return
		case <-heartbeatTicker.C:
			// Send a heartbeat
			err := s.trySendSSEEvent(client, "heartbeat", map[string]interface{}{
				"time": time.Now().Format(time.RFC3339),
			})
			if err != nil {
				if s.logger != nil {
					s.logger.Error("Failed to send heartbeat", zap.Error(err))
				}
				fmt.Printf("Failed to send heartbeat: %v\n", err)
				return
			}
		}
	}
}

// sendSSEEvent sends an event to an SSE client
func (s *Server) sendSSEEvent(client *SSEClient, eventType string, data interface{}) {
	// Create event
	event := map[string]interface{}{
		"type": eventType,
		"data": data,
	}

	// Marshal event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		s.logger.Error("Failed to marshal SSE event", zap.Error(err))
		return
	}

	// Send event
	fmt.Fprintf(client.Writer, "data: %s\n\n", eventJSON)
	client.Flusher.Flush()
}

// trySendSSEEvent는 안전하게 SSE 이벤트를 전송하고 오류를 반환합니다.
func (s *Server) trySendSSEEvent(client *SSEClient, eventType string, data interface{}) error {
	if client == nil {
		return fmt.Errorf("client is nil")
	}

	// 패닉 복구
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			if s.logger != nil {
				s.logger.Error("Panic in trySendSSEEvent",
					zap.Any("error", r),
					zap.String("stack", stack),
					zap.String("eventType", eventType),
					zap.String("clientId", client.ID),
				)
			}

			// 콘솔에도 출력
			fmt.Printf("Panic in trySendSSEEvent: %v\nStack: %s\n", r, stack)
		}
	}()

	// 이벤트 생성
	event := map[string]interface{}{
		"type": eventType,
		"data": data,
	}

	// JSON으로 마샬링
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// 이벤트 전송 시도
	var writeErr error
	func() {
		// 이 함수 내부의 패닉을 복구
		defer func() {
			if r := recover(); r != nil {
				writeErr = fmt.Errorf("panic while writing event: %v", r)
			}
		}()

		// 이벤트 전송
		_, err = fmt.Fprintf(client.Writer, "data: %s\n\n", eventJSON)
		if err != nil {
			writeErr = fmt.Errorf("failed to write event: %w", err)
			return
		}

		// 플러시 시도
		func() {
			defer func() {
				if r := recover(); r != nil {
					writeErr = fmt.Errorf("panic while flushing: %v", r)
				}
			}()

			client.Flusher.Flush()
		}()
	}()

	return writeErr
}

// SSEManager manages SSE clients
type SSEManager struct {
	clients map[string]*SSEClient
	mu      sync.RWMutex
	logger  *zap.Logger
}

// NewSSEManager creates a new SSE manager
func NewSSEManager(logger *zap.Logger) *SSEManager {
	return &SSEManager{
		clients: make(map[string]*SSEClient),
		logger:  logger,
	}
}

// AddClient adds a client to the manager
func (m *SSEManager) AddClient(client *SSEClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[client.ID] = client
	m.logger.Debug("Client added", zap.String("clientId", client.ID))
}

// RemoveClient removes a client from the manager
func (m *SSEManager) RemoveClient(clientID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, clientID)
	m.logger.Debug("Client removed", zap.String("clientId", clientID))
}

// GetClient gets a client by ID
func (m *SSEManager) GetClient(clientID string) (*SSEClient, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, ok := m.clients[clientID]
	return client, ok
}

// BroadcastEvent broadcasts an event to all clients
func (m *SSEManager) BroadcastEvent(eventType string, data interface{}) {
	if m == nil {
		fmt.Println("SSEManager is nil in BroadcastEvent")
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create event
	event := map[string]interface{}{
		"type": eventType,
		"data": data,
	}

	// Marshal event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		if m.logger != nil {
			m.logger.Error("Failed to marshal SSE event", zap.Error(err))
		}
		fmt.Printf("Failed to marshal SSE event: %v\n", err)
		return
	}

	// Send event to all clients
	for id, client := range m.clients {
		if client == nil {
			continue
		}

		// 패닉 복구
		func(clientID string, c *SSEClient) {
			defer func() {
				if r := recover(); r != nil {
					stack := string(debug.Stack())
					if m.logger != nil {
						m.logger.Error("Panic in BroadcastEvent",
							zap.Any("error", r),
							zap.String("clientId", clientID),
							zap.String("stack", stack),
						)
					}
					fmt.Printf("Panic in BroadcastEvent: %v\nStack: %s\n", r, stack)

					// 문제가 있는 클라이언트 제거
					m.RemoveClient(clientID)
				}
			}()

			// 이벤트 전송 시도
			func() {
				defer func() {
					if r := recover(); r != nil {
						panic(fmt.Sprintf("nested panic: %v", r))
					}
				}()

				// 이벤트 전송
				_, err := fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON)
				if err != nil {
					if m.logger != nil {
						m.logger.Error("Failed to write event",
							zap.Error(err),
							zap.String("clientId", clientID),
						)
					}
					panic(fmt.Sprintf("write error: %v", err))
				}

				// 플러시
				c.Flusher.Flush()
			}()
		}(id, client)
	}
}

// BroadcastEventToGame broadcasts an event to all clients in a game
func (m *SSEManager) BroadcastEventToGame(gameID, eventType string, data interface{}) {
	if m == nil {
		fmt.Println("SSEManager is nil in BroadcastEventToGame")
		return
	}

	if gameID == "" {
		if m.logger != nil {
			m.logger.Error("Empty gameID in BroadcastEventToGame")
		}
		fmt.Println("Empty gameID in BroadcastEventToGame")
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create event
	event := map[string]interface{}{
		"type": eventType,
		"data": data,
	}

	// Marshal event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		if m.logger != nil {
			m.logger.Error("Failed to marshal SSE event", zap.Error(err))
		}
		fmt.Printf("Failed to marshal SSE event: %v\n", err)
		return
	}

	// 게임에 참여 중인 클라이언트 수 카운트
	clientCount := 0
	for _, client := range m.clients {
		if client != nil && client.GameID == gameID {
			clientCount++
		}
	}

	if clientCount == 0 {
		if m.logger != nil {
			m.logger.Debug("No clients in game", zap.String("gameId", gameID))
		}
		return
	}

	if m.logger != nil {
		m.logger.Debug("Broadcasting event to game",
			zap.String("gameId", gameID),
			zap.String("eventType", eventType),
			zap.Int("clientCount", clientCount),
		)
	}

	// Send event to clients in the game
	for id, client := range m.clients {
		if client == nil || client.GameID != gameID {
			continue
		}

		// 패닉 복구
		func(clientID string, c *SSEClient) {
			defer func() {
				if r := recover(); r != nil {
					stack := string(debug.Stack())
					if m.logger != nil {
						m.logger.Error("Panic in BroadcastEventToGame",
							zap.Any("error", r),
							zap.String("clientId", clientID),
							zap.String("gameId", gameID),
							zap.String("stack", stack),
						)
					}
					fmt.Printf("Panic in BroadcastEventToGame: %v\nStack: %s\n", r, stack)

					// 문제가 있는 클라이언트 제거
					m.RemoveClient(clientID)
				}
			}()

			// 이벤트 전송 시도
			func() {
				defer func() {
					if r := recover(); r != nil {
						panic(fmt.Sprintf("nested panic: %v", r))
					}
				}()

				// 이벤트 전송
				_, err := fmt.Fprintf(c.Writer, "data: %s\n\n", eventJSON)
				if err != nil {
					if m.logger != nil {
						m.logger.Error("Failed to write event",
							zap.Error(err),
							zap.String("clientId", clientID),
							zap.String("gameId", gameID),
						)
					}
					panic(fmt.Sprintf("write error: %v", err))
				}

				// 플러시
				c.Flusher.Flush()
			}()
		}(id, client)
	}
}
