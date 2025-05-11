package server

import (
	"context"
	"fmt"
	"idledungeon/internal/model"
	"idledungeon/internal/storage"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"tictactoe/pkg/utils"
	"time"

	"go.uber.org/zap"
)

// Config represents the server configuration
type Config struct {
	Port       int    `json:"port"`
	MongoURI   string `json:"mongoUri"`
	DBName     string `json:"dbName"`
	ClientPath string `json:"clientPath"`
}

// DefaultConfig returns the default server configuration
func DefaultConfig() Config {
	return Config{
		Port:       8080,
		MongoURI:   "mongodb://localhost:27017",
		DBName:     "idledungeon",
		ClientPath: "./client",
	}
}

// Server represents the game server
type Server struct {
	config     Config
	storage    *storage.GameStorage
	router     *http.ServeMux
	server     *http.Server
	logger     *zap.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	sseManager *SSEManager
}

// NewServer creates a new game server
func NewServer(config Config, logger *zap.Logger) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create storage
	gameStorage, err := storage.NewGameStorage(ctx, config.MongoURI, config.DBName, logger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create game storage: %w", err)
	}

	// Create router
	router := http.NewServeMux()

	// Create SSE manager
	sseManager := NewSSEManager(logger)

	// Create server
	server := &Server{
		config:     config,
		storage:    gameStorage,
		router:     router,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		sseManager: sseManager,
	}

	// 로그 추가
	logger.Debug("Server created",
		zap.String("mongoURI", config.MongoURI),
		zap.String("dbName", config.DBName),
		zap.Int("port", config.Port),
		zap.String("clientPath", config.ClientPath),
	)

	// Set up routes
	server.setupRoutes()

	return server, nil
}

// setupRoutes sets up the HTTP routes
func (s *Server) setupRoutes() {
	// 로그 추가
	s.logger.Debug("Setting up routes")

	// Serve static files
	s.logger.Debug("Serving static files from", zap.String("path", s.config.ClientPath))
	s.router.Handle("/", http.FileServer(http.Dir(s.config.ClientPath)))

	// API routes
	s.logger.Debug("Setting up API routes")
	s.router.HandleFunc("/api/games", s.handleGames)
	s.router.HandleFunc("/api/games/", s.handleGame)
	s.router.HandleFunc("/api/players", s.handlePlayers)
	s.router.HandleFunc("/api/players/", s.handlePlayer)
	s.router.HandleFunc("/api/sync", s.handleSync)

	// SSE route
	s.logger.Debug("Setting up SSE route")
	s.router.HandleFunc("/events", s.handleSSE)

	s.logger.Debug("Routes setup complete")
}

// Start starts the server
func (s *Server) Start() error {
	// 미들웨어 적용
	handler := MiddlewareChain(s.router,
		func(h http.Handler) http.Handler { return LoggingMiddleware(s.logger, h) },
		func(h http.Handler) http.Handler { return utils.ErrorHandlerMiddleware(s.logger, h) },
		utils.RequestIDMiddleware,
		CORSMiddleware,
	)

	// Create HTTP server
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.Port),
		Handler: handler,
	}

	// Start server in a goroutine
	go func() {
		s.logger.Info("Starting server", zap.Int("port", s.config.Port))
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Fatal("Server error", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	// Shutdown server
	s.logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Error("Server shutdown error", zap.Error(err))
		return err
	}

	// Close storage
	if err := s.storage.Close(); err != nil {
		s.logger.Error("Storage close error", zap.Error(err))
		return err
	}

	s.logger.Info("Server stopped")
	return nil
}

// Stop stops the server
func (s *Server) Stop() error {
	s.cancel()
	return nil
}

// GetWorldConfig returns the world configuration
func (s *Server) GetWorldConfig() model.WorldConfig {
	return model.DefaultWorldConfig()
}
