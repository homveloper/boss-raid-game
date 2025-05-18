package main

// import (
// 	"context"
// 	"log"
// 	"net/http"
// 	"os"
// 	"os/signal"
// 	"syscall"
// 	"time"

// 	"github.com/go-redis/redis/v8"
// 	"github.com/gorilla/mux"
// 	"go.mongodb.org/mongo-driver/mongo"
// 	"go.mongodb.org/mongo-driver/mongo/options"

// 	"tictactoe/transport/cmd/cqrs_demo/api"
// 	"tictactoe/transport/cmd/cqrs_demo/business"
// 	"tictactoe/transport/cqrs/application"
// 	"tictactoe/transport/cqrs/infrastructure"
// )

// func main() {
// 	// 로그 설정
// 	log.SetFlags(log.LstdFlags | log.Lshortfile)
// 	log.Println("Starting CQRS Demo Application with JSON-RPC...")

// 	// MongoDB 연결
// 	mongoClient, err := connectMongoDB()
// 	if err != nil {
// 		log.Fatalf("Failed to connect to MongoDB: %v", err)
// 	}
// 	defer func() {
// 		if err := mongoClient.Disconnect(context.Background()); err != nil {
// 			log.Printf("Failed to disconnect from MongoDB: %v", err)
// 		}
// 	}()

// 	// Redis 연결
// 	redisClient := redis.NewClient(&redis.Options{
// 		Addr:     "localhost:6379",
// 		Password: "", // no password set
// 		DB:       0,  // use default DB
// 	})
// 	defer redisClient.Close()

// 	// 인프라 컴포넌트 초기화
// 	eventStore := infrastructure.NewMongoEventStore(
// 		mongoClient,
// 		"transport_cqrs",
// 		"events",
// 	)

// 	eventBus := infrastructure.NewRedisEventBus(
// 		redisClient,
// 		"transport_events",
// 		"transport_consumer",
// 		"transport_group",
// 	)

// 	repository := infrastructure.NewMongoRepository(
// 		mongoClient,
// 		"transport_cqrs",
// 		eventStore,
// 		eventBus,
// 	)

// 	commandBus := infrastructure.NewInMemoryCommandBus()

// 	// 애플리케이션 서비스 초기화
// 	transportProjector := application.NewTransportProjector(
// 		mongoClient,
// 		"transport_cqrs",
// 		"transport_read_models",
// 		"processed_events",
// 	)

// 	// 이벤트 핸들러 등록
// 	eventBus.SubscribeAll(transportProjector)

// 	// 커맨드 핸들러 초기화 및 등록
// 	transportCommandHandler := application.NewTransportCommandHandler(repository)
// 	raidCommandHandler := application.NewRaidCommandHandler(repository)

// 	// 커맨드 핸들러 등록
// 	commandBus.RegisterHandler("CreateTransport", transportCommandHandler)
// 	commandBus.RegisterHandler("StartTransport", transportCommandHandler)
// 	commandBus.RegisterHandler("CompleteTransport", transportCommandHandler)
// 	commandBus.RegisterHandler("RaidTransport", transportCommandHandler)
// 	commandBus.RegisterHandler("DefendTransport", transportCommandHandler)
// 	commandBus.RegisterHandler("AddParticipant", transportCommandHandler)

// 	commandBus.RegisterHandler("CreateRaid", raidCommandHandler)
// 	commandBus.RegisterHandler("StartRaid", raidCommandHandler)
// 	commandBus.RegisterHandler("RaidSucceed", raidCommandHandler)
// 	commandBus.RegisterHandler("RaidFail", raidCommandHandler)
// 	commandBus.RegisterHandler("CancelRaid", raidCommandHandler)

// 	// 비즈니스 서비스 초기화
// 	guildService := business.NewGuildService(
// 		mongoClient.Database("transport_cqrs").Collection("guilds"),
// 	)

// 	ticketService := business.NewTicketService(
// 		mongoClient.Database("transport_cqrs").Collection("tickets"),
// 	)

// 	transportService := business.NewTransportService(
// 		commandBus,
// 		mongoClient.Database("transport_cqrs").Collection("transport_read_models"),
// 		guildService,
// 		ticketService,
// 	)

// 	// API 핸들러 초기화
// 	rpcHandler := api.NewBatchRPCHandler(transportService)

// 	// HTTP 라우터 설정
// 	router := mux.NewRouter()
// 	router.HandleFunc("/rpc", rpcHandler.HandleRPC).Methods("POST")

// 	// 이벤트 버스 시작
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	if err := eventBus.Start(ctx); err != nil {
// 		log.Fatalf("Failed to start event bus: %v", err)
// 	}

// 	// 스케줄러 초기화
// 	scheduler := NewScheduler(
// 		commandBus,
// 		mongoClient.Database("transport_cqrs").Collection("transport_read_models"),
// 		1*time.Minute, // 1분마다 체크
// 	)

// 	// 스케줄러 시작
// 	scheduler.Start()

// 	// HTTP 서버 시작
// 	server := &http.Server{
// 		Addr:    ":8080",
// 		Handler: router,
// 	}

// 	// 서버를 고루틴으로 시작
// 	go func() {
// 		log.Println("HTTP server listening on :8080")
// 		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
// 			log.Fatalf("HTTP server error: %v", err)
// 		}
// 	}()

// 	// 종료 시그널 처리
// 	sigChan := make(chan os.Signal, 1)
// 	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
// 	<-sigChan

// 	log.Println("Shutting down...")

// 	// 스케줄러 종료
// 	scheduler.Stop()

// 	// HTTP 서버 종료
// 	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer shutdownCancel()
// 	if err := server.Shutdown(shutdownCtx); err != nil {
// 		log.Printf("HTTP server shutdown error: %v", err)
// 	}

// 	// 이벤트 버스 종료
// 	if err := eventBus.Stop(); err != nil {
// 		log.Printf("Event bus stop error: %v", err)
// 	}

// 	log.Println("Server stopped")
// }

// // connectMongoDB connects to MongoDB
// func connectMongoDB() (*mongo.Client, error) {
// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Ping the database to verify connection
// 	if err := client.Ping(ctx, nil); err != nil {
// 		return nil, err
// 	}

// 	log.Println("Connected to MongoDB")
// 	return client, nil
// }
