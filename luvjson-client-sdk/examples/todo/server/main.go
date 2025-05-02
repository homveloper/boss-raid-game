package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtstorage"

	"github.com/gorilla/websocket"
)

// Todo 항목 구조체
type TodoItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// 클라이언트 구조체
type Client struct {
	ID   string
	Conn *websocket.Conn
}

// 패치 구조체 (클라이언트-서버 통신용)
type ClientPatch struct {
	DocumentID  string     `json:"documentId"`
	BaseVersion int64      `json:"baseVersion"`
	Operations  []ClientOp `json:"operations"`
	ClientID    string     `json:"clientId"`
}

// 작업 구조체 (클라이언트-서버 통신용)
type ClientOp struct {
	Type      string          `json:"type"`
	Path      string          `json:"path"`
	Value     json.RawMessage `json:"value,omitempty"`
	Timestamp int64           `json:"timestamp"`
	ClientID  string          `json:"clientId"`
}

// 서버 구조체
type Server struct {
	// CRDT 문서 맵 (문서 ID -> CRDT 문서)
	documents   map[string]*crdt.Document
	documentsMu sync.RWMutex

	// 문서 버전 맵 (문서 ID -> 버전)
	docVersions   map[string]int64
	docVersionsMu sync.RWMutex

	// CRDT 스토리지
	storage crdtstorage.Storage

	// 클라이언트 맵 (문서 ID -> 클라이언트 ID -> 클라이언트)
	clients   map[string]map[string]*Client
	clientsMu sync.RWMutex

	// WebSocket 업그레이더
	upgrader websocket.Upgrader
}

// 새 서버 생성
func NewServer() (*Server, error) {
	// 메모리 스토리지 생성
	storage, err := crdtstorage.NewStorageWithCustomPersistence(context.Background(), crdtstorage.DefaultStorageOptions(), crdtstorage.NewMemoryPersistence())
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %v", err)
	}

	return &Server{
		documents:   make(map[string]*crdt.Document),
		docVersions: make(map[string]int64),
		storage:     storage,
		clients:     make(map[string]map[string]*Client),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // 모든 오리진 허용 (개발용)
			},
		},
	}, nil
}

// 로깅 미들웨어
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 요청 정보 로깅
		log.Printf("[%s] %s %s", r.Method, r.RequestURI, r.RemoteAddr)

		// 요청 헤더 로깅
		log.Printf("Headers: %v", r.Header)

		// 다음 핸들러 호출
		next.ServeHTTP(w, r)

		// 응답 시간 로깅
		log.Printf("Completed in %v", time.Since(start))
	})
}

// 로깅 핸들러 래퍼
func loggingHandlerFunc(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 요청 정보 로깅
		log.Printf("[%s] %s %s", r.Method, r.RequestURI, r.RemoteAddr)

		// 요청 헤더 로깅
		log.Printf("Headers: %v", r.Header)

		// 원래 핸들러 호출
		f(w, r)

		// 응답 시간 로깅
		log.Printf("Completed in %v", time.Since(start))
	}
}

func main() {
	// 명령줄 인자 파싱
	port := flag.String("port", "8080", "서버 포트")
	flag.Parse()

	// 서버 생성
	server, err := NewServer()
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// 기본 Todo 문서 생성
	if err := server.createDefaultDocument(); err != nil {
		log.Fatalf("Failed to create default document: %v", err)
	}

	// 정적 파일 서버 설정
	fs := http.FileServer(http.Dir("../client"))
	http.Handle("/", loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// WASM 파일에 대한 올바른 MIME 타입 설정
		if filepath.Ext(r.URL.Path) == ".wasm" {
			w.Header().Set("Content-Type", "application/wasm")
		}
		fs.ServeHTTP(w, r)
	})))

	// API 엔드포인트 설정
	http.HandleFunc("/api/documents", loggingHandlerFunc(server.handleDocuments))
	http.HandleFunc("/api/documents/", loggingHandlerFunc(server.handleDocument))
	http.HandleFunc("/ws", loggingHandlerFunc(server.handleWebSocket))

	// 서버 시작
	log.Printf("Todo 앱 서버가 http://localhost:%s 에서 실행 중입니다", *port)
	log.Fatal(http.ListenAndServe(":"+*port, nil))
}

// 기본 Todo 문서 생성
func (s *Server) createDefaultDocument() error {
	s.documentsMu.Lock()
	defer s.documentsMu.Unlock()

	docID := "todos"

	// 스토리지에서 문서 가져오기 시도
	doc, err := s.storage.GetDocument(context.Background(), docID)
	if err == nil {
		// 문서가 존재하면 메모리에 저장
		crdtDoc := doc.CRDTDoc
		s.documents[docID] = crdtDoc
		log.Printf("문서 '%s'를 스토리지에서 로드했습니다.", docID)

		// 문서 버전 초기화 (기존 문서는 버전 1로 시작)
		s.docVersionsMu.Lock()
		s.docVersions[docID] = 1
		s.docVersionsMu.Unlock()

		// 클라이언트 맵 초기화
		s.clientsMu.Lock()
		s.clients[docID] = make(map[string]*Client)
		s.clientsMu.Unlock()

		return nil
	}

	// 문서가 없으면 새로 생성
	log.Printf("문서 '%s'가 없어 새로 생성합니다.", docID)
	sessionID := common.NewSessionID()
	crdtDoc := crdt.NewDocument(sessionID)
	s.documents[docID] = crdtDoc

	// 클라이언트 맵 초기화
	s.clientsMu.Lock()
	s.clients[docID] = make(map[string]*Client)
	s.clientsMu.Unlock()

	// 샘플 Todo 항목 추가
	pb := crdtpatch.NewPatchBuilder(sessionID, 1)

	// 루트 객체 생성
	rootOp := pb.NewObject()

	// 첫 번째 Todo 항목 추가
	item1 := map[string]interface{}{
		"id":        "1",
		"title":     "CRDT 학습하기",
		"completed": false,
		"createdAt": time.Now().Format(time.RFC3339),
		"updatedAt": time.Now().Format(time.RFC3339),
	}
	pb.InsertObjectField(rootOp.ID, "1", item1)

	// 두 번째 Todo 항목 추가
	item2 := map[string]interface{}{
		"id":        "2",
		"title":     "Todo 앱 만들기",
		"completed": false,
		"createdAt": time.Now().Format(time.RFC3339),
		"updatedAt": time.Now().Format(time.RFC3339),
	}
	pb.InsertObjectField(rootOp.ID, "2", item2)

	// 패치 생성
	patch := pb.Flush()

	// 패치 직접 적용 (json 마샬링 없이)
	if err := patch.Apply(crdtDoc); err != nil {
		return fmt.Errorf("failed to apply initial patch: %v", err)
	}

	// 스토리지에 문서 생성
	storageDoc, err := s.storage.CreateDocument(context.Background(), docID)
	if err != nil {
		return fmt.Errorf("failed to create document in storage: %v", err)
	}

	// 문서 내용 설정 - 직접 스토리지 문서의 CRDT 문서에 패치 적용
	docView, err := crdtDoc.View()
	if err != nil {
		return fmt.Errorf("failed to get document view: %v", err)
	}

	// 스토리지 문서의 CRDT 문서에 직접 패치 적용
	storagePb := crdtpatch.NewPatchBuilder(sessionID, 1)
	storageRootOp := storagePb.NewObject()

	// 문서 내용을 루트 객체에 추가
	if m, ok := docView.(map[string]interface{}); ok {
		for k, v := range m {
			// 시간 값을 문자열로 변환
			if todoMap, ok := v.(map[string]interface{}); ok {
				todoMapCopy := make(map[string]interface{})
				for tk, tv := range todoMap {
					if tk == "createdAt" || tk == "updatedAt" {
						if t, ok := tv.(time.Time); ok {
							todoMapCopy[tk] = t.Format(time.RFC3339)
						} else {
							todoMapCopy[tk] = tv
						}
					} else {
						todoMapCopy[tk] = tv
					}
				}
				storagePb.InsertObjectField(storageRootOp.ID, k, todoMapCopy)
			} else {
				storagePb.InsertObjectField(storageRootOp.ID, k, v)
			}
		}
	}

	// 패치 생성 및 적용
	storagePatch := storagePb.Flush()
	if err := storagePatch.Apply(storageDoc.CRDTDoc); err != nil {
		return fmt.Errorf("failed to apply patch to storage document: %v", err)
	}

	// 문서 버전 초기화 (새 문서는 버전 1로 시작)
	s.docVersionsMu.Lock()
	s.docVersions[docID] = 1
	s.docVersionsMu.Unlock()

	log.Printf("문서 '%s'를 생성하고 초기화했습니다.", docID)
	return nil
}

// 문서 목록 처리 핸들러
func (s *Server) handleDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.documentsMu.RLock()
	defer s.documentsMu.RUnlock()

	// 문서 ID 목록 반환
	var ids []string
	for id := range s.documents {
		ids = append(ids, id)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ids)
}

// 특정 문서 처리 핸들러
func (s *Server) handleDocument(w http.ResponseWriter, r *http.Request) {
	// 문서 ID 추출
	docID := filepath.Base(r.URL.Path)

	switch r.Method {
	case http.MethodGet:
		// 문서 조회
		s.documentsMu.RLock()
		doc, exists := s.documents[docID]
		s.documentsMu.RUnlock()

		if !exists {
			http.Error(w, "Document not found", http.StatusNotFound)
			return
		}

		// CRDT 문서에서 Todo 항목 추출
		content := make(map[string]TodoItem)
		docView, err := doc.View()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get document view: %v", err), http.StatusInternalServerError)
			return
		}

		// 루트 객체의 모든 키를 순회하며 Todo 항목 추출
		if docMap, ok := docView.(map[string]interface{}); ok {
			for key, value := range docMap {
				if todoMap, ok := value.(map[string]interface{}); ok {
					// map[string]interface{}를 TodoItem으로 변환
					todo := TodoItem{
						ID: key,
					}

					if title, ok := todoMap["title"].(string); ok {
						todo.Title = title
					}

					if completed, ok := todoMap["completed"].(bool); ok {
						todo.Completed = completed
					}

					if createdAt, ok := todoMap["createdAt"].(time.Time); ok {
						todo.CreatedAt = createdAt
					} else if createdAtStr, ok := todoMap["createdAt"].(string); ok {
						// 문자열을 시간으로 파싱
						if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
							todo.CreatedAt = t
						}
					}

					if updatedAt, ok := todoMap["updatedAt"].(time.Time); ok {
						todo.UpdatedAt = updatedAt
					} else if updatedAtStr, ok := todoMap["updatedAt"].(string); ok {
						// 문자열을 시간으로 파싱
						if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
							todo.UpdatedAt = t
						}
					}

					content[key] = todo
				}
			}
		}

		// 현재 문서 버전 가져오기
		s.docVersionsMu.RLock()
		docVersion := s.docVersions[docID]
		s.docVersionsMu.RUnlock()

		// 응답 생성
		response := map[string]interface{}{
			"id":      docID,
			"version": docVersion, // 실제 문서 버전 사용
			"content": content,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	case http.MethodPost:
		// 패치 적용
		var clientPatch ClientPatch
		if err := json.NewDecoder(r.Body).Decode(&clientPatch); err != nil {
			http.Error(w, "Invalid patch format", http.StatusBadRequest)
			return
		}

		// 문서 존재 확인
		s.documentsMu.RLock()
		_, exists := s.documents[docID]
		s.documentsMu.RUnlock()

		if !exists {
			http.Error(w, "Document not found", http.StatusNotFound)
			return
		}

		// 패치 적용
		if err := s.applyClientPatch(docID, &clientPatch); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// 현재 문서 버전 가져오기
		s.docVersionsMu.RLock()
		docVersion := s.docVersions[docID]
		s.docVersionsMu.RUnlock()

		// 성공 응답
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"version": docVersion, // 실제 문서 버전 사용
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// WebSocket 핸들러
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// WebSocket 연결 업그레이드
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	// 클라이언트 ID 생성
	clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())

	// 초기 메시지 수신 (문서 ID)
	var initMsg struct {
		DocumentID string `json:"documentId"`
	}
	if err := conn.ReadJSON(&initMsg); err != nil {
		log.Println("WebSocket read error:", err)
		return
	}

	docID := initMsg.DocumentID
	if docID == "" {
		docID = "todos" // 기본 문서 ID
	}

	// 문서 존재 확인 및 생성
	s.documentsMu.RLock()
	doc, exists := s.documents[docID]
	s.documentsMu.RUnlock()

	if !exists {
		// 문서가 없으면 생성
		s.documentsMu.Lock()
		sessionID := common.NewSessionID()
		doc = crdt.NewDocument(sessionID)
		s.documents[docID] = doc
		s.documentsMu.Unlock()

		// 문서 버전 초기화 (새 문서는 버전 1로 시작)
		s.docVersionsMu.Lock()
		s.docVersions[docID] = 1
		s.docVersionsMu.Unlock()

		// 스토리지에 문서 생성
		storageDoc, err := s.storage.CreateDocument(context.Background(), docID)
		if err != nil {
			log.Printf("Failed to create document in storage: %v", err)
		} else {
			// 문서 내용 설정 - 직접 스토리지 문서의 CRDT 문서에 패치 적용
			docView, err := doc.View()
			if err != nil {
				log.Printf("Failed to get document view: %v", err)
			} else {
				// 스토리지 문서의 CRDT 문서에 직접 패치 적용
				storagePb := crdtpatch.NewPatchBuilder(sessionID, 1)
				storageRootOp := storagePb.NewObject()

				// 문서 내용을 루트 객체에 추가
				if m, ok := docView.(map[string]interface{}); ok {
					for k, v := range m {
						// 시간 값을 문자열로 변환
						if todoMap, ok := v.(map[string]interface{}); ok {
							todoMapCopy := make(map[string]interface{})
							for tk, tv := range todoMap {
								if tk == "createdAt" || tk == "updatedAt" {
									if t, ok := tv.(time.Time); ok {
										todoMapCopy[tk] = t.Format(time.RFC3339)
									} else {
										todoMapCopy[tk] = tv
									}
								} else {
									todoMapCopy[tk] = tv
								}
							}
							storagePb.InsertObjectField(storageRootOp.ID, k, todoMapCopy)
						} else {
							storagePb.InsertObjectField(storageRootOp.ID, k, v)
						}
					}
				}

				// 패치 생성 및 적용
				storagePatch := storagePb.Flush()
				if err := storagePatch.Apply(storageDoc.CRDTDoc); err != nil {
					log.Printf("Failed to apply patch to storage document: %v", err)
				}
			}
		}

		s.clientsMu.Lock()
		s.clients[docID] = make(map[string]*Client)
		s.clientsMu.Unlock()
	}

	// 클라이언트 등록
	s.clientsMu.Lock()
	if _, ok := s.clients[docID]; !ok {
		s.clients[docID] = make(map[string]*Client)
	}
	s.clients[docID][clientID] = &Client{
		ID:   clientID,
		Conn: conn,
	}
	s.clientsMu.Unlock()

	// 초기 문서 상태 전송
	s.documentsMu.RLock()
	docView, err := doc.View()
	s.documentsMu.RUnlock()

	if err != nil {
		log.Printf("Failed to get document view: %v", err)
		return
	}

	// CRDT 문서에서 Todo 항목 추출
	content := make(map[string]TodoItem)

	// 문서 내용 로깅
	docJSON, _ := json.MarshalIndent(docView, "", "  ")
	log.Printf("초기 문서 상태 [클라이언트: %s, 문서: %s]: %s", clientID, docID, string(docJSON))

	// 루트 객체의 모든 키를 순회하며 Todo 항목 추출
	if docMap, ok := docView.(map[string]interface{}); ok {
		log.Printf("문서 맵 변환 성공 [클라이언트: %s, 문서: %s, 키 수: %d]", clientID, docID, len(docMap))

		for key, value := range docMap {
			log.Printf("Todo 항목 처리 [키: %s, 값 타입: %T]", key, value)

			if todoMap, ok := value.(map[string]interface{}); ok {
				// map[string]interface{}를 TodoItem으로 변환
				todo := TodoItem{
					ID: key,
				}

				if title, ok := todoMap["title"].(string); ok {
					todo.Title = title
				} else {
					log.Printf("제목 필드 변환 실패 [키: %s, 값: %v, 타입: %T]", key, todoMap["title"], todoMap["title"])
				}

				if completed, ok := todoMap["completed"].(bool); ok {
					todo.Completed = completed
				} else {
					log.Printf("완료 상태 필드 변환 실패 [키: %s, 값: %v, 타입: %T]", key, todoMap["completed"], todoMap["completed"])
				}

				if createdAt, ok := todoMap["createdAt"].(time.Time); ok {
					todo.CreatedAt = createdAt
				} else if createdAtStr, ok := todoMap["createdAt"].(string); ok {
					// 문자열을 시간으로 파싱
					if t, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
						todo.CreatedAt = t
					} else {
						log.Printf("생성 시간 파싱 실패 [키: %s, 값: %s, 오류: %v]", key, createdAtStr, err)
					}
				} else {
					log.Printf("생성 시간 필드 변환 실패 [키: %s, 값: %v, 타입: %T]", key, todoMap["createdAt"], todoMap["createdAt"])
				}

				if updatedAt, ok := todoMap["updatedAt"].(time.Time); ok {
					todo.UpdatedAt = updatedAt
				} else if updatedAtStr, ok := todoMap["updatedAt"].(string); ok {
					// 문자열을 시간으로 파싱
					if t, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
						todo.UpdatedAt = t
					} else {
						log.Printf("수정 시간 파싱 실패 [키: %s, 값: %s, 오류: %v]", key, updatedAtStr, err)
					}
				} else {
					log.Printf("수정 시간 필드 변환 실패 [키: %s, 값: %v, 타입: %T]", key, todoMap["updatedAt"], todoMap["updatedAt"])
				}

				content[key] = todo
				log.Printf("Todo 항목 추가 성공 [키: %s, 제목: %s]", key, todo.Title)
			} else {
				log.Printf("Todo 항목 맵 변환 실패 [키: %s, 값 타입: %T]", key, value)
			}
		}
	} else {
		log.Printf("문서 맵 변환 실패 [클라이언트: %s, 문서: %s, 값 타입: %T]", clientID, docID, docView)
	}

	log.Printf("추출된 Todo 항목 수: %d", len(content))

	// 현재 문서 버전 가져오기
	s.docVersionsMu.RLock()
	docVersion := s.docVersions[docID]
	s.docVersionsMu.RUnlock()

	initialState := map[string]interface{}{
		"type": "init",
		"document": map[string]interface{}{
			"id":      docID,
			"version": docVersion, // 실제 문서 버전 사용
			"content": content,
		},
	}

	if err := conn.WriteJSON(initialState); err != nil {
		log.Println("WebSocket write error:", err)
		return
	}

	// 클라이언트 메시지 처리 루프
	for {
		var clientPatch ClientPatch
		if err := conn.ReadJSON(&clientPatch); err != nil {
			log.Println("WebSocket read error:", err)
			break
		}

		// 메시지 로깅
		patchJSON, _ := json.Marshal(clientPatch)
		log.Printf("WebSocket 메시지 수신 [클라이언트: %s, 문서: %s]: %s",
			clientID, docID, string(patchJSON))

		// 패치에 클라이언트 ID 설정
		clientPatch.ClientID = clientID

		// 작업 로깅
		for i, op := range clientPatch.Operations {
			opValue, _ := json.Marshal(op.Value)
			log.Printf("작업 상세 [인덱스: %d, 유형: %s, 경로: %s, 값: %s]",
				i, op.Type, op.Path, string(opValue))
		}

		// 패치 적용
		if err := s.applyClientPatch(docID, &clientPatch); err != nil {
			// 오류 메시지 전송
			errMsg := map[string]interface{}{
				"type":  "error",
				"error": err.Error(),
			}
			log.Printf("패치 적용 오류 [클라이언트: %s, 문서: %s]: %s",
				clientID, docID, err.Error())
			conn.WriteJSON(errMsg)
			continue
		}

		log.Printf("패치 적용 성공 [클라이언트: %s, 문서: %s, 작업 수: %d]",
			clientID, docID, len(clientPatch.Operations))

		// 다른 클라이언트에게 패치 브로드캐스트
		s.broadcastClientPatch(docID, &clientPatch, clientID)
	}

	// 클라이언트 연결 종료 시 정리
	s.clientsMu.Lock()
	if clients, ok := s.clients[docID]; ok {
		delete(clients, clientID)
	}
	s.clientsMu.Unlock()
}

// 클라이언트 패치를 CRDT 패치로 변환하여 적용
func (s *Server) applyClientPatch(docID string, clientPatch *ClientPatch) error {
	log.Printf("패치 적용 시작 [문서: %s, 클라이언트: %s, 작업 수: %d]",
		docID, clientPatch.ClientID, len(clientPatch.Operations))

	// 문서 가져오기
	s.documentsMu.RLock()
	doc, exists := s.documents[docID]
	s.documentsMu.RUnlock()

	if !exists {
		return fmt.Errorf("document not found: %s", docID)
	}

	// 현재 문서 버전 확인
	s.docVersionsMu.RLock()
	currentVersion := s.docVersions[docID]
	s.docVersionsMu.RUnlock()

	// 클라이언트의 baseVersion이 현재 버전과 일치하는지 확인
	if clientPatch.BaseVersion > 0 && clientPatch.BaseVersion != currentVersion {
		return fmt.Errorf("version mismatch: client base version %d, server version %d",
			clientPatch.BaseVersion, currentVersion)
	}

	// 세션 ID 생성
	sessionID := common.NewSessionID()

	// ClientPatch를 CRDT 패치로 변환
	pb := crdtpatch.NewPatchBuilder(sessionID, 1)

	// 루트 객체 가져오기 - 한 번만 생성
	rootOp := pb.NewObject()

	for i, op := range clientPatch.Operations {
		log.Printf("작업 적용 [문서: %s, 인덱스: %d, 유형: %s, 경로: %s]",
			docID, i, op.Type, op.Path)

		switch op.Type {
		case "add":
			// Todo 항목 추가
			var todo TodoItem
			var todoStr string

			if err := json.Unmarshal(op.Value, &todoStr); err == nil {
				// 문자열에서 TodoItem으로 변환
				if err := json.Unmarshal([]byte(todoStr), &todo); err != nil {
					return fmt.Errorf("invalid todo item in string: %v", err)
				}
			} else {
				// 직접 JSON 객체로 시도
				if err := json.Unmarshal(op.Value, &todo); err != nil {
					return fmt.Errorf("invalid todo item format: %v", err)
				}
			}

			// 시간 값을 문자열로 변환
			createdAtStr := todo.CreatedAt.Format(time.RFC3339)
			updatedAtStr := todo.UpdatedAt.Format(time.RFC3339)

			// Todo 항목 추가
			pb.InsertObjectField(rootOp.ID, op.Path, map[string]interface{}{
				"id":        todo.ID,
				"title":     todo.Title,
				"completed": todo.Completed,
				"createdAt": createdAtStr,
				"updatedAt": updatedAtStr,
			})

		case "update":
			// Todo 항목 업데이트
			var updates map[string]interface{}
			var updatesStr string

			if err := json.Unmarshal(op.Value, &updatesStr); err == nil {
				// 문자열에서 맵으로 변환
				if err := json.Unmarshal([]byte(updatesStr), &updates); err != nil {
					return fmt.Errorf("invalid updates in string: %v", err)
				}
			} else {
				// 직접 JSON 객체로 시도
				if err := json.Unmarshal(op.Value, &updates); err != nil {
					return fmt.Errorf("invalid updates format: %v", err)
				}
			}

			// 업데이트할 필드만 패치 빌더에 추가
			if title, ok := updates["title"].(string); ok {
				pb.InsertObjectField(rootOp.ID, op.Path+"/title", title)
			}
			if completed, ok := updates["completed"].(bool); ok {
				pb.InsertObjectField(rootOp.ID, op.Path+"/completed", completed)
			}

			// UpdatedAt 필드 업데이트
			updatedAtStr := time.Now().Format(time.RFC3339)
			pb.InsertObjectField(rootOp.ID, op.Path+"/updatedAt", updatedAtStr)

		case "remove":
			// Todo 항목 삭제
			pb.DeleteObjectField(rootOp.ID, op.Path)

		default:
			return fmt.Errorf("unknown operation type: %s", op.Type)
		}
	}

	// 패치 생성
	patch := pb.Flush()
	if patch == nil {
		return fmt.Errorf("no operations to apply")
	}

	// 패치 직접 적용 (JSON 마샬링 없이)
	if err := patch.Apply(doc); err != nil {
		log.Printf("패치 적용 오류: %v", err)
		return err
	}

	// 스토리지에 문서 업데이트
	storageDoc, err := s.storage.GetDocument(context.Background(), docID)
	if err != nil {
		log.Printf("Failed to get document from storage: %v", err)
	} else {
		// 문서 내용 설정 - 직접 스토리지 문서의 CRDT 문서에 패치 적용
		docView, err := doc.View()
		if err != nil {
			log.Printf("Failed to get document view: %v", err)
		} else {
			// 스토리지 문서의 CRDT 문서에 직접 패치 적용
			storagePb := crdtpatch.NewPatchBuilder(sessionID, 1)
			storageRootOp := storagePb.NewObject()

			// 문서 내용을 루트 객체에 추가
			if m, ok := docView.(map[string]interface{}); ok {
				for k, v := range m {
					// 시간 값을 문자열로 변환
					if todoMap, ok := v.(map[string]interface{}); ok {
						todoMapCopy := make(map[string]interface{})
						for tk, tv := range todoMap {
							if tk == "createdAt" || tk == "updatedAt" {
								if t, ok := tv.(time.Time); ok {
									todoMapCopy[tk] = t.Format(time.RFC3339)
								} else {
									todoMapCopy[tk] = tv
								}
							} else {
								todoMapCopy[tk] = tv
							}
						}
						storagePb.InsertObjectField(storageRootOp.ID, k, todoMapCopy)
					} else {
						storagePb.InsertObjectField(storageRootOp.ID, k, v)
					}
				}
			}

			// 패치 생성 및 적용
			storagePatch := storagePb.Flush()
			if err := storagePatch.Apply(storageDoc.CRDTDoc); err != nil {
				log.Printf("Failed to apply patch to storage document: %v", err)
			}
		}
	}

	// 문서 버전 증가
	s.docVersionsMu.Lock()
	s.docVersions[docID]++
	log.Printf("문서 버전 증가 [문서: %s, 새 버전: %d]", docID, s.docVersions[docID])
	s.docVersionsMu.Unlock()

	log.Printf("패치 적용 완료 [문서: %s]", docID)
	return nil
}

// 문서 루트를 객체로 초기화
func (s *Server) initializeDocumentRoot(docID string) error {
	// 문서 가져오기
	s.documentsMu.RLock()
	doc, exists := s.documents[docID]
	s.documentsMu.RUnlock()

	if !exists {
		return fmt.Errorf("document not found: %s", docID)
	}

	// 문서 상태 확인
	docView, err := doc.View()
	if err != nil {
		return fmt.Errorf("failed to get document view: %v", err)
	}

	// 문서 내용 로깅
	docJSON, _ := json.MarshalIndent(docView, "", "  ")
	log.Printf("문서 초기화 전 상태 [문서: %s]: %s", docID, string(docJSON))

	// 문서가 이미 맵인지 확인
	if _, ok := docView.(map[string]interface{}); ok {
		log.Printf("문서 '%s'의 루트가 이미 객체입니다.", docID)
		return nil
	}

	// 세션 ID 생성
	sessionID := common.NewSessionID()

	// 루트를 객체로 초기화하는 패치 생성
	pb := crdtpatch.NewPatchBuilder(sessionID, 1)
	pb.NewObject() // 루트를 객체로 설정

	// 패치 적용
	patch := pb.Flush()
	if err := patch.Apply(doc); err != nil {
		return fmt.Errorf("failed to initialize document root: %v", err)
	}

	log.Printf("문서 '%s'의 루트를 객체로 초기화했습니다.", docID)
	return nil
}

// 클라이언트 패치 브로드캐스트
func (s *Server) broadcastClientPatch(docID string, clientPatch *ClientPatch, senderID string) {
	log.Printf("패치 브로드캐스트 시작 [문서: %s, 발신자: %s]", docID, senderID)

	// 현재 문서 버전 가져오기
	s.docVersionsMu.RLock()
	docVersion := s.docVersions[docID]
	s.docVersionsMu.RUnlock()

	// 패치 메시지 생성 (현재 문서 버전 포함)
	patchMsg := map[string]interface{}{
		"type":    "patch",
		"patch":   clientPatch,
		"version": docVersion, // 현재 문서 버전 포함
	}

	// 확인 메시지 생성 (발신자용)
	ackMsg := map[string]interface{}{
		"type":    "ack",
		"version": docVersion, // 실제 문서 버전 사용
		"success": true,
	}

	// 모든 클라이언트에게 전송
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	if clients, ok := s.clients[docID]; ok {
		log.Printf("브로드캐스트 대상 클라이언트 수: %d", len(clients))

		for id, client := range clients {
			if id == senderID {
				// 발신자에게는 확인 메시지 전송
				log.Printf("확인 메시지 전송 [클라이언트: %s]", id)
				if err := client.Conn.WriteJSON(ackMsg); err != nil {
					log.Printf("확인 메시지 전송 실패 [클라이언트: %s]: %v", id, err)
				}
			} else {
				// 다른 클라이언트에게는 패치 전송
				log.Printf("패치 전송 [클라이언트: %s]", id)
				if err := client.Conn.WriteJSON(patchMsg); err != nil {
					log.Printf("패치 전송 실패 [클라이언트: %s]: %v", id, err)
				}
			}
		}
	} else {
		log.Printf("문서 %s에 대한 클라이언트가 없습니다", docID)
	}

	log.Printf("패치 브로드캐스트 완료 [문서: %s]", docID)
}
