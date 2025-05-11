/**
 * IdleDungeon Sync Client
 * Handles synchronization with the server using SSE
 */
class SyncClient {
  /**
   * Create a new sync client
   * @param {Object} options - Client options
   * @param {string} options.serverUrl - Server URL
   * @param {string} options.clientId - Client ID
   * @param {string} options.gameId - Game ID
   * @param {Function} options.onConnect - Connect callback
   * @param {Function} options.onDisconnect - Disconnect callback
   * @param {Function} options.onEvent - Event callback
   * @param {Function} options.onError - Error callback
   */
  constructor(options) {
    this.options = {
      serverUrl: 'http://localhost:8080',
      clientId: null,
      gameId: null,
      onConnect: null,
      onDisconnect: null,
      onEvent: null,
      onError: null,
      ...options
    };

    this.connected = false;
    this.eventSource = null;
    this.vectorClock = {};
  }

  /**
   * Connect to the server
   * @param {string} gameId - Game ID
   * @returns {Promise<void>}
   */
  async connect(gameId) {
    if (this.connected) {
      return;
    }

    // Update game ID
    if (gameId) {
      this.options.gameId = gameId;
    }

    // Validate client ID and game ID
    if (!this.options.clientId) {
      throw new Error('Client ID is required');
    }

    // Create SSE URL
    const url = new URL(`${this.options.serverUrl}/events`);
    url.searchParams.append('clientId', this.options.clientId);
    if (this.options.gameId) {
      url.searchParams.append('gameId', this.options.gameId);
    }

    // Create event source
    this.eventSource = new EventSource(url.toString());

    // Set up event handlers
    this.eventSource.onopen = () => {
      this.connected = true;
      if (this.options.onConnect) {
        this.options.onConnect();
      }
      this._syncState();
    };

    this.eventSource.onerror = (error) => {
      console.error('SSE error:', error);
      if (this.options.onError) {
        this.options.onError(error);
      }
      this.disconnect();
    };

    this.eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        this._handleEvent(data);
      } catch (error) {
        console.error('Failed to parse SSE event:', error);
        if (this.options.onError) {
          this.options.onError(error);
        }
      }
    };
  }

  /**
   * Disconnect from the server
   */
  disconnect() {
    if (!this.connected) {
      return;
    }

    if (this.eventSource) {
      this.eventSource.close();
      this.eventSource = null;
    }

    this.connected = false;
    if (this.options.onDisconnect) {
      this.options.onDisconnect();
    }
  }

  /**
   * Sync state with the server
   * @private
   */
  _syncState() {
    if (!this.connected || !this.options.gameId) {
      return;
    }

    // Send sync request
    fetch(`${this.options.serverUrl}/api/sync`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        clientId: this.options.clientId,
        documentId: this.options.gameId,
        vectorClock: this.vectorClock
      })
    }).catch(error => {
      console.error('Failed to sync state:', error);
      if (this.options.onError) {
        this.options.onError(error);
      }
    });
  }

  /**
   * Handle an event from the server
   * @param {Object} event - Event data
   * @private
   */
  _handleEvent(event) {
    // Update vector clock
    if (event.data && event.data.vectorClock) {
      for (const [key, value] of Object.entries(event.data.vectorClock)) {
        this.vectorClock[key] = Math.max(this.vectorClock[key] || 0, value);
      }
    }

    // Call event handler
    if (this.options.onEvent) {
      this.options.onEvent(event);
    }
  }

  /**
   * Update a unit in the game
   * @param {string} unitId - Unit ID
   * @param {string} action - Action to perform
   * @param {Object} data - Action data
   * @returns {Promise<Object>} - Updated game state
   */
  async updateUnit(unitId, action, data = {}) {
    if (!this.connected || !this.options.gameId) {
      throw new Error('Not connected to a game');
    }

    const response = await fetch(`${this.options.serverUrl}/api/games/${this.options.gameId}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        unitId,
        action,
        ...data
      })
    });

    if (!response.ok) {
      throw new Error(`Failed to update unit: ${response.statusText}`);
    }

    return response.json();
  }
}
