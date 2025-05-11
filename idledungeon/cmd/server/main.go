package main

import (
	"flag"
	"idledungeon/pkg/server"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 8080, "HTTP server port")
	mongoURI := flag.String("mongo", "mongodb://localhost:27017", "MongoDB URI")
	dbName := flag.String("db", "idledungeon", "Database name")
	clientPath := flag.String("client", "./client", "Path to client files")
	debug := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	// Create logger
	logger := createLogger(*debug)
	defer logger.Sync()

	// Create server config
	config := server.Config{
		Port:       *port,
		MongoURI:   *mongoURI,
		DBName:     *dbName,
		ClientPath: *clientPath,
	}

	// Create server
	srv, err := server.NewServer(config, logger)
	if err != nil {
		logger.Fatal("Failed to create server", zap.Error(err))
	}

	// Start server
	if err := srv.Start(); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}

	os.Exit(0)
}

// createLogger creates a new zap logger
func createLogger(debug bool) *zap.Logger {
	// Create encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// Create core
	var core zapcore.Core
	if debug {
		// Debug mode: console encoder, debug level
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		core = zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			zap.DebugLevel,
		)
	} else {
		// Production mode: JSON encoder, info level
		core = zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(os.Stdout),
			zap.InfoLevel,
		)
	}

	// Create logger
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))
}
