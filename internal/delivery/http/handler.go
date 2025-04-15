package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"tictactoe/internal/delivery/sse"
	"tictactoe/internal/domain"

	"github.com/google/uuid"
)

// Handler handles HTTP requests
type Handler struct {
	roomUC    domain.RoomUseCase
	gameUC    domain.GameUseCase
	sseRouter *sse.Router
}

// NewHandler creates a new HTTP handler
func NewHandler(roomUC domain.RoomUseCase, gameUC domain.GameUseCase, sseRouter *sse.Router) *Handler {
	h := &Handler{
		roomUC:    roomUC,
		gameUC:    gameUC,
		sseRouter: sseRouter,
	}

	return h
}

// CreateRoom handles the creation of a new room
func (h *Handler) CreateRoom(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(room)
}

// GetRooms handles the retrieval of all rooms
func (h *Handler) GetRooms(w http.ResponseWriter, r *http.Request) {
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
func (h *Handler) GetRoom(w http.ResponseWriter, r *http.Request) {
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

// JoinGame handles a player joining a game
func (h *Handler) JoinGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		GameID     string `json:"gameId"`
		PlayerName string `json:"playerName"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Generate a unique ID for the player
	playerID := uuid.New().String()

	game, err := h.gameUC.Join(req.GameID, playerID, req.PlayerName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Publish player join event to all clients
	h.sseRouter.PublishGameEvent(game, "player_join", "Player "+req.PlayerName+" joined the game", map[string]interface{}{
		"playerId":   playerID,
		"playerName": req.PlayerName,
	})

	// Return the player ID along with the game
	response := struct {
		PlayerID string      `json:"playerId"`
		Game     domain.Game `json:"game"`
	}{
		PlayerID: playerID,
		Game:     *game,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ReadyGame handles a player setting their ready status
func (h *Handler) ReadyGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		GameID   string `json:"gameId"`
		PlayerID string `json:"playerId"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	game, err := h.gameUC.Ready(req.GameID, req.PlayerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Publish player ready event
	playerName := game.Players[req.PlayerID].Name
	h.sseRouter.PublishGameEvent(game, "player_ready", playerName+" is ready", map[string]interface{}{
		"playerId":   req.PlayerID,
		"playerName": playerName,
	})

	// If game state changed to playing, publish game start event
	if game.State == domain.GameStatePlaying {
		h.sseRouter.PublishGameEvent(game, "game_start", "The battle against "+game.Boss.Name+" has begun!", game.Boss)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game)
}

// TriggerBossAction handles a request to trigger a boss action
func (h *Handler) TriggerBossAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		GameID   string `json:"gameId"`
		PlayerID string `json:"playerId"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Process boss action
	game, targetPlayer, damage, err := h.gameUC.ProcessBossAction(req.GameID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Publish boss attack event
	h.sseRouter.PublishGameEvent(game, "boss_attack", game.Boss.Name+" attacked "+targetPlayer.Name+" for "+strconv.Itoa(damage)+" damage", map[string]interface{}{
		"targetPlayerId":   targetPlayer.ID,
		"targetPlayerName": targetPlayer.Name,
		"damage":           damage,
		"bossName":         game.Boss.Name,
	})

	// If player was defeated, publish player defeated event
	if targetPlayer.Character.Stats.Health <= 0 {
		h.sseRouter.PublishGameEvent(game, "player_defeated", targetPlayer.Name+" was defeated by the "+game.Boss.Name, map[string]interface{}{
			"playerId":   targetPlayer.ID,
			"playerName": targetPlayer.Name,
		})
	}

	// If game state changed to finished, publish game end event
	if game.State == domain.GameStateFinished {
		if game.Result == domain.GameResultVictory {
			h.sseRouter.PublishGameEvent(game, "game_end", "Victory! The "+game.Boss.Name+" has been defeated!", game.Rewards)
		} else {
			h.sseRouter.PublishGameEvent(game, "game_end", "Defeat! The party has been wiped out by the "+game.Boss.Name+"!", nil)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game)
}

// AttackBoss handles a player attacking the boss
func (h *Handler) AttackBoss(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		GameID   string `json:"gameId"`
		PlayerID string `json:"playerId"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	game, err := h.gameUC.Attack(req.GameID, req.PlayerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Publish player attack event
	h.sseRouter.PublishGameEvent(game, "player_attack", game.Players[req.PlayerID].Name+" attacked the boss", map[string]interface{}{
		"playerId":   req.PlayerID,
		"playerName": game.Players[req.PlayerID].Name,
		"bossHealth": game.Boss.Health,
	})

	// If game state changed to finished, publish game end event
	if game.State == domain.GameStateFinished {
		if game.Result == domain.GameResultVictory {
			h.sseRouter.PublishGameEvent(game, "game_end", "Victory! The "+game.Boss.Name+" has been defeated!", game.Rewards)
		} else {
			h.sseRouter.PublishGameEvent(game, "game_end", "Defeat! The party has been wiped out by the "+game.Boss.Name+"!", nil)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game)
}

// GetGame handles the retrieval of a game by ID
func (h *Handler) GetGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Game ID is required", http.StatusBadRequest)
		return
	}

	// Check for test panic parameter
	if r.URL.Query().Get("test_panic") == "true" {
		panic("This is a test panic to verify the recovery middleware")
	}

	game, err := h.gameUC.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game)
}

// JoinRoom handles a player joining a room
func (h *Handler) JoinRoom(w http.ResponseWriter, r *http.Request) {
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
	playerID := uuid.New().String()

	// Join the room's game
	game, err := h.roomUC.Join(req.RoomID, playerID, req.PlayerName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the room
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
