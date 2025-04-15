package http

import (
	"encoding/json"
	"net/http"
	"tictactoe/internal/delivery/sse"
	"tictactoe/internal/domain"
)

// RoomHandler handles HTTP requests for rooms
type RoomHandler struct {
	roomUC    domain.RoomUseCase
	sseRouter *sse.Router
}

// NewRoomHandler creates a new room handler
func NewRoomHandler(roomUC domain.RoomUseCase, sseRouter *sse.Router) *RoomHandler {
	return &RoomHandler{
		roomUC:    roomUC,
		sseRouter: sseRouter,
	}
}

// CreateRoom handles the creation of a new room
func (h *RoomHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	room, err := h.roomUC.Create(req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Publish room created event
	h.sseRouter.PublishRoomEvent("room_created", "Room "+room.Name+" created", room)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(room)
}

// GetRooms handles the retrieval of all rooms
func (h *RoomHandler) GetRooms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rooms, err := h.roomUC.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rooms)
}

// GetRoom handles the retrieval of a room by ID
func (h *RoomHandler) GetRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Room ID is required", http.StatusBadRequest)
		return
	}

	room, err := h.roomUC.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(room)
}

// JoinRoom handles a player joining a room
func (h *RoomHandler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RoomID     string `json:"roomId"`
		PlayerName string `json:"playerName"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate a unique ID for the player
	playerID := generateID()

	game, err := h.roomUC.Join(req.RoomID, playerID, req.PlayerName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the room to return with the response
	room, err := h.roomUC.Get(req.RoomID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Publish player join event
	h.sseRouter.PublishRoomEvent("player_join_room", req.PlayerName+" joined room "+room.Name, map[string]interface{}{
		"roomId":     req.RoomID,
		"playerId":   playerID,
		"playerName": req.PlayerName,
	})

	// Return the player ID, room, and game
	response := struct {
		PlayerID string      `json:"playerId"`
		Room     domain.Room `json:"room"`
		Game     domain.Game `json:"game"`
	}{
		PlayerID: playerID,
		Room:     *room,
		Game:     *game,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// LeaveRoom handles a player leaving a room
func (h *RoomHandler) LeaveRoom(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		RoomID   string `json:"roomId"`
		PlayerID string `json:"playerId"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err = h.roomUC.Leave(req.RoomID, req.PlayerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the room to return with the response
	room, err := h.roomUC.Get(req.RoomID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Publish player leave event
	h.sseRouter.PublishRoomEvent("player_leave_room", "Player left room "+room.Name, map[string]interface{}{
		"roomId":   req.RoomID,
		"playerId": req.PlayerID,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(room)
}
