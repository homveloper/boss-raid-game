package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	httpDelivery "tictactoe/internal/delivery/http"
	"tictactoe/internal/delivery/sse"
	"tictactoe/internal/repository/memory"
	"tictactoe/internal/usecase"
	"time"
)

func main() {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	// Create repositories
	gameRepo := memory.NewGameRepository()
	roomRepo := memory.NewRoomRepository()

	// Create use cases
	gameUC := usecase.NewGameUseCase(gameRepo)
	roomUC := usecase.NewRoomUseCase(roomRepo, gameUC)

	// Create SSE router
	sseRouter := sse.NewRouter(gameUC)

	// Create HTTP handler
	handler := httpDelivery.NewHandler(roomUC, gameUC, sseRouter)

	// Create HTTP router
	router := httpDelivery.NewRouter(handler, sseRouter)

	// Set up HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: router.Setup(),
	}

	// Start the server
	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(server.ListenAndServe())
}
