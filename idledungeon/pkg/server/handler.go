package server

import (
	"encoding/json"
	"fmt"
	"idledungeon/internal/model"
	"net/http"
	"strings"
	"tictactoe/pkg/utils"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// handleGames handles the /api/games endpoint
func (s *Server) handleGames(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetGames(w, r)
	case http.MethodPost:
		s.handleCreateGame(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGame handles the /api/games/{id} endpoint
func (s *Server) handleGame(w http.ResponseWriter, r *http.Request) {
	// Extract game ID from URL
	idStr := strings.TrimPrefix(r.URL.Path, "/api/games/")
	if idStr == "" {
		r = utils.WithErrorAndCodeAndMessage(r, fmt.Errorf("game ID is required"), http.StatusBadRequest, "Game ID is required")
		utils.WriteError(w, r)
		return
	}

	// Parse game ID
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		r = utils.WithErrorAndCodeAndMessage(r, err, http.StatusBadRequest, "Invalid game ID")
		utils.WriteError(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGetGame(w, r, id)
	case http.MethodPut:
		s.handleUpdateGame(w, r, id)
	case http.MethodDelete:
		s.handleDeleteGame(w, r, id)
	default:
		r = utils.WithErrorAndCodeAndMessage(r, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed, "Method not allowed")
		utils.WriteError(w, r)
	}
}

// handleGetGames handles GET /api/games
func (s *Server) handleGetGames(w http.ResponseWriter, r *http.Request) {
	// In a real application, we would query the database for all games
	// For this demo, we'll just return a simple response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Get games endpoint"})
}

// handleCreateGame handles POST /api/games
func (s *Server) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		r = utils.WithErrorAndCodeAndMessage(r, err, http.StatusBadRequest, "Invalid request body")
		utils.WriteError(w, r)
		return
	}

	// Create game
	game, err := s.storage.CreateGame(r.Context(), req.Name, s.GetWorldConfig())
	if err != nil {
		s.logger.Error("Failed to create game", zap.Error(err))
		r = utils.WithErrorAndCodeAndMessage(r, err, http.StatusInternalServerError, "Failed to create game")
		utils.WriteError(w, r)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(game)
}

// handleGetGame handles GET /api/games/{id}
func (s *Server) handleGetGame(w http.ResponseWriter, r *http.Request, id primitive.ObjectID) {
	// Get game
	game, err := s.storage.GetGame(r.Context(), id)
	if err != nil {
		s.logger.Error("Failed to get game", zap.Error(err), zap.String("id", id.Hex()))
		r = utils.WithErrorAndCodeAndMessage(r, err, http.StatusInternalServerError, "Failed to get game")
		utils.WriteError(w, r)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(game)
}

// handleUpdateGame handles PUT /api/games/{id}
func (s *Server) handleUpdateGame(w http.ResponseWriter, r *http.Request, id primitive.ObjectID) {
	// Parse request body
	var req struct {
		UnitID string  `json:"unitId"`
		Action string  `json:"action"`
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Damage int     `json:"damage"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		r = utils.WithErrorAndCodeAndMessage(r, err, http.StatusBadRequest, "Invalid request body")
		utils.WriteError(w, r)
		return
	}

	// Update game
	game, err := s.storage.UpdateGame(r.Context(), id, func(g *model.GameState) (*model.GameState, error) {
		// Get unit
		unit, exists := g.GetUnit(req.UnitID)
		if !exists {
			return g, nil
		}

		// Apply action
		switch req.Action {
		case "move":
			unit.MoveTo(req.X, req.Y)
		case "attack":
			unit.TakeDamage(req.Damage)
		case "heal":
			unit.Heal(req.Damage)
		}

		// Update unit
		g.UpdateUnit(unit)
		g.LastUpdated = unit.UpdatedAt
		g.Version++

		return g, nil
	})
	if err != nil {
		s.logger.Error("Failed to update game", zap.Error(err), zap.String("id", id.Hex()))
		r = utils.WithErrorAndCodeAndMessage(r, err, http.StatusInternalServerError, "Failed to update game")
		utils.WriteError(w, r)
		return
	}

	// 게임 상태 변경 이벤트 브로드캐스트
	go func() {
		// 게임 ID를 문자열로 변환
		gameIDStr := id.Hex()

		// 게임 상태 변경 이벤트 브로드캐스트
		s.logger.Debug("Broadcasting game update",
			zap.String("gameId", gameIDStr),
			zap.String("action", req.Action),
			zap.String("unitId", req.UnitID),
		)

		// 게임에 참여 중인 모든 클라이언트에게 이벤트 전송
		s.sseManager.BroadcastEventToGame(gameIDStr, "game_update", map[string]interface{}{
			"game": game,
			"action": map[string]interface{}{
				"type":   req.Action,
				"unitId": req.UnitID,
				"x":      req.X,
				"y":      req.Y,
				"damage": req.Damage,
			},
		})
	}()

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(game)
}

// handleDeleteGame handles DELETE /api/games/{id}
func (s *Server) handleDeleteGame(w http.ResponseWriter, r *http.Request, id primitive.ObjectID) {
	// In a real application, we would delete the game from the database
	// For this demo, we'll just return a simple response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Game deleted"})
}

// handlePlayers handles the /api/players endpoint
func (s *Server) handlePlayers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleCreatePlayer(w, r)
	default:
		r = utils.WithErrorAndCodeAndMessage(r, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed, "Method not allowed")
		utils.WriteError(w, r)
	}
}

// handlePlayer handles the /api/players/{id} endpoint
func (s *Server) handlePlayer(w http.ResponseWriter, r *http.Request) {
	// Extract player ID from URL
	idStr := strings.TrimPrefix(r.URL.Path, "/api/players/")
	if idStr == "" {
		r = utils.WithErrorAndCodeAndMessage(r, fmt.Errorf("player ID is required"), http.StatusBadRequest, "Player ID is required")
		utils.WriteError(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleGetPlayer(w, r, idStr)
	case http.MethodPut:
		s.handleUpdatePlayer(w, r, idStr)
	default:
		r = utils.WithErrorAndCodeAndMessage(r, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed, "Method not allowed")
		utils.WriteError(w, r)
	}
}

// handleCreatePlayer handles POST /api/players
func (s *Server) handleCreatePlayer(w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req struct {
		GameID string `json:"gameId"`
		Name   string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		r = utils.WithErrorAndCodeAndMessage(r, err, http.StatusBadRequest, "Invalid request body")
		utils.WriteError(w, r)
		return
	}

	// Parse game ID
	gameID, err := primitive.ObjectIDFromHex(req.GameID)
	if err != nil {
		r = utils.WithErrorAndCodeAndMessage(r, err, http.StatusBadRequest, "Invalid game ID")
		utils.WriteError(w, r)
		return
	}

	// Generate player ID
	playerID := model.GenerateUUID()

	// Update game
	game, err := s.storage.UpdateGame(r.Context(), gameID, func(g *model.GameState) (*model.GameState, error) {
		// Create player
		x, y := model.GenerateRandomPosition(s.GetWorldConfig().Width, s.GetWorldConfig().Height)
		player := model.NewPlayer(playerID, req.Name, x, y)

		// Add player to game
		g.AddUnit(player)
		g.LastUpdated = player.UpdatedAt
		g.Version++

		return g, nil
	})
	if err != nil {
		s.logger.Error("Failed to create player", zap.Error(err), zap.String("gameId", req.GameID))
		r = utils.WithErrorAndCodeAndMessage(r, err, http.StatusInternalServerError, "Failed to create player")
		utils.WriteError(w, r)
		return
	}

	// Get player
	player, exists := game.GetUnit(playerID)
	if !exists {
		s.logger.Error("Player not found after creation", zap.String("playerId", playerID))
		r = utils.WithErrorAndCodeAndMessage(r, fmt.Errorf("player not found after creation"), http.StatusInternalServerError, "Failed to create player")
		utils.WriteError(w, r)
		return
	}

	// 플레이어 생성 이벤트 브로드캐스트
	go func() {
		// 게임 ID를 문자열로 변환
		gameIDStr := gameID.Hex()

		// 플레이어 생성 이벤트 브로드캐스트
		s.logger.Debug("Broadcasting player creation",
			zap.String("gameId", gameIDStr),
			zap.String("playerId", playerID),
			zap.String("playerName", req.Name),
		)

		// 게임에 참여 중인 모든 클라이언트에게 이벤트 전송
		s.sseManager.BroadcastEventToGame(gameIDStr, "player_joined", map[string]interface{}{
			"game":   game,
			"player": player,
		})
	}()

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(player)
}

// handleGetPlayer handles GET /api/players/{id}
func (s *Server) handleGetPlayer(w http.ResponseWriter, r *http.Request, id string) {
	// In a real application, we would query the database for the player
	// For this demo, we'll just return a simple response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Get player endpoint", "id": id})
}

// handleUpdatePlayer handles PUT /api/players/{id}
func (s *Server) handleUpdatePlayer(w http.ResponseWriter, r *http.Request, id string) {
	// In a real application, we would update the player in the database
	// For this demo, we'll just return a simple response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Player updated", "id": id})
}

// handleSync handles the /api/sync endpoint
func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		r = utils.WithErrorAndCodeAndMessage(r, fmt.Errorf("method not allowed"), http.StatusMethodNotAllowed, "Method not allowed")
		utils.WriteError(w, r)
		return
	}

	// Parse request body
	var req struct {
		ClientID    string           `json:"clientId"`
		DocumentID  string           `json:"documentId"`
		VectorClock map[string]int64 `json:"vectorClock"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		r = utils.WithErrorAndCodeAndMessage(r, err, http.StatusBadRequest, "Invalid request body")
		utils.WriteError(w, r)
		return
	}

	// Parse document ID
	docID, err := primitive.ObjectIDFromHex(req.DocumentID)
	if err != nil {
		r = utils.WithErrorAndCodeAndMessage(r, err, http.StatusBadRequest, "Invalid document ID")
		utils.WriteError(w, r)
		return
	}

	// Update vector clock
	syncService := s.storage.GetSyncService()
	if err := syncService.UpdateVectorClock(r.Context(), req.ClientID, docID, req.VectorClock); err != nil {
		s.logger.Error("Failed to update vector clock", zap.Error(err))
		r = utils.WithErrorAndCodeAndMessage(r, err, http.StatusInternalServerError, "Failed to update vector clock")
		utils.WriteError(w, r)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Vector clock updated"})
}
