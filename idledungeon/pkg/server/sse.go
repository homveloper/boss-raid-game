package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"eventsync"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// SSEHandler handles Server-Sent Events
type SSEHandler struct {
	syncService eventsync.SyncService
	clients     map[string]*SSEClient
	clientsMu   sync.RWMutex
	logger      *zap.Logger
}

// SSEClient represents a connected SSE client
type SSEClient struct {
	ID           string
	DocumentID   primitive.ObjectID
	VectorClock  map[string]int64
	ResponseChan chan []byte
	Done         chan struct{}
}

// NewSSEHandler creates a new SSE handler
func NewSSEHandler(syncService eventsync.SyncService, logger *zap.Logger) *SSEHandler {
	return &SSEHandler{
		syncService: syncService,
		clients:     make(map[string]*SSEClient),
		logger:      logger,
	}
}

// ServeHTTP implements the http.Handler interface
func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only allow GET method
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get client ID and document ID from query parameters
	clientID := r.URL.Query().Get("clientId")
	documentIDStr := r.URL.Query().Get("documentId")

	// Validate parameters
	if clientID == "" {
		http.Error(w, "Client ID is required", http.StatusBadRequest)
		return
	}
	if documentIDStr == "" {
		http.Error(w, "Document ID is required", http.StatusBadRequest)
		return
	}

	// Parse document ID
	documentID, err := primitive.ObjectIDFromHex(documentIDStr)
	if err != nil {
		http.Error(w, "Invalid document ID", http.StatusBadRequest)
		return
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create channels for communication
	responseChan := make(chan []byte, 10)
	done := make(chan struct{})

	// Create client
	client := &SSEClient{
		ID:           clientID,
		DocumentID:   documentID,
		VectorClock:  make(map[string]int64),
		ResponseChan: responseChan,
		Done:         done,
	}

	// Register client
	h.registerClient(client)
	defer h.unregisterClient(client)

	// Register client with sync service
	ctx := r.Context()
	// Store response writer in context
	ctx = context.WithValue(ctx, "responseWriter", w)

	if err := h.syncService.RegisterClient(ctx, clientID); err != nil {
		h.logger.Error("Failed to register client with sync service", zap.String("client_id", clientID), zap.Error(err))
		http.Error(w, "Failed to register client", http.StatusInternalServerError)
		return
	}

	// Create flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Notify client of successful connection
	fmt.Fprintf(w, "event: connected\ndata: {\"clientId\":\"%s\"}\n\n", clientID)
	flusher.Flush()

	// Start goroutine to send events to client
	go h.sendEvents(ctx, client)

	// Wait for client to disconnect
	select {
	case <-ctx.Done():
		// Client disconnected
		close(done)
	case <-r.Context().Done():
		// Request context done
		close(done)
	}
}

// registerClient registers a new SSE client
func (h *SSEHandler) registerClient(client *SSEClient) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	h.clients[client.ID] = client
	h.logger.Debug("Client registered", zap.String("client_id", client.ID))
}

// unregisterClient unregisters an SSE client
func (h *SSEHandler) unregisterClient(client *SSEClient) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	delete(h.clients, client.ID)
	close(client.ResponseChan)
	h.logger.Debug("Client unregistered", zap.String("client_id", client.ID))
}

// sendEvents sends events to a client
func (h *SSEHandler) sendEvents(ctx context.Context, client *SSEClient) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Get the response writer and flusher from context
	w := ctx.Value("responseWriter").(http.ResponseWriter)
	flusher := w.(http.Flusher)

	for {
		select {
		case <-client.Done:
			return
		case <-ticker.C:
			// Get missing events for client
			events, err := h.syncService.GetMissingEvents(ctx, client.ID, client.DocumentID, client.VectorClock)
			if err != nil {
				h.logger.Error("Failed to get missing events", zap.String("client_id", client.ID), zap.Error(err))
				continue
			}

			// Send events to client
			for _, event := range events {
				// Update client vector clock
				client.VectorClock[event.ClientID] = event.SequenceNum

				// Send event to client
				eventData, err := json.Marshal(event)
				if err != nil {
					h.logger.Error("Failed to marshal event", zap.Error(err))
					continue
				}

				// Write event to response
				fmt.Fprintf(w, "event: update\ndata: %s\n\n", eventData)
				flusher.Flush()
			}

			// Update client vector clock in sync service
			if err := h.syncService.UpdateVectorClock(ctx, client.ID, client.DocumentID, client.VectorClock); err != nil {
				h.logger.Error("Failed to update vector clock", zap.String("client_id", client.ID), zap.Error(err))
			}
		case eventData := <-client.ResponseChan:
			// Send event to client
			fmt.Fprintf(w, "event: update\ndata: %s\n\n", eventData)
			flusher.Flush()
		}
	}
}

// BroadcastEvent broadcasts an event to all clients
func (h *SSEHandler) BroadcastEvent(event *eventsync.Event) {
	// Marshal event
	eventData, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("Failed to marshal event", zap.Error(err))
		return
	}

	// Send event to all clients
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	for _, client := range h.clients {
		if client.DocumentID == event.DocumentID {
			select {
			case client.ResponseChan <- eventData:
				// Event sent to client
			default:
				// Client buffer full, skip
			}
		}
	}
}
