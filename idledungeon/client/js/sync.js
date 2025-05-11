/**
 * EventSyncClient handles synchronization with the server using Server-Sent Events (SSE)
 */
class EventSyncClient {
    /**
     * Create a new EventSyncClient
     * @param {Object} options - Configuration options
     * @param {string} options.serverUrl - Server URL
     * @param {string} options.clientId - Client ID (optional, will be generated if not provided)
     * @param {Function} options.onConnect - Callback when connected
     * @param {Function} options.onDisconnect - Callback when disconnected
     * @param {Function} options.onEvent - Callback when an event is received
     * @param {Function} options.onError - Callback when an error occurs
     */
    constructor(options) {
        this.serverUrl = options.serverUrl || '';
        this.clientId = options.clientId || this.generateClientId();
        this.onConnect = options.onConnect || (() => {});
        this.onDisconnect = options.onDisconnect || (() => {});
        this.onEvent = options.onEvent || (() => {});
        this.onError = options.onError || (() => {});
        
        this.eventSource = null;
        this.connected = false;
        this.documentId = null;
        this.vectorClock = {};
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 1000; // 1 second
    }
    
    /**
     * Connect to the server
     * @param {string} documentId - Document ID to subscribe to
     */
    connect(documentId) {
        if (this.connected) {
            this.disconnect();
        }
        
        this.documentId = documentId;
        
        // Create event source URL
        const url = `${this.serverUrl}/api/events?clientId=${this.clientId}&documentId=${this.documentId}`;
        
        // Create event source
        this.eventSource = new EventSource(url);
        
        // Set up event handlers
        this.eventSource.addEventListener('connected', (event) => {
            this.connected = true;
            this.reconnectAttempts = 0;
            console.log('Connected to server', event.data);
            this.onConnect(JSON.parse(event.data));
        });
        
        this.eventSource.addEventListener('update', (event) => {
            const eventData = JSON.parse(event.data);
            console.log('Received event', eventData);
            
            // Update vector clock
            if (eventData.clientId && eventData.sequenceNum) {
                this.vectorClock[eventData.clientId] = eventData.sequenceNum;
            }
            
            // Call event handler
            this.onEvent(eventData);
        });
        
        this.eventSource.onerror = (error) => {
            console.error('SSE error', error);
            this.onError(error);
            
            // Handle reconnection
            if (this.connected) {
                this.connected = false;
                this.onDisconnect();
            }
            
            if (this.reconnectAttempts < this.maxReconnectAttempts) {
                this.reconnectAttempts++;
                const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);
                console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
                
                setTimeout(() => {
                    this.connect(this.documentId);
                }, delay);
            }
        };
    }
    
    /**
     * Disconnect from the server
     */
    disconnect() {
        if (this.eventSource) {
            this.eventSource.close();
            this.eventSource = null;
        }
        
        if (this.connected) {
            this.connected = false;
            this.onDisconnect();
        }
    }
    
    /**
     * Generate a random client ID
     * @returns {string} Random client ID
     */
    generateClientId() {
        return 'client-' + Math.random().toString(36).substring(2, 15);
    }
    
    /**
     * Get the current vector clock
     * @returns {Object} Vector clock
     */
    getVectorClock() {
        return { ...this.vectorClock };
    }
    
    /**
     * Check if connected to the server
     * @returns {boolean} True if connected
     */
    isConnected() {
        return this.connected;
    }
}
