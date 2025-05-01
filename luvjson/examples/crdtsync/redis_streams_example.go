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
	documentID := flag.String("doc", "example-doc", "문서 ID")
	redisAddr := flag.String("redis", "localhost:6379", "Redis 서버 주소")
	flag.Parse()

	// 컨텍스트 생성
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 시그널 처리
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("종료 신호 수신, 종료 중...")
		cancel()
	}()

	// 피어 ID 생성
	peerID := fmt.Sprintf("peer-%d", time.Now().UnixNano()%1000)
	fmt.Printf("피어 ID: %s\n", peerID)

	// 세션 ID 생성
	sessionID := common.NewSessionID()

	// CRDT 문서 생성
	doc := crdt.NewDocument(sessionID)

	// API 모델 생성
	model := api.NewModelWithDocument(doc)

	// Redis 클라이언트 생성
	redisClient := redis.NewClient(&redis.Options{
		Addr: *redisAddr,
	})

	// Redis 연결 테스트
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Redis 연결 실패: %v", err)
	}
	defer redisClient.Close()

	// Redis Streams 브로드캐스터 생성
	broadcaster, err := crdtsync.NewRedisStreamsBroadcaster(
		redisClient,
		fmt.Sprintf("%s-patches-stream", *documentID),
		crdtpubsub.EncodingFormatJSON,
		sessionID,
	)
	if err != nil {
		log.Fatalf("Redis Streams 브로드캐스터 생성 실패: %v", err)
	}

	// Redis Streams 패치 저장소 생성
	patchStore, err := crdtsync.NewRedisStreamsPatchStore(
		redisClient,
		fmt.Sprintf("%s-patches-store", *documentID),
		crdtpubsub.EncodingFormatJSON,
	)
	if err != nil {
		log.Fatalf("Redis Streams 패치 저장소 생성 실패: %v", err)
	}

	// 상태 벡터 생성
	stateVector := crdtsync.NewStateVector()

	// Redis 피어 발견 생성
	peerDiscovery := crdtsync.NewRedisPeerDiscovery(redisClient, *documentID, peerID)
	if err := peerDiscovery.Start(ctx); err != nil {
		log.Fatalf("피어 발견 시작 실패: %v", err)
	}
	defer peerDiscovery.Close()

	// 싱커 생성
	syncer := crdtsync.NewPubSubSyncer(
		nil, // PubSub은 사용하지 않음
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

	// 주기적으로 문서 내용 업데이트
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("종료 중...")
			return
		case <-ticker.C:
			// 현재 문서 내용 가져오기
			content := model.GetApi().View()
			if content == nil {
				continue
			}

			// 문서 내용 출력
			fmt.Println("현재 문서 내용:")
			fmt.Printf("%+v\n", content)

			// 문서 내용 업데이트
			model.GetApi().Root(map[string]interface{}{
				"title":    fmt.Sprintf("문서 %s", *documentID),
				"content":  fmt.Sprintf("업데이트된 내용 - %s", time.Now().Format(time.RFC3339)),
				"authors":  append(getAuthors(content), peerID),
				"modified": time.Now().Format(time.RFC3339),
			})

			// 변경사항 플러시 및 브로드캐스트
			patch := model.GetApi().Flush()
			if err := syncManager.ApplyPatch(ctx, patch); err != nil {
				fmt.Printf("패치 적용 실패: %v\n", err)
				continue
			}

			// 모든 피어와 동기화
			if err := syncManager.SyncWithAllPeers(ctx); err != nil {
				fmt.Printf("동기화 실패: %v\n", err)
			}
		}
	}
}

// getAuthors는 문서 내용에서 작성자 목록을 추출합니다.
func getAuthors(content interface{}) []string {
	if content == nil {
		return []string{}
	}

	contentMap, ok := content.(map[string]interface{})
	if !ok {
		return []string{}
	}

	authors, ok := contentMap["authors"].([]interface{})
	if !ok {
		return []string{}
	}

	result := make([]string, 0, len(authors))
	for _, author := range authors {
		if authorStr, ok := author.(string); ok {
			// 중복 제거
			found := false
			for _, existingAuthor := range result {
				if existingAuthor == authorStr {
					found = true
					break
				}
			}
			if !found {
				result = append(result, authorStr)
			}
		}
	}

	return result
}
