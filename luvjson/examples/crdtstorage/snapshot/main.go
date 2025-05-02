// 이 예제는 스냅샷 기능을 사용하는 방법을 보여줍니다.
// 실제로 실행하려면 go-sqlite3 패키지를 설치해야 합니다.
// go get github.com/mattn/go-sqlite3

package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
	"tictactoe/luvjson/crdtpatch"
	"tictactoe/luvjson/crdtstorage"
)

// 문서 타입 정의
type ExampleDocument struct {
	Title    string    `json:"title"`
	Content  string    `json:"content"`
	Authors  []string  `json:"authors"`
	Modified time.Time `json:"modified"`
	Version  int       `json:"version"`
}

// RunSnapshotExample은 스냅샷 기능을 사용하는 예제를 실행합니다.
// 이 함수는 실제로 실행되지 않으며, 코드 예제로만 제공됩니다.
func RunSnapshotExample() {
	// 컨텍스트 생성
	ctx := context.Background()

	// SQLite 데이터베이스 연결
	// 실제로 실행하려면 go-sqlite3 패키지를 설치해야 합니다.
	// db, err := sql.Open("sqlite3", ":memory:")
	var db *sql.DB
	var err error

	// 고급 SQL 어댑터 생성
	adapter, err := crdtstorage.NewAdvancedSQLAdapter(db, "documents", "document_snapshots")
	if err != nil {
		log.Fatalf("Failed to create SQL adapter: %v", err)
	}

	// 스냅샷 옵션 설정
	snapshotOptions := &crdtstorage.SnapshotOptions{
		Enabled:       true,
		Interval:      time.Hour,
		MaxSnapshots:  5,
		SnapshotOnSave: true,
	}
	adapter.SetSnapshotOptions(snapshotOptions)

	// 저장소 옵션 생성
	options := crdtstorage.DefaultStorageOptions()
	options.PersistenceType = "custom"
	options.EnableSnapshots = true

	// 저장소 생성
	// 실제로는 AdvancedSQLAdapter를 PersistenceAdapter로 변환해야 합니다.
	// 이 예제에서는 코드 설명을 위해 타입 변환을 생략합니다.
	var persistenceAdapter crdtstorage.PersistenceAdapter
	storage, err := crdtstorage.NewStorageWithCustomPersistence(ctx, options, persistenceAdapter)
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// 문서 생성
	doc, err := storage.CreateDocument(ctx, "example-doc")
	if err != nil {
		log.Fatalf("Failed to create document: %v", err)
	}

	// 초기 문서 내용 설정
	result := doc.Edit(ctx, func(crdtDoc *crdt.Document, patchBuilder *crdtpatch.PatchBuilder) error {
		// 루트 노드 생성
		rootID := crdtDoc.NextTimestamp()
		rootOp := &crdtpatch.NewOperation{
			ID:       rootID,
			NodeType: common.NodeTypeCon,
			Value: map[string]interface{}{
				"title":    "Example Document",
				"content":  "Initial content",
				"authors":  []string{"user1"},
				"modified": time.Now().Format(time.RFC3339),
				"version":  1,
			},
		}

		// 패치 생성
		patchBuilder.AddOperation(rootOp)

		// 루트 설정 작업 추가
		rootSetOp := &crdtpatch.InsOperation{
			ID:       crdtDoc.NextTimestamp(),
			TargetID: common.RootID,
			Value:    rootID,
		}
		patchBuilder.AddOperation(rootSetOp)

		return nil
	})
	if !result.Success {
		log.Fatalf("Failed to initialize document: %v", result.Error)
	}

	// 문서 저장 (첫 번째 스냅샷 생성)
	if err := doc.Save(ctx); err != nil {
		log.Fatalf("Failed to save document: %v", err)
	}
	fmt.Println("Initial document saved with version 1")

	// 문서 편집
	result = doc.Edit(ctx, func(crdtDoc *crdt.Document, patchBuilder *crdtpatch.PatchBuilder) error {
		// 현재 내용 가져오기
		view, err := crdtDoc.View()
		if err != nil {
			return err
		}

		currentContent, ok := view.(map[string]interface{})
		if !ok {
			return fmt.Errorf("root node value is not a map")
		}

		// 내용 수정
		updatedContent := make(map[string]interface{})
		for k, v := range currentContent {
			updatedContent[k] = v
		}
		updatedContent["title"] = "Updated Title"
		updatedContent["content"] = "Updated content"
		updatedContent["modified"] = time.Now().Format(time.RFC3339)
		updatedContent["version"] = 2

		// 새 루트 노드 생성
		newRootID := crdtDoc.NextTimestamp()
		newRootOp := &crdtpatch.NewOperation{
			ID:       newRootID,
			NodeType: common.NodeTypeCon,
			Value:    updatedContent,
		}

		// 패치 생성
		patchBuilder.AddOperation(newRootOp)

		// 루트 설정 작업 추가
		rootSetOp := &crdtpatch.InsOperation{
			ID:       crdtDoc.NextTimestamp(),
			TargetID: common.RootID,
			Value:    newRootID,
		}
		patchBuilder.AddOperation(rootSetOp)

		return nil
	})
	if !result.Success {
		log.Fatalf("Failed to edit document: %v", result.Error)
	}

	// 문서 저장 (두 번째 스냅샷 생성)
	if err := doc.Save(ctx); err != nil {
		log.Fatalf("Failed to save document: %v", err)
	}
	fmt.Println("Document updated with version 2")

	// 스냅샷 목록 가져오기
	snapshots, err := doc.ListSnapshots(ctx)
	if err != nil {
		log.Fatalf("Failed to list snapshots: %v", err)
	}
	fmt.Printf("Available snapshots: %v\n", snapshots)

	// 첫 번째 스냅샷 로드
	if len(snapshots) > 0 {
		snapshot, err := doc.LoadSnapshot(ctx, snapshots[0])
		if err != nil {
			log.Fatalf("Failed to load snapshot: %v", err)
		}
		fmt.Printf("Loaded snapshot: %+v\n", snapshot)

		// 스냅샷 데이터 확인
		data, ok := snapshot.Data.(map[string]interface{})
		if ok {
			fmt.Printf("Snapshot data: title=%v, version=%v\n", data["title"], data["version"])
		}
	}

	// 문서 내용 확인
	var content ExampleDocument
	if err := doc.GetContentAs(&content); err != nil {
		log.Fatalf("Failed to get document content: %v", err)
	}
	fmt.Printf("Current document content: %+v\n", content)

	// 첫 번째 스냅샷으로 복원
	if len(snapshots) > 0 {
		if err := doc.RestoreFromSnapshot(ctx, snapshots[0]); err != nil {
			log.Fatalf("Failed to restore from snapshot: %v", err)
		}
		fmt.Println("Document restored from snapshot")

		// 복원된 문서 내용 확인
		var restoredContent ExampleDocument
		if err := doc.GetContentAs(&restoredContent); err != nil {
			log.Fatalf("Failed to get restored document content: %v", err)
		}
		fmt.Printf("Restored document content: %+v\n", restoredContent)
	}
}

// 이 예제는 실제로 실행되지 않으며, 코드 예제로만 제공됩니다.
func main() {
	fmt.Println("이 예제는 실제로 실행되지 않으며, 코드 예제로만 제공됩니다.")
	fmt.Println("스냅샷 기능을 사용하려면 다음 단계를 따르세요:")
	fmt.Println("1. SQL 데이터베이스 연결 설정")
	fmt.Println("2. AdvancedSQLAdapter 생성")
	fmt.Println("3. 스냅샷 옵션 설정")
	fmt.Println("4. 저장소 생성")
	fmt.Println("5. 문서 생성 및 편집")
	fmt.Println("6. 스냅샷 생성 및 관리")
}
