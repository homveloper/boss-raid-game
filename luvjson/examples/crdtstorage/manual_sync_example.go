package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"tictactoe/luvjson/api"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtstorage"
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
		storageType = flag.String("storage", "redis", "저장소 유형 (memory, file, redis)")
		redisAddr   = flag.String("redis", "localhost:6379", "Redis 서버 주소")
		filePath    = flag.String("path", "documents", "파일 저장소 경로")
		documentID  = flag.String("doc", "sync-example-doc", "문서 ID")
		nodeID      = flag.String("node", "", "노드 ID (비워두면 자동 생성)")
	)
	flag.Parse()

	// 노드 ID 설정
	var peerID string
	if *nodeID == "" {
		peerID = fmt.Sprintf("node-%d", time.Now().UnixNano()%1000)
	} else {
		peerID = *nodeID
	}

	fmt.Printf("노드 ID: %s\n", peerID)
	fmt.Printf("문서 ID: %s\n", *documentID)

	// 컨텍스트 생성
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 저장소 옵션 생성
	options := crdtstorage.DefaultStorageOptions()
	options.PubSubType = *storageType
	options.RedisAddr = *redisAddr
	options.PersistencePath = *filePath
	options.PersistenceType = *storageType
	options.AutoSave = true
	options.AutoSaveInterval = time.Second * 10
	options.KeyPrefix = fmt.Sprintf("sync-%s", peerID)

	// 저장소 생성
	storage, err := crdtstorage.NewStorage(ctx, options)
	if err != nil {
		log.Fatalf("저장소 생성 실패: %v", err)
	}
	defer storage.Close()

	// 문서 생성 또는 로드
	var doc *crdtstorage.Document
	doc, err = storage.GetDocument(ctx, *documentID)
	if err != nil {
		fmt.Printf("문서 로드 실패: %v, 새 문서 생성\n", err)
		doc, err = storage.CreateDocument(ctx, *documentID)
		if err != nil {
			log.Fatalf("문서 생성 실패: %v", err)
		}

		// 초기 문서 내용 설정
		result := doc.Edit(ctx, func(api *api.ModelApi) error {
			api.Root(map[string]interface{}{
				"title":    fmt.Sprintf("동기화 예제 문서 %s", *documentID),
				"content":  "이 문서는 수동 동기화 예제를 보여줍니다.",
				"authors":  []string{peerID},
				"modified": time.Now().Format(time.RFC3339),
			})
			return nil
		})
		if !result.Success {
			log.Fatalf("문서 초기화 실패: %v", result.Error)
		}
	}

	// 문서 변경 콜백 등록
	doc.OnChange(func(d *crdtstorage.Document, patch *crdtpatch.Patch) {
		fmt.Println("\n=== 문서 변경됨 ===")
		printDocument(d)
	})

	// 문서 내용 출력
	printDocument(doc)

	// 사용자 입력 처리
	go handleUserInput(ctx, doc, storage, peerID)

	// 종료 신호 대기
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("종료 중...")
}

// printDocument는 문서 내용을 출력합니다.
func printDocument(doc *crdtstorage.Document) {
	// 문서 내용 가져오기
	var content Document
	if err := doc.GetContentAs(&content); err != nil {
		fmt.Printf("문서 내용 가져오기 실패: %v\n", err)
		return
	}

	fmt.Println("\n=== 문서 내용 ===")
	fmt.Printf("ID: %s\n", doc.ID)
	fmt.Printf("제목: %s\n", content.Title)
	fmt.Printf("내용: %s\n", content.Content)
	fmt.Printf("작성자: %v\n", content.Authors)
	fmt.Printf("수정일: %s\n", content.Modified)
	fmt.Printf("마지막 수정: %s\n", doc.LastModified.Format(time.RFC3339))
	fmt.Println("================")
}

// handleUserInput은 사용자 입력을 처리합니다.
func handleUserInput(ctx context.Context, doc *crdtstorage.Document, storage crdtstorage.Storage, peerID string) {
	for {
		fmt.Println("\n명령어:")
		fmt.Println("1. 제목 변경")
		fmt.Println("2. 내용 변경")
		fmt.Println("3. 작성자 추가")
		fmt.Println("4. 문서 보기")
		fmt.Println("5. 문서 저장")
		fmt.Println("6. 문서 동기화")
		fmt.Println("7. 모든 문서 동기화")
		fmt.Println("8. 특정 피어와 동기화")
		fmt.Println("9. 종료")
		fmt.Print("> ")

		var choice int
		fmt.Scanln(&choice)

		switch choice {
		case 1:
			fmt.Print("새 제목: ")
			var title string
			fmt.Scanln(&title)
			updateDocument(ctx, doc, "title", title)
		case 2:
			fmt.Print("새 내용: ")
			var content string
			fmt.Scanln(&content)
			updateDocument(ctx, doc, "content", content)
		case 3:
			fmt.Print("새 작성자: ")
			var author string
			fmt.Scanln(&author)

			// 현재 작성자 목록 가져오기
			var content Document
			if err := doc.GetContentAs(&content); err != nil {
				fmt.Printf("문서 내용 가져오기 실패: %v\n", err)
				continue
			}

			// 새 작성자 추가
			newAuthors := append(content.Authors, author)
			updateDocument(ctx, doc, "authors", newAuthors)
		case 4:
			printDocument(doc)
		case 5:
			if err := doc.Save(ctx); err != nil {
				fmt.Printf("문서 저장 실패: %v\n", err)
			} else {
				fmt.Println("문서가 저장되었습니다.")
			}
		case 6:
			// 문서 동기화 (모든 피어와)
			if err := doc.Sync(ctx, ""); err != nil {
				fmt.Printf("문서 동기화 실패: %v\n", err)
			} else {
				fmt.Println("문서가 모든 피어와 동기화되었습니다.")
			}
		case 7:
			// 모든 문서 동기화
			if err := storage.SyncAllDocuments(ctx, ""); err != nil {
				fmt.Printf("모든 문서 동기화 실패: %v\n", err)
			} else {
				fmt.Println("모든 문서가 동기화되었습니다.")
			}
		case 8:
			// 특정 피어와 동기화
			fmt.Print("피어 ID: ")
			var targetPeer string
			fmt.Scanln(&targetPeer)
			targetPeer = strings.TrimSpace(targetPeer)

			if targetPeer == "" {
				fmt.Println("피어 ID가 비어 있습니다.")
				continue
			}

			if err := doc.Sync(ctx, targetPeer); err != nil {
				fmt.Printf("피어 %s와 동기화 실패: %v\n", targetPeer, err)
			} else {
				fmt.Printf("문서가 피어 %s와 동기화되었습니다.\n", targetPeer)
			}
		case 9:
			return
		default:
			fmt.Println("잘못된 선택입니다.")
		}
	}
}

// updateDocument는 문서를 업데이트합니다.
func updateDocument(ctx context.Context, doc *crdtstorage.Document, field string, value interface{}) {
	// 필드 업데이트
	result := doc.Edit(ctx, func(api *api.ModelApi) error {
		// 현재 내용 가져오기
		currentContent, err := api.View()
		if err != nil {
			return fmt.Errorf("failed to get current content: %w", err)
		}

		// 맵으로 변환
		contentMap, ok := currentContent.(map[string]interface{})
		if !ok {
			return fmt.Errorf("content is not a map")
		}

		// 필드 업데이트
		contentMap[field] = value
		contentMap["modified"] = time.Now().Format(time.RFC3339)

		// 루트 설정
		api.Root(contentMap)
		return nil
	})

	if !result.Success {
		fmt.Printf("문서 업데이트 실패: %v\n", result.Error)
		return
	}

	fmt.Printf("%s 필드가 업데이트되었습니다.\n", field)
}
