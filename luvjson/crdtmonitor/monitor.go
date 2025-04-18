package crdtmonitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtpubsub"

	"github.com/google/uuid"
)

// MonitorEventType represents the type of monitor event.
type MonitorEventType string

const (
	// EventTypePatchReceived is triggered when a patch is received.
	EventTypePatchReceived MonitorEventType = "patch_received"
	// EventTypePatchApplied is triggered when a patch is applied.
	EventTypePatchApplied MonitorEventType = "patch_applied"
	// EventTypePatchRejected is triggered when a patch is rejected.
	EventTypePatchRejected MonitorEventType = "patch_rejected"
	// EventTypeConflictDetected is triggered when a conflict is detected.
	EventTypeConflictDetected MonitorEventType = "conflict_detected"
	// EventTypeConflictResolved is triggered when a conflict is resolved.
	EventTypeConflictResolved MonitorEventType = "conflict_resolved"
	// EventTypeDocumentChanged is triggered when the document is changed.
	EventTypeDocumentChanged MonitorEventType = "document_changed"
	// EventTypeError is triggered when an error occurs.
	EventTypeError MonitorEventType = "error"
)

// MonitorEvent represents an event in the CRDT monitor.
type MonitorEvent struct {
	// Type is the type of the event.
	Type MonitorEventType
	// Timestamp is when the event occurred.
	Timestamp time.Time
	// DocumentID is the ID of the document associated with the event.
	DocumentID string
	// SessionID is the ID of the session associated with the event.
	SessionID common.SessionID
	// PatchID is the ID of the patch associated with the event, if any.
	PatchID *common.LogicalTimestamp
	// Patch is the patch associated with the event, if any.
	Patch *crdtpatch.Patch
	// Error is the error associated with the event, if any.
	Error error
	// Metadata contains additional information about the event.
	Metadata map[string]any
}

// EventHandler is a function that handles monitor events.
type EventHandler func(event MonitorEvent)

// MonitorOptions represents configuration options for the CRDT monitor.
type MonitorOptions struct {
	// DocumentID is the ID of the document to monitor.
	DocumentID string
	// PubSubTopic is the topic to subscribe to for patches.
	PubSubTopic string
	// PatchFormat is the format of the patches.
	PatchFormat crdtpubsub.EncodingFormat
	// CollectStats indicates whether to collect statistics.
	CollectStats bool
	// StatInterval is the interval at which to collect statistics.
	StatInterval time.Duration
	// LogEvents indicates whether to log events.
	LogEvents bool
	// EventHandlers is a map of event types to handlers.
	EventHandlers map[MonitorEventType][]EventHandler
}

// NewMonitorOptions creates a new MonitorOptions with default values.
func NewMonitorOptions() *MonitorOptions {
	return &MonitorOptions{
		DocumentID:    "default",
		PubSubTopic:   "crdt-patches",
		PatchFormat:   crdtpubsub.EncodingFormatJSON,
		CollectStats:  true,
		StatInterval:  5 * time.Second,
		LogEvents:     true,
		EventHandlers: make(map[MonitorEventType][]EventHandler),
	}
}

// MonitorStats represents statistics collected by the monitor.
type MonitorStats struct {
	// StartTime is when the monitor started.
	StartTime time.Time
	// LastUpdateTime is when the stats were last updated.
	LastUpdateTime time.Time
	// TotalPatchesReceived is the total number of patches received.
	TotalPatchesReceived int64
	// TotalPatchesApplied is the total number of patches applied.
	TotalPatchesApplied int64
	// TotalPatchesRejected is the total number of patches rejected.
	TotalPatchesRejected int64
	// TotalConflictsDetected is the total number of conflicts detected.
	TotalConflictsDetected int64
	// TotalConflictsResolved is the total number of conflicts resolved.
	TotalConflictsResolved int64
	// TotalErrors is the total number of errors.
	TotalErrors int64
	// AveragePatchSize is the average size of patches in bytes.
	AveragePatchSize float64
	// MaxPatchSize is the maximum size of a patch in bytes.
	MaxPatchSize int64
	// MinPatchSize is the minimum size of a patch in bytes.
	MinPatchSize int64
	// PatchesPerSecond is the number of patches received per second.
	PatchesPerSecond float64
	// OperationsPerSecond is the number of operations processed per second.
	OperationsPerSecond float64
	// SessionStats is a map of session IDs to session statistics.
	SessionStats map[common.SessionID]*SessionStats
	// Custom contains custom statistics.
	Custom map[string]any
}

// SessionStats represents statistics for a session.
type SessionStats struct {
	// SessionID is the ID of the session.
	SessionID common.SessionID
	// LastActiveTime is when the session was last active.
	LastActiveTime time.Time
	// TotalPatchesSent is the total number of patches sent by the session.
	TotalPatchesSent int64
	// TotalOperationsSent is the total number of operations sent by the session.
	TotalOperationsSent int64
	// Custom contains custom statistics for the session.
	Custom map[string]any
}

// Monitor is the interface for a CRDT monitor.
type Monitor interface {
	// Start starts the monitor.
	Start(ctx context.Context) error
	// Stop stops the monitor.
	Stop() error
	// AddEventHandler adds an event handler for the specified event type.
	AddEventHandler(eventType MonitorEventType, handler EventHandler)
	// RemoveEventHandler removes an event handler for the specified event type.
	RemoveEventHandler(eventType MonitorEventType, handler EventHandler)
	// GetStats returns the current statistics.
	GetStats() *MonitorStats
	// ResetStats resets the statistics.
	ResetStats()
	// IsRunning returns whether the monitor is running.
	IsRunning() bool
}

// CRDTMonitor implements the Monitor interface.
type CRDTMonitor struct {
	// options contains the configuration options.
	options *MonitorOptions
	// pubsub is the PubSub client.
	pubsub crdtpubsub.PubSub
	// document is the CRDT document being monitored.
	document *crdt.Document
	// stats contains the statistics.
	stats *MonitorStats
	// statsMutex protects the stats.
	statsMutex sync.RWMutex
	// running indicates whether the monitor is running.
	running bool
	// runningMutex protects the running flag.
	runningMutex sync.RWMutex
	// cancel is the cancel function for the context.
	cancel context.CancelFunc
	// subscriberID is the ID used for subscribing to the PubSub topic.
	subscriberID string
}

// NewCRDTMonitor creates a new CRDTMonitor with the specified options.
func NewCRDTMonitor(pubsub crdtpubsub.PubSub, document *crdt.Document, options *MonitorOptions) (*CRDTMonitor, error) {
	if pubsub == nil {
		return nil, fmt.Errorf("pubsub cannot be nil")
	}
	if document == nil {
		return nil, fmt.Errorf("document cannot be nil")
	}
	if options == nil {
		options = NewMonitorOptions()
	}

	return &CRDTMonitor{
		options:      options,
		pubsub:       pubsub,
		document:     document,
		stats:        newMonitorStats(),
		statsMutex:   sync.RWMutex{},
		running:      false,
		runningMutex: sync.RWMutex{},
	}, nil
}

// newMonitorStats creates a new MonitorStats.
func newMonitorStats() *MonitorStats {
	now := time.Now()
	return &MonitorStats{
		StartTime:      now,
		LastUpdateTime: now,
		MinPatchSize:   -1, // -1 indicates no patches received yet
		SessionStats:   make(map[common.SessionID]*SessionStats),
		Custom:         make(map[string]any),
	}
}

// Start starts the monitor.
func (m *CRDTMonitor) Start(ctx context.Context) error {
	m.runningMutex.Lock()
	defer m.runningMutex.Unlock()

	if m.running {
		return fmt.Errorf("monitor is already running")
	}

	// Create a new context with cancel
	monitorCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	// Subscribe to the PubSub topic
	subscriberID := fmt.Sprintf("monitor-%s", uuid.New().String())
	if err := m.pubsub.Subscribe(monitorCtx, m.options.PubSubTopic, subscriberID, func(ctx context.Context, topic string, data []byte, format crdtpubsub.EncodingFormat) error {
		msg := crdtpubsub.PatchMessage{
			Topic:   topic,
			Payload: data,
			Format:  format,
			Metadata: map[string]string{
				"format": string(format),
			},
		}
		return m.handlePatchMessage(msg)
	}); err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	// Store the subscriber ID for unsubscribing later
	m.subscriberID = subscriberID

	// Start collecting stats if enabled
	if m.options.CollectStats {
		go m.collectStats(monitorCtx)
	}

	m.running = true
	m.emitEvent(EventTypeDocumentChanged, nil, nil, map[string]any{
		"message": "Monitor started",
	})

	return nil
}

// Stop stops the monitor.
func (m *CRDTMonitor) Stop() error {
	m.runningMutex.Lock()
	defer m.runningMutex.Unlock()

	if !m.running {
		return fmt.Errorf("monitor is not running")
	}

	// Cancel the context
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}

	// Unsubscribe from the PubSub topic
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.pubsub.Unsubscribe(ctx, m.options.PubSubTopic, m.subscriberID); err != nil {
		return fmt.Errorf("failed to unsubscribe from topic: %w", err)
	}

	m.running = false
	m.emitEvent(EventTypeDocumentChanged, nil, nil, map[string]any{
		"message": "Monitor stopped",
	})

	return nil
}

// AddEventHandler adds an event handler for the specified event type.
func (m *CRDTMonitor) AddEventHandler(eventType MonitorEventType, handler EventHandler) {
	if handler == nil {
		return
	}

	m.options.EventHandlers[eventType] = append(m.options.EventHandlers[eventType], handler)
}

// RemoveEventHandler removes an event handler for the specified event type.
func (m *CRDTMonitor) RemoveEventHandler(eventType MonitorEventType, handler EventHandler) {
	if handler == nil {
		return
	}

	handlers := m.options.EventHandlers[eventType]
	for i, h := range handlers {
		if fmt.Sprintf("%p", h) == fmt.Sprintf("%p", handler) {
			// Remove the handler
			newHandlers := make([]EventHandler, 0, len(handlers)-1)
			newHandlers = append(newHandlers, handlers[:i]...)
			newHandlers = append(newHandlers, handlers[i+1:]...)
			m.options.EventHandlers[eventType] = newHandlers
			break
		}
	}
}

// GetStats returns the current statistics.
func (m *CRDTMonitor) GetStats() *MonitorStats {
	m.statsMutex.RLock()
	defer m.statsMutex.RUnlock()

	// Return a copy of the stats
	statsCopy := *m.stats
	return &statsCopy
}

// ResetStats resets the statistics.
func (m *CRDTMonitor) ResetStats() {
	m.statsMutex.Lock()
	defer m.statsMutex.Unlock()

	m.stats = newMonitorStats()
}

// IsRunning returns whether the monitor is running.
func (m *CRDTMonitor) IsRunning() bool {
	m.runningMutex.RLock()
	defer m.runningMutex.RUnlock()

	return m.running
}

// handlePatchMessage handles a patch message from the PubSub.
func (m *CRDTMonitor) handlePatchMessage(msg crdtpubsub.PatchMessage) error {
	// Decode the patch
	decoder, err := crdtpubsub.GetEncoderDecoder(msg.Format)
	if err != nil {
		m.emitEvent(EventTypeError, nil, nil, map[string]any{
			"error":   err.Error(),
			"message": "Failed to get decoder",
		})
		return fmt.Errorf("failed to get decoder: %w", err)
	}

	patch, err := decoder.Decode(msg.Payload)
	if err != nil {
		m.emitEvent(EventTypeError, nil, nil, map[string]any{
			"error":   err.Error(),
			"message": "Failed to decode patch",
		})
		return fmt.Errorf("failed to decode patch: %w", err)
	}

	// Update stats
	m.updateStatsForPatch(patch, len(msg.Payload))

	// Emit patch received event
	patchID := patch.ID()
	m.emitEvent(EventTypePatchReceived, &patchID, patch, nil)

	// Apply the patch to the document
	if err := patch.Apply(m.document); err != nil {
		patchID := patch.ID()
		m.emitEvent(EventTypePatchRejected, &patchID, patch, map[string]any{
			"error":   err.Error(),
			"message": "Failed to apply patch",
		})
		return fmt.Errorf("failed to apply patch: %w", err)
	}

	// Emit patch applied event
	appliedPatchID := patch.ID()
	m.emitEvent(EventTypePatchApplied, &appliedPatchID, patch, nil)

	return nil
}

// updateStatsForPatch updates the statistics for a received patch.
func (m *CRDTMonitor) updateStatsForPatch(patch *crdtpatch.Patch, patchSize int) {
	m.statsMutex.Lock()
	defer m.statsMutex.Unlock()

	// Update patch stats
	m.stats.TotalPatchesReceived++
	m.stats.TotalPatchesApplied++

	// Update patch size stats
	if m.stats.MinPatchSize == -1 || int64(patchSize) < m.stats.MinPatchSize {
		m.stats.MinPatchSize = int64(patchSize)
	}
	if int64(patchSize) > m.stats.MaxPatchSize {
		m.stats.MaxPatchSize = int64(patchSize)
	}

	// Update average patch size
	m.stats.AveragePatchSize = (m.stats.AveragePatchSize*float64(m.stats.TotalPatchesReceived-1) + float64(patchSize)) / float64(m.stats.TotalPatchesReceived)

	// Update session stats
	sessionID := patch.ID().SID
	sessionStats, ok := m.stats.SessionStats[sessionID]
	if !ok {
		sessionStats = &SessionStats{
			SessionID: sessionID,
			Custom:    make(map[string]any),
		}
		m.stats.SessionStats[sessionID] = sessionStats
	}
	sessionStats.LastActiveTime = time.Now()
	sessionStats.TotalPatchesSent++
	sessionStats.TotalOperationsSent += int64(len(patch.Operations()))

	// Update last update time
	m.stats.LastUpdateTime = time.Now()
}

// collectStats collects statistics periodically.
func (m *CRDTMonitor) collectStats(ctx context.Context) {
	ticker := time.NewTicker(m.options.StatInterval)
	defer ticker.Stop()

	var lastPatchCount int64
	var lastTime time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-ticker.C:
			m.statsMutex.Lock()

			// Calculate patches per second
			if !lastTime.IsZero() {
				elapsed := t.Sub(lastTime).Seconds()
				if elapsed > 0 {
					patchesDelta := m.stats.TotalPatchesReceived - lastPatchCount
					m.stats.PatchesPerSecond = float64(patchesDelta) / elapsed
				}
			}

			// Update last values
			lastPatchCount = m.stats.TotalPatchesReceived
			lastTime = t

			// Update last update time
			m.stats.LastUpdateTime = t

			m.statsMutex.Unlock()
		}
	}
}

// emitEvent emits an event to all registered handlers.
func (m *CRDTMonitor) emitEvent(eventType MonitorEventType, patchID *common.LogicalTimestamp, patch *crdtpatch.Patch, metadata map[string]any) {
	// Create the event
	event := MonitorEvent{
		Type:       eventType,
		Timestamp:  time.Now(),
		DocumentID: m.options.DocumentID,
		SessionID:  m.document.GetSessionID(),
		PatchID:    patchID,
		Patch:      patch,
		Metadata:   metadata,
	}

	// Log the event if enabled
	if m.options.LogEvents {
		var patchIDStr string
		if patchID != nil {
			patchIDStr = fmt.Sprintf("%v", *patchID)
		} else {
			patchIDStr = "nil"
		}
		fmt.Printf("[%s] %s: DocumentID=%s, SessionID=%v, PatchID=%s\n",
			event.Timestamp.Format(time.RFC3339),
			event.Type,
			event.DocumentID,
			event.SessionID,
			patchIDStr)
	}

	// Call the handlers
	for _, handler := range m.options.EventHandlers[eventType] {
		go handler(event)
	}
}
