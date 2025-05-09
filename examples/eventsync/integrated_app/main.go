package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"nodestorage/v2"
	"nodestorage/v2/cache"
	"eventsync"
)

// GameState는 게임 상태를 나타내는 구조체입니다.
type GameState struct {
	ID          primitive.ObjectID `bson:"_id" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Gold        int                `bson:"gold" json:"gold"`
	Level       int                `bson:"level" json:"level"`
	Experience  int                `bson:"experience" json:"experience"`
	LastUpdated time.Time          `bson:"last_updated" json:"lastUpdated"`
	Version     int64              `bson:"version" json:"version"`
}

// StorageEventHandler는 nodestorage 이벤트를 처리하는 핸들러입니다.
type StorageEventHandler struct {
	eventStore eventsync.EventStore
	logger     *zap.Logger
}

// NewStorageEventHandler는 새로운 StorageEventHandler를 생성합니다.
func NewStorageEventHandler(eventStore eventsync.EventStore, logger *zap.Logger) *StorageEventHandler {
	return &StorageEventHandler{
		eventStore: eventStore,
		logger:     logger,
	}
}

// HandleEvent는 nodestorage 이벤트를 처리합니다.
func (h *StorageEventHandler) HandleEvent(ctx context.Context, event nodestorage.WatchEvent[*GameState]) error {
	// nodestorage 이벤트를 eventsync 이벤트로 변환
	syncEvent := &eventsync.Event{
		ID:          primitive.NewObjectID(),
		DocumentID:  event.ID,
		Timestamp:   time.Now(),
		Operation:   event.Operation,
		Diff:        event.Diff,
		ClientID:    "server",
		VectorClock: make(map[string]int64),
	}

	// 이벤트 저장
	err := h.eventStore.StoreEvent(ctx, syncEvent)
	if err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}

	h.logger.Info("Storage event converted and stored",
		zap.String("document_id", event.ID.Hex()),
		zap.String("operation", event.Operation))

	return nil
}

// App 구조체는 애플리케이션의 주요 컴포넌트를 포함합니다.
type App struct {
	storage         nodestorage.Storage[*GameState]
	eventStore      eventsync.EventStore
	snapshotStore   eventsync.SnapshotStore
	stateVectorManager eventsync.StateVectorManager
	syncService     eventsync.SyncService
	eventHandler    *StorageEventHandler
	watchCh         <-chan nodestorage.WatchEvent[*GameState]
	logger          *zap.Logger
	router          *mux.Router
	wg              sync.WaitGroup
	stopCh          chan struct{}
}

// NewApp은 새로운 App 인스턴스를 생성합니다.
func NewApp(ctx context.Context, client *mongo.Client, dbName string, logger *zap.Logger) (*App, error) {
	// 컬렉션 설정
	gameCollection := client.Database(dbName).Collection("games")
	eventCollection := client.Database(dbName).Collection("events")
	snapshotCollection := client.Database(dbName).Collection("snapshots")
	stateVectorCollection := client.Database(dbName).Collection("state_vectors")

	// 1. nodestorage 설정
	memCache := cache.NewMemoryCache[*GameState](nil)
	storageOptions := &nodestorage.Options{
		VersionField:      "version",
		CacheTTL:          time.Minute * 10,
		WatchEnabled:      true,
		WatchFullDocument: "updateLookup",
	}

	storage, err := nodestorage.NewStorage[*GameState](ctx, gameCollection, memCache, storageOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	// 2. eventsync 설정
	eventStore, err := eventsync.NewMongoEventStore(ctx, client, dbName, "events", logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create event store: %w", err)
	}

	// 3. 스냅샷 저장소 설정
	snapshotStore, err := eventsync.NewMongoSnapshotStore(ctx, client, dbName, "snapshots", eventStore, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %w", err)
	}

	// 4. 상태 벡터 관리자 설정
	stateVectorManager, err := eventsync.NewMongoStateVectorManager(ctx, client, dbName, "state_vectors", eventStore, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create state vector manager: %w", err)
	}

	// 5. 동기화 서비스 설정
	syncService := eventsync.NewSyncService(eventStore, stateVectorManager, logger)

	// 6. nodestorage 이벤트 핸들러 설정
	eventHandler := NewStorageEventHandler(eventStore, logger)

	// 7. nodestorage 이벤트 구독
	watchCh, err := storage.Watch(ctx, mongo.Pipeline{}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to watch storage: %w", err)
	}

	// 8. 라우터 설정
	router := mux.NewRouter()

	return &App{
		storage:         storage,
		eventStore:      eventStore,
		snapshotStore:   snapshotStore,
		stateVectorManager: stateVectorManager,
		syncService:     syncService,
		eventHandler:    eventHandler,
		watchCh:         watchCh,
		logger:          logger,
		router:          router,
		stopCh:          make(chan struct{}),
	}, nil
}

// Start는 애플리케이션을 시작합니다.
func (a *App) Start(port int) error {
	// 1. 이벤트 처리 고루틴 시작
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		for {
			select {
			case <-a.stopCh:
				return
			case event, ok := <-a.watchCh:
				if !ok {
					a.logger.Warn("Watch channel closed")
					return
				}
				if err := a.eventHandler.HandleEvent(context.Background(), event); err != nil {
					a.logger.Error("Failed to handle event", zap.Error(err))
				}
			}
		}
	}()

	// 2. API 라우트 설정
	a.setupRoutes()

	// 3. 주기적 스냅샷 생성 고루틴 시작
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(10 * time.Minute) // 10분마다 스냅샷 생성
		defer ticker.Stop()

		for {
			select {
			case <-a.stopCh:
				return
			case <-ticker.C:
				a.createSnapshots()
			}
		}
	}()

	// 4. HTTP 서버 시작
	a.logger.Info("Starting server", zap.Int("port", port))
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: a.router,
	}

	// 5. 서버 종료 처리
	go func() {
		<-a.stopCh
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			a.logger.Error("Server shutdown error", zap.Error(err))
		}
	}()

	// 6. 서버 시작
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

// Stop은 애플리케이션을 종료합니다.
func (a *App) Stop() {
	close(a.stopCh)
	a.wg.Wait()
	a.storage.Close()
	a.logger.Info("Application stopped")
}

// setupRoutes는 API 라우트를 설정합니다.
func (a *App) setupRoutes() {
	// 정적 파일 제공
	a.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./client/static"))))
	a.router.HandleFunc("/", a.handleIndex)

	// API 라우트
	api := a.router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/games", a.handleGetGames).Methods("GET")
	api.HandleFunc("/games", a.handleCreateGame).Methods("POST")
	api.HandleFunc("/games/{id}", a.handleGetGame).Methods("GET")
	api.HandleFunc("/games/{id}", a.handleUpdateGame).Methods("PUT")
	api.HandleFunc("/games/{id}", a.handleDeleteGame).Methods("DELETE")

	// WebSocket 라우트
	a.router.Handle("/sync", eventsync.NewWebSocketHandler(a.syncService, a.logger))

	// SSE 라우트
	a.router.Handle("/events", eventsync.NewSSEHandler(a.syncService, a.logger))

	// 동기화 API
	api.HandleFunc("/sync/{id}", a.handleSync).Methods("POST")
}

// handleIndex는 인덱스 페이지를 처리합니다.
func (a *App) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./client/eventsync-example.html")
}

// handleGetGames는 모든 게임을 조회합니다.
func (a *App) handleGetGames(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 모든 게임 조회
	cursor, err := a.storage.Find(ctx, bson.M{})
	if err != nil {
		a.logger.Error("Failed to find games", zap.Error(err))
		http.Error(w, "Failed to find games", http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var games []*GameState
	if err := cursor.All(ctx, &games); err != nil {
		a.logger.Error("Failed to decode games", zap.Error(err))
		http.Error(w, "Failed to decode games", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(games)
}

// handleCreateGame은 새 게임을 생성합니다.
func (a *App) handleCreateGame(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var game GameState
	if err := json.NewDecoder(r.Body).Decode(&game); err != nil {
		a.logger.Error("Failed to decode request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// ID 및 버전 설정
	game.ID = primitive.NewObjectID()
	game.LastUpdated = time.Now()
	game.Version = 1

	// 게임 생성
	createdGame, err := a.storage.CreateAndGet(ctx, &game)
	if err != nil {
		a.logger.Error("Failed to create game", zap.Error(err))
		http.Error(w, "Failed to create game", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(createdGame)
}

// handleGetGame은 특정 게임을 조회합니다.
func (a *App) handleGetGame(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	idStr := vars["id"]

	// ID 파싱
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		a.logger.Error("Invalid ID", zap.String("id", idStr), zap.Error(err))
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// 게임 조회
	game, err := a.storage.FindOne(ctx, id)
	if err != nil {
		a.logger.Error("Failed to find game", zap.String("id", idStr), zap.Error(err))
		http.Error(w, "Game not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(game)
}

// handleUpdateGame은 게임을 업데이트합니다.
func (a *App) handleUpdateGame(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	idStr := vars["id"]

	// ID 파싱
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		a.logger.Error("Invalid ID", zap.String("id", idStr), zap.Error(err))
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var updateData GameState
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		a.logger.Error("Failed to decode request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 게임 업데이트
	updatedGame, diff, err := a.storage.FindOneAndUpdate(ctx, id, func(game *GameState) (*GameState, error) {
		// 필드 업데이트
		if updateData.Name != "" {
			game.Name = updateData.Name
		}
		if updateData.Gold > 0 {
			game.Gold = updateData.Gold
		}
		if updateData.Level > 0 {
			game.Level = updateData.Level
		}
		if updateData.Experience > 0 {
			game.Experience = updateData.Experience
		}
		game.LastUpdated = time.Now()
		game.Version++
		return game, nil
	})

	if err != nil {
		a.logger.Error("Failed to update game", zap.String("id", idStr), zap.Error(err))
		http.Error(w, "Failed to update game", http.StatusInternalServerError)
		return
	}

	// 응답에 diff 포함
	response := map[string]interface{}{
		"game": updatedGame,
		"diff": diff,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDeleteGame은 게임을 삭제합니다.
func (a *App) handleDeleteGame(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	idStr := vars["id"]

	// ID 파싱
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		a.logger.Error("Invalid ID", zap.String("id", idStr), zap.Error(err))
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// 게임 삭제
	if err := a.storage.DeleteOne(ctx, id); err != nil {
		a.logger.Error("Failed to delete game", zap.String("id", idStr), zap.Error(err))
		http.Error(w, "Failed to delete game", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleSync는 클라이언트 동기화 요청을 처리합니다.
func (a *App) handleSync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	idStr := vars["id"]

	// ID 파싱
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		a.logger.Error("Invalid ID", zap.String("id", idStr), zap.Error(err))
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// 요청 본문 파싱
	var syncRequest struct {
		ClientID    string            `json:"clientId"`
		VectorClock map[string]int64  `json:"vectorClock"`
	}

	if err := json.NewDecoder(r.Body).Decode(&syncRequest); err != nil {
		a.logger.Error("Failed to decode request", zap.Error(err))
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 클라이언트 동기화
	if err := a.syncService.SyncClient(ctx, syncRequest.ClientID, id, syncRequest.VectorClock); err != nil {
		a.logger.Error("Failed to sync client", zap.String("clientId", syncRequest.ClientID), zap.Error(err))
		http.Error(w, "Failed to sync client", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// createSnapshots는 모든 문서에 대해 스냅샷을 생성합니다.
func (a *App) createSnapshots() {
	ctx := context.Background()
	a.logger.Info("Creating snapshots")

	// 모든 문서 ID 조회
	cursor, err := a.storage.Find(ctx, bson.M{})
	if err != nil {
		a.logger.Error("Failed to find documents", zap.Error(err))
		return
	}
	defer cursor.Close(ctx)

	var games []*GameState
	if err := cursor.All(ctx, &games); err != nil {
		a.logger.Error("Failed to decode documents", zap.Error(err))
		return
	}

	for _, game := range games {
		// 스냅샷 생성
		state := map[string]interface{}{
			"name":        game.Name,
			"gold":        game.Gold,
			"level":       game.Level,
			"experience":  game.Experience,
			"last_updated": game.LastUpdated,
			"version":     game.Version,
		}

		_, err := a.snapshotStore.CreateSnapshot(ctx, game.ID, state, game.Version)
		if err != nil {
			a.logger.Error("Failed to create snapshot", zap.String("id", game.ID.Hex()), zap.Error(err))
			continue
		}

		a.logger.Info("Snapshot created", zap.String("id", game.ID.Hex()), zap.Int64("version", game.Version))
	}
}

func main() {
	// 명령행 인자 파싱
	port := flag.Int("port", 8080, "HTTP 서버 포트")
	mongoURI := flag.String("mongo", "mongodb://localhost:27017", "MongoDB 연결 URI")
	dbName := flag.String("db", "eventsync_example", "데이터베이스 이름")
	flag.Parse()

	// 로거 설정
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("로거 생성 실패: %v", err)
	}
	defer logger.Sync()

	// MongoDB 연결
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(*mongoURI))
	if err != nil {
		logger.Fatal("MongoDB 연결 실패", zap.Error(err))
	}
	defer client.Disconnect(context.Background())

	// 애플리케이션 생성
	app, err := NewApp(ctx, client, *dbName, logger)
	if err != nil {
		logger.Fatal("애플리케이션 생성 실패", zap.Error(err))
	}

	// 종료 신호 처리
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// 애플리케이션 시작
	go func() {
		if err := app.Start(*port); err != nil {
			logger.Fatal("애플리케이션 시작 실패", zap.Error(err))
		}
	}()

	// 종료 신호 대기
	<-stop
	logger.Info("종료 신호 수신")
	app.Stop()
}
