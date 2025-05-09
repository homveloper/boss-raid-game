/**
 * EventSync 클라이언트 라이브러리
 *
 * 이 라이브러리는 서버와의 실시간 동기화를 위한 클라이언트 측 구현을 제공합니다.
 * WebSocket 또는 SSE(Server-Sent Events)를 통해 서버와 통신하며,
 * 상태 벡터를 기반으로 이벤트 소싱 패턴을 구현합니다.
 */

class EventSyncClient {
  /**
   * EventSync 클라이언트를 생성합니다.
   *
   * @param {Object} options 클라이언트 옵션
   * @param {string} options.serverUrl 서버 URL
   * @param {string} options.documentId 문서 ID
   * @param {string} options.clientId 클라이언트 ID (선택 사항)
   * @param {string} options.transport 전송 방식 ('websocket' 또는 'sse', 기본값: 'websocket')
   * @param {Function} options.onEvent 이벤트 수신 콜백
   * @param {Function} options.onConnect 연결 성공 콜백
   * @param {Function} options.onDisconnect 연결 종료 콜백
   * @param {Function} options.onError 오류 발생 콜백
   */
  constructor(options) {
    this.options = Object.assign({
      transport: 'websocket',
      clientId: `client-${Date.now()}-${Math.floor(Math.random() * 1000)}`,
      retryInterval: 3000,
      maxRetries: 10
    }, options);

    if (!this.options.serverUrl) {
      throw new Error('serverUrl is required');
    }

    if (!this.options.documentId) {
      throw new Error('documentId is required');
    }

    this.connected = false;
    this.connecting = false;
    this.retryCount = 0;
    this.vectorClock = {};
    this.pendingEvents = [];
    this.eventBuffer = {};
    this.connection = null;
  }

  /**
   * 서버에 연결합니다.
   */
  connect() {
    if (this.connected || this.connecting) {
      return;
    }

    this.connecting = true;

    if (this.options.transport === 'websocket') {
      this._connectWebSocket();
    } else if (this.options.transport === 'sse') {
      this._connectSSE();
    } else {
      throw new Error(`Unsupported transport: ${this.options.transport}`);
    }
  }

  /**
   * WebSocket을 통해 서버에 연결합니다.
   * @private
   */
  _connectWebSocket() {
    const url = new URL(`${this.options.serverUrl}/sync`);
    url.protocol = url.protocol.replace('http', 'ws');
    url.searchParams.append('documentId', this.options.documentId);
    url.searchParams.append('clientId', this.options.clientId);

    this.connection = new WebSocket(url.toString());

    this.connection.onopen = () => {
      this._onConnected();
      this._syncState();
    };

    this.connection.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);
        this._handleMessage(message);
      } catch (error) {
        console.error('Failed to parse WebSocket message:', error);
        if (this.options.onError) {
          this.options.onError(error);
        }
      }
    };

    this.connection.onclose = (event) => {
      this._onDisconnected(event.code !== 1000 && event.code !== 1001);
    };

    this.connection.onerror = (error) => {
      console.error('WebSocket error:', error);
      if (this.options.onError) {
        this.options.onError(error);
      }
    };
  }

  /**
   * SSE를 통해 서버에 연결합니다.
   * @private
   */
  _connectSSE() {
    const url = new URL(`${this.options.serverUrl}/events`);
    url.searchParams.append('documentId', this.options.documentId);
    url.searchParams.append('clientId', this.options.clientId);

    this.connection = new EventSource(url.toString());

    this.connection.onopen = () => {
      this._onConnected();
      this._syncState();
    };

    this.connection.addEventListener('connect', (event) => {
      try {
        const data = JSON.parse(event.data);
        if (this.options.onConnect) {
          this.options.onConnect(data);
        }
      } catch (error) {
        console.error('Failed to parse SSE connect event:', error);
      }
    });

    this.connection.addEventListener('event', (event) => {
      try {
        const data = JSON.parse(event.data);
        this._handleEvent(data);
      } catch (error) {
        console.error('Failed to parse SSE event:', error);
        if (this.options.onError) {
          this.options.onError(error);
        }
      }
    });

    this.connection.addEventListener('events', (event) => {
      try {
        const data = JSON.parse(event.data);
        if (data.events && Array.isArray(data.events)) {
          data.events.forEach(event => this._handleEvent(event));
        }
      } catch (error) {
        console.error('Failed to parse SSE events:', error);
        if (this.options.onError) {
          this.options.onError(error);
        }
      }
    });

    this.connection.onerror = (error) => {
      console.error('SSE error:', error);
      if (this.options.onError) {
        this.options.onError(error);
      }
      this._onDisconnected(true);
    };
  }

  /**
   * 연결 성공 시 호출됩니다.
   * @private
   */
  _onConnected() {
    this.connected = true;
    this.connecting = false;
    this.retryCount = 0;
    console.log(`Connected to ${this.options.transport}`);

    if (this.options.onConnect) {
      this.options.onConnect();
    }

    // 보류 중인 이벤트 처리
    this._processPendingEvents();
  }

  /**
   * 연결 종료 시 호출됩니다.
   * @param {boolean} retry 재연결 시도 여부
   * @private
   */
  _onDisconnected(retry = true) {
    if (!this.connected && !this.connecting) {
      return;
    }

    this.connected = false;
    this.connecting = false;

    if (this.options.onDisconnect) {
      this.options.onDisconnect();
    }

    if (retry && this.retryCount < this.options.maxRetries) {
      this.retryCount++;
      console.log(`Reconnecting in ${this.options.retryInterval}ms (attempt ${this.retryCount}/${this.options.maxRetries})...`);
      setTimeout(() => this.connect(), this.options.retryInterval);
    }
  }

  /**
   * 서버와의 연결을 종료합니다.
   */
  disconnect() {
    if (!this.connected && !this.connecting) {
      return;
    }

    if (this.connection) {
      if (this.options.transport === 'websocket') {
        this.connection.close();
      } else if (this.options.transport === 'sse') {
        this.connection.close();
      }
      this.connection = null;
    }

    this._onDisconnected(false);
  }

  /**
   * 상태를 동기화합니다.
   * @private
   */
  _syncState() {
    if (!this.connected) {
      return;
    }

    const message = {
      type: 'sync',
      vectorClock: this.vectorClock
    };

    this._sendMessage(message);
  }

  /**
   * 메시지를 처리합니다.
   * @param {Object} message 수신된 메시지
   * @private
   */
  _handleMessage(message) {
    switch (message.type) {
      case 'event':
        if (message.event) {
          this._handleEvent(message.event);
        }
        break;
      case 'events':
        if (message.events && Array.isArray(message.events)) {
          message.events.forEach(event => this._handleEvent(event));
        }
        break;
      case 'error':
        console.error('Server error:', message.error);
        if (this.options.onError) {
          this.options.onError(new Error(message.error));
        }
        break;
      default:
        console.warn('Unknown message type:', message.type);
    }
  }

  /**
   * 이벤트를 처리합니다.
   * @param {Object} event 수신된 이벤트
   * @private
   */
  _handleEvent(event) {
    // 이미 처리한 이벤트인지 확인
    if (event.clientId && event.sequenceNum) {
      const currentSeq = this.vectorClock[event.clientId] || 0;
      if (currentSeq >= event.sequenceNum) {
        console.log(`Skipping already processed event: ${event.clientId}:${event.sequenceNum}`);
        return;
      }

      // 이벤트 순서 확인
      // 이벤트 시퀀스 번호가 현재 상태 벡터보다 1보다 크게 앞서 있으면 버퍼에 저장
      if (event.sequenceNum > currentSeq + 1) {
        console.log(`Event ${event.clientId}:${event.sequenceNum} is ahead of current state (${currentSeq}). Buffering.`);
        this._bufferEvent(event);
        return;
      }
    }

    // 이벤트 콜백 호출
    if (this.options.onEvent) {
      this.options.onEvent(event);
    }

    // 벡터 시계 업데이트
    if (event.clientId && event.sequenceNum) {
      this.vectorClock[event.clientId] = event.sequenceNum;

      // 버퍼에서 다음 이벤트 처리 시도
      this._processEventBuffer(event.clientId);
    }
  }

  /**
   * 이벤트를 버퍼에 저장합니다.
   * @param {Object} event 버퍼링할 이벤트
   * @private
   */
  _bufferEvent(event) {
    if (!this.eventBuffer) {
      this.eventBuffer = {};
    }

    const clientId = event.clientId;
    if (!this.eventBuffer[clientId]) {
      this.eventBuffer[clientId] = {};
    }

    this.eventBuffer[clientId][event.sequenceNum] = event;
    console.log(`Event ${clientId}:${event.sequenceNum} buffered. Buffer size: ${Object.keys(this.eventBuffer[clientId]).length}`);
  }

  /**
   * 버퍼에서 다음 이벤트를 처리합니다.
   * @param {string} clientId 클라이언트 ID
   * @private
   */
  _processEventBuffer(clientId) {
    if (!this.eventBuffer || !this.eventBuffer[clientId]) {
      return;
    }

    const currentSeq = this.vectorClock[clientId] || 0;
    const nextSeq = currentSeq + 1;

    // 버퍼에서 다음 시퀀스 번호의 이벤트 찾기
    const nextEvent = this.eventBuffer[clientId][nextSeq];
    if (nextEvent) {
      // 버퍼에서 이벤트 제거
      delete this.eventBuffer[clientId][nextSeq];

      console.log(`Processing buffered event ${clientId}:${nextSeq}`);

      // 이벤트 처리 (재귀적으로 버퍼 처리를 위해 _handleEvent 호출)
      this._handleEvent(nextEvent);
    }
  }

  /**
   * 보류 중인 이벤트를 처리합니다.
   * @private
   */
  _processPendingEvents() {
    // 서버 권한 모델에서는 클라이언트에서 이벤트를 전송하지 않음
    if (this.pendingEvents.length > 0) {
      console.log(`서버 권한 모델에서는 클라이언트에서 이벤트를 전송할 수 없습니다. ${this.pendingEvents.length}개의 보류 중인 이벤트가 무시됩니다.`);
      this.pendingEvents = [];
    }
  }

  /**
   * 메시지를 전송합니다.
   * @param {Object} message 전송할 메시지
   * @private
   */
  _sendMessage(message) {
    if (!this.connected) {
      console.warn('Cannot send message: not connected');
      return;
    }

    if (this.options.transport === 'websocket') {
      this.connection.send(JSON.stringify(message));
    } else if (this.options.transport === 'sse') {
      // SSE는 단방향 통신이므로 별도의 HTTP 요청 필요
      fetch(`${this.options.serverUrl}/api/sync/${this.options.documentId}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify(message)
      }).catch(error => {
        console.error('Failed to send message via HTTP:', error);
        if (this.options.onError) {
          this.options.onError(error);
        }
      });
    }
  }
}

// CommonJS 및 ES 모듈 지원
if (typeof module !== 'undefined' && module.exports) {
  module.exports = EventSyncClient;
} else if (typeof define === 'function' && define.amd) {
  define([], function() { return EventSyncClient; });
} else {
  window.EventSyncClient = EventSyncClient;
}
