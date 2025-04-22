package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"tictactoe/luvjson/api"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtstorage"
)

// 게임 아이템 구조체
type GameItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Rarity      string   `json:"rarity"`
	Attributes  []string `json:"attributes"`
	Owner       string   `json:"owner"`
	LastUpdated string   `json:"lastUpdated"`
}

func main() {
	// 명령줄 인수 파싱
	var (
		storageType = flag.String("storage", "redis", "저장소 유형 (memory, file, redis)")
		redisAddr   = flag.String("redis", "localhost:6379", "Redis 서버 주소")
		filePath    = flag.String("path", "documents", "파일 저장소 경로")
		documentID  = flag.String("doc", "game-items", "문서 ID")
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
	options.KeyPrefix = fmt.Sprintf("edit-%s", peerID)
	options.EnableDistributedLock = true
	options.EnableTransactionTracking = true

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
				"items": []interface{}{},
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
		metadata := patch.Metadata()
		if metadata != nil {
			if txID, ok := metadata["transactionId"].(string); ok && txID != "" {
				fmt.Printf("트랜잭션 ID: %s\n", txID)
			}
			if retryCount, ok := metadata["retryCount"].(int); ok {
				fmt.Printf("재시도 횟수: %d\n", retryCount)
			}
		}
		printItems(d)
	})

	// 문서 내용 출력
	printItems(doc)

	// 사용자 입력 처리
	go handleUserInputs(ctx, doc)

	// 종료 신호 대기
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("종료 중...")
}

// printItems는 문서의 아이템 목록을 출력합니다.
func printItems(doc *crdtstorage.Document) {
	// 문서 내용 가져오기
	content, err := doc.GetContent()
	if err != nil {
		fmt.Printf("문서 내용 가져오기 실패: %v\n", err)
		return
	}

	// 아이템 목록 가져오기
	contentMap, ok := content.(map[string]interface{})
	if !ok {
		fmt.Println("문서 내용이 맵 형식이 아닙니다.")
		return
	}

	items, ok := contentMap["items"].([]interface{})
	if !ok {
		fmt.Println("아이템 목록이 배열 형식이 아닙니다.")
		return
	}

	fmt.Printf("\n=== 게임 아이템 목록 (%d개) ===\n", len(items))
	for i, item := range items {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		fmt.Printf("%d. %s (%s)\n", i+1, itemMap["name"], itemMap["rarity"])
		fmt.Printf("   소유자: %s\n", itemMap["owner"])
		fmt.Printf("   설명: %s\n", itemMap["description"])
		fmt.Printf("   속성: %v\n", itemMap["attributes"])
		fmt.Printf("   마지막 업데이트: %s\n", itemMap["lastUpdated"])
		fmt.Println()
	}
	fmt.Println("==========================")
}

// handleUserInputs는 사용자 입력을 처리합니다.
func handleUserInputs(ctx context.Context, doc *crdtstorage.Document) {
	for {
		fmt.Println("\n명령어:")
		fmt.Println("1. 아이템 추가 (낙관적 동시성 제어)")
		fmt.Println("2. 아이템 추가 (분산 트랜잭션)")
		fmt.Println("3. 아이템 수정")
		fmt.Println("4. 아이템 삭제")
		fmt.Println("5. 아이템 목록 보기")
		fmt.Println("6. 동시 편집 시뮬레이션")
		fmt.Println("7. 종료")
		fmt.Print("> ")

		var choice int
		fmt.Scanln(&choice)

		switch choice {
		case 1:
			addItem(ctx, doc, false)
		case 2:
			addItem(ctx, doc, true)
		case 3:
			updateItem(ctx, doc)
		case 4:
			deleteItem(ctx, doc)
		case 5:
			printItems(doc)
		case 6:
			simulateConcurrentEdits(ctx, doc)
		case 7:
			return
		default:
			fmt.Println("잘못된 선택입니다.")
		}
	}
}

// addItem은 새 아이템을 추가합니다.
func addItem(ctx context.Context, doc *crdtstorage.Document, useTransaction bool) {
	// 아이템 정보 입력
	var item GameItem
	item.ID = fmt.Sprintf("item-%d", time.Now().UnixNano())

	fmt.Print("아이템 이름: ")
	fmt.Scanln(&item.Name)

	fmt.Print("아이템 설명: ")
	var description string
	fmt.Scanln(&description)
	item.Description = description

	fmt.Print("희귀도 (common, rare, epic, legendary): ")
	fmt.Scanln(&item.Rarity)

	fmt.Print("속성 (쉼표로 구분): ")
	var attributesStr string
	fmt.Scanln(&attributesStr)
	if attributesStr != "" {
		item.Attributes = []string{attributesStr}
	} else {
		item.Attributes = []string{"default"}
	}

	fmt.Print("소유자: ")
	fmt.Scanln(&item.Owner)

	item.LastUpdated = time.Now().Format(time.RFC3339)

	// 편집 함수 정의
	editFunc := func(api *api.ModelApi) error {
		// 현재 내용 가져오기
		content, err := api.View()
		if err != nil {
			return fmt.Errorf("failed to get content: %w", err)
		}

		// 아이템 목록 가져오기
		contentMap := content.(map[string]interface{})
		items := contentMap["items"].([]interface{})

		// 새 아이템 추가
		items = append(items, item)
		contentMap["items"] = items

		// 루트 설정
		api.Root(contentMap)
		return nil
	}

	// 아이템 추가
	startTime := time.Now()
	var result interface{}

	if useTransaction {
		// 분산 트랜잭션 사용
		txResult := doc.EditTransaction(ctx, editFunc)
		result = txResult
		if !txResult.Success {
			fmt.Printf("아이템 추가 실패: %v\n", txResult.Error)
			return
		}
		fmt.Printf("아이템이 추가되었습니다. (분산 트랜잭션 사용, 소요 시간: %v)\n", time.Since(startTime))
	} else {
		// 낙관적 동시성 제어 사용
		editResult := doc.Edit(ctx, editFunc, 
			crdtstorage.WithMaxRetries(5),
			crdtstorage.WithRetryDelay(200*time.Millisecond),
			crdtstorage.WithExponentialBackoff(true))
		result = editResult
		if !editResult.Success {
			fmt.Printf("아이템 추가 실패: %v\n", editResult.Error)
			return
		}
		fmt.Printf("아이템이 추가되었습니다. (낙관적 동시성 제어 사용, 소요 시간: %v)\n", time.Since(startTime))
	}
}

// updateItem은 기존 아이템을 수정합니다.
func updateItem(ctx context.Context, doc *crdtstorage.Document) {
	// 문서 내용 가져오기
	content, err := doc.GetContent()
	if err != nil {
		fmt.Printf("문서 내용 가져오기 실패: %v\n", err)
		return
	}

	// 아이템 목록 가져오기
	contentMap := content.(map[string]interface{})
	items := contentMap["items"].([]interface{})

	if len(items) == 0 {
		fmt.Println("수정할 아이템이 없습니다.")
		return
	}

	// 아이템 선택
	fmt.Print("수정할 아이템 번호: ")
	var index int
	fmt.Scanln(&index)
	index-- // 0-based 인덱스로 변환

	if index < 0 || index >= len(items) {
		fmt.Println("잘못된 아이템 번호입니다.")
		return
	}

	// 수정할 필드 선택
	fmt.Println("수정할 필드:")
	fmt.Println("1. 이름")
	fmt.Println("2. 설명")
	fmt.Println("3. 희귀도")
	fmt.Println("4. 속성")
	fmt.Println("5. 소유자")
	fmt.Print("> ")

	var fieldChoice int
	fmt.Scanln(&fieldChoice)

	var fieldName string
	var fieldValue interface{}

	switch fieldChoice {
	case 1:
		fieldName = "name"
		fmt.Print("새 이름: ")
		var value string
		fmt.Scanln(&value)
		fieldValue = value
	case 2:
		fieldName = "description"
		fmt.Print("새 설명: ")
		var value string
		fmt.Scanln(&value)
		fieldValue = value
	case 3:
		fieldName = "rarity"
		fmt.Print("새 희귀도 (common, rare, epic, legendary): ")
		var value string
		fmt.Scanln(&value)
		fieldValue = value
	case 4:
		fieldName = "attributes"
		fmt.Print("새 속성 (쉼표로 구분): ")
		var value string
		fmt.Scanln(&value)
		if value != "" {
			fieldValue = []string{value}
		} else {
			fieldValue = []string{"default"}
		}
	case 5:
		fieldName = "owner"
		fmt.Print("새 소유자: ")
		var value string
		fmt.Scanln(&value)
		fieldValue = value
	default:
		fmt.Println("잘못된 선택입니다.")
		return
	}

	// 아이템 수정
	result := doc.Edit(ctx, func(api *api.ModelApi) error {
		// 현재 내용 가져오기
		content, err := api.View()
		if err != nil {
			return fmt.Errorf("failed to get content: %w", err)
		}

		// 아이템 목록 가져오기
		contentMap := content.(map[string]interface{})
		items := contentMap["items"].([]interface{})

		// 아이템 수정
		itemMap := items[index].(map[string]interface{})
		itemMap[fieldName] = fieldValue
		itemMap["lastUpdated"] = time.Now().Format(time.RFC3339)

		// 루트 설정
		api.Root(contentMap)
		return nil
	}, crdtstorage.WithMaxRetries(3))

	if !result.Success {
		fmt.Printf("아이템 수정 실패: %v\n", result.Error)
		return
	}
	fmt.Println("아이템이 수정되었습니다.")
}

// deleteItem은 아이템을 삭제합니다.
func deleteItem(ctx context.Context, doc *crdtstorage.Document) {
	// 문서 내용 가져오기
	content, err := doc.GetContent()
	if err != nil {
		fmt.Printf("문서 내용 가져오기 실패: %v\n", err)
		return
	}

	// 아이템 목록 가져오기
	contentMap := content.(map[string]interface{})
	items := contentMap["items"].([]interface{})

	if len(items) == 0 {
		fmt.Println("삭제할 아이템이 없습니다.")
		return
	}

	// 아이템 선택
	fmt.Print("삭제할 아이템 번호: ")
	var index int
	fmt.Scanln(&index)
	index-- // 0-based 인덱스로 변환

	if index < 0 || index >= len(items) {
		fmt.Println("잘못된 아이템 번호입니다.")
		return
	}

	// 아이템 삭제
	result := doc.Edit(ctx, func(api *api.ModelApi) error {
		// 현재 내용 가져오기
		content, err := api.View()
		if err != nil {
			return fmt.Errorf("failed to get content: %w", err)
		}

		// 아이템 목록 가져오기
		contentMap := content.(map[string]interface{})
		items := contentMap["items"].([]interface{})

		// 아이템 삭제
		newItems := make([]interface{}, 0, len(items)-1)
		for i, item := range items {
			if i != index {
				newItems = append(newItems, item)
			}
		}
		contentMap["items"] = newItems

		// 루트 설정
		api.Root(contentMap)
		return nil
	}, crdtstorage.WithDistributedLock(true))

	if !result.Success {
		fmt.Printf("아이템 삭제 실패: %v\n", result.Error)
		return
	}
	fmt.Println("아이템이 삭제되었습니다.")
}

// simulateConcurrentEdits는 동시 편집을 시뮬레이션합니다.
func simulateConcurrentEdits(ctx context.Context, doc *crdtstorage.Document) {
	fmt.Print("동시 편집 수: ")
	var count int
	fmt.Scanln(&count)
	if count <= 0 {
		count = 5
	}

	fmt.Printf("%d개의 동시 편집을 시뮬레이션합니다...\n", count)

	var wg sync.WaitGroup
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func(index int) {
			defer wg.Done()

			// 새 아이템 생성
			item := GameItem{
				ID:          fmt.Sprintf("item-%d-%d", time.Now().UnixNano(), index),
				Name:        fmt.Sprintf("동시 아이템 %d", index),
				Description: fmt.Sprintf("동시 편집 시뮬레이션으로 생성된 아이템 %d", index),
				Rarity:      []string{"common", "rare", "epic", "legendary"}[index%4],
				Attributes:  []string{"동시", "편집", "테스트"},
				Owner:       fmt.Sprintf("시뮬레이션-%d", index),
				LastUpdated: time.Now().Format(time.RFC3339),
			}

			// 약간의 지연 추가 (충돌 가능성 증가)
			time.Sleep(time.Millisecond * time.Duration(50+index*10))

			// 홀수 인덱스는 분산 트랜잭션 사용, 짝수 인덱스는 낙관적 동시성 제어 사용
			useTransaction := index%2 == 1

			// 편집 함수 정의
			editFunc := func(api *api.ModelApi) error {
				// 현재 내용 가져오기
				content, err := api.View()
				if err != nil {
					return fmt.Errorf("failed to get content: %w", err)
				}

				// 아이템 목록 가져오기
				contentMap := content.(map[string]interface{})
				items := contentMap["items"].([]interface{})

				// 새 아이템 추가
				items = append(items, item)
				contentMap["items"] = items

				// 루트 설정
				api.Root(contentMap)
				return nil
			}

			// 아이템 추가
			startTime := time.Now()
			if useTransaction {
				// 분산 트랜잭션 사용
				txResult := doc.EditTransaction(ctx, editFunc)
				if !txResult.Success {
					fmt.Printf("동시 편집 %d 실패: %v\n", index, txResult.Error)
				} else {
					fmt.Printf("동시 편집 %d 성공 (분산 트랜잭션, 소요 시간: %v)\n", index, time.Since(startTime))
				}
			} else {
				// 낙관적 동시성 제어 사용
				editResult := doc.Edit(ctx, editFunc, 
					crdtstorage.WithMaxRetries(5),
					crdtstorage.WithRetryDelay(100*time.Millisecond),
					crdtstorage.WithExponentialBackoff(true))
				if !editResult.Success {
					fmt.Printf("동시 편집 %d 실패: %v\n", index, editResult.Error)
				} else {
					fmt.Printf("동시 편집 %d 성공 (낙관적 동시성 제어, 소요 시간: %v)\n", index, time.Since(startTime))
				}
			}
		}(i)
	}

	wg.Wait()
	fmt.Println("동시 편집 시뮬레이션 완료!")
}
