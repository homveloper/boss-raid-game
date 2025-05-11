/**
 * IdleDungeon Main Application
 */
class IdleDungeonApp {
  constructor() {
    // DOM elements
    this.connectBtn = document.getElementById('connectBtn');
    this.disconnectBtn = document.getElementById('disconnectBtn');
    this.createGameBtn = document.getElementById('createGameBtn');
    this.joinGameBtn = document.getElementById('joinGameBtn');
    this.connectionStatus = document.getElementById('connectionStatus');
    this.gameIdDisplay = document.getElementById('gameIdDisplay');
    this.playerIdDisplay = document.getElementById('playerIdDisplay');
    this.gameState = document.getElementById('gameState');
    this.eventLog = document.getElementById('eventLog');
    this.gameCanvas = document.getElementById('gameCanvas');

    // App state
    this.clientId = this._generateClientId();
    this.gameId = null;
    this.playerId = null;
    this.connected = false;
    this.vectorClock = {};

    // Initialize game renderer
    this.renderer = new GameRenderer(this.gameCanvas);

    // Initialize sync client
    this.syncClient = new SyncClient({
      serverUrl: window.location.origin,
      clientId: this.clientId,
      onConnect: () => this._handleConnect(),
      onDisconnect: () => this._handleDisconnect(),
      onEvent: (event) => this._handleEvent(event),
      onError: (error) => this._handleError(error)
    });

    // Set up event listeners
    this._setupEventListeners();
  }

  /**
   * Set up event listeners
   * @private
   */
  _setupEventListeners() {
    // Connect button
    this.connectBtn.addEventListener('click', () => {
      this._connect();
    });

    // Disconnect button
    this.disconnectBtn.addEventListener('click', () => {
      this._disconnect();
    });

    // Create game button
    this.createGameBtn.addEventListener('click', () => {
      this._createGame();
    });

    // Join game button
    this.joinGameBtn.addEventListener('click', () => {
      this._joinGame();
    });
  }

  /**
   * Generate a client ID
   * @returns {string} - Client ID
   * @private
   */
  _generateClientId() {
    return 'client-' + Math.random().toString(36).substring(2, 15);
  }

  /**
   * Connect to the server
   * @private
   */
  _connect() {
    this.addEvent({
      type: 'info',
      message: 'Connecting to server...'
    });

    this.syncClient.connect(this.gameId)
      .catch(error => {
        this.addEvent({
          type: 'error',
          message: `Connection error: ${error.message}`
        });
      });
  }

  /**
   * Disconnect from the server
   * @private
   */
  _disconnect() {
    this.syncClient.disconnect();
  }

  /**
   * Create a new game
   * @private
   */
  async _createGame() {
    if (!this.connected) {
      this.addEvent({
        type: 'error',
        message: 'Not connected to server'
      });
      return;
    }

    try {
      const gameName = prompt('Enter game name:', 'New Game');
      if (!gameName) return;

      this.addEvent({
        type: 'info',
        message: 'Creating game...'
      });

      const response = await fetch('/api/games', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          name: gameName
        })
      });

      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`);
      }

      const game = await response.json();
      this.gameId = game.id;
      this.gameIdDisplay.textContent = this.gameId;

      this.addEvent({
        type: 'success',
        message: `Game created: ${game.name} (${game.id})`
      });

      // Update sync client
      this.syncClient.options.gameId = this.gameId;
      this.syncClient._syncState();

      // Create player
      this._createPlayer();
    } catch (error) {
      this.addEvent({
        type: 'error',
        message: `Failed to create game: ${error.message}`
      });
    }
  }

  /**
   * Join an existing game
   * @private
   */
  async _joinGame() {
    if (!this.connected) {
      this.addEvent({
        type: 'error',
        message: 'Not connected to server'
      });
      return;
    }

    try {
      const gameId = prompt('Enter game ID:');
      if (!gameId) return;

      this.addEvent({
        type: 'info',
        message: `Joining game ${gameId}...`
      });

      // Validate game ID
      const response = await fetch(`/api/games/${gameId}`);
      if (!response.ok) {
        throw new Error(`Game not found: ${gameId}`);
      }

      const game = await response.json();
      this.gameId = game.id;
      this.gameIdDisplay.textContent = this.gameId;

      this.addEvent({
        type: 'success',
        message: `Joined game: ${game.name} (${game.id})`
      });

      // Update game renderer
      this.renderer.updateGameState(game);

      // Update sync client
      this.syncClient.options.gameId = this.gameId;
      this.syncClient._syncState();

      // Create player
      this._createPlayer();
    } catch (error) {
      this.addEvent({
        type: 'error',
        message: `Failed to join game: ${error.message}`
      });
    }
  }

  /**
   * Create a player
   * @private
   */
  async _createPlayer() {
    if (!this.connected || !this.gameId) {
      this.addEvent({
        type: 'error',
        message: 'Not connected to a game'
      });
      return;
    }

    try {
      const playerName = prompt('Enter player name:', 'Player');
      if (!playerName) return;

      this.addEvent({
        type: 'info',
        message: 'Creating player...'
      });

      const response = await fetch('/api/players', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          gameId: this.gameId,
          name: playerName
        })
      });

      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`);
      }

      const player = await response.json();
      this.playerId = player.id;
      this.playerIdDisplay.textContent = this.playerId;

      this.addEvent({
        type: 'success',
        message: `Player created: ${player.name} (${player.id})`
      });

      // Refresh game state
      this._refreshGameState();
    } catch (error) {
      this.addEvent({
        type: 'error',
        message: `Failed to create player: ${error.message}`
      });
    }
  }

  /**
   * Refresh the game state
   * @private
   */
  async _refreshGameState() {
    if (!this.gameId) return;

    try {
      const response = await fetch(`/api/games/${this.gameId}`);
      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`);
      }

      const game = await response.json();
      this._updateGameState(game);
    } catch (error) {
      console.error('Failed to refresh game state:', error);
    }
  }

  /**
   * Handle connect event
   * @private
   */
  _handleConnect() {
    this.connected = true;
    this.connectionStatus.textContent = 'Online';
    this.connectionStatus.className = 'status online';
    this.connectBtn.disabled = true;
    this.disconnectBtn.disabled = false;

    this.addEvent({
      type: 'success',
      message: 'Connected to server'
    });
  }

  /**
   * Handle disconnect event
   * @private
   */
  _handleDisconnect() {
    this.connected = false;
    this.connectionStatus.textContent = 'Offline';
    this.connectionStatus.className = 'status offline';
    this.connectBtn.disabled = false;
    this.disconnectBtn.disabled = true;

    this.addEvent({
      type: 'info',
      message: 'Disconnected from server'
    });
  }

  /**
   * Handle event from server
   * @param {Object} event - Event data
   * @private
   */
  _handleEvent(event) {
    console.log('Event received:', event);

    // Add event to log
    this.addEvent({
      type: 'event',
      message: `Event: ${event.type}`,
      data: event.data
    });

    // Handle different event types
    switch (event.type) {
      case 'connected':
        // Already handled by _handleConnect
        break;
      case 'game_update':
        if (event.data && event.data.game) {
          this._updateGameState(event.data.game);
        }
        break;
      case 'heartbeat':
        // Ignore heartbeat events in the log
        break;
      default:
        // Unknown event type
        break;
    }
  }

  /**
   * Handle error
   * @param {Error} error - Error object
   * @private
   */
  _handleError(error) {
    console.error('Error:', error);

    this.addEvent({
      type: 'error',
      message: `Error: ${error.message}`
    });
  }

  /**
   * Update the game state
   * @param {Object} game - Game state
   * @private
   */
  _updateGameState(game) {
    // Update renderer
    this.renderer.updateGameState(game);

    // Update game state display
    this.gameState.innerHTML = `
      <h3>${game.name}</h3>
      <p>Units: ${Object.keys(game.units).length}</p>
      <p>Version: ${game.version}</p>
      <p>Last updated: ${new Date(game.lastUpdated).toLocaleString()}</p>
    `;
  }

  /**
   * Add an event to the event log
   * @param {Object} event - Event data
   */
  addEvent(event) {
    const eventElement = document.createElement('div');
    eventElement.className = `event ${event.type}`;

    const timestamp = document.createElement('div');
    timestamp.className = 'timestamp';
    timestamp.textContent = new Date().toLocaleTimeString();

    const content = document.createElement('div');
    content.className = 'content';
    content.textContent = event.message;

    eventElement.appendChild(timestamp);
    eventElement.appendChild(content);

    this.eventLog.appendChild(eventElement);
    this.eventLog.scrollTop = this.eventLog.scrollHeight;

    // Limit event log size
    while (this.eventLog.children.length > 100) {
      this.eventLog.removeChild(this.eventLog.firstChild);
    }
  }
}

// Initialize app when DOM is loaded
window.addEventListener('DOMContentLoaded', () => {
  window.app = new IdleDungeonApp();
});
