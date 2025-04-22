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

	"tictactoe/luvjson/api"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtstorage"
)

// 사용자 정의 키 타입
type CustomKey struct {
	Collection string
	ID         string
	Version    int
}

// 사용자 정의 키 생성 함수
func CustomDocumentKeyFunc(documentID string) crdtstorage.Key {
	return &CustomKey{
		Collection: "documents",
		ID:         documentID,
		Version:    1,
	}
}

// 사용자 정의 영구 저장소
type CustomPersistence struct {
	// 내부적으로 메모리 저장소 사용
	memory *crdtstorage.MemoryPersistence
}

// NewCustomPersistence는 새 사용자 정의 영구 저장소를 생성합니다.
func NewCustomPersistence() *CustomPersistence {
	return &CustomPersistence{
		memory: crdtstorage.NewMemoryPersistence(),
	}
}

// GetDocumentKeyFunc는 문서 키 생성 함수를 반환합니다.
func (p *CustomPersistence) GetDocumentKeyFunc() crdtstorage.DocumentKeyFunc {
	return CustomDocumentKeyFunc
}

// SaveDocument는 문서를 저장합니다.
func (p *CustomPersistence) SaveDocument(ctx context.Context, doc *crdtstorage.Document) error {
	fmt.Printf("사용자 정의 저장소: 문서 %s 저장\n", doc.ID)
	return p.memory.SaveDocument(ctx, doc)
}

// LoadDocument는 문서를 로드합니다.
func (p *CustomPersistence) LoadDocument(ctx context.Context, key crdtstorage.Key) ([]byte, error) {
	// 사용자 정의 키 처리
	var documentID string
	switch k := key.(type) {
	case *CustomKey:
		fmt.Printf("사용자 정의 키 사용: 컬렉션=%s, ID=%s, 버전=%d\n", k.Collection, k.ID, k.Version)
		documentID = k.ID
	default:
		// 다른 키 타입은 메모리 저장소에 위임
		return p.memory.LoadDocument(ctx, key)
	}

	// 메모리 저장소에서 로드
	return p.memory.LoadDocumentByID(ctx, documentID)
}

// LoadDocumentByID는 문서 ID로 문서를 로드합니다.
func (p *CustomPersistence) LoadDocumentByID(ctx context.Context, documentID string) ([]byte, error) {
	// 사용자 정의 키 생성
	key := CustomDocumentKeyFunc(documentID)
	return p.LoadDocument(ctx, key)
}

// QueryDocuments는 쿼리에 맞는 문서를 검색합니다.
func (p *CustomPersistence) QueryDocuments(ctx context.Context, query interface{}) ([]string, error) {
	return p.memory.QueryDocuments(ctx, query)
}

// ListDocuments는 모든 문서 목록을 반환합니다.
func (p *CustomPersistence) ListDocuments(ctx context.Context) ([]string, error) {
	return p.memory.ListDocuments(ctx)
}

// DeleteDocument는 문서를 삭제합니다.
func (p *CustomPersistence) DeleteDocument(ctx context.Context, key crdtstorage.Key) error {
	// 사용자 정의 키 처리
	var documentID string
	switch k := key.(type) {
	case *CustomKey:
		fmt.Printf("사용자 정의 키로 삭제: 컬렉션=%s, ID=%s, 버전=%d\n", k.Collection, k.ID, k.Version)
		documentID = k.ID
	default:
		// 다른 키 타입은 메모리 저장소에 위임
		return p.memory.DeleteDocument(ctx, key)
	}

	// 메모리 저장소에서 삭제
	return p.memory.DeleteDocumentByID(ctx, documentID)
}

// DeleteDocumentByID는 문서 ID로 문서를 삭제합니다.
func (p *CustomPersistence) DeleteDocumentByID(ctx context.Context, documentID string) error {
	// 사용자 정의 키 생성
	key := CustomDocumentKeyFunc(documentID)
	return p.DeleteDocument(ctx, key)
}

// Close는 영구 저장소를 닫습니다.
func (p *CustomPersistence) Close() error {
	return p.memory.Close()
}

func main() {
	// 명령줄 인수 파싱
	var (
		documentID = flag.String("doc", "custom-key-example", "문서 ID")
		nodeID     = flag.String("node", "", "노드 ID (비워두면 자동 생성)")
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
	options.PubSubType = "memory"
	options.PersistenceType = "custom" // 사용자 정의 저장소 사용
	options.KeyPrefix = fmt.Sprintf("custom-%s", peerID)
	options.AutoSave = true
	options.AutoSaveInterval = time.Second * 10

	// 사용자 정의 영구 저장소 생성
	customPersistence := NewCustomPersistence()

	// 저장소 생성 (사용자 정의 영구 저장소 등록)
	storage, err := crdtstorage.NewStorageWithCustomPersistence(ctx, options, customPersistence)
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
				"title":    "사용자 정의 키 예제",
				"content":  "이 문서는 사용자 정의 키를 사용하는 예제입니다.",
				"created":  time.Now().Format(time.RFC3339),
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
	go handleUserInputs(ctx, doc)

	// 종료 신호 대기
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("종료 중...")
}

// printDocument는 문서 내용을 출력합니다.
func printDocument(doc *crdtstorage.Document) {
	// 문서 내용 가져오기
	content, err := doc.GetContent()
	if err != nil {
		fmt.Printf("문서 내용 가져오기 실패: %v\n", err)
		return
	}

	// 내용 출력
	contentMap, ok := content.(map[string]interface{})
	if !ok {
		fmt.Println("문서 내용이 맵 형식이 아닙니다.")
		return
	}

	fmt.Println("\n=== 문서 내용 ===")
	fmt.Printf("ID: %s\n", doc.ID)
	fmt.Printf("제목: %s\n", contentMap["title"])
	fmt.Printf("내용: %s\n", contentMap["content"])
	fmt.Printf("생성일: %s\n", contentMap["created"])
	fmt.Printf("수정일: %s\n", contentMap["modified"])
	fmt.Printf("마지막 수정: %s\n", doc.LastModified.Format(time.RFC3339))
	fmt.Println("================")
}

// handleUserInputs는 사용자 입력을 처리합니다.
func handleUserInputs(ctx context.Context, doc *crdtstorage.Document) {
	for {
		fmt.Println("\n명령어:")
		fmt.Println("1. 제목 변경")
		fmt.Println("2. 내용 변경")
		fmt.Println("3. 문서 보기")
		fmt.Println("4. 문서 저장")
		fmt.Println("5. 종료")
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
			printDocument(doc)
		case 4:
			if err := doc.Save(ctx); err != nil {
				fmt.Printf("문서 저장 실패: %v\n", err)
			} else {
				fmt.Println("문서가 저장되었습니다.")
			}
		case 5:
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
		content, err := api.View()
		if err != nil {
			return fmt.Errorf("failed to get current content: %w", err)
		}

		// 맵으로 변환
		contentMap, ok := content.(map[string]interface{})
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
