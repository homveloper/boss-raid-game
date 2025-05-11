/**
 * SSE 클라이언트 모듈
 * 서버와의 SSE 연결을 관리하는 간단한 클라이언트
 */

class SSEClient {
    constructor() {
        this.eventSource = null;
        this.connected = false;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 3000; // 3초
        this.listeners = {};
        this.clientId = this.generateClientId();
        this.gameId = '';
    }

    /**
     * 랜덤 클라이언트 ID 생성
     */
    generateClientId() {
        return 'client-' + Math.random().toString(36).substring(2, 15);
    }

    /**
     * SSE 서버에 연결
     * @param {string} gameId - 게임 ID (선택적)
     * @param {string} clientId - 클라이언트 ID (선택적)
     */
    connect(gameId = '', clientId = '') {
        // 기존 연결 종료
        this.disconnect();

        // 클라이언트 ID와 게임 ID 설정
        this.clientId = clientId || this.clientId;
        this.gameId = gameId || this.gameId;

        // 연결 URL 생성
        const url = `/events?clientId=${this.clientId}&gameId=${this.gameId}`;
        console.log(`[SSE] 연결 시도: ${url}`);

        try {
            // 디버깅을 위한 로그 추가
            console.log('[SSE] EventSource 생성 전');
            document.getElementById('event-log').innerHTML += '<p>EventSource 생성 시도...</p>';

            // EventSource 생성
            this.eventSource = new EventSource(url);
            console.log('[SSE] EventSource 객체 생성됨:', this.eventSource);
            document.getElementById('event-log').innerHTML += '<p>EventSource 객체 생성됨</p>';

            // 연결 이벤트 핸들러
            this.eventSource.onopen = (event) => {
                console.log('[SSE] 연결 성공:', event);
                document.getElementById('event-log').innerHTML += '<p>SSE 연결 성공!</p>';
                this.connected = true;
                this.reconnectAttempts = 0;
                this.dispatchEvent('connected', { message: '서버에 연결되었습니다.' });
            };

            // 메시지 이벤트 핸들러
            this.eventSource.onmessage = (event) => {
                console.log(`[SSE] 메시지 수신: ${event.data}`);
                document.getElementById('event-log').innerHTML += `<p>메시지 수신: ${event.data}</p>`;

                try {
                    const data = JSON.parse(event.data);
                    console.log('[SSE] 파싱된 메시지:', data);
                    document.getElementById('event-log').innerHTML += `<p>파싱된 메시지: ${JSON.stringify(data)}</p>`;

                    if (data.type && data.data) {
                        // 서버에서 보내는 이벤트 형식: { type: 'eventType', data: { ... } }
                        console.log(`[SSE] 이벤트 타입: ${data.type}`);
                        document.getElementById('event-log').innerHTML += `<p>이벤트 타입: ${data.type}</p>`;
                        this.dispatchEvent(data.type, data.data);
                    } else {
                        // 기타 이벤트 형식
                        console.log('[SSE] 일반 메시지 이벤트');
                        document.getElementById('event-log').innerHTML += '<p>일반 메시지 이벤트</p>';
                        this.dispatchEvent('message', data);
                    }
                } catch (error) {
                    console.error('[SSE] 메시지 파싱 오류:', error);
                    console.error('[SSE] 원본 메시지:', event.data);
                    document.getElementById('event-log').innerHTML += `<p>메시지 파싱 오류: ${error.message}</p>`;
                    document.getElementById('event-log').innerHTML += `<p>원본 메시지: ${event.data}</p>`;
                }
            };

            // 오류 이벤트 핸들러
            this.eventSource.onerror = (error) => {
                console.error('[SSE] 연결 오류:', error);
                console.error('[SSE] 연결 상태:', this.eventSource.readyState);
                document.getElementById('event-log').innerHTML += `<p>SSE 연결 오류! 상태: ${this.eventSource.readyState}</p>`;

                // readyState 값 해석
                let stateText = '알 수 없음';
                if (this.eventSource.readyState === 0) stateText = 'CONNECTING';
                if (this.eventSource.readyState === 1) stateText = 'OPEN';
                if (this.eventSource.readyState === 2) stateText = 'CLOSED';

                document.getElementById('event-log').innerHTML += `<p>연결 상태: ${stateText}</p>`;

                this.connected = false;

                // 브라우저 네트워크 탭 확인 안내
                document.getElementById('event-log').innerHTML +=
                    '<p>브라우저 개발자 도구의 네트워크 탭에서 /events 요청을 확인해 보세요.</p>';

                // 재연결 시도
                if (this.reconnectAttempts < this.maxReconnectAttempts) {
                    this.reconnectAttempts++;
                    console.log(`[SSE] 재연결 시도 ${this.reconnectAttempts}/${this.maxReconnectAttempts} (${this.reconnectDelay}ms 후)`);
                    document.getElementById('event-log').innerHTML +=
                        `<p>재연결 시도 ${this.reconnectAttempts}/${this.maxReconnectAttempts} (${this.reconnectDelay}ms 후)</p>`;

                    this.dispatchEvent('reconnecting', {
                        attempt: this.reconnectAttempts,
                        maxAttempts: this.maxReconnectAttempts
                    });

                    setTimeout(() => {
                        this.connect(this.gameId, this.clientId);
                    }, this.reconnectDelay);
                } else {
                    console.error('[SSE] 최대 재연결 시도 횟수 초과');
                    document.getElementById('event-log').innerHTML += '<p>최대 재연결 시도 횟수 초과</p>';

                    this.dispatchEvent('disconnected', {
                        reason: 'max_attempts_exceeded',
                        message: '서버 연결에 실패했습니다. 페이지를 새로고침해 주세요.'
                    });
                }
            };

            return true;
        } catch (error) {
            console.error('[SSE] 연결 생성 오류:', error);
            document.getElementById('event-log').innerHTML += `<p>연결 생성 오류: ${error.message}</p>`;
            document.getElementById('event-log').innerHTML += '<p>브라우저가 SSE를 지원하는지 확인하세요.</p>';

            // 브라우저 SSE 지원 여부 확인
            if (typeof EventSource === 'undefined') {
                console.error('[SSE] 이 브라우저는 Server-Sent Events를 지원하지 않습니다.');
                document.getElementById('event-log').innerHTML += '<p>이 브라우저는 Server-Sent Events를 지원하지 않습니다.</p>';
            }

            return false;
        }
    }

    /**
     * SSE 서버 연결 종료
     */
    disconnect() {
        if (this.eventSource) {
            console.log('[SSE] 연결 종료');
            this.eventSource.close();
            this.eventSource = null;
            this.connected = false;
            this.dispatchEvent('disconnected', { reason: 'user_action', message: '연결이 종료되었습니다.' });
        }
    }

    /**
     * 이벤트 리스너 등록
     * @param {string} eventType - 이벤트 타입
     * @param {Function} callback - 콜백 함수
     */
    on(eventType, callback) {
        if (!this.listeners[eventType]) {
            this.listeners[eventType] = [];
        }
        this.listeners[eventType].push(callback);
    }

    /**
     * 이벤트 리스너 제거
     * @param {string} eventType - 이벤트 타입
     * @param {Function} callback - 콜백 함수 (선택적)
     */
    off(eventType, callback) {
        if (!this.listeners[eventType]) return;

        if (callback) {
            this.listeners[eventType] = this.listeners[eventType].filter(cb => cb !== callback);
        } else {
            delete this.listeners[eventType];
        }
    }

    /**
     * 이벤트 발생
     * @param {string} eventType - 이벤트 타입
     * @param {Object} data - 이벤트 데이터
     */
    dispatchEvent(eventType, data) {
        if (!this.listeners[eventType]) return;

        this.listeners[eventType].forEach(callback => {
            try {
                callback(data);
            } catch (error) {
                console.error(`[SSE] 이벤트 핸들러 오류 (${eventType}):`, error);
            }
        });
    }

    /**
     * 연결 상태 확인
     * @returns {boolean} 연결 상태
     */
    isConnected() {
        return this.connected;
    }

    /**
     * 클라이언트 ID 가져오기
     * @returns {string} 클라이언트 ID
     */
    getClientId() {
        return this.clientId;
    }

    /**
     * 게임 ID 가져오기
     * @returns {string} 게임 ID
     */
    getGameId() {
        return this.gameId;
    }
}

// 전역 SSE 클라이언트 인스턴스 생성
window.sseClient = new SSEClient();
