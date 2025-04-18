package crdtmonitor

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"sync"
	"time"

	"tictactoe/luvjson/common"
)

// WebMonitor provides a web interface for the CRDT monitor.
type WebMonitor struct {
	// monitor is the underlying CRDT monitor.
	monitor Monitor
	// server is the HTTP server.
	server *http.Server
	// addr is the address to listen on.
	addr string
	// templates contains the HTML templates.
	templates *template.Template
	// events is a channel of events.
	events chan MonitorEvent
	// clients is a map of client IDs to event channels.
	clients map[string]chan MonitorEvent
	// clientsMutex protects the clients map.
	clientsMutex sync.RWMutex
	// running indicates whether the web monitor is running.
	running bool
	// runningMutex protects the running flag.
	runningMutex sync.RWMutex
}

// WebMonitorOptions represents configuration options for the web monitor.
type WebMonitorOptions struct {
	// Addr is the address to listen on.
	Addr string
	// TemplatePath is the path to the HTML templates.
	TemplatePath string
	// MaxEvents is the maximum number of events to keep in memory.
	MaxEvents int
	// EventBufferSize is the size of the event buffer.
	EventBufferSize int
}

// NewWebMonitorOptions creates a new WebMonitorOptions with default values.
func NewWebMonitorOptions() *WebMonitorOptions {
	return &WebMonitorOptions{
		Addr:            ":8080",
		TemplatePath:    "",
		MaxEvents:       1000,
		EventBufferSize: 100,
	}
}

// NewWebMonitor creates a new WebMonitor with the specified options.
func NewWebMonitor(monitor Monitor, options *WebMonitorOptions) (*WebMonitor, error) {
	if monitor == nil {
		return nil, fmt.Errorf("monitor cannot be nil")
	}
	if options == nil {
		options = NewWebMonitorOptions()
	}

	// Create the web monitor
	webMonitor := &WebMonitor{
		monitor:      monitor,
		addr:         options.Addr,
		events:       make(chan MonitorEvent, options.EventBufferSize),
		clients:      make(map[string]chan MonitorEvent),
		clientsMutex: sync.RWMutex{},
		runningMutex: sync.RWMutex{},
	}

	// Parse templates
	if options.TemplatePath != "" {
		tmpl, err := template.ParseFiles(options.TemplatePath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse templates: %w", err)
		}
		webMonitor.templates = tmpl
	} else {
		// Use embedded template
		tmpl, err := template.New("index").Parse(defaultTemplate)
		if err != nil {
			return nil, fmt.Errorf("failed to parse default template: %w", err)
		}
		webMonitor.templates = tmpl
	}

	// Register event handlers
	monitor.AddEventHandler(EventTypePatchReceived, webMonitor.handleEvent)
	monitor.AddEventHandler(EventTypePatchApplied, webMonitor.handleEvent)
	monitor.AddEventHandler(EventTypePatchRejected, webMonitor.handleEvent)
	monitor.AddEventHandler(EventTypeConflictDetected, webMonitor.handleEvent)
	monitor.AddEventHandler(EventTypeConflictResolved, webMonitor.handleEvent)
	monitor.AddEventHandler(EventTypeDocumentChanged, webMonitor.handleEvent)
	monitor.AddEventHandler(EventTypeError, webMonitor.handleEvent)

	return webMonitor, nil
}

// Start starts the web monitor.
func (wm *WebMonitor) Start(ctx context.Context) error {
	wm.runningMutex.Lock()
	defer wm.runningMutex.Unlock()

	if wm.running {
		return fmt.Errorf("web monitor is already running")
	}

	// Create a new HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", wm.handleIndex)
	mux.HandleFunc("/stats", wm.handleStats)
	mux.HandleFunc("/events", wm.handleEvents)
	mux.HandleFunc("/stream", wm.handleStream)

	wm.server = &http.Server{
		Addr:    wm.addr,
		Handler: mux,
	}

	// Start the server in a goroutine
	go func() {
		if err := wm.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Failed to start web monitor: %v\n", err)
		}
	}()

	// Start the event broadcaster
	go wm.broadcastEvents(ctx)

	wm.running = true
	fmt.Printf("Web monitor started on %s\n", wm.addr)

	return nil
}

// Stop stops the web monitor.
func (wm *WebMonitor) Stop() error {
	wm.runningMutex.Lock()
	defer wm.runningMutex.Unlock()

	if !wm.running {
		return fmt.Errorf("web monitor is not running")
	}

	// Create a context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown the server
	if err := wm.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown web monitor: %w", err)
	}

	wm.running = false
	fmt.Println("Web monitor stopped")

	return nil
}

// IsRunning returns whether the web monitor is running.
func (wm *WebMonitor) IsRunning() bool {
	wm.runningMutex.RLock()
	defer wm.runningMutex.RUnlock()

	return wm.running
}

// handleEvent handles an event from the monitor.
func (wm *WebMonitor) handleEvent(event MonitorEvent) {
	// Send the event to the events channel
	select {
	case wm.events <- event:
		// Event sent successfully
	default:
		// Channel is full, discard the event
		fmt.Println("Event channel is full, discarding event")
	}
}

// broadcastEvents broadcasts events to all connected clients.
func (wm *WebMonitor) broadcastEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-wm.events:
			// Broadcast the event to all clients
			wm.clientsMutex.RLock()
			for _, clientCh := range wm.clients {
				select {
				case clientCh <- event:
					// Event sent successfully
				default:
					// Client channel is full, discard the event for this client
				}
			}
			wm.clientsMutex.RUnlock()
		}
	}
}

// handleIndex handles requests to the index page.
func (wm *WebMonitor) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Get the current stats
	stats := wm.monitor.GetStats()

	// Render the template
	data := struct {
		Stats *MonitorStats
	}{
		Stats: stats,
	}
	if err := wm.templates.ExecuteTemplate(w, "index", data); err != nil {
		http.Error(w, fmt.Sprintf("Failed to render template: %v", err), http.StatusInternalServerError)
		return
	}
}

// handleStats handles requests for the current stats.
func (wm *WebMonitor) handleStats(w http.ResponseWriter, r *http.Request) {
	// Get the current stats
	stats := wm.monitor.GetStats()

	// Set the content type
	w.Header().Set("Content-Type", "application/json")

	// Encode the stats as JSON
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode stats: %v", err), http.StatusInternalServerError)
		return
	}
}

// handleEvents handles requests for recent events.
func (wm *WebMonitor) handleEvents(w http.ResponseWriter, r *http.Request) {
	// Set the content type
	w.Header().Set("Content-Type", "application/json")

	// Create a dummy event for now (in a real implementation, we would keep a history of events)
	events := []MonitorEvent{
		{
			Type:       EventTypeDocumentChanged,
			Timestamp:  time.Now(),
			DocumentID: "default",
			SessionID:  common.SessionID{},
			Metadata: map[string]any{
				"message": "No events available",
			},
		},
	}

	// Encode the events as JSON
	if err := json.NewEncoder(w).Encode(events); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode events: %v", err), http.StatusInternalServerError)
		return
	}
}

// handleStream handles SSE (Server-Sent Events) connections.
func (wm *WebMonitor) handleStream(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create a unique client ID
	clientID := fmt.Sprintf("%p", r)

	// Create a channel for this client
	clientCh := make(chan MonitorEvent, 10)

	// Register the client
	wm.clientsMutex.Lock()
	wm.clients[clientID] = clientCh
	wm.clientsMutex.Unlock()

	// Ensure the client is removed when the connection is closed
	defer func() {
		wm.clientsMutex.Lock()
		delete(wm.clients, clientID)
		wm.clientsMutex.Unlock()
		close(clientCh)
	}()

	// Create a context that is canceled when the client disconnects
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Create a notification channel for flushing
	notify := w.(http.Flusher)

	// Send events to the client
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-clientCh:
			if !ok {
				return
			}

			// Encode the event as JSON
			eventData, err := json.Marshal(event)
			if err != nil {
				fmt.Printf("Failed to encode event: %v\n", err)
				continue
			}

			// Write the event to the response
			fmt.Fprintf(w, "data: %s\n\n", eventData)
			notify.Flush()
		}
	}
}

// defaultTemplate is the default HTML template for the web interface.
const defaultTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>CRDT Monitor</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            margin: 0;
            padding: 20px;
            background-color: #f5f5f5;
        }
        h1, h2 {
            color: #333;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        .card {
            background-color: white;
            border-radius: 5px;
            box-shadow: 0 2px 5px rgba(0,0,0,0.1);
            padding: 20px;
            margin-bottom: 20px;
        }
        .stats {
            display: grid;
            grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
            gap: 15px;
        }
        .stat-item {
            background-color: #f9f9f9;
            border-radius: 5px;
            padding: 15px;
            text-align: center;
        }
        .stat-value {
            font-size: 24px;
            font-weight: bold;
            color: #007bff;
        }
        .stat-label {
            font-size: 14px;
            color: #666;
        }
        .events {
            height: 300px;
            overflow-y: auto;
            background-color: #f9f9f9;
            border-radius: 5px;
            padding: 10px;
        }
        .event {
            padding: 10px;
            margin-bottom: 5px;
            border-radius: 3px;
            background-color: white;
            border-left: 4px solid #007bff;
        }
        .event-time {
            font-size: 12px;
            color: #666;
        }
        .event-type {
            font-weight: bold;
            color: #007bff;
        }
        .event-details {
            margin-top: 5px;
            font-size: 14px;
        }
        .sessions {
            margin-top: 20px;
        }
        table {
            width: 100%;
            border-collapse: collapse;
        }
        th, td {
            padding: 10px;
            text-align: left;
            border-bottom: 1px solid #ddd;
        }
        th {
            background-color: #f2f2f2;
        }
        .refresh-btn {
            background-color: #007bff;
            color: white;
            border: none;
            padding: 10px 15px;
            border-radius: 5px;
            cursor: pointer;
            margin-bottom: 20px;
        }
        .refresh-btn:hover {
            background-color: #0056b3;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>CRDT Monitor</h1>

        <button id="refresh-btn" class="refresh-btn">Refresh Stats</button>

        <div class="card">
            <h2>Statistics</h2>
            <div class="stats">
                <div class="stat-item">
                    <div class="stat-value" id="patches-received">{{.Stats.TotalPatchesReceived}}</div>
                    <div class="stat-label">Patches Received</div>
                </div>
                <div class="stat-item">
                    <div class="stat-value" id="patches-applied">{{.Stats.TotalPatchesApplied}}</div>
                    <div class="stat-label">Patches Applied</div>
                </div>
                <div class="stat-item">
                    <div class="stat-value" id="patches-rejected">{{.Stats.TotalPatchesRejected}}</div>
                    <div class="stat-label">Patches Rejected</div>
                </div>
                <div class="stat-item">
                    <div class="stat-value" id="conflicts-detected">{{.Stats.TotalConflictsDetected}}</div>
                    <div class="stat-label">Conflicts Detected</div>
                </div>
                <div class="stat-item">
                    <div class="stat-value" id="conflicts-resolved">{{.Stats.TotalConflictsResolved}}</div>
                    <div class="stat-label">Conflicts Resolved</div>
                </div>
                <div class="stat-item">
                    <div class="stat-value" id="errors">{{.Stats.TotalErrors}}</div>
                    <div class="stat-label">Errors</div>
                </div>
                <div class="stat-item">
                    <div class="stat-value" id="patches-per-second">{{printf "%.2f" .Stats.PatchesPerSecond}}</div>
                    <div class="stat-label">Patches/Second</div>
                </div>
                <div class="stat-item">
                    <div class="stat-value" id="avg-patch-size">{{printf "%.2f" .Stats.AveragePatchSize}}</div>
                    <div class="stat-label">Avg Patch Size (bytes)</div>
                </div>
            </div>
        </div>

        <div class="card">
            <h2>Recent Events</h2>
            <div class="events" id="events-container">
                <!-- Events will be added here dynamically -->
            </div>
        </div>

        <div class="card sessions">
            <h2>Active Sessions</h2>
            <table>
                <thead>
                    <tr>
                        <th>Session ID</th>
                        <th>Last Active</th>
                        <th>Patches Sent</th>
                        <th>Operations Sent</th>
                    </tr>
                </thead>
                <tbody id="sessions-table">
                    <!-- Sessions will be added here dynamically -->
                </tbody>
            </table>
        </div>
    </div>

    <script>
        // Function to format a timestamp
        function formatTime(timestamp) {
            const date = new Date(timestamp);
            return date.toLocaleTimeString();
        }

        // Function to update the stats
        function updateStats() {
            fetch('/stats')
                .then(response => response.json())
                .then(stats => {
                    document.getElementById('patches-received').textContent = stats.TotalPatchesReceived;
                    document.getElementById('patches-applied').textContent = stats.TotalPatchesApplied;
                    document.getElementById('patches-rejected').textContent = stats.TotalPatchesRejected;
                    document.getElementById('conflicts-detected').textContent = stats.TotalConflictsDetected;
                    document.getElementById('conflicts-resolved').textContent = stats.TotalConflictsResolved;
                    document.getElementById('errors').textContent = stats.TotalErrors;
                    document.getElementById('patches-per-second').textContent = stats.PatchesPerSecond.toFixed(2);
                    document.getElementById('avg-patch-size').textContent = stats.AveragePatchSize.toFixed(2);

                    // Update sessions table
                    const sessionsTable = document.getElementById('sessions-table');
                    sessionsTable.innerHTML = '';

                    for (const sessionID in stats.SessionStats) {
                        const session = stats.SessionStats[sessionID];
                        const row = document.createElement('tr');

                        const idCell = document.createElement('td');
                        idCell.textContent = session.SessionID;
                        row.appendChild(idCell);

                        const timeCell = document.createElement('td');
                        timeCell.textContent = formatTime(session.LastActiveTime);
                        row.appendChild(timeCell);

                        const patchesCell = document.createElement('td');
                        patchesCell.textContent = session.TotalPatchesSent;
                        row.appendChild(patchesCell);

                        const opsCell = document.createElement('td');
                        opsCell.textContent = session.TotalOperationsSent;
                        row.appendChild(opsCell);

                        sessionsTable.appendChild(row);
                    }
                })
                .catch(error => console.error('Error fetching stats:', error));
        }

        // Function to add an event to the events container
        function addEvent(event) {
            const eventsContainer = document.getElementById('events-container');

            const eventElement = document.createElement('div');
            eventElement.className = 'event';

            const timeElement = document.createElement('div');
            timeElement.className = 'event-time';
            timeElement.textContent = formatTime(event.Timestamp);
            eventElement.appendChild(timeElement);

            const typeElement = document.createElement('div');
            typeElement.className = 'event-type';
            typeElement.textContent = event.Type;
            eventElement.appendChild(typeElement);

            const detailsElement = document.createElement('div');
            detailsElement.className = 'event-details';

            let details = 'Document: ' + event.DocumentID + ', Session: ' + event.SessionID;
            if (event.PatchID) {
                details += ', Patch: ' + JSON.stringify(event.PatchID);
            }
            if (event.Metadata && event.Metadata.message) {
                details += ' - ' + event.Metadata.message;
            }

            detailsElement.textContent = details;
            eventElement.appendChild(detailsElement);

            eventsContainer.insertBefore(eventElement, eventsContainer.firstChild);

            // Limit the number of events
            if (eventsContainer.children.length > 50) {
                eventsContainer.removeChild(eventsContainer.lastChild);
            }
        }

        // Set up event source for real-time updates
        const eventSource = new EventSource('/stream');
        eventSource.onmessage = function(e) {
            const event = JSON.parse(e.data);
            addEvent(event);
            updateStats();
        };

        // Set up refresh button
        document.getElementById('refresh-btn').addEventListener('click', updateStats);

        // Initial update
        updateStats();

        // Fetch initial events
        fetch('/events')
            .then(response => response.json())
            .then(events => {
                events.forEach(addEvent);
            })
            .catch(error => console.error('Error fetching events:', error));
    </script>
</body>
</html>
`
