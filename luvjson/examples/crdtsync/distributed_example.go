package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"

	"tictactoe/luvjson/api"
	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtpubsub"
	"tictactoe/luvjson/crdtpubsub/memory"
	redispubsub "tictactoe/luvjson/crdtpubsub/redis"
	"tictactoe/luvjson/crdtsync"
)

// 문서 구조체
type Document struct {
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Authors  []string `json:"authors"`
	Modified string   `json:"modified"`
}

func main() {
	// 명령줄 인수 파싱
	var (
		redisAddr  = flag.String("redis", "localhost:6379", "Redis 서버 주소")
		useRedis   = flag.Bool("use-redis", false, "Redis PubSub 사용 여부")
		nodeID     = flag.String("node-id", "", "노드 ID (비워두면 자동 생성)")
		documentID = flag.String("doc-id", "example-doc", "문서 ID")
	)
	flag.Parse()

	// 노드 ID 설정
	var peerID string
	if *nodeID == "" {
		peerID = common.NewSessionID().String()[:8]
	} else {
		peerID = *nodeID
	}

	fmt.Printf("노드 ID: %s\n", peerID)
	fmt.Printf("문서 ID: %s\n", *documentID)

	// 컨텍스트 생성
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 세션 ID 생성
	sessionID := common.NewSessionID()
	fmt.Printf("세션 ID: %s\n", sessionID)

	// CRDT 문서 생성
	doc := crdt.NewDocument(sessionID)

	// API 모델 생성
	model := api.NewModelWithDocument(doc)

	// PubSub 생성
	var pubsub crdtpubsub.PubSub
	var err error

	if *useRedis {
		// Redis 클라이언트 생성
		redisClient := redis.NewClient(&redis.Options{
			Addr: *redisAddr,
		})

		// Redis 연결 테스트
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Fatalf("Redis 연결 실패: %v", err)
		}

		// Redis PubSub 생성
		pubsub, err = redispubsub.NewRedisPubSub(redisClient, crdtpubsub.NewOptions())
		if err != nil {
			log.Fatalf("Redis PubSub 생성 실패: %v", err)
		}
	} else {
		// 메모리 PubSub 생성
		pubsub, err = memory.NewPubSub()
		if err != nil {
			log.Fatalf("메모리 PubSub 생성 실패: %v", err)
		}
	}
	defer pubsub.Close()

	// 브로드캐스터 생성
	broadcaster := crdtsync.NewPubSubBroadcaster(
		pubsub,
		fmt.Sprintf("%s-patches", *documentID),
		crdtpubsub.EncodingFormatJSON,
		sessionID,
	)

	// 패치 저장소 생성
	patchStore := crdtsync.NewMemoryPatchStore()

	// 상태 벡터 생성
	stateVector := crdtsync.NewStateVector()

	// 피어 발견 생성
	var peerDiscovery crdtsync.PeerDiscovery
	if *useRedis {
		// Redis 클라이언트 생성
		redisClient := redis.NewClient(&redis.Options{
			Addr: *redisAddr,
		})

		// Redis 피어 발견 생성
		peerDiscovery = crdtsync.NewRedisPeerDiscovery(redisClient, *documentID, peerID)
		if err := peerDiscovery.(*crdtsync.RedisPeerDiscovery).Start(ctx); err != nil {
			log.Fatalf("피어 발견 시작 실패: %v", err)
		}
	} else {
		// TODO: 메모리 피어 발견 구현
		peerDiscovery = &dummyPeerDiscovery{}
	}
	defer peerDiscovery.Close()

	// 싱커 생성
	syncer := crdtsync.NewPubSubSyncer(
		pubsub,
		fmt.Sprintf("%s-sync", *documentID),
		peerID,
		stateVector,
		patchStore,
		crdtpubsub.EncodingFormatJSON,
	)

	// 동기화 매니저 생성
	syncManager := crdtsync.NewSyncManager(doc, broadcaster, syncer, peerDiscovery, patchStore)

	// 동기화 매니저 시작
	if err := syncManager.Start(ctx); err != nil {
		log.Fatalf("동기화 매니저 시작 실패: %v", err)
	}
	defer syncManager.Stop()

	// 초기 문서 설정
	if model.GetApi().View() == nil {
		fmt.Println("초기 문서 생성")
		model.GetApi().Root(map[string]interface{}{
			"title":    fmt.Sprintf("문서 %s", *documentID),
			"content":  "초기 내용",
			"authors":  []string{peerID},
			"modified": time.Now().Format(time.RFC3339),
		})

		// 변경사항 플러시 및 브로드캐스트
		patch := model.GetApi().Flush()
		if err := syncManager.ApplyPatch(ctx, patch); err != nil {
			log.Fatalf("패치 적용 실패: %v", err)
		}
	}

	// 문서 내용 출력
	printDocument(model)

	// 사용자 입력 처리
	go handleUserInput(ctx, model, syncManager)

	// 종료 신호 대기
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("종료 중...")
}

// printDocument는 문서 내용을 출력합니다.
func printDocument(model *api.Model) {
	view, err := model.View()
	if err != nil {
		fmt.Printf("문서 보기 실패: %v\n", err)
		return
	}

	fmt.Println("\n=== 문서 내용 ===")
	if view == nil {
		fmt.Println("문서가 비어 있습니다.")
		return
	}

	docMap, ok := view.(map[string]interface{})
	if !ok {
		fmt.Printf("예상치 못한 문서 형식: %T\n", view)
		return
	}

	fmt.Printf("제목: %v\n", docMap["title"])
	fmt.Printf("내용: %v\n", docMap["content"])
	fmt.Printf("작성자: %v\n", docMap["authors"])
	fmt.Printf("수정일: %v\n", docMap["modified"])
	fmt.Println("================")
}

// handleUserInput은 사용자 입력을 처리합니다.
func handleUserInput(ctx context.Context, model *api.Model, syncManager crdtsync.SyncManager) {
	for {
		fmt.Println("\n명령어:")
		fmt.Println("1. 제목 변경")
		fmt.Println("2. 내용 변경")
		fmt.Println("3. 작성자 추가")
		fmt.Println("4. 문서 보기")
		fmt.Println("5. 종료")
		fmt.Print("> ")

		var choice int
		fmt.Scanln(&choice)

		switch choice {
		case 1:
			fmt.Print("새 제목: ")
			var title string
			fmt.Scanln(&title)
			updateDocument(ctx, model, syncManager, "title", title)
		case 2:
			fmt.Print("새 내용: ")
			var content string
			fmt.Scanln(&content)
			updateDocument(ctx, model, syncManager, "content", content)
		case 3:
			fmt.Print("새 작성자: ")
			var author string
			fmt.Scanln(&author)

			// 현재 작성자 목록 가져오기
			view, _ := model.View()
			docMap := view.(map[string]interface{})
			authors := docMap["authors"].([]interface{})

			// 새 작성자 추가
			newAuthors := make([]string, len(authors)+1)
			for i, a := range authors {
				newAuthors[i] = a.(string)
			}
			newAuthors[len(authors)] = author

			updateDocument(ctx, model, syncManager, "authors", newAuthors)
		case 4:
			printDocument(model)
		case 5:
			return
		default:
			fmt.Println("잘못된 선택입니다.")
		}
	}
}

// updateDocument는 문서를 업데이트합니다.
func updateDocument(ctx context.Context, model *api.Model, syncManager crdtsync.SyncManager, field string, value interface{}) {
	// 필드 업데이트
	model.GetApi().Obj([]interface{}{}).Set(map[string]interface{}{
		field:      value,
		"modified": time.Now().Format(time.RFC3339),
	})

	// 변경사항 플러시 및 브로드캐스트
	patch := model.GetApi().Flush()
	if err := syncManager.ApplyPatch(ctx, patch); err != nil {
		fmt.Printf("패치 적용 실패: %v\n", err)
		return
	}

	fmt.Printf("%s 필드가 업데이트되었습니다.\n", field)
	printDocument(model)
}

// dummyPeerDiscovery는 더미 피어 발견 구현입니다.
type dummyPeerDiscovery struct{}

func (d *dummyPeerDiscovery) DiscoverPeers(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (d *dummyPeerDiscovery) RegisterPeer(ctx context.Context, peerID string) error {
	return nil
}

func (d *dummyPeerDiscovery) UnregisterPeer(ctx context.Context, peerID string) error {
	return nil
}

func (d *dummyPeerDiscovery) Close() error {
	return nil
}
