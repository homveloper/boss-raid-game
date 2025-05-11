package server

import (
	"context"
	"fmt"
	"idledungeon/internal/model"
	"idledungeon/internal/storage"
	"net/http"
	"sync"
	"time"

	"eventsync"
	"nodestorage/v2"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// GameServer represents the game server
type GameServer struct {
	ctx                context.Context
	cancel             context.CancelFunc
	gameStorage        *storage.GameStorage
	syncService        eventsync.SyncService
	eventStore         eventsync.EventStore
	stateVectorManager eventsync.StateVectorManager
	storageListener    *eventsync.StorageListener
	sseHandler         *SSEHandler
	httpServer         *http.Server
	clientDir          string
	logger             *zap.Logger
	updateTicker       *time.Ticker
	updateMutex        sync.Mutex
	games              map[primitive.ObjectID]*model.Game
	gamesMutex         sync.RWMutex
}

// NewGameServer creates a new game server
func NewGameServer(ctx context.Context, mongoClient *mongo.Client, dbName, clientDir string, logger *zap.Logger) (*GameServer, error) {
	// Create context with cancel
	serverCtx, cancel := context.WithCancel(ctx)

	// Create game storage
	gameStorage, err := storage.NewGameStorage(serverCtx, mongoClient, dbName, logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create game storage: %w", err)
	}

	// Create event store
	eventStore, err := eventsync.NewMongoEventStore(serverCtx, mongoClient, dbName, "events", logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create event store: %w", err)
	}

	// Create state vector manager
	stateVectorManager, err := eventsync.NewMongoStateVectorManager(serverCtx, mongoClient, dbName, "state_vectors", eventStore, logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create state vector manager: %w", err)
	}

	// Create sync service
	syncService := eventsync.NewSyncService(eventStore, stateVectorManager, logger)

	// Create SSE handler
	sseHandler := NewSSEHandler(syncService, logger)

	// Create server
	server := &GameServer{
		ctx:                serverCtx,
		cancel:             cancel,
		gameStorage:        gameStorage,
		syncService:        syncService,
		eventStore:         eventStore,
		stateVectorManager: stateVectorManager,
		sseHandler:         sseHandler,
		clientDir:          clientDir,
		logger:             logger,
		games:              make(map[primitive.ObjectID]*model.Game),
	}

	// Create storage adapter
	storageAdapter := eventsync.NewStorageAdapter[*model.Game](gameStorage.GetStorage(), logger)

	// Create storage listener
	storageListener := eventsync.NewStorageListener(storageAdapter, syncService, logger)
	server.storageListener = storageListener

	return server, nil
}

// Start starts the game server
func (s *GameServer) Start(port int) error {
	// Start storage listener
	if err := s.storageListener.Start(); err != nil {
		return fmt.Errorf("failed to start storage listener: %w", err)
	}

	// Create router
	router := http.NewServeMux()

	// API 라우터 생성 (미들웨어 적용)
	apiRouter := http.NewServeMux()
	apiRouter.HandleFunc("/api/games", s.handleGetGames)
	apiRouter.HandleFunc("/api/games/get", s.handleGetGame)
	apiRouter.HandleFunc("/api/games/create", s.handleCreateGame)
	apiRouter.HandleFunc("/api/games/join", s.handleJoinGame)
	apiRouter.HandleFunc("/api/games/move", s.handleMovePlayer)
	apiRouter.HandleFunc("/api/games/attack", s.handleAttackMonster)

	// API 라우터에 미들웨어 적용
	apiHandler := LoggingMiddleware(s.logger, apiRouter)
	apiHandler = RecoveryMiddleware(s.logger, apiHandler)

	// 메인 라우터에 API 핸들러 등록
	router.Handle("/api/games", apiHandler)
	router.Handle("/api/games/get", apiHandler)
	router.Handle("/api/games/create", apiHandler)
	router.Handle("/api/games/join", apiHandler)
	router.Handle("/api/games/move", apiHandler)
	router.Handle("/api/games/attack", apiHandler)

	// Register SSE handler (미들웨어 적용 없이)
	router.Handle("/api/events", s.sseHandler)

	// Register static file handler
	fs := http.FileServer(http.Dir(s.clientDir))
	router.Handle("/", fs)

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}

	// Start game update ticker (200ms 간격으로 업데이트)
	s.updateTicker = time.NewTicker(200 * time.Millisecond)
	go s.runGameUpdates()

	// Start HTTP server
	s.logger.Info("Starting game server", zap.Int("port", port))
	return s.httpServer.ListenAndServe()
}

// Stop stops the game server
func (s *GameServer) Stop() error {
	// Stop update ticker
	if s.updateTicker != nil {
		s.updateTicker.Stop()
	}

	// Stop HTTP server
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(s.ctx); err != nil {
			return fmt.Errorf("failed to shutdown HTTP server: %w", err)
		}
	}

	// Stop storage listener
	if s.storageListener != nil {
		s.storageListener.Stop()
	}

	// Close game storage
	if s.gameStorage != nil {
		if err := s.gameStorage.Close(); err != nil {
			return fmt.Errorf("failed to close game storage: %w", err)
		}
	}

	// Cancel context
	s.cancel()

	return nil
}

// runGameUpdates runs the game update loop
func (s *GameServer) runGameUpdates() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.updateTicker.C:
			s.updateGames()
		}
	}
}

// updateGames updates all active games
func (s *GameServer) updateGames() {
	s.updateMutex.Lock()
	defer s.updateMutex.Unlock()

	// Get all games
	s.gamesMutex.RLock()
	gameIDs := make([]primitive.ObjectID, 0, len(s.games))
	for id := range s.games {
		gameIDs = append(gameIDs, id)
	}
	s.gamesMutex.RUnlock()

	// Update each game
	for _, id := range gameIDs {
		// Update game
		_, _, err := s.gameStorage.UpdateGame(s.ctx, id, func(game *model.Game) (*model.Game, error) {
			// Only update games in playing state
			if game.State == model.GameStatePlaying {
				game.UpdateGame()
			}
			return game, nil
		})
		if err != nil {
			s.logger.Error("Failed to update game", zap.String("game_id", id.Hex()), zap.Error(err))
		}
	}
}

// GetStorage returns the game storage
func (s *GameServer) GetStorage() nodestorage.Storage[*model.Game] {
	return s.gameStorage.GetStorage()
}

// GetAllGames gets all games
func (s *GameServer) GetAllGames(ctx context.Context) ([]*model.Game, error) {
	return s.gameStorage.GetAllGames(ctx)
}
