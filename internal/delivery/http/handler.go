package http

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"tictactoe/internal/delivery/sse"
	"tictactoe/internal/domain"
	"time"

	"github.com/google/uuid"
)

// Handler handles HTTP requests
type Handler struct {
	roomUC    domain.RoomUseCase
	gameUC    domain.GameUseCase
	sseRouter *sse.Router

	// 게임 캐시 (게임 ID -> 게임 객체)
	gameCache map[string]*domain.Game
	// 캐시 말소
	cacheMu sync.RWMutex
	// 마지막 업데이트 시간 (게임 ID -> 시간)
	lastUpdated map[string]time.Time
}

// NewHandler creates a new HTTP handler
func NewHandler(roomUC domain.RoomUseCase, gameUC domain.GameUseCase, sseRouter *sse.Router) *Handler {
	h := &Handler{
		roomUC:      roomUC,
		gameUC:      gameUC,
		sseRouter:   sseRouter,
		gameCache:   make(map[string]*domain.Game),
		lastUpdated: make(map[string]time.Time),
	}

	return h
}

// manageCacheSync 캐시 동기화 및 청소를 관리하는 메서드
func (h *Handler) manageCacheSync() {
	ticker := time.NewTicker(5 * time.Minute) // 5분마다 캐시 청소 및 동기화 검사
	defer ticker.Stop()

	for range ticker.C {
		h.cleanupCache()
	}
}

// cleanupCache 오래된 캐시 항목을 정리하는 메서드
func (h *Handler) cleanupCache() {
	h.cacheMu.Lock()
	defer h.cacheMu.Unlock()

	// 현재 시간
	now := time.Now()
	// 오래된 항목 삭제 (1시간 이상 업데이트되지 않은 항목)
	for gameID, lastUpdate := range h.lastUpdated {
		if now.Sub(lastUpdate) > time.Hour {
			delete(h.gameCache, gameID)
			delete(h.lastUpdated, gameID)
			log.Printf("Removed stale cache entry for game: %s", gameID)
		}
	}
}

// CreateRoom handles the creation of a new room
func (h *Handler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	log.Printf("CreateRoom handler called with method: %s", r.Method)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		log.Printf("Method not allowed: %s %s", r.Method, r.URL.Path)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	log.Printf("Reading request body")
	body, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		errorMsg := fmt.Sprintf("Error reading request body: %v", readErr)
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error reading request body: %v\n%s", readErr, debug.Stack())
		return
	}
	log.Printf("Request body: %s", string(body))

	err := json.Unmarshal(body, &req)
	if err != nil {
		errorMsg := fmt.Sprintf("Invalid request body: %v", err)
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error decoding request body: %v\n%s", err, debug.Stack())
		return
	}

	log.Printf("Creating room with name: %s", req.Name)

	room, err := h.roomUC.Create(req.Name)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to create room: %v", err)
		http.Error(w, errorMsg, http.StatusInternalServerError)
		log.Printf("Error creating room: %v\n%s", err, debug.Stack())

		// 오류 원인 추적
		var cause error = err
		for cause != nil {
			if unwrapped, ok := cause.(interface{ Unwrap() error }); ok {
				log.Printf("Caused by: %v", cause)
				cause = unwrapped.Unwrap()
			} else {
				log.Printf("Root cause: %v", cause)
				break
			}
		}
		return
	}

	log.Printf("Room created successfully: %s", room.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	encodeErr := json.NewEncoder(w).Encode(room)
	if encodeErr != nil {
		log.Printf("Error encoding response: %v", encodeErr)
	}
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
	// 메서드 검증
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		log.Printf("Method not allowed: %s %s", r.Method, r.URL.Path)
		return
	}

	var req struct {
		GameID   string `json:"gameId"`
		PlayerID string `json:"playerId"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		errorMsg := fmt.Sprintf("Invalid request body: %v", err)
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error decoding request body: %v\n%s", err, debug.Stack())
		return
	}

	// 요청 파라미터 검증
	if req.GameID == "" {
		errorMsg := "Game ID is required"
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error: %s\n%s", errorMsg, debug.Stack())
		return
	}

	if req.PlayerID == "" {
		errorMsg := "Player ID is required"
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error: %s\n%s", errorMsg, debug.Stack())
		return
	}

	// 요청 로깅
	log.Printf("Ready request: gameID=%s, playerID=%s", req.GameID, req.PlayerID)

	// 게임 가져오기
	gameBefore, err := h.gameUC.Get(req.GameID)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to get game: %v", err)
		http.Error(w, errorMsg, http.StatusInternalServerError)
		log.Printf("Error getting game: %v\n%s", err, debug.Stack())
		return
	}

	// 플레이어 준비 상태 변경
	game, err := h.gameUC.Ready(req.GameID, req.PlayerID)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to set player ready: %v", err)
		http.Error(w, errorMsg, http.StatusInternalServerError)
		log.Printf("Error setting player ready: %v\n%s", err, debug.Stack())
		return
	}

	// 플레이어 정보 가져오기
	player, exists := game.Players[req.PlayerID]
	if !exists {
		errorMsg := "Player not found in game"
		http.Error(w, errorMsg, http.StatusInternalServerError)
		log.Printf("Error: %s\n%s", errorMsg, debug.Stack())
		return
	}

	// 준비 상태 변경 이벤트 발행
	if player.Ready {
		// 플레이어가 준비 상태로 변경됨
		h.sseRouter.PublishGameEvent(game, "player_ready", player.Name+" is ready", map[string]interface{}{
			"playerId":   req.PlayerID,
			"playerName": player.Name,
			"ready":      true,
		})
		log.Printf("Player %s (%s) is now ready", player.Name, req.PlayerID)
	} else {
		// 플레이어가 준비 취소 상태로 변경됨
		h.sseRouter.PublishGameEvent(game, "player_not_ready", player.Name+" is not ready", map[string]interface{}{
			"playerId":   req.PlayerID,
			"playerName": player.Name,
			"ready":      false,
		})
		log.Printf("Player %s (%s) is now not ready", player.Name, req.PlayerID)
	}

	// 게임 상태가 변경되었는지 확인
	if gameBefore.State != domain.GameStatePlaying && game.State == domain.GameStatePlaying {
		// 게임 시작 이벤트 발행
		h.sseRouter.PublishGameEvent(game, "game_start", "The battle against "+game.Boss.Name+" has begun!", game.Boss)
		log.Printf("Game %s started with boss %s", req.GameID, game.Boss.Name)
	}

	// 응답 전송
	w.Header().Set("Content-Type", "application/json")
	encodeErr := json.NewEncoder(w).Encode(game)
	if encodeErr != nil {
		log.Printf("Error encoding response: %v", encodeErr)
	}

	log.Printf("Ready request completed successfully for player %s in game %s", req.PlayerID, req.GameID)
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

// GetCraftableItems handles the retrieval of craftable items
func (h *Handler) GetCraftableItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		log.Printf("Method not allowed: %s %s", r.Method, r.URL.Path)
		return
	}

	gameID := r.URL.Query().Get("gameId")
	if gameID == "" {
		errorMsg := "Game ID is required"
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error: %s\n%s", errorMsg, debug.Stack())
		return
	}

	// 게임 캐시에서 게임 객체 가져오기
	var game *domain.Game
	var craftableItems []domain.CraftableItem

	// 캐시 동기화를 위한 락 획득
	h.cacheMu.RLock()
	var exists bool
	game, exists = h.gameCache[gameID]
	h.cacheMu.RUnlock()

	if !exists {
		// 캐시에 없는 경우 새 게임 생성
		log.Printf("Creating new game for ID: %s", gameID)
		game = domain.NewGame(gameID, "")

		// 캐시에 추가 (쓰기 락 필요)
		h.cacheMu.Lock()
		h.gameCache[gameID] = game
		h.lastUpdated[gameID] = time.Now()
		h.cacheMu.Unlock()
	} else {
		log.Printf("Using cached game for ID: %s", gameID)

		// 마지막 업데이트 시간 갱신
		h.cacheMu.Lock()
		h.lastUpdated[gameID] = time.Now()
		h.cacheMu.Unlock()
	}

	// 제작 가능한 아이템 목록 가져오기
	craftableItems = game.GetCraftableItems()

	// 응답 전송
	w.Header().Set("Content-Type", "application/json")
	response := struct {
		GameID         string                 `json:"gameId"`
		CraftableItems []domain.CraftableItem `json:"craftableItems"`
	}{
		GameID:         gameID,
		CraftableItems: craftableItems,
	}

	encodeErr := json.NewEncoder(w).Encode(response)
	if encodeErr != nil {
		log.Printf("Error encoding response: %v", encodeErr)
	}

	log.Printf("Get craftable items request completed successfully for game %s", gameID)
}

// GetCraftingItems handles the retrieval of crafting items
func (h *Handler) GetCraftingItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		log.Printf("Method not allowed: %s %s", r.Method, r.URL.Path)
		return
	}

	gameID := r.URL.Query().Get("gameId")
	if gameID == "" {
		errorMsg := "Game ID is required"
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error: %s\n%s", errorMsg, debug.Stack())
		return
	}

	// 게임 캐시에서 게임 객체 가져오기
	var game *domain.Game
	var craftingItems []domain.CraftingItem

	// 캐시 동기화를 위한 락 획득
	h.cacheMu.RLock()
	var exists bool
	game, exists = h.gameCache[gameID]
	h.cacheMu.RUnlock()

	if !exists {
		// 캐시에 없는 경우 새 게임 생성
		log.Printf("Creating new game for ID: %s", gameID)
		game = domain.NewGame(gameID, "")

		// 캐시에 추가 (쓰기 락 필요)
		h.cacheMu.Lock()
		h.gameCache[gameID] = game
		h.lastUpdated[gameID] = time.Now()
		h.cacheMu.Unlock()
	} else {
		log.Printf("Using cached game for ID: %s", gameID)

		// 마지막 업데이트 시간 갱신
		h.cacheMu.Lock()
		h.lastUpdated[gameID] = time.Now()
		h.cacheMu.Unlock()
	}

	// 제작 중인 아이템 목록 가져오기
	craftingItems = game.GetCraftingItems()

	// 응답 전송
	w.Header().Set("Content-Type", "application/json")
	response := struct {
		GameID        string                `json:"gameId"`
		CraftingItems []domain.CraftingItem `json:"craftingItems"`
	}{
		GameID:        gameID,
		CraftingItems: craftingItems,
	}

	encodeErr := json.NewEncoder(w).Encode(response)
	if encodeErr != nil {
		log.Printf("Error encoding response: %v", encodeErr)
	}

	log.Printf("Get crafting items request completed successfully for game %s", gameID)
}

// StartCrafting handles starting crafting an item
func (h *Handler) StartCrafting(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		log.Printf("Method not allowed: %s %s", r.Method, r.URL.Path)
		return
	}

	var req struct {
		GameID   string `json:"gameId"`
		PlayerID string `json:"playerId"`
		ItemID   string `json:"itemId"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		errorMsg := fmt.Sprintf("Invalid request body: %v", err)
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error decoding request body: %v\n%s", err, debug.Stack())
		return
	}

	// 요청 파라미터 검증
	if req.GameID == "" {
		errorMsg := "Game ID is required"
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error: %s\n%s", errorMsg, debug.Stack())
		return
	}

	if req.PlayerID == "" {
		errorMsg := "Player ID is required"
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error: %s\n%s", errorMsg, debug.Stack())
		return
	}

	if req.ItemID == "" {
		errorMsg := "Item ID is required"
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error: %s\n%s", errorMsg, debug.Stack())
		return
	}

	// 게임 캐시에서 게임 객체 가져오기
	var game *domain.Game

	// 캐시 동기화를 위한 락 획득
	h.cacheMu.RLock()
	var exists bool
	game, exists = h.gameCache[req.GameID]
	h.cacheMu.RUnlock()

	if !exists {
		// 캐시에 없는 경우 새 게임 생성
		log.Printf("Creating new game for ID: %s", req.GameID)
		game = domain.NewGame(req.GameID, "")

		// 캐시에 추가 (쓰기 락 필요)
		h.cacheMu.Lock()
		h.gameCache[req.GameID] = game
		h.lastUpdated[req.GameID] = time.Now()
		h.cacheMu.Unlock()
	} else {
		log.Printf("Using cached game for ID: %s", req.GameID)
	}

	// 플레이어 이름 생성
	playerName := "Guest"
	if strings.HasPrefix(req.PlayerID, "user_") {
		playerName = "Guest_" + req.PlayerID[5:10]
	}

	// 플레이어가 없으면 추가
	if _, exists := game.Players[req.PlayerID]; !exists {
		game.AddPlayer(req.PlayerID, playerName)
	}

	// 제작 시작
	craftingItem, err := game.StartCrafting(req.PlayerID, req.ItemID)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to start crafting: %v", err)
		http.Error(w, errorMsg, http.StatusInternalServerError)
		log.Printf("Error starting crafting: %v\n%s", err, debug.Stack())
		return
	}

	// 이벤트 추가
	game.AddEvent("crafting_started", playerName+" started crafting "+game.CraftingSystem.CraftableItems[req.ItemID].Name, map[string]interface{}{
		"craftingId": craftingItem.ID,
		"itemId":     req.ItemID,
		"playerId":   req.PlayerID,
	})

	// SSE를 통해 제작 시작 이벤트 전파
	// 완료 예상 시간 계산
	completionTime := craftingItem.StartTime.Add(time.Duration(craftingItem.CurrentTimeMinutes) * time.Minute)
	h.sseRouter.PublishGameEvent(game, "crafting_update", playerName+" started crafting "+game.CraftingSystem.CraftableItems[req.ItemID].Name, map[string]interface{}{
		"type":           "start",
		"gameId":         req.GameID,
		"craftingId":     craftingItem.ID,
		"itemId":         req.ItemID,
		"playerId":       req.PlayerID,
		"playerName":     playerName,
		"itemName":       game.CraftingSystem.CraftableItems[req.ItemID].Name,
		"startTime":      craftingItem.StartTime,
		"completionTime": completionTime,
		"originalTime":   craftingItem.OriginalTimeMinutes,
		"currentTime":    craftingItem.CurrentTimeMinutes,
	})

	// 마지막 업데이트 시간 갱신
	h.cacheMu.Lock()
	h.lastUpdated[req.GameID] = time.Now()
	h.cacheMu.Unlock()

	// 응답 전송
	w.Header().Set("Content-Type", "application/json")
	encodeErr := json.NewEncoder(w).Encode(game)
	if encodeErr != nil {
		log.Printf("Error encoding response: %v", encodeErr)
	}

	log.Printf("Start crafting request completed successfully for player %s in game %s", req.PlayerID, req.GameID)
}

// HelpCrafting handles helping another player's crafting
func (h *Handler) HelpCrafting(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		log.Printf("Method not allowed: %s %s", r.Method, r.URL.Path)
		return
	}

	var req struct {
		GameID     string `json:"gameId"`
		PlayerID   string `json:"playerId"`
		CraftingID string `json:"craftingId"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		errorMsg := fmt.Sprintf("Invalid request body: %v", err)
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error decoding request body: %v\n%s", err, debug.Stack())
		return
	}

	// 요청 파라미터 검증
	if req.GameID == "" {
		errorMsg := "Game ID is required"
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error: %s\n%s", errorMsg, debug.Stack())
		return
	}

	if req.PlayerID == "" {
		errorMsg := "Player ID is required"
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error: %s\n%s", errorMsg, debug.Stack())
		return
	}

	if req.CraftingID == "" {
		errorMsg := "Crafting ID is required"
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error: %s\n%s", errorMsg, debug.Stack())
		return
	}

	// 게임 캐시에서 게임 객체 가져오기
	var game *domain.Game

	// 캐시 동기화를 위한 락 획득
	h.cacheMu.RLock()
	var exists bool
	game, exists = h.gameCache[req.GameID]
	h.cacheMu.RUnlock()

	if !exists {
		// 캐시에 없는 경우 새 게임 생성
		log.Printf("Creating new game for ID: %s", req.GameID)
		game = domain.NewGame(req.GameID, "")

		// 캐시에 추가 (쓰기 락 필요)
		h.cacheMu.Lock()
		h.gameCache[req.GameID] = game
		h.lastUpdated[req.GameID] = time.Now()
		h.cacheMu.Unlock()
	} else {
		log.Printf("Using cached game for ID: %s", req.GameID)
	}

	// 플레이어 이름 생성
	playerName := "Guest"
	if strings.HasPrefix(req.PlayerID, "user_") {
		playerName = "Guest_" + req.PlayerID[5:10]
	}

	// 플레이어가 없으면 추가
	if _, exists := game.Players[req.PlayerID]; !exists {
		game.AddPlayer(req.PlayerID, playerName)
	}

	// 제작 도움
	craftingItem, err := game.HelpCrafting(req.PlayerID, req.CraftingID)
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to help crafting: %v", err)
		http.Error(w, errorMsg, http.StatusInternalServerError)
		log.Printf("Error helping crafting: %v\n%s", err, debug.Stack())
		return
	}

	// 이벤트 추가
	game.AddEvent("crafting_helped", playerName+" helped "+craftingItem.CrafterName+"'s crafting", map[string]interface{}{
		"craftingId":    req.CraftingID,
		"itemId":        craftingItem.ItemID,
		"helperId":      req.PlayerID,
		"timeReduction": 1,
	})

	// SSE를 통해 제작 도움 이벤트 전파
	// 완료 예상 시간 계산
	completionTime := craftingItem.StartTime.Add(time.Duration(craftingItem.CurrentTimeMinutes) * time.Minute)
	h.sseRouter.PublishGameEvent(game, "crafting_update", playerName+" helped "+craftingItem.CrafterName+"'s crafting", map[string]interface{}{
		"type":           "help",
		"gameId":         req.GameID,
		"craftingId":     req.CraftingID,
		"itemId":         craftingItem.ItemID,
		"helperId":       req.PlayerID,
		"helperName":     playerName,
		"crafterId":      craftingItem.CrafterID,
		"crafterName":    craftingItem.CrafterName,
		"itemName":       craftingItem.ItemID, // 아이템 이름 대신 ID 사용 (이름은 클라이언트에서 조회)
		"startTime":      craftingItem.StartTime,
		"completionTime": completionTime,
		"originalTime":   craftingItem.OriginalTimeMinutes,
		"currentTime":    craftingItem.CurrentTimeMinutes,
		"timeReduction":  1,
	})

	// 마지막 업데이트 시간 갱신
	h.cacheMu.Lock()
	h.lastUpdated[req.GameID] = time.Now()
	h.cacheMu.Unlock()

	// 응답 전송
	w.Header().Set("Content-Type", "application/json")
	encodeErr := json.NewEncoder(w).Encode(game)
	if encodeErr != nil {
		log.Printf("Error encoding response: %v", encodeErr)
	}

	log.Printf("Help crafting request completed successfully for player %s in game %s", req.PlayerID, req.GameID)
}

// JoinRoom handles a player joining a room
func (h *Handler) JoinRoom(w http.ResponseWriter, r *http.Request) {
	// 메서드 검증
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		log.Printf("Method not allowed: %s %s", r.Method, r.URL.Path)
		return
	}

	var req struct {
		RoomID     string `json:"roomId"`
		PlayerName string `json:"playerName"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		errorMsg := fmt.Sprintf("Invalid request body: %v", err)
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error decoding request body: %v\n%s", err, debug.Stack())
		return
	}

	// 요청 파라미터 검증
	if req.RoomID == "" {
		errorMsg := "Room ID is required"
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error: %s\n%s", errorMsg, debug.Stack())
		return
	}

	if req.PlayerName == "" {
		errorMsg := "Player name is required"
		http.Error(w, errorMsg, http.StatusBadRequest)
		log.Printf("Error: %s\n%s", errorMsg, debug.Stack())
		return
	}

	// Generate a unique ID for the player
	playerID := uuid.New().String()

	// Join the room's game
	game, err := h.roomUC.Join(req.RoomID, playerID, req.PlayerName)
	if err != nil {
		// 오류 메시지에 더 자세한 정보 포함
		joinErr := fmt.Errorf("failed to join room: %w", err)
		http.Error(w, joinErr.Error(), http.StatusInternalServerError)
		log.Printf("Error joining room: %v\n%s", joinErr, debug.Stack())
		return
	}

	// Get the room
	room, err := h.roomUC.Get(req.RoomID)
	if err != nil {
		// 오류 메시지에 더 자세한 정보 포함
		getRoomErr := fmt.Errorf("failed to get room after joining: %w", err)
		http.Error(w, getRoomErr.Error(), http.StatusInternalServerError)
		log.Printf("Error getting room: %v\n%s", getRoomErr, debug.Stack())
		return
	}

	// Publish player join event
	h.sseRouter.PublishGameEvent(game, "player_join", req.PlayerName+" joined room "+room.Name, map[string]interface{}{
		"roomId":     req.RoomID,
		"playerId":   playerID,
		"playerName": req.PlayerName,
		"game":       game,
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
	encodeErr := json.NewEncoder(w).Encode(response)
	if encodeErr != nil {
		// 인코딩 오류 로깅 (이미 응답이 시작되었으므로 클라이언트에게 오류를 반환할 수 없음)
		log.Printf("Error encoding response: %v", encodeErr)
	}
}
