package http

import (
	"net/http"
	"tictactoe/internal/delivery/sse"
)

// Router handles HTTP routing
type Router struct {
	handler   *Handler
	sseRouter *sse.Router
}

// NewRouter creates a new HTTP router
func NewRouter(handler *Handler, sseRouter *sse.Router) *Router {
	return &Router{
		handler:   handler,
		sseRouter: sseRouter,
	}
}

// Setup sets up the HTTP routes
func (r *Router) Setup() http.Handler {
	// Create two separate muxes - one for SSE and one for regular API
	apiMux := http.NewServeMux()
	sseMux := http.NewServeMux()

	// API routes
	apiMux.HandleFunc("/api/rooms", r.handler.GetRooms)
	apiMux.HandleFunc("/api/rooms/create", r.handler.CreateRoom)
	apiMux.HandleFunc("/api/rooms/get", r.handler.GetRoom)
	apiMux.HandleFunc("/api/rooms/join", r.handler.JoinRoom)
	apiMux.HandleFunc("/api/games/join", r.handler.JoinGame)
	apiMux.HandleFunc("/api/games/ready", r.handler.ReadyGame)
	apiMux.HandleFunc("/api/games/get", r.handler.GetGame)
	apiMux.HandleFunc("/api/games/attack", r.handler.AttackBoss)
	apiMux.HandleFunc("/api/games/boss-action", r.handler.TriggerBossAction)

	// Static files
	fs := http.FileServer(http.Dir("./client"))
	apiMux.Handle("/", fs)

	// SSE routes - no middleware
	sseMux.HandleFunc("/api/events", r.sseRouter.HandleEvents)

	// Apply middleware to API routes
	apiHandler := ApplyMiddleware(apiMux, RecoveryMiddleware, LoggingMiddleware)

	// Final handler that routes to either API or SSE
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/events" {
			sseMux.ServeHTTP(w, r)
		} else {
			apiHandler.ServeHTTP(w, r)
		}
	})
}
