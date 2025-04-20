package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"tictactoe/internal/domain"
)

// Client represents a connected SSE client
type Client struct {
	ID      string
	GameID  string
	Writer  http.ResponseWriter
	Flusher http.Flusher
}

// Event represents an SSE event
type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Router handles SSE connections and events
type Router struct {
	gameUC   domain.GameUseCase
	clients  map[string]*Client
	mu       sync.RWMutex
	eventsCh chan Event
}

// NewRouter creates a new SSE router
func NewRouter(gameUC domain.GameUseCase) *Router {
	router := &Router{
		gameUC:   gameUC,
		clients:  make(map[string]*Client),
		eventsCh: make(chan Event, 100),
	}

	// Start the event broadcaster
	go router.broadcastEvents()

	return router
}

// HandleEvents handles SSE connections
func (r *Router) HandleEvents(w http.ResponseWriter, req *http.Request) {
	// 패닉 복구
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("SSE Panic recovered: %v\n", err)
		}
	}()
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 연결 성공 응답 전송
	fmt.Fprintf(w, "data: {\"type\":\"connected\",\"payload\":{\"message\":\"Connected to event stream\"}}\n\n")
	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Get client info from query params
	gameID := req.URL.Query().Get("gameId")
	playerID := req.URL.Query().Get("playerId")

	// 게임 ID와 플레이어 ID가 필요함
	if gameID == "" || playerID == "" {
		http.Error(w, "gameId and playerId are required", http.StatusBadRequest)
		return
	}

	// Create client
	client := &Client{
		ID:      playerID,
		GameID:  gameID,
		Writer:  w,
		Flusher: flusher,
	}

	// Register client
	r.mu.Lock()
	r.clients[playerID] = client
	r.mu.Unlock()

	// 연결 성공 메시지 전송
	fmt.Fprintf(w, "data: {\"type\":\"connected\",\"payload\":{\"message\":\"Connected to event stream\",\"playerId\":\"%s\",\"gameId\":\"%s\"}}\n\n", playerID, gameID)
	flusher.Flush()

	// 게임 상태 전송
	game, err := r.gameUC.Get(gameID)
	if err == nil {
		r.sendEvent(client, "game_state", game)
	}

	// 연결 유지
	notify := req.Context().Done()
	<-notify

	// 연결이 끊어지면 클라이언트 제거
	r.mu.Lock()
	delete(r.clients, playerID)
	r.mu.Unlock()

	fmt.Printf("Client %s disconnected\n", playerID)
}

// PublishGameEvent publishes an event to all clients in a game
func (r *Router) PublishGameEvent(game *domain.Game, eventType, description string, data interface{}) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create event payload
	payload := map[string]interface{}{
		"game":        game,
		"description": description,
		"data":        data,
	}

	// Send event to all clients in the game
	for _, client := range r.clients {
		if client.GameID == game.ID {
			r.sendEvent(client, eventType, payload)
		}
	}
}

// PublishRoomEvent publishes an event to all clients
func (r *Router) PublishRoomEvent(eventType, description string, data interface{}) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Create event payload
	payload := map[string]interface{}{
		"description": description,
		"data":        data,
	}

	// Send event to all clients
	for _, client := range r.clients {
		r.sendEvent(client, eventType, payload)
	}
}

// sendEvent sends an event to a client
func (r *Router) sendEvent(client *Client, eventType string, payload interface{}) {
	// 패닉 복구
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("SSE sendEvent Panic recovered: %v\n", err)
		}
	}()

	event := Event{
		Type:    eventType,
		Payload: payload,
	}

	data, err := json.Marshal(event)
	if err != nil {
		fmt.Printf("Error marshaling event: %v\n", err)
		return
	}

	// 안전하게 이벤트 전송 시도
	try := func() (success bool) {
		defer func() {
			if err := recover(); err != nil {
				success = false
			}
		}()

		fmt.Fprintf(client.Writer, "data: %s\n\n", data)
		client.Flusher.Flush()
		return true
	}

	if !try() {
		// 전송 실패 시 클라이언트 제거
		r.mu.Lock()
		delete(r.clients, client.ID)
		r.mu.Unlock()
	}
}

// broadcastEvents broadcasts events to all clients
func (r *Router) broadcastEvents() {
	for event := range r.eventsCh {
		r.mu.RLock()
		for _, client := range r.clients {
			r.sendEvent(client, event.Type, event.Payload)
		}
		r.mu.RUnlock()
	}
}
