package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"tictactoe/internal/delivery/sse"
	"tictactoe/internal/domain"
	"tictactoe/internal/repository/crdt"
	"tictactoe/internal/repository/memory"
	"tictactoe/internal/usecase"
	"time"

	ds "github.com/ipfs/go-datastore"

	httpHandler "tictactoe/internal/delivery/http"
)

// GameServer represents the boss raid game server
type GameServer struct {
	ctx           context.Context
	cancel        context.CancelFunc
	crdtStore     ds.Datastore
	gameRepo      domain.GameRepository
	roomRepo      domain.RoomRepository
	itemRepo      domain.ItemRepository
	characterRepo domain.CharacterRepository
	gameUC        domain.GameUseCase
	roomUC        domain.RoomUseCase
	itemUC        domain.ItemUseCase
	characterUC   domain.CharacterUseCase
	sseRouter     *sse.Router
	handler       *httpHandler.Handler
	clientDir     string
}

// NewGameServer creates a new game server
func NewGameServer(ctx context.Context, crdtStore ds.Datastore, clientDir string) *GameServer {
	ctx, cancel := context.WithCancel(ctx)

	// CRDT 작업을 위한 타임아웃이 있는 컨텍스트 생성 (30초)
	crdtCtx, _ := context.WithTimeout(ctx, 30*time.Second)

	// Create repositories
	gameRepo := crdt.NewGameRepository(crdtCtx, crdtStore, "/bossraid")
	roomRepo := crdt.NewRoomRepository(crdtCtx, crdtStore, "/bossraid")
	itemRepo := memory.NewItemRepository() // Using memory repository for items as they are static
	characterRepo := crdt.NewCharacterRepository(crdtCtx, crdtStore, "/bossraid")

	// Create use cases
	gameUC := usecase.NewGameUseCase(gameRepo)
	itemUC := usecase.NewItemUseCase(itemRepo)
	characterUC := usecase.NewCharacterUseCase(characterRepo, itemRepo)
	roomUC := usecase.NewRoomUseCase(roomRepo, gameUC)

	// Create SSE router
	sseRouter := sse.NewRouter(gameUC)

	// Create HTTP handler
	handler := httpHandler.NewHandler(roomUC, gameUC, sseRouter)

	// Initialize default items
	initializeDefaultItems(itemRepo)

	return &GameServer{
		ctx:           ctx,
		cancel:        cancel,
		crdtStore:     crdtStore,
		gameRepo:      gameRepo,
		roomRepo:      roomRepo,
		itemRepo:      itemRepo,
		characterRepo: characterRepo,
		gameUC:        gameUC,
		roomUC:        roomUC,
		itemUC:        itemUC,
		characterUC:   characterUC,
		sseRouter:     sseRouter,
		handler:       handler,
		clientDir:     clientDir,
	}
}

// RecoveryMiddleware recovers from panics and logs the error
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// 스택 트레이스 가져오기
				stackTrace := debug.Stack()

				// 패닉 발생 시 로그 기록
				log.Printf("PANIC in %s %s: %v\n%s", r.Method, r.URL.Path, err, stackTrace)

				// 디버그 모드에서는 스택 트레이스를 포함한 상세 오류 정보 반환
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)

				// 오류 응답 생성
				errorResponse := map[string]interface{}{
					"error":     "Internal server error",
					"message":   fmt.Sprintf("%v", err),
					"path":      r.URL.Path,
					"method":    r.Method,
					"timestamp": time.Now().Format(time.RFC3339),
				}

				// 디버그 모드에서는 스택 트레이스 포함
				if os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1" {
					errorResponse["stack"] = string(stackTrace)
				}

				json.NewEncoder(w).Encode(errorResponse)
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// LogError logs an error with stack trace
func LogError(err error, message string, keysAndValues ...interface{}) {
	// 에러 메시지 및 스택 트레이스 기록
	log.Printf("%s: %v\n%s", message, err, debug.Stack())

	// 추가 정보 기록
	if len(keysAndValues) > 0 {
		for i := 0; i < len(keysAndValues); i += 2 {
			if i+1 < len(keysAndValues) {
				log.Printf("  %v: %v", keysAndValues[i], keysAndValues[i+1])
			} else {
				log.Printf("  %v: <no value>", keysAndValues[i])
			}
		}
	}
}

// ErrorResponseWriter is a wrapper for http.ResponseWriter that captures errors
type ErrorResponseWriter struct {
	*ResponseWriter
	errors []string
}

// NewErrorResponseWriter creates a new ErrorResponseWriter
func NewErrorResponseWriter(w http.ResponseWriter) *ErrorResponseWriter {
	return &ErrorResponseWriter{
		ResponseWriter: NewResponseWriter(w),
		errors:         []string{},
	}
}

// WriteError writes an error response and logs it
func (erw *ErrorResponseWriter) WriteError(err error, status int, r *http.Request) {
	// 에러 메시지 기록
	errorMsg := fmt.Sprintf("%v", err)
	erw.errors = append(erw.errors, errorMsg)

	// 에러 로그 기록
	LogError(err, fmt.Sprintf("HTTP Error in %s %s", r.Method, r.URL.Path),
		"status", status,
		"method", r.Method,
		"path", r.URL.Path,
		"query", r.URL.RawQuery,
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent(),
	)

	// 클라이언트에게 오류 응답 전송
	erw.Header().Set("Content-Type", "application/json")
	erw.WriteHeader(status)

	// 오류 응답 생성
	errorResponse := map[string]interface{}{
		"error":     http.StatusText(status),
		"message":   errorMsg,
		"path":      r.URL.Path,
		"method":    r.Method,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	// 디버그 모드에서는 스택 트08이스 포함
	if os.Getenv("DEBUG") == "true" || os.Getenv("DEBUG") == "1" {
		errorResponse["stack"] = string(debug.Stack())
	}

	json.NewEncoder(erw).Encode(errorResponse)
}

// HasErrors returns true if there are errors
func (erw *ErrorResponseWriter) HasErrors() bool {
	return len(erw.errors) > 0
}

// GetErrors returns the errors
func (erw *ErrorResponseWriter) GetErrors() []string {
	return erw.errors
}

// LoggingMiddleware logs all requests
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 요청 정보 로그
		log.Printf("Started %s %s", r.Method, r.URL.Path)

		// 응답 정보를 기록하기 위한 ResponseWriter 래퍼
		ww := NewErrorResponseWriter(w)

		// 다음 핸들러 호출
		next.ServeHTTP(ww, r)

		// 요청 처리 완료 후 로그
		status := ww.Status()
		statusText := http.StatusText(status)
		duration := time.Since(start)

		if ww.HasErrors() {
			// 에러가 있는 경우 오류 로그 추가
			log.Printf("Completed %s %s %d %s in %v with errors: %v",
				r.Method, r.URL.Path, status, statusText, duration, ww.GetErrors())
		} else {
			// 성공적인 요청 로그
			log.Printf("Completed %s %s %d %s in %v",
				r.Method, r.URL.Path, status, statusText, duration)
		}
	})
}

// ResponseWriter is a wrapper for http.ResponseWriter that captures the status code
type ResponseWriter struct {
	http.ResponseWriter
	status int
}

// NewResponseWriter creates a new ResponseWriter
func NewResponseWriter(w http.ResponseWriter) *ResponseWriter {
	return &ResponseWriter{w, http.StatusOK}
}

// WriteHeader captures the status code and calls the underlying WriteHeader
func (rw *ResponseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Status returns the HTTP status code
func (rw *ResponseWriter) Status() int {
	return rw.status
}

// Flush implements the http.Flusher interface
func (rw *ResponseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// ApplyMiddleware applies middleware to a handler
func ApplyMiddleware(h http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for _, m := range middleware {
		h = m(h)
	}
	return h
}

// Start starts the game server
func (s *GameServer) Start(port int) error {
	// Set up HTTP server
	// API 라우트 설정
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/rooms", s.handler.GetRooms)
	apiMux.HandleFunc("/api/rooms/create", s.handler.CreateRoom)
	apiMux.HandleFunc("/api/rooms/get", s.handler.GetRoom)
	apiMux.HandleFunc("/api/rooms/join", s.handler.JoinRoom)
	apiMux.HandleFunc("/api/games/join", s.handler.JoinGame)
	apiMux.HandleFunc("/api/games/ready", s.handler.ReadyGame)
	apiMux.HandleFunc("/api/games/get", s.handler.GetGame)
	apiMux.HandleFunc("/api/games/attack", s.handler.AttackBoss)
	apiMux.HandleFunc("/api/games/boss-action", s.handler.TriggerBossAction)

	// 제작 시스템 라우트 설정
	apiMux.HandleFunc("/api/crafting/items", s.handler.GetCraftableItems)
	apiMux.HandleFunc("/api/crafting/in-progress", s.handler.GetCraftingItems)
	apiMux.HandleFunc("/api/crafting/start", s.handler.StartCrafting)
	apiMux.HandleFunc("/api/crafting/help", s.handler.HelpCrafting)

	// SSE 라우트 설정
	apiMux.HandleFunc("/api/events", s.sseRouter.HandleEvents)

	// 정적 파일 제공
	log.Printf("Serving client files from: %s", s.clientDir)
	fs := http.FileServer(http.Dir(s.clientDir))
	// 정적 파일 요청 처리
	apiMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 디버그 로그
		log.Printf("Received request for: %s", r.URL.Path)

		// index.html 파일 요청 처리
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(s.clientDir, "index.html"))
			return
		}

		// 나머지 정적 파일 요청 처리
		fs.ServeHTTP(w, r)
	})

	// 미들웨어 적용
	handler := ApplyMiddleware(apiMux, RecoveryMiddleware, LoggingMiddleware)

	// Start HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}

	// Start boss attack processor
	go s.processBossAttacks()

	log.Printf("Game server started on port %d", port)
	return server.ListenAndServe()
}

// Stop stops the game server
func (s *GameServer) Stop() {
	s.cancel()
}

// processBossAttacks processes boss attacks for all active games
func (s *GameServer) processBossAttacks() {
	ticker := time.NewTicker(500 * time.Millisecond) // Check every 500ms
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// Get all games
			games, err := s.gameRepo.List()
			if err != nil {
				logger.Warnf("Error listing games: %v", err)
				continue
			}

			// Process boss attacks for active games
			for _, game := range games {
				if game.State == domain.GameStatePlaying && !game.Boss.IsDefeated() {
					if game.Boss.CanAttack() {
						// Process boss attack
						game, targetPlayer, damage, err := s.gameUC.ProcessBossAction(game.ID)
						if err != nil {
							continue
						}

						// Publish boss attack event
						if targetPlayer != nil {
							s.sseRouter.PublishGameEvent(game, "boss_attack",
								fmt.Sprintf("%s attacked %s for %d damage", game.Boss.Name, targetPlayer.Name, damage),
								map[string]interface{}{
									"playerID": targetPlayer.ID,
									"damage":   damage,
								})

							// Check if player is defeated
							if targetPlayer.Character.Stats.Health <= 0 {
								s.sseRouter.PublishGameEvent(game, "player_defeated",
									fmt.Sprintf("%s has been defeated!", targetPlayer.Name),
									targetPlayer)
							}

							// Check if game is over
							if game.State == domain.GameStateFinished {
								if game.Result == domain.GameResultVictory {
									s.sseRouter.PublishGameEvent(game, "game_end",
										fmt.Sprintf("Victory! The %s has been defeated!", game.Boss.Name),
										game.Rewards)
								} else {
									s.sseRouter.PublishGameEvent(game, "game_end",
										fmt.Sprintf("Defeat! The party has been wiped out by the %s!", game.Boss.Name),
										nil)
								}
							}
						}
					}
				}
			}
		}
	}
}

// initializeDefaultItems initializes default items in the item repository
func initializeDefaultItems(itemRepo domain.ItemRepository) {
	// Create default weapons
	weapons := []*domain.Item{
		{
			ID:          "weapon_sword",
			Name:        "Steel Sword",
			Description: "A standard steel sword",
			Type:        domain.ItemTypeWeapon,
			Stats: domain.ItemStats{
				Damage:      15,
				AttackSpeed: 1200 * time.Millisecond,
				WeaponType:  domain.WeaponTypeLongsword,
			},
		},
		{
			ID:          "weapon_dagger",
			Name:        "Shadow Dagger",
			Description: "A quick dagger for fast attacks",
			Type:        domain.ItemTypeWeapon,
			Stats: domain.ItemStats{
				Damage:      8,
				AttackSpeed: 800 * time.Millisecond,
				WeaponType:  domain.WeaponTypeDagger,
			},
		},
		{
			ID:          "weapon_bow",
			Name:        "Longbow",
			Description: "A powerful bow for ranged attacks",
			Type:        domain.ItemTypeWeapon,
			Stats: domain.ItemStats{
				Damage:      12,
				AttackSpeed: 1500 * time.Millisecond,
				WeaponType:  domain.WeaponTypeBow,
			},
		},
		{
			ID:          "weapon_axe",
			Name:        "Battle Axe",
			Description: "A heavy axe for powerful attacks",
			Type:        domain.ItemTypeWeapon,
			Stats: domain.ItemStats{
				Damage:      20,
				AttackSpeed: 1800 * time.Millisecond,
				WeaponType:  domain.WeaponTypeAxe,
			},
		},
	}

	// Create default armors
	armors := []*domain.Item{
		{
			ID:          "armor_leather",
			Name:        "Leather Armor",
			Description: "Basic leather armor",
			Type:        domain.ItemTypeArmor,
			Stats: domain.ItemStats{
				Defense:   5,
				ArmorType: domain.ArmorTypeLeather,
			},
		},
		{
			ID:          "armor_chain",
			Name:        "Chain Mail",
			Description: "Chain mail armor",
			Type:        domain.ItemTypeArmor,
			Stats: domain.ItemStats{
				Defense:   10,
				ArmorType: domain.ArmorTypeLeather,
			},
		},
		{
			ID:          "armor_plate",
			Name:        "Plate Armor",
			Description: "Heavy plate armor",
			Type:        domain.ItemTypeArmor,
			Stats: domain.ItemStats{
				Defense:   15,
				ArmorType: domain.ArmorTypeLeather,
			},
		},
	}

	// Add weapons to repository
	for _, weapon := range weapons {
		if repo, ok := itemRepo.(*crdt.ItemRepository); ok {
			// CRDT 저장소에 아이템 추가
			_ = repo.Create(weapon)
		}
	}

	// Add armors to repository
	for _, armor := range armors {
		if repo, ok := itemRepo.(*crdt.ItemRepository); ok {
			// CRDT 저장소에 아이템 추가
			_ = repo.Create(armor)
		}
	}
}

// GetGameState returns the state of a game
func (s *GameServer) GetGameState(gameID string) ([]byte, error) {
	game, err := s.gameUC.Get(gameID)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(game)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// CreateRoom creates a new room
func (s *GameServer) CreateRoom(name string) ([]byte, error) {
	room, err := s.roomUC.Create(name)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(room)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// JoinRoom adds a player to a room
func (s *GameServer) JoinRoom(roomID, playerName string) ([]byte, error) {
	// Generate a unique ID for the player
	playerID := fmt.Sprintf("player_%d", time.Now().UnixNano())

	game, err := s.roomUC.Join(roomID, playerID, playerName)
	if err != nil {
		return nil, err
	}

	response := struct {
		PlayerID string      `json:"playerId"`
		Game     domain.Game `json:"game"`
	}{
		PlayerID: playerID,
		Game:     *game,
	}

	data, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// ReadyPlayer sets a player's ready status
func (s *GameServer) ReadyPlayer(gameID, playerID string) ([]byte, error) {
	game, err := s.gameUC.Ready(gameID, playerID)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(game)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// AttackBoss performs a player attack on the boss
func (s *GameServer) AttackBoss(gameID, playerID string) ([]byte, error) {
	game, err := s.gameUC.Attack(gameID, playerID)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(game)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// EquipItem equips an item for a player
func (s *GameServer) EquipItem(gameID, playerID, itemID string) ([]byte, error) {
	game, err := s.gameUC.EquipItem(gameID, playerID, itemID)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(game)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// ListRooms returns all rooms
func (s *GameServer) ListRooms() ([]byte, error) {
	rooms, err := s.roomUC.List()
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(rooms)
	if err != nil {
		return nil, err
	}

	return data, nil
}
