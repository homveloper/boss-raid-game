# IdleDungeon

IdleDungeon is a demo application that showcases the integration of `nodestorage/v2` and `eventsync` packages to create a real-time multiplayer game with synchronized state.

## Features

- Real-time synchronization of game state across multiple clients
- 2D open field with monsters and players
- Combat and movement synchronization
- Server-Sent Events (SSE) for real-time updates
- MongoDB for persistent storage
- Optimistic concurrency control using nodestorage
- Event sourcing and state vector synchronization using eventsync

## Project Structure

```
idledungeon/
├── cmd/
│   └── server/
│       └── main.go         # Main server entry point
├── internal/
│   ├── model/
│   │   ├── game.go         # Game state model
│   │   ├── unit.go         # Unit model (players, monsters)
│   │   └── world.go        # World model
│   ├── server/
│   │   ├── handler.go      # HTTP handlers
│   │   ├── sse.go          # SSE implementation
│   │   └── server.go       # Server implementation
│   └── storage/
│       └── storage.go      # Storage implementation using nodestorage
├── client/
│   ├── index.html          # Main HTML file
│   ├── css/
│   │   └── style.css       # CSS styles
│   └── js/
│       ├── app.js          # Main application logic
│       ├── game.js         # Game rendering and logic
│       └── sync.js         # Synchronization with server
└── go.mod                  # Go module file
```

## Prerequisites

- Go 1.21 or higher
- MongoDB 4.4 or higher
- Modern web browser (Chrome, Firefox, Edge, etc.)

## Getting Started

### 1. Start MongoDB

Make sure MongoDB is running on your local machine:

```bash
# Using Docker
docker run -d -p 27017:27017 --name mongodb mongo:latest
```

### 2. Build and Run the Server

```bash
cd idledungeon
go run cmd/server/main.go
```

The server will start on port 8080 by default. You can customize the port and other settings using command-line flags:

```bash
go run cmd/server/main.go --port=8080 --mongo=mongodb://localhost:27017 --db=idledungeon --debug
```

### 3. Open the Client

Open your web browser and navigate to:

```
http://localhost:8080
```

## Usage

1. Click "Connect" to establish a connection to the server
2. Click "Create Game" to create a new game instance
3. Enter a player name to join the game
4. Use the mouse to navigate the game world:
   - Drag to pan the view
   - Scroll to zoom in/out
   - Click on units to select them

## How It Works

### Server-Side

1. The server uses `nodestorage/v2` to store and manage game state in MongoDB
2. When a client makes a change (e.g., moving a player), the server updates the game state using optimistic concurrency control
3. The `eventsync` package captures these changes and converts them to events
4. The server tracks client state vectors to determine which events each client needs

### Client-Side

1. The client connects to the server using Server-Sent Events (SSE)
2. The client maintains a state vector to track which events it has received
3. When the client receives events, it updates its local game state
4. The client renders the game state on a canvas

## Development

### Running in Debug Mode

To run the server in debug mode with more verbose logging:

```bash
go run cmd/server/main.go --debug
```

### Building for Production

```bash
go build -o idledungeon-server cmd/server/main.go
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- `nodestorage/v2` package for optimistic concurrency control
- `eventsync` package for event sourcing and state vector synchronization
