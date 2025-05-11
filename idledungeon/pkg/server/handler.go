package server

import (
	"encoding/json"
	"idledungeon/internal/model"
	"net/http"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// Response represents a standard API response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// CreateGameRequest represents a request to create a game
type CreateGameRequest struct {
	Name string `json:"name"`
}

// JoinGameRequest represents a request to join a game
type JoinGameRequest struct {
	GameID string `json:"gameId"`
	Name   string `json:"name"`
}

// MovePlayerRequest represents a request to move a player
type MovePlayerRequest struct {
	GameID   string  `json:"gameId"`
	PlayerID string  `json:"playerId"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
}

// AttackMonsterRequest represents a request to attack a monster
type AttackMonsterRequest struct {
	GameID    string `json:"gameId"`
	PlayerID  string `json:"playerId"`
	MonsterID string `json:"monsterId"`
}

// handleGetGames handles GET /api/games
func (s *GameServer) handleGetGames(w http.ResponseWriter, r *http.Request) {
	// Only allow GET method
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get games from storage
	games, err := s.gameStorage.GetAllGames(s.ctx)
	if err != nil {
		s.logger.Error("Failed to get games", zap.Error(err))
		sendJSONResponse(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to get games",
		})
		return
	}

	// Send response
	sendJSONResponse(w, http.StatusOK, Response{
		Success: true,
		Data:    games,
	})
}

// handleGetGame handles GET /api/games/get
func (s *GameServer) handleGetGame(w http.ResponseWriter, r *http.Request) {
	// Only allow GET method
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get game ID from query parameters
	gameIDStr := r.URL.Query().Get("id")
	if gameIDStr == "" {
		sendJSONResponse(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Game ID is required",
		})
		return
	}

	// Parse game ID
	gameID, err := primitive.ObjectIDFromHex(gameIDStr)
	if err != nil {
		s.logger.Error("Invalid game ID", zap.String("game_id", gameIDStr), zap.Error(err))
		sendJSONResponse(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid game ID format",
		})
		return
	}

	// Get game from storage
	game, err := s.gameStorage.GetGame(s.ctx, gameID)
	if err != nil {
		s.logger.Error("Failed to get game", zap.String("game_id", gameIDStr), zap.Error(err))
		sendJSONResponse(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to get game",
		})
		return
	}

	// Send response
	sendJSONResponse(w, http.StatusOK, Response{
		Success: true,
		Data:    game,
	})
}

// handleCreateGame handles POST /api/games/create
func (s *GameServer) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req CreateGameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("Failed to parse request", zap.Error(err))
		sendJSONResponse(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// Validate request
	if req.Name == "" {
		sendJSONResponse(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Game name is required",
		})
		return
	}

	// Create game
	game := model.NewGame(req.Name)
	createdGame, err := s.gameStorage.CreateGame(s.ctx, game)
	if err != nil {
		s.logger.Error("Failed to create game", zap.Error(err))
		sendJSONResponse(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to create game",
		})
		return
	}

	// Add game to cache
	s.gamesMutex.Lock()
	s.games[createdGame.ID] = createdGame
	s.gamesMutex.Unlock()

	// Send response
	sendJSONResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Game created successfully",
		Data:    createdGame,
	})
}

// handleJoinGame handles POST /api/games/join
func (s *GameServer) handleJoinGame(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req JoinGameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("Failed to parse request", zap.Error(err))
		sendJSONResponse(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// Validate request
	if req.GameID == "" {
		sendJSONResponse(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Game ID is required",
		})
		return
	}
	if req.Name == "" {
		sendJSONResponse(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Player name is required",
		})
		return
	}

	// Parse game ID
	gameID, err := primitive.ObjectIDFromHex(req.GameID)
	if err != nil {
		s.logger.Error("Invalid game ID", zap.String("game_id", req.GameID), zap.Error(err))
		sendJSONResponse(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid game ID format",
		})
		return
	}

	// Add player to game
	updatedGame, _, err := s.gameStorage.UpdateGame(s.ctx, gameID, func(game *model.Game) (*model.Game, error) {
		game.AddPlayer(req.Name)

		// 게임 상태를 플레이 중으로 변경
		if game.State == model.GameStateWaiting {
			game.State = model.GameStatePlaying
			s.logger.Info("Game state changed to playing", zap.String("game_id", game.ID.Hex()))
		}

		return game, nil
	})
	if err != nil {
		s.logger.Error("Failed to join game", zap.String("game_id", req.GameID), zap.Error(err))
		sendJSONResponse(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to join game",
		})
		return
	}

	// Update game in cache
	s.gamesMutex.Lock()
	s.games[updatedGame.ID] = updatedGame
	s.gamesMutex.Unlock()

	// Find the player that was just added
	var player *model.Player
	for _, p := range updatedGame.Players {
		if p.Name == req.Name {
			player = p
			break
		}
	}

	// Send response
	sendJSONResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Joined game successfully",
		Data: map[string]interface{}{
			"game":   updatedGame,
			"player": player,
		},
	})
}

// handleMovePlayer handles POST /api/games/move
func (s *GameServer) handleMovePlayer(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req MovePlayerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("Failed to parse request", zap.Error(err))
		sendJSONResponse(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// Validate request
	if req.GameID == "" || req.PlayerID == "" {
		sendJSONResponse(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Game ID and Player ID are required",
		})
		return
	}

	// Parse game ID
	gameID, err := primitive.ObjectIDFromHex(req.GameID)
	if err != nil {
		s.logger.Error("Invalid game ID", zap.String("game_id", req.GameID), zap.Error(err))
		sendJSONResponse(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid game ID format",
		})
		return
	}

	// Move player
	updatedGame, _, err := s.gameStorage.UpdateGame(s.ctx, gameID, func(game *model.Game) (*model.Game, error) {
		// Find player
		player, exists := game.Players[req.PlayerID]
		if !exists {
			return nil, nil
		}

		// Move player
		player.Move(req.X, req.Y, game.World)

		return game, nil
	})
	if err != nil {
		s.logger.Error("Failed to move player", zap.String("game_id", req.GameID), zap.Error(err))
		sendJSONResponse(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to move player",
		})
		return
	}

	// Update game in cache
	s.gamesMutex.Lock()
	s.games[updatedGame.ID] = updatedGame
	s.gamesMutex.Unlock()

	// Send response
	sendJSONResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Player moved successfully",
		Data:    updatedGame,
	})
}

// handleAttackMonster handles POST /api/games/attack
func (s *GameServer) handleAttackMonster(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req AttackMonsterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("Failed to parse request", zap.Error(err))
		sendJSONResponse(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid request format",
		})
		return
	}

	// Validate request
	if req.GameID == "" || req.PlayerID == "" || req.MonsterID == "" {
		sendJSONResponse(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Game ID, Player ID, and Monster ID are required",
		})
		return
	}

	// Parse game ID
	gameID, err := primitive.ObjectIDFromHex(req.GameID)
	if err != nil {
		s.logger.Error("Invalid game ID", zap.String("game_id", req.GameID), zap.Error(err))
		sendJSONResponse(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "Invalid game ID format",
		})
		return
	}

	// Attack monster
	updatedGame, _, err := s.gameStorage.UpdateGame(s.ctx, gameID, func(game *model.Game) (*model.Game, error) {
		// Find player and monster
		player, playerExists := game.Players[req.PlayerID]
		monster, monsterExists := game.Monsters[req.MonsterID]

		if !playerExists || !monsterExists {
			return game, nil
		}

		// Check if player is near monster
		if !player.IsNear(monster, 100) { // Attack range of 100 units
			return game, nil
		}

		// Attack monster
		player.Attack(monster)

		return game, nil
	})
	if err != nil {
		s.logger.Error("Failed to attack monster", zap.String("game_id", req.GameID), zap.Error(err))
		sendJSONResponse(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "Failed to attack monster",
		})
		return
	}

	// Update game in cache
	s.gamesMutex.Lock()
	s.games[updatedGame.ID] = updatedGame
	s.gamesMutex.Unlock()

	// Send response
	sendJSONResponse(w, http.StatusOK, Response{
		Success: true,
		Message: "Monster attacked successfully",
		Data:    updatedGame,
	})
}

// Helper function to send JSON response
func sendJSONResponse(w http.ResponseWriter, statusCode int, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
