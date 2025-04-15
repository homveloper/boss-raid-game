package http

import (
	"encoding/json"
	"net/http"
	"tictactoe/internal/delivery/sse"
	"tictactoe/internal/domain"

	"github.com/google/uuid"
)

// GameHandler handles HTTP requests for games
type GameHandler struct {
	gameUC    domain.GameUseCase
	sseRouter *sse.Router
}

// NewGameHandler creates a new game handler
func NewGameHandler(gameUC domain.GameUseCase, sseRouter *sse.Router) *GameHandler {
	return &GameHandler{
		gameUC:    gameUC,
		sseRouter: sseRouter,
	}
}

// GetGame handles the retrieval of a game by ID
func (h *GameHandler) GetGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Game ID is required", http.StatusBadRequest)
		return
	}

	game, err := h.gameUC.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game)
}

// JoinGame handles a player joining a game
func (h *GameHandler) JoinGame(w http.ResponseWriter, r *http.Request) {
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
	playerID := generateID()

	game, err := h.gameUC.Join(req.GameID, playerID, req.PlayerName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Publish player join event to all clients
	h.sseRouter.PublishGameEvent(game, "player_join", req.PlayerName+" joined the game", map[string]interface{}{
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
func (h *GameHandler) ReadyGame(w http.ResponseWriter, r *http.Request) {
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

// AttackBoss handles a player attacking the boss
func (h *GameHandler) AttackBoss(w http.ResponseWriter, r *http.Request) {
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
	playerName := game.Players[req.PlayerID].Name
	h.sseRouter.PublishGameEvent(game, "player_attack", playerName+" attacked the "+game.Boss.Name, map[string]interface{}{
		"playerId":   req.PlayerID,
		"playerName": playerName,
		"bossHealth": game.Boss.Health,
	})

	// If game state changed to finished, publish game end event
	if game.State == domain.GameStateFinished {
		if game.Result == domain.GameResultVictory {
			h.sseRouter.PublishGameEvent(game, "game_end", "Victory! The "+game.Boss.Name+" has been defeated!", game.Rewards)
		} else {
			h.sseRouter.PublishGameEvent(game, "game_end", "Defeat! The party has been wiped out by the "+game.Boss.Name+"!", nil)
		}
	} else {
		// Process boss attack if game is still in progress
		game, err = h.gameUC.ProcessBossAttack(req.GameID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// If boss attacked, publish boss attack event
		if len(game.Events) > 0 && game.Events[len(game.Events)-1].Type == "boss_attack" {
			h.sseRouter.PublishGameEvent(game, "boss_attack", game.Events[len(game.Events)-1].Description, game.Events[len(game.Events)-1].Data)
		}

		// If game state changed to finished after boss attack, publish game end event
		if game.State == domain.GameStateFinished {
			if game.Result == domain.GameResultVictory {
				h.sseRouter.PublishGameEvent(game, "game_end", "Victory! The "+game.Boss.Name+" has been defeated!", game.Rewards)
			} else {
				h.sseRouter.PublishGameEvent(game, "game_end", "Defeat! The party has been wiped out by the "+game.Boss.Name+"!", nil)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game)
}

// EquipItem handles a player equipping an item
func (h *GameHandler) EquipItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		GameID   string `json:"gameId"`
		PlayerID string `json:"playerId"`
		ItemID   string `json:"itemId"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	game, err := h.gameUC.EquipItem(req.GameID, req.PlayerID, req.ItemID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Publish item equip event
	playerName := game.Players[req.PlayerID].Name
	itemName := game.Players[req.PlayerID].Character.Inventory[req.ItemID].Name
	h.sseRouter.PublishGameEvent(game, "item_equip", playerName+" equipped "+itemName, map[string]interface{}{
		"playerId":   req.PlayerID,
		"playerName": playerName,
		"itemId":     req.ItemID,
		"itemName":   itemName,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game)
}

// generateID generates a random ID
func generateID() string {
	return "player_" + uuid.New().String()
}
