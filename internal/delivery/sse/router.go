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
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Get client info from query params
	gameID := req.URL.Query().Get("gameId")
	playerID := req.URL.Query().Get("playerId")

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

	// Send initial game state
	game, err := r.gameUC.Get(gameID)
	if err == nil {
		r.sendEvent(client, "game_state", game)
	}

	// Keep connection open until client disconnects
	notify := req.Context().Done()
	<-notify

	// Unregister client
	r.mu.Lock()
	delete(r.clients, playerID)
	r.mu.Unlock()
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
	event := Event{
		Type:    eventType,
		Payload: payload,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	fmt.Fprintf(client.Writer, "data: %s\n\n", data)
	client.Flusher.Flush()
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
