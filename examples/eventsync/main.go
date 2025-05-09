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
	"syscall"
	"time"

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
	Players     map[string]Player  `bson:"players" json:"players"`
	Resources   Resources          `bson:"resources" json:"resources"`
	LastUpdated time.Time          `bson:"last_updated" json:"lastUpdated"`
	Version     int64              `bson:"version" json:"version"`
}

// Player는 플레이어 정보를 나타내는 구조체입니다.
type Player struct {
	ID       string    `bson:"id" json:"id"`
	Name     string    `bson:"name" json:"name"`
	Level    int       `bson:"level" json:"level"`
	JoinedAt time.Time `bson:"joined_at" json:"joinedAt"`
}

// Resources는 게임 리소스를 나타내는 구조체입니다.
type Resources struct {
	Gold   int `bson:"gold" json:"gold"`
	Wood   int `bson:"wood" json:"wood"`
	Stone  int `bson:"stone" json:"stone"`
	Food   int `bson:"food" json:"food"`
	Energy int `bson:"energy" json:"energy"`
}

func main() {
	// 명령행 인자 파싱
	port := flag.Int("port", 8080, "HTTP 서버 포트")
	mongoURI := flag.String("mongo", "mongodb://localhost:27017", "MongoDB 연결 URI")
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

	// 컬렉션 설정
	gameCollection := client.Database("eventsync_example").Collection("games")
	eventCollection := client.Database("eventsync_example").Collection("events")
	stateVectorCollection := client.Database("eventsync_example").Collection("state_vectors")

	// 캐시 생성
	memCache := cache.NewMemoryCache[*GameState](nil)
	defer memCache.Close()

	// Storage 생성
	storageOptions := &nodestorage.Options{
		VersionField:      "version",
		CacheTTL:          time.Minute * 10,
		WatchEnabled:      true,
		WatchFullDocument: "updateLookup",
	}

	storage, err := nodestorage.NewStorage[*GameState](ctx, gameCollection, memCache, storageOptions)
	if err != nil {
		logger.Fatal("Storage 생성 실패", zap.Error(err))
	}
	defer storage.Close()

	// 이벤트 저장소 생성
	eventStore, err := eventsync.NewMongoEventStore(ctx, client, "eventsync_example", "events", logger)
	if err != nil {
		logger.Fatal("이벤트 저장소 생성 실패", zap.Error(err))
	}

	// 상태 벡터 관리자 생성
	stateVectorManager, err := eventsync.NewMongoStateVectorManager(ctx, client, "eventsync_example", "state_vectors", eventStore, logger)
	if err != nil {
		logger.Fatal("상태 벡터 관리자 생성 실패", zap.Error(err))
	}

	// 동기화 서비스 생성
	syncService := eventsync.NewSyncService(eventStore, stateVectorManager, logger)

	// 스토리지 리스너 생성 및 시작
	storageListener := eventsync.NewStorageListener[*GameState](storage, syncService, logger)
	if err := storageListener.Start(); err != nil {
		logger.Fatal("스토리지 리스너 시작 실패", zap.Error(err))
	}
	defer storageListener.Stop()

	// HTTP 핸들러 설정
	mux := http.NewServeMux()

	// 정적 파일 제공
	mux.Handle("/", http.FileServer(http.Dir("./client")))

	// WebSocket 핸들러
	wsHandler := eventsync.NewWebSocketHandler(syncService, logger)
	mux.Handle("/sync", wsHandler)

	// SSE 핸들러
	sseHandler := eventsync.NewSSEHandler(syncService, logger)
	mux.Handle("/events", sseHandler)

	// API 핸들러
	mux.HandleFunc("/api/games", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// 게임 목록 조회
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"message": "게임 목록 API"})
		case http.MethodPost:
			// 새 게임 생성
			var game GameState
			if err := json.NewDecoder(r.Body).Decode(&game); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			game.ID = primitive.NewObjectID()
			game.LastUpdated = time.Now()
			game.Version = 1

			// 게임 저장
			_, err := storage.CreateAndGet(ctx, &game)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(game)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 특정 게임 API
	mux.HandleFunc("/api/games/", func(w http.ResponseWriter, r *http.Request) {
		// 게임 ID 추출
		idStr := r.URL.Path[len("/api/games/"):]
		if idStr == "" {
			http.Error(w, "Game ID is required", http.StatusBadRequest)
			return
		}

		id, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, "Invalid game ID", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			// 게임 조회
			game, err := storage.FindOne(ctx, id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(game)
		case http.MethodPut:
			// 게임 업데이트
			var updateData GameState
			if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// 게임 업데이트
			updatedGame, diff, err := storage.FindOneAndUpdate(ctx, id, func(game *GameState) (*GameState, error) {
				// 필드 업데이트
				if updateData.Name != "" {
					game.Name = updateData.Name
				}
				if updateData.Players != nil {
					game.Players = updateData.Players
				}
				if updateData.Resources.Gold > 0 {
					game.Resources.Gold = updateData.Resources.Gold
				}
				if updateData.Resources.Wood > 0 {
					game.Resources.Wood = updateData.Resources.Wood
				}
				if updateData.Resources.Stone > 0 {
					game.Resources.Stone = updateData.Resources.Stone
				}
				if updateData.Resources.Food > 0 {
					game.Resources.Food = updateData.Resources.Food
				}
				if updateData.Resources.Energy > 0 {
					game.Resources.Energy = updateData.Resources.Energy
				}

				game.LastUpdated = time.Now()
				return game, nil
			})

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// 응답에 diff 포함
			response := map[string]interface{}{
				"game": updatedGame,
				"diff": diff,
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// 동기화 API
	mux.HandleFunc("/api/sync/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 문서 ID 추출
		idStr := r.URL.Path[len("/api/sync/"):]
		if idStr == "" {
			http.Error(w, "Document ID is required", http.StatusBadRequest)
			return
		}

		id, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			http.Error(w, "Invalid document ID", http.StatusBadRequest)
			return
		}

		// 요청 본문 파싱
		var syncRequest struct {
			ClientID    string            `json:"clientId"`
			VectorClock map[string]int64  `json:"vectorClock"`
		}

		if err := json.NewDecoder(r.Body).Decode(&syncRequest); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// 클라이언트 동기화
		if err := syncService.SyncClient(ctx, syncRequest.ClientID, id, syncRequest.VectorClock); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	// HTTP 서버 시작
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: mux,
	}

	// 서버 시작
	go func() {
		logger.Info("HTTP 서버 시작", zap.Int("port", *port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP 서버 실패", zap.Error(err))
		}
	}()

	// 종료 신호 처리
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("서버 종료 중...")
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("서버 종료 실패", zap.Error(err))
	}

	logger.Info("서버 종료 완료")
}
