package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	ds "github.com/ipfs/go-datastore"
	dsquery "github.com/ipfs/go-datastore/query"
	crdt "github.com/ipfs/go-ds-crdt"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	format "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

var logger = logging.Logger("crdtserver")

// Config 서버 설정
type Config struct {
	HTTPPort       int
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	PubSubTopic    string
	BootstrapPeers string
	DataNamespace  string
	Debug          bool
	UseIPFSLite    bool // IPFS-Lite 사용 여부
}

// Server CRDT 서버 구조체
type Server struct {
	config       Config
	host         host.Host
	pubsub       *pubsub.PubSub
	topic        *pubsub.Topic
	subscription *pubsub.Subscription
	store        ds.Batching
	crdt         *crdt.Datastore
	bstore       blockstore.Blockstore
	dagService   format.DAGService
	server       *http.Server
	mux          *http.ServeMux
	ctx          context.Context
	cancel       context.CancelFunc
	redisClient  *redis.Client
	peerRegistry *PeerRegistry
	// SSE 관련 필드
	sseClients   map[string]chan []byte
	sseClientsMu sync.Mutex
	// 서버 시작 시간
	startTime time.Time
}

func NewDAGService(bs blockstore.Blockstore, useIPFSLite bool, ctx context.Context) (format.DAGService, error) {
	if useIPFSLite {
		// IPFS-Lite 기반 DAGSyncer 생성
		ipfsLite, err := NewIPFSLiteDAGSyncer(ctx, bs)
		if err != nil {
			return nil, fmt.Errorf("failed to create IPFS-Lite DAGSyncer: %w", err)
		}
		return ipfsLite, nil
	}

	// 기본 SimpleDAGService 사용
	return NewSimpleDAGService(bs), nil
}

// NewServer 새 서버 인스턴스 생성
func NewServer(config Config) (*Server, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// 로깅 설정
	if config.Debug {
		logging.SetLogLevel("*", "debug")
	} else {
		logging.SetLogLevel("*", "info")
	}

	// libp2p 호스트 생성
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
		libp2p.DisableRelay(),
	)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	// 호스트 정보 출력
	logger.Infof("libp2p host created. ID: %s", h.ID())
	for _, addr := range h.Addrs() {
		logger.Infof("Listening on: %s/p2p/%s", addr, h.ID())
	}

	// PubSub 설정
	ps, err := pubsub.NewGossipSub(ctx, h)
	if err != nil {
		h.Close()
		cancel()
		return nil, fmt.Errorf("failed to create pubsub: %w", err)
	}

	// Redis 데이터스토어 설정
	redisOpts := &redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	}

	redisClient := redis.NewClient(redisOpts)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		h.Close()
		cancel()
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// 피어 레지스트리 생성
	peerRegistry, err := NewPeerRegistry(redisClient, h.ID().String())
	if err != nil {
		h.Close()
		redisClient.Close()
		cancel()
		return nil, fmt.Errorf("failed to create peer registry: %w", err)
	}

	// 자신의 피어 정보 등록
	err = peerRegistry.RegisterPeer(h.ID().String(), h.Addrs())
	if err != nil {
		logger.Warnf("Failed to register peer: %v", err)
	}

	// 주기적인 상태 업데이트 시작 (10초마다)
	go peerRegistry.StartHeartbeat(h, 10*time.Second)

	// Redis 데이터스토어 옵션 설정
	dsOpts := &Options{
		TTL: time.Hour * 24 * 7, // 1주일
	}

	// Redis 데이터스토어 생성
	redisDatastore, err := NewRedisDatastore(redisClient, dsOpts)
	if err != nil {
		h.Close()
		redisClient.Close()
		peerRegistry.Close()
		cancel()
		return nil, fmt.Errorf("failed to create Redis datastore: %w", err)
	}

	// 블록스토어 설정
	bstore := blockstore.NewBlockstore(redisDatastore)

	// DAG 서비스 설정
	dagService, err := NewDAGService(bstore, config.UseIPFSLite, ctx)
	if err != nil {
		h.Close()
		redisClient.Close()
		peerRegistry.Close()
		cancel()
		return nil, fmt.Errorf("failed to create DAG service: %w", err)
	}

	// PubSub 토픽 설정
	topic, err := ps.Join(config.PubSubTopic)
	if err != nil {
		h.Close()
		redisClient.Close()
		cancel()
		return nil, fmt.Errorf("failed to join pubsub topic: %w", err)
	}

	// PubSub 구독 설정
	subscription, err := topic.Subscribe()
	if err != nil {
		h.Close()
		redisClient.Close()
		topic.Close()
		cancel()
		return nil, fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	// PubSub 브로드캐스터 생성
	broadcaster := NewPubSubBroadcaster(ctx, topic, subscription)

	// CRDT 데이터스토어 생성
	opts := crdt.DefaultOptions()
	// 재브로드캐스트 주기 (기본값 5분)
	opts.RebroadcastInterval = time.Minute * 5
	// 동기화 주기 (기본값 1초)
	opts.DAGSyncerTimeout = time.Second * 1
	// 병합 주기 (기본값 1초)
	opts.RepairInterval = time.Second * 1
	opts.PutHook = func(k ds.Key, v []byte) {
		logger.Debugf("CRDT Put: %s", k)
	}
	opts.DeleteHook = func(k ds.Key) {
		logger.Debugf("CRDT Delete: %s", k)
	}

	namespace := ds.NewKey(config.DataNamespace)
	crdtDatastore, err := crdt.New(redisDatastore, namespace, dagService, broadcaster, opts)
	if err != nil {
		h.Close()
		redisClient.Close()
		topic.Close()
		subscription.Cancel()
		cancel()
		return nil, fmt.Errorf("failed to create CRDT datastore: %w", err)
	}

	server := &Server{
		config:       config,
		host:         h,
		pubsub:       ps,
		topic:        topic,
		subscription: subscription,
		store:        redisDatastore,
		crdt:         crdtDatastore,
		bstore:       bstore,
		dagService:   dagService,
		ctx:          ctx,
		cancel:       cancel,
		redisClient:  redisClient,
		peerRegistry: peerRegistry,
		sseClients:   make(map[string]chan []byte),
		startTime:    time.Now(),
	}

	// API 라우트 설정
	server.setupRoutes()

	return server, nil
}

// GetCRDTDatastore returns the CRDT datastore
func (s *Server) GetCRDTDatastore() ds.Datastore {
	return s.crdt
}

// setupRoutes API 라우트 설정
func (s *Server) setupRoutes() {
	// 기본 ServeMux 생성
	s.mux = http.NewServeMux()

	// CORS 미들웨어 적용
	corsMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}

	// 상태 확인 엔드포인트
	s.mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status": "ok",
			"peerId": s.host.ID().String(),
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// 피어 정보 엔드포인트
	s.mux.HandleFunc("/peers", func(w http.ResponseWriter, r *http.Request) {
		peers := []string{}
		for _, p := range s.host.Network().Peers() {
			peers = append(peers, p.String())
		}

		response := map[string]interface{}{
			"peerId":         s.host.ID().String(),
			"peerAddrs":      s.host.Addrs(),
			"connectedPeers": peers,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// SSE 엔드포인트 - 실시간 업데이트 수신
	s.mux.HandleFunc("/events", s.handleSSE)

	// 데이터 API
	s.mux.HandleFunc("/api/data/", func(w http.ResponseWriter, r *http.Request) {
		// 경로에서 키 추출
		path := r.URL.Path
		key := strings.TrimPrefix(path, "/api/data/")

		// 키가 비어있고 쿼리 파라미터가 있는 경우 목록 조회
		if key == "" && r.Method == http.MethodGet {
			s.handleListData(w, r)
			return
		}

		// 요청 메서드에 따른 핸들러 호출
		switch r.Method {
		case http.MethodGet:
			s.handleGetData(w, r, key)
		case http.MethodPost, http.MethodPut:
			s.handlePutData(w, r, key)
		case http.MethodDelete:
			s.handleDeleteData(w, r, key)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// CRDT 데이터 뷰어 API
	s.mux.HandleFunc("/api/crdt-viewer/", s.handleCRDTViewer)

	// CORS 미들웨어 적용
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.config.HTTPPort),
		Handler: corsMiddleware(s.mux),
	}

	// SSE 클라이언트 초기화
	s.sseClients = make(map[string]chan []byte)
}

// handleGetData 데이터 조회 핸들러
func (s *Server) handleGetData(w http.ResponseWriter, r *http.Request, key string) {
	if key == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "key is required"})
		return
	}

	dsKey := ds.NewKey(key)
	value, err := s.crdt.Get(s.ctx, dsKey)
	if err != nil {
		if err == ds.ErrNotFound {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "key not found"})
			return
		}
		logger.Errorf("Failed to get data: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 이벤트 알림
	s.broadcastSSEEvent("get", key, value)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	w.Write(value)
}

// handlePutData 데이터 저장 핸들러
func (s *Server) handlePutData(w http.ResponseWriter, r *http.Request, key string) {
	if key == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "key is required"})
		return
	}

	// 요청 본문에서 데이터 읽기
	data, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Errorf("Failed to read request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to read request body"})
		return
	}

	dsKey := ds.NewKey(key)
	err = s.crdt.Put(s.ctx, dsKey, data)
	if err != nil {
		logger.Errorf("Failed to put data: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 이벤트 알림
	s.broadcastSSEEvent("put", key, data)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleDeleteData 데이터 삭제 핸들러
func (s *Server) handleDeleteData(w http.ResponseWriter, r *http.Request, key string) {
	if key == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "key is required"})
		return
	}

	dsKey := ds.NewKey(key)
	err := s.crdt.Delete(s.ctx, dsKey)
	if err != nil {
		logger.Errorf("Failed to delete data: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 이벤트 알림
	s.broadcastSSEEvent("delete", key, nil)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleListData 데이터 목록 조회 핸들러
func (s *Server) handleListData(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")

	q := dsquery.Query{Prefix: prefix}
	results, err := s.crdt.Query(s.ctx, q)
	if err != nil {
		logger.Errorf("Failed to query data: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer results.Close()

	entries := []map[string]interface{}{}
	for {
		result, ok := results.NextSync()
		if !ok {
			break
		}

		if result.Error != nil {
			logger.Errorf("Error in query result: %v", result.Error)
			continue
		}

		entries = append(entries, map[string]interface{}{
			"key":   result.Key,
			"value": string(result.Value), // 문자열로 변환 (JSON 응답용)
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(entries)
}

// handleSSE SSE 핸들러 - 실시간 업데이트 수신
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	// SSE 헤더 설정
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 클라이언트 ID 생성
	clientID := uuid.New().String()

	// 클라이언트용 메시지 채널 생성
	messageChan := make(chan []byte)

	// 클라이언트 등록
	s.sseClientsMu.Lock()
	s.sseClients[clientID] = messageChan
	s.sseClientsMu.Unlock()

	// 클라이언트 연결 종료 시 정리
	defer func() {
		s.sseClientsMu.Lock()
		delete(s.sseClients, clientID)
		close(messageChan)
		s.sseClientsMu.Unlock()
	}()

	// 클라이언트에게 초기 연결 확인 메시지 전송
	fmt.Fprintf(w, "data: {\"event\": \"connected\", \"clientId\": \"%s\"}\n\n", clientID)
	w.(http.Flusher).Flush()

	// 클라이언트 연결 상태 확인
	clientGone := r.Context().Done()

	// 메시지 수신 및 전송
	for {
		select {
		case <-clientGone:
			return
		case msg := <-messageChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			w.(http.Flusher).Flush()
		}
	}
}

// broadcastSSEEvent SSE 이벤트 브로드캐스트
func (s *Server) broadcastSSEEvent(eventType, key string, data []byte) {
	// 이벤트 데이터 생성
	event := map[string]interface{}{
		"event": eventType,
		"key":   key,
	}

	// 데이터가 있는 경우 추가
	if data != nil {
		// 바이너리 데이터를 문자열로 변환 시도
		if utf8.Valid(data) {
			event["value"] = string(data)
		} else {
			// 바이너리 데이터는 base64로 인코딩
			event["value"] = base64.StdEncoding.EncodeToString(data)
			event["encoding"] = "base64"
		}
	}

	// JSON으로 직렬화
	eventJSON, err := json.Marshal(event)
	if err != nil {
		logger.Errorf("Failed to marshal SSE event: %v", err)
		return
	}

	// 모든 클라이언트에게 이벤트 전송
	s.sseClientsMu.Lock()
	for _, clientChan := range s.sseClients {
		select {
		case clientChan <- eventJSON:
			// 성공적으로 전송
		default:
			// 채널이 차면 건너뛰기 (블록하지 않음)
		}
	}
	s.sseClientsMu.Unlock()
}

// handleCRDTViewer CRDT 데이터 뷰어 핸들러
func (s *Server) handleCRDTViewer(w http.ResponseWriter, r *http.Request) {
	// URL 경로에서 하위 경로 추출
	path := strings.TrimPrefix(r.URL.Path, "/api/crdt-viewer/")

	// 경로에 따라 다른 처리
	switch {
	case path == "" || path == "index.html":
		// 메인 뷰어 페이지 제공
		s.handleCRDTViewerUI(w, r)
	case path == "keys":
		// 모든 키 목록 조회
		s.handleCRDTViewerKeys(w, r)
	case strings.HasPrefix(path, "value/"):
		// 특정 키의 값 조회
		key := strings.TrimPrefix(path, "value/")
		s.handleCRDTViewerValue(w, r, key)
	case path == "stats":
		// CRDT 상태 정보 조회
		s.handleCRDTViewerStats(w, r)
	case path == "heads":
		// 현재 heads 목록 조회
		s.handleCRDTViewerHeads(w, r)
	default:
		http.NotFound(w, r)
	}
}

// handleCRDTViewerUI CRDT 데이터 뷰어 UI 제공
func (s *Server) handleCRDTViewerUI(w http.ResponseWriter, r *http.Request) {
	// HTML 템플릿 정의
	html := `<!DOCTYPE html>
<html lang="ko">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>CRDT 데이터 뷰어</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; line-height: 1.6; }
        h1, h2 { color: #333; }
        .container { max-width: 1200px; margin: 0 auto; }
        .card { background: #f9f9f9; border-radius: 5px; padding: 15px; margin-bottom: 20px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .key-list { height: 400px; overflow-y: auto; border: 1px solid #ddd; padding: 10px; background: white; }
        .key-item { cursor: pointer; padding: 5px; border-bottom: 1px solid #eee; }
        .key-item:hover { background: #f0f0f0; }
        .value-display { height: 400px; overflow-y: auto; border: 1px solid #ddd; padding: 10px; background: white; white-space: pre-wrap; font-family: monospace; }
        .stats-display { height: 200px; overflow-y: auto; border: 1px solid #ddd; padding: 10px; background: white; font-family: monospace; }
        .flex-container { display: flex; gap: 20px; }
        .flex-item { flex: 1; }
        button { padding: 8px 12px; background: #4CAF50; color: white; border: none; border-radius: 4px; cursor: pointer; }
        button:hover { background: #45a049; }
    </style>
</head>
<body>
    <div class="container">
        <h1>CRDT 데이터 뷰어</h1>

        <div class="card">
            <h2>시스템 정보</h2>
            <button onclick="loadStats()">상태 정보 새로고침</button>
            <div id="stats" class="stats-display">로딩 중...</div>
        </div>

        <div class="flex-container">
            <div class="flex-item card">
                <h2>키 목록</h2>
                <button onclick="loadKeys()">키 목록 새로고침</button>
                <div id="key-list" class="key-list">로딩 중...</div>
            </div>

            <div class="flex-item card">
                <h2>값 내용</h2>
                <div id="value-display" class="value-display">키를 선택하세요...</div>
            </div>
        </div>

        <div class="card">
            <h2>Heads 정보</h2>
            <button onclick="loadHeads()">Heads 새로고침</button>
            <div id="heads" class="stats-display">로딩 중...</div>
        </div>
    </div>

    <script>
        // 페이지 로드 시 초기 데이터 로드
        document.addEventListener('DOMContentLoaded', function() {
            loadKeys();
            loadStats();
            loadHeads();
        });

        // 키 목록 로드
        function loadKeys() {
            fetch('/api/crdt-viewer/keys')
                .then(response => response.json())
                .then(data => {
                    const keyList = document.getElementById('key-list');
                    keyList.innerHTML = '';

                    if (data.length === 0) {
                        keyList.innerHTML = '<div>키가 없습니다.</div>';
                        return;
                    }

                    data.forEach(key => {
                        const keyItem = document.createElement('div');
                        keyItem.className = 'key-item';
                        keyItem.textContent = key;
                        keyItem.onclick = function() { loadValue(key); };
                        keyList.appendChild(keyItem);
                    });
                })
                .catch(error => {
                    console.error('Error loading keys:', error);
                    document.getElementById('key-list').innerHTML = '<div>키 로드 오류</div>';
                });
        }

        // 특정 키의 값 로드
        function loadValue(key) {
            fetch('/api/crdt-viewer/value/' + encodeURIComponent(key))
                .then(response => response.json())
                .then(data => {
                    const valueDisplay = document.getElementById('value-display');
                    valueDisplay.innerHTML = '<h3>' + key + '</h3>';

                    if (data.error) {
                        valueDisplay.innerHTML += '<div>오류: ' + data.error + '</div>';
                        return;
                    }

                    // 값이 JSON인 경우 예쁘게 표시
                    try {
                        const jsonData = JSON.parse(data.value);
                        valueDisplay.innerHTML += '<pre>' + JSON.stringify(jsonData, null, 2) + '</pre>';
                    } catch (e) {
                        // JSON이 아닌 경우 그대로 표시
                        valueDisplay.innerHTML += '<div>' + data.value + '</div>';
                    }
                })
                .catch(error => {
                    console.error('Error loading value:', error);
                    document.getElementById('value-display').innerHTML = '<div>값 로드 오류</div>';
                });
        }

        // 상태 정보 로드
        function loadStats() {
            fetch('/api/crdt-viewer/stats')
                .then(response => response.json())
                .then(data => {
                    const statsDisplay = document.getElementById('stats');
                    statsDisplay.innerHTML = '<pre>' + JSON.stringify(data, null, 2) + '</pre>';
                })
                .catch(error => {
                    console.error('Error loading stats:', error);
                    document.getElementById('stats').innerHTML = '<div>상태 정보 로드 오류</div>';
                });
        }

        // Heads 정보 로드
        function loadHeads() {
            fetch('/api/crdt-viewer/heads')
                .then(response => response.json())
                .then(data => {
                    const headsDisplay = document.getElementById('heads');
                    headsDisplay.innerHTML = '<pre>' + JSON.stringify(data, null, 2) + '</pre>';
                })
                .catch(error => {
                    console.error('Error loading heads:', error);
                    document.getElementById('heads').innerHTML = '<div>Heads 정보 로드 오류</div>';
                });
        }
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// handleCRDTViewerKeys 모든 키 목록 조회
func (s *Server) handleCRDTViewerKeys(w http.ResponseWriter, r *http.Request) {
	// CRDT 데이터스토어에서 모든 키 조회
	q := dsquery.Query{}
	results, err := s.crdt.Query(s.ctx, q)
	if err != nil {
		logger.Errorf("Failed to query CRDT keys: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer results.Close()

	// 키 목록 수집
	keys := []string{}
	for {
		result, ok := results.NextSync()
		if !ok {
			break
		}

		if result.Error != nil {
			logger.Errorf("Error in query result: %v", result.Error)
			continue
		}

		keys = append(keys, result.Key)
	}

	// JSON 응답 반환
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(keys)
}

// handleCRDTViewerValue 특정 키의 값 조회
func (s *Server) handleCRDTViewerValue(w http.ResponseWriter, r *http.Request, key string) {
	if key == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "key is required"})
		return
	}

	// CRDT 데이터스토어에서 값 조회
	dsKey := ds.NewKey(key)
	value, err := s.crdt.Get(s.ctx, dsKey)
	if err != nil {
		if err == ds.ErrNotFound {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "key not found"})
			return
		}
		logger.Errorf("Failed to get CRDT value: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// 값 처리
	response := map[string]interface{}{
		"key": key,
	}

	// 값이 유효한 UTF-8 문자열인지 확인
	if utf8.Valid(value) {
		response["value"] = string(value)
	} else {
		// 바이너리 데이터는 base64로 인코딩
		response["value"] = base64.StdEncoding.EncodeToString(value)
		response["encoding"] = "base64"
	}

	// JSON 응답 반환
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleCRDTViewerStats CRDT 상태 정보 조회
func (s *Server) handleCRDTViewerStats(w http.ResponseWriter, r *http.Request) {
	// CRDT 데이터스토어 상태 정보 수집
	// 참고: 실제 go-ds-crdt에는 상태 정보를 직접 조회하는 API가 없으므로
	// 여기서는 기본적인 정보만 제공합니다.

	// 키 개수 조회
	keyCount := 0
	q := dsquery.Query{}
	results, err := s.crdt.Query(s.ctx, q)
	if err == nil {
		defer results.Close()
		for {
			_, ok := results.NextSync()
			if !ok {
				break
			}
			keyCount++
		}
	}

	// 시스템 정보 수집
	stats := map[string]interface{}{
		"keyCount":       keyCount,
		"peerID":         s.host.ID().String(),
		"connectedPeers": len(s.host.Network().Peers()),
		"uptime":         time.Since(s.startTime).String(),
		"dataNamespace":  s.config.DataNamespace,
		"pubSubTopic":    s.config.PubSubTopic,
	}

	// JSON 응답 반환
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleCRDTViewerHeads 현재 heads 목록 조회
func (s *Server) handleCRDTViewerHeads(w http.ResponseWriter, r *http.Request) {
	// 참고: go-ds-crdt에는 현재 heads를 직접 조회하는 API가 없으므로
	// 여기서는 기본적인 정보만 제공합니다.

	// 더미 데이터 반환
	heads := map[string]interface{}{
		"note": "현재 go-ds-crdt에서는 heads 정보를 직접 조회할 수 없습니다.",
	}

	// JSON 응답 반환
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(heads)
}

// Start 서버 시작
func (s *Server) Start() error {
	// 부트스트랩 피어 연결
	if s.config.BootstrapPeers != "" {
		go s.connectToBootstrapPeers()
	}

	// 비동기로 서버 시작
	go func() {
		logger.Infof("HTTP server listening on %s", s.server.Addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("HTTP server error: %v", err)
		}
	}()

	// 종료 신호 처리
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// HTTP 서버 종료
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil {
		logger.Errorf("Server forced to shutdown: %v", err)
	}

	// 리소스 정리
	s.Close()

	return nil
}

// connectToBootstrapPeers 부트스트랩 피어에 연결
func (s *Server) connectToBootstrapPeers() {
	peerAddrs := strings.Split(s.config.BootstrapPeers, ",")
	for _, addrStr := range peerAddrs {
		if addrStr == "" {
			continue
		}

		addr, err := multiaddr.NewMultiaddr(strings.TrimSpace(addrStr))
		if err != nil {
			logger.Errorf("Invalid bootstrap peer address: %v", err)
			continue
		}

		peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
		if err != nil {
			logger.Errorf("Failed to parse peer info: %v", err)
			continue
		}

		// 자기 자신은 건너뜀
		if peerInfo.ID == s.host.ID() {
			continue
		}

		logger.Infof("Connecting to bootstrap peer: %s", peerInfo.ID)
		if err := s.host.Connect(s.ctx, *peerInfo); err != nil {
			logger.Warnf("Failed to connect to bootstrap peer %s: %v", peerInfo.ID, err)
		} else {
			logger.Infof("Connected to bootstrap peer: %s", peerInfo.ID)
		}
	}
}

// Close 서버 종료 및 리소스 정리
func (s *Server) Close() {
	logger.Info("Cleaning up resources...")

	if s.crdt != nil {
		s.crdt.Close()
	}

	if s.subscription != nil {
		s.subscription.Cancel()
	}

	if s.topic != nil {
		s.topic.Close()
	}

	if s.peerRegistry != nil {
		s.peerRegistry.Close()
	}

	if s.redisClient != nil {
		s.redisClient.Close()
	}

	if s.host != nil {
		s.host.Close()
	}

	s.cancel()

	logger.Info("Server stopped")
}

func main() {
	// 커맨드 라인 플래그 파싱
	httpPort := flag.Int("port", 8080, "HTTP server port")
	gamePort := flag.Int("game-port", 8081, "Game server port")
	redisAddr := flag.String("redis", "localhost:6379", "Redis server address")
	redisPassword := flag.String("redis-password", "", "Redis password")
	redisDB := flag.Int("redis-db", 0, "Redis database number")
	pubSubTopic := flag.String("topic", "crdt-sync", "PubSub topic for CRDT synchronization")
	bootstrapPeers := flag.String("bootstrap", "", "Comma-separated list of bootstrap peers")
	dataNamespace := flag.String("namespace", "/crdt-data", "Namespace for CRDT data")
	debug := flag.Bool("debug", true, "Enable debug logging")
	useIPFSLite := flag.Bool("ipfs-lite", false, "Use IPFS-Lite as DAGSyncer")
	enableGameServer := flag.Bool("enable-game", true, "Enable boss raid game server")
	clientDir := flag.String("client-dir", "../client", "Directory containing client files")

	flag.Parse()

	// 서버 설정
	config := Config{
		HTTPPort:       *httpPort,
		RedisAddr:      *redisAddr,
		RedisPassword:  *redisPassword,
		RedisDB:        *redisDB,
		PubSubTopic:    *pubSubTopic,
		BootstrapPeers: *bootstrapPeers,
		DataNamespace:  *dataNamespace,
		Debug:          *debug,
		UseIPFSLite:    *useIPFSLite,
	}

	// 서버 생성
	server, err := NewServer(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// 게임 서버 활성화 여부 확인
	if *enableGameServer {
		log.Printf("Starting boss raid game server on port %d", *gamePort)

		// 게임 서버 생성
		gameServer := NewGameServer(server.ctx, server.GetCRDTDatastore(), *clientDir)

		// 게임 서버 시작 (별도 고루틴으로 실행)
		go func() {
			if err := gameServer.Start(*gamePort); err != nil {
				log.Printf("Game server error: %v", err)
			}
		}()
	}

	// CRDT 서버 시작
	if err := server.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
