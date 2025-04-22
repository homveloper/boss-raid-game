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

// 사용자 정보 구조체
type User struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Email     string            `json:"email"`
	Roles     []string          `json:"roles"`
	Metadata  map[string]string `json:"metadata"`
	LastLogin time.Time         `json:"lastLogin"`
}

func main() {
	// 명령줄 인수 파싱
	var (
		storageType = flag.String("storage", "redis", "저장소 유형 (memory, file, redis)")
		redisAddr   = flag.String("redis", "localhost:6379", "Redis 서버 주소")
		filePath    = flag.String("path", "documents", "파일 저장소 경로")
		documentID  = flag.String("doc", "users-db", "문서 ID")
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
	options.KeyPrefix = fmt.Sprintf("edit-options-%s", peerID)
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
		result := doc.EditWithOptions(ctx, func(api *api.ModelApi) error {
			api.Root(map[string]interface{}{
				"users": []interface{}{},
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
		printUsers(d)
	})

	// 문서 내용 출력
	printUsers(doc)

	// 사용자 입력 처리
	go handleUserInputs(ctx, doc)

	// 종료 신호 대기
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("종료 중...")
}

// printUsers는 문서의 사용자 목록을 출력합니다.
func printUsers(doc *crdtstorage.Document) {
	// 문서 내용 가져오기
	content, err := doc.GetContent()
	if err != nil {
		fmt.Printf("문서 내용 가져오기 실패: %v\n", err)
		return
	}

	// 사용자 목록 가져오기
	contentMap, ok := content.(map[string]interface{})
	if !ok {
		fmt.Println("문서 내용이 맵 형식이 아닙니다.")
		return
	}

	users, ok := contentMap["users"].([]interface{})
	if !ok {
		fmt.Println("사용자 목록이 배열 형식이 아닙니다.")
		return
	}

	fmt.Printf("\n=== 사용자 목록 (%d명) ===\n", len(users))
	for i, user := range users {
		userMap, ok := user.(map[string]interface{})
		if !ok {
			continue
		}

		fmt.Printf("%d. %s (%s)\n", i+1, userMap["name"], userMap["id"])
		fmt.Printf("   이메일: %s\n", userMap["email"])
		fmt.Printf("   역할: %v\n", userMap["roles"])
		fmt.Printf("   마지막 로그인: %v\n", userMap["lastLogin"])
		fmt.Println()
	}
	fmt.Println("==========================")
}

// handleUserInputs는 사용자 입력을 처리합니다.
func handleUserInputs(ctx context.Context, doc *crdtstorage.Document) {
	for {
		fmt.Println("\n명령어:")
		fmt.Println("1. 사용자 추가 (기본 옵션)")
		fmt.Println("2. 사용자 추가 (분산 락 사용)")
		fmt.Println("3. 사용자 추가 (재시도 횟수 증가)")
		fmt.Println("4. 사용자 수정")
		fmt.Println("5. 사용자 삭제")
		fmt.Println("6. 사용자 목록 보기")
		fmt.Println("7. 동시 편집 시뮬레이션")
		fmt.Println("8. 종료")
		fmt.Print("> ")

		var choice int
		fmt.Scanln(&choice)

		switch choice {
		case 1:
			addUser(ctx, doc, false, 3)
		case 2:
			addUser(ctx, doc, true, 3)
		case 3:
			addUser(ctx, doc, false, 10)
		case 4:
			updateUser(ctx, doc)
		case 5:
			deleteUser(ctx, doc)
		case 6:
			printUsers(doc)
		case 7:
			simulateConcurrentEdits(ctx, doc)
		case 8:
			return
		default:
			fmt.Println("잘못된 선택입니다.")
		}
	}
}

// addUser는 새 사용자를 추가합니다.
func addUser(ctx context.Context, doc *crdtstorage.Document, useDistributedLock bool, maxRetries int) {
	// 사용자 정보 입력
	var user User
	user.ID = fmt.Sprintf("user-%d", time.Now().UnixNano())

	fmt.Print("사용자 이름: ")
	fmt.Scanln(&user.Name)

	fmt.Print("이메일: ")
	fmt.Scanln(&user.Email)

	fmt.Print("역할 (쉼표로 구분): ")
	var rolesStr string
	fmt.Scanln(&rolesStr)
	if rolesStr != "" {
		user.Roles = []string{rolesStr}
	} else {
		user.Roles = []string{"user"}
	}

	user.Metadata = map[string]string{
		"createdBy": "admin",
		"source":    "example",
	}
	user.LastLogin = time.Now()

	// 옵션 설정
	var opts []crdtstorage.EditOption
	
	// 분산 락 사용 여부
	opts = append(opts, crdtstorage.WithDistributedLock(useDistributedLock))
	
	// 최대 재시도 횟수
	opts = append(opts, crdtstorage.WithMaxRetries(maxRetries))
	
	// 재시도 지연 시간
	opts = append(opts, crdtstorage.WithRetryDelay(200*time.Millisecond))
	
	// 지수 백오프 사용
	opts = append(opts, crdtstorage.WithExponentialBackoff(true))

	// 사용자 추가
	startTime := time.Now()
	result := doc.EditWithOptions(ctx, func(api *api.ModelApi) error {
		// 현재 내용 가져오기
		content, err := api.View()
		if err != nil {
			return fmt.Errorf("failed to get content: %w", err)
		}

		// 사용자 목록 가져오기
		contentMap := content.(map[string]interface{})
		users := contentMap["users"].([]interface{})

		// 새 사용자 추가
		users = append(users, user)
		contentMap["users"] = users

		// 루트 설정
		api.Root(contentMap)
		return nil
	}, opts...)

	duration := time.Since(startTime)
	
	if !result.Success {
		fmt.Printf("사용자 추가 실패: %v\n", result.Error)
		return
	}
	
	fmt.Printf("사용자가 추가되었습니다. (소요 시간: %v)\n", duration)
	if useDistributedLock {
		fmt.Println("분산 락을 사용하여 추가되었습니다.")
	} else {
		fmt.Println("낙관적 동시성 제어를 사용하여 추가되었습니다.")
	}
}

// updateUser는 기존 사용자를 수정합니다.
func updateUser(ctx context.Context, doc *crdtstorage.Document) {
	// 문서 내용 가져오기
	content, err := doc.GetContent()
	if err != nil {
		fmt.Printf("문서 내용 가져오기 실패: %v\n", err)
		return
	}

	// 사용자 목록 가져오기
	contentMap := content.(map[string]interface{})
	users := contentMap["users"].([]interface{})

	if len(users) == 0 {
		fmt.Println("수정할 사용자가 없습니다.")
		return
	}

	// 사용자 선택
	fmt.Print("수정할 사용자 번호: ")
	var index int
	fmt.Scanln(&index)
	index-- // 0-based 인덱스로 변환

	if index < 0 || index >= len(users) {
		fmt.Println("잘못된 사용자 번호입니다.")
		return
	}

	// 수정할 필드 선택
	fmt.Println("수정할 필드:")
	fmt.Println("1. 이름")
	fmt.Println("2. 이메일")
	fmt.Println("3. 역할")
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
		fieldName = "email"
		fmt.Print("새 이메일: ")
		var value string
		fmt.Scanln(&value)
		fieldValue = value
	case 3:
		fieldName = "roles"
		fmt.Print("새 역할 (쉼표로 구분): ")
		var value string
		fmt.Scanln(&value)
		if value != "" {
			fieldValue = []string{value}
		} else {
			fieldValue = []string{"user"}
		}
	default:
		fmt.Println("잘못된 선택입니다.")
		return
	}

	// 사용자 수정
	result := doc.EditWithOptions(ctx, func(api *api.ModelApi) error {
		// 현재 내용 가져오기
		content, err := api.View()
		if err != nil {
			return fmt.Errorf("failed to get content: %w", err)
		}

		// 사용자 목록 가져오기
		contentMap := content.(map[string]interface{})
		users := contentMap["users"].([]interface{})

		// 사용자 수정
		userMap := users[index].(map[string]interface{})
		userMap[fieldName] = fieldValue
		userMap["lastLogin"] = time.Now()

		// 루트 설정
		api.Root(contentMap)
		return nil
	}, crdtstorage.WithMaxRetries(5))

	if !result.Success {
		fmt.Printf("사용자 수정 실패: %v\n", result.Error)
		return
	}
	fmt.Println("사용자가 수정되었습니다.")
}

// deleteUser는 사용자를 삭제합니다.
func deleteUser(ctx context.Context, doc *crdtstorage.Document) {
	// 문서 내용 가져오기
	content, err := doc.GetContent()
	if err != nil {
		fmt.Printf("문서 내용 가져오기 실패: %v\n", err)
		return
	}

	// 사용자 목록 가져오기
	contentMap := content.(map[string]interface{})
	users := contentMap["users"].([]interface{})

	if len(users) == 0 {
		fmt.Println("삭제할 사용자가 없습니다.")
		return
	}

	// 사용자 선택
	fmt.Print("삭제할 사용자 번호: ")
	var index int
	fmt.Scanln(&index)
	index-- // 0-based 인덱스로 변환

	if index < 0 || index >= len(users) {
		fmt.Println("잘못된 사용자 번호입니다.")
		return
	}

	// 사용자 삭제
	result := doc.EditWithOptions(ctx, func(api *api.ModelApi) error {
		// 현재 내용 가져오기
		content, err := api.View()
		if err != nil {
			return fmt.Errorf("failed to get content: %w", err)
		}

		// 사용자 목록 가져오기
		contentMap := content.(map[string]interface{})
		users := contentMap["users"].([]interface{})

		// 사용자 삭제
		newUsers := make([]interface{}, 0, len(users)-1)
		for i, user := range users {
			if i != index {
				newUsers = append(newUsers, user)
			}
		}
		contentMap["users"] = newUsers

		// 루트 설정
		api.Root(contentMap)
		return nil
	}, crdtstorage.WithDistributedLock(true))

	if !result.Success {
		fmt.Printf("사용자 삭제 실패: %v\n", result.Error)
		return
	}
	fmt.Println("사용자가 삭제되었습니다.")
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

			// 새 사용자 생성
			user := User{
				ID:        fmt.Sprintf("user-%d-%d", time.Now().UnixNano(), index),
				Name:      fmt.Sprintf("동시 사용자 %d", index),
				Email:     fmt.Sprintf("user%d@example.com", index),
				Roles:     []string{"user", "tester"},
				Metadata:  map[string]string{"source": "simulation"},
				LastLogin: time.Now(),
			}

			// 약간의 지연 추가 (충돌 가능성 증가)
			time.Sleep(time.Millisecond * time.Duration(50+index*10))

			// 옵션 설정
			var opts []crdtstorage.EditOption
			
			// 홀수 인덱스는 분산 락 사용, 짝수 인덱스는 낙관적 동시성 제어 사용
			useDistributedLock := index%2 == 1
			opts = append(opts, crdtstorage.WithDistributedLock(useDistributedLock))
			
			// 최대 재시도 횟수
			opts = append(opts, crdtstorage.WithMaxRetries(5))

			// 사용자 추가
			startTime := time.Now()
			result := doc.EditWithOptions(ctx, func(api *api.ModelApi) error {
				// 현재 내용 가져오기
				content, err := api.View()
				if err != nil {
					return fmt.Errorf("failed to get content: %w", err)
				}

				// 사용자 목록 가져오기
				contentMap := content.(map[string]interface{})
				users := contentMap["users"].([]interface{})

				// 새 사용자 추가
				users = append(users, user)
				contentMap["users"] = users

				// 루트 설정
				api.Root(contentMap)
				return nil
			}, opts...)

			duration := time.Since(startTime)

			if !result.Success {
				fmt.Printf("동시 편집 %d 실패: %v\n", index, result.Error)
			} else {
				lockType := "낙관적 동시성 제어"
				if useDistributedLock {
					lockType = "분산 락"
				}
				fmt.Printf("동시 편집 %d 성공 (%s, 소요 시간: %v)\n", index, lockType, duration)
			}
		}(i)
	}

	wg.Wait()
	fmt.Println("동시 편집 시뮬레이션 완료!")
}
