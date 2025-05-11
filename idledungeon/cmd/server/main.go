package main

import (
	"context"
	"flag"
	"idledungeon/pkg/server"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 8080, "HTTP server port")
	mongoURI := flag.String("mongo", "mongodb://localhost:27017", "MongoDB connection URI")
	dbName := flag.String("db", "idledungeon", "MongoDB database name")
	clientDir := flag.String("client-dir", "./client", "Directory containing client files")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Create logger
	logger := createLogger(*debug)
	defer logger.Sync()

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(*mongoURI))
	if err != nil {
		logger.Fatal("Failed to connect to MongoDB", zap.Error(err))
	}
	defer mongoClient.Disconnect(context.Background())

	// Ping MongoDB to verify connection
	if err := mongoClient.Ping(ctx, nil); err != nil {
		logger.Fatal("Failed to ping MongoDB", zap.Error(err))
	}
	logger.Info("Connected to MongoDB", zap.String("uri", *mongoURI))

	// Resolve client directory path
	absClientDir, err := filepath.Abs(*clientDir)
	if err != nil {
		logger.Fatal("Failed to resolve client directory path", zap.Error(err))
	}

	// Check if client directory exists
	if _, err := os.Stat(absClientDir); os.IsNotExist(err) {
		logger.Fatal("Client directory does not exist", zap.String("path", absClientDir))
	}
	logger.Info("Using client directory", zap.String("path", absClientDir))

	// Create game server
	gameServer, err := server.NewGameServer(context.Background(), mongoClient, *dbName, absClientDir, logger)
	if err != nil {
		logger.Fatal("Failed to create game server", zap.Error(err))
	}

	// Handle graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		logger.Info("Received signal, shutting down", zap.String("signal", sig.String()))

		// Stop game server
		if err := gameServer.Stop(); err != nil {
			logger.Error("Failed to stop game server", zap.Error(err))
		}

		// Exit
		os.Exit(0)
	}()

	// Start game server
	logger.Info("Starting game server", zap.Int("port", *port))
	if err := gameServer.Start(*port); err != nil {
		logger.Fatal("Failed to start game server", zap.Error(err))
	}
}

// createLogger creates a new logger
func createLogger(debug bool) *zap.Logger {
	// Create logger config
	config := zap.NewProductionConfig()

	// Set log level
	if debug {
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	} else {
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	// Create logger
	logger, err := config.Build()
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}

	return logger
}
