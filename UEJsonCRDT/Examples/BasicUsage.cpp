// Copyright Your Company. All Rights Reserved.

#include "JsonCRDTSyncManager.h"
#include "JsonCRDTDocument.h"
#include "JsonCRDTBlueprintLibrary.h"

// 기본 사용 예제
void BasicUsageExample()
{
    // 동기화 관리자 생성
    UJsonCRDTSyncManager* SyncManager = UJsonCRDTBlueprintLibrary::CreateSyncManager(nullptr, "http://localhost:8080/api", "ws://localhost:8080/ws");

    // 네트워크 상태 변경 이벤트 핸들러 등록
    SyncManager->OnNetworkStatusChanged.AddLambda([](bool bIsOnline, const FString& StatusMessage) {
        if (bIsOnline)
        {
            UE_LOG(LogTemp, Log, TEXT("Online: %s"), *StatusMessage);
        }
        else
        {
            UE_LOG(LogTemp, Warning, TEXT("Offline: %s"), *StatusMessage);
        }
    });

    // 서버에 연결
    SyncManager->Connect();

    // 문서 생성
    UJsonCRDTDocument* Document = UJsonCRDTBlueprintLibrary::CreateDocument(nullptr, SyncManager, "example-doc");

    // 자동 로컬 저장 활성화
    Document->SetAutoLocalSave(true);

    // 문서 내용 설정
    Document->SetContentFromString("{\"title\":\"Example Document\",\"content\":\"Hello, World!\",\"tags\":[\"example\",\"crdt\"]}");

    // 문서 저장 (로컬 및 서버)
    Document->Save();

    // 문서 내용 가져오기
    FString Content = Document->GetContentAsString();
    UE_LOG(LogTemp, Log, TEXT("Document content: %s"), *Content);

    // 문서 편집
    TSharedPtr<FJsonObject> JsonObject = Document->GetContent();
    if (JsonObject.IsValid())
    {
        // 내용 수정
        JsonObject->SetStringField("content", "Updated content");

        // 태그 추가
        TArray<TSharedPtr<FJsonValue>>* Tags;
        if (JsonObject->TryGetArrayField("tags", Tags))
        {
            Tags->Add(MakeShared<FJsonValueString>("updated"));
        }

        // 수정된 내용 설정
        Document->SetContent(JsonObject);
    }

    // 문서 저장 (로컬 및 서버)
    Document->Save();

    // 문서 동기화
    Document->Sync();

    // 로컬 저장 확인
    UE_LOG(LogTemp, Log, TEXT("Document saved locally: %s"), Document->IsAutoLocalSaveEnabled() ? TEXT("Yes") : TEXT("No"));

    // 오류 발생 시 복구 가능 여부 확인
    UE_LOG(LogTemp, Log, TEXT("Document can be recovered: %s"), Document->RecoverDocument() ? TEXT("Yes") : TEXT("No"));
}

// 패치 적용 예제
void ApplyPatchExample()
{
    // 동기화 관리자 생성
    UJsonCRDTSyncManager* SyncManager = UJsonCRDTBlueprintLibrary::CreateSyncManager(nullptr, "http://localhost:8080/api", "ws://localhost:8080/ws");

    // 문서 생성
    UJsonCRDTDocument* Document = UJsonCRDTBlueprintLibrary::CreateDocument(nullptr, SyncManager, "example-doc");
    Document->SetContentFromString("{\"title\":\"Example Document\",\"content\":\"Hello, World!\",\"tags\":[\"example\",\"crdt\"]}");

    // 패치 생성
    TArray<FJsonCRDTOperation> Operations;

    // 내용 수정 작업
    FJsonCRDTOperation ReplaceOperation = UJsonCRDTBlueprintLibrary::CreateOperation(
        EJsonCRDTOperationType::Replace,
        "/content",
        "\"Updated via patch\""
    );
    Operations.Add(ReplaceOperation);

    // 태그 추가 작업
    FJsonCRDTOperation AddOperation = UJsonCRDTBlueprintLibrary::CreateOperation(
        EJsonCRDTOperationType::Add,
        "/tags/-",
        "\"patched\""
    );
    Operations.Add(AddOperation);

    // 패치 생성
    FJsonCRDTPatch Patch = UJsonCRDTBlueprintLibrary::CreatePatch(
        Document->GetDocumentID(),
        Document->GetVersion(),
        Operations
    );

    // 패치 적용
    Document->ApplyPatch(Patch);

    // 문서 내용 가져오기
    FString Content = Document->GetContentAsString();
    UE_LOG(LogTemp, Log, TEXT("Document content after patch: %s"), *Content);
}

// 스냅샷 관리 예제
void SnapshotExample()
{
    // 동기화 관리자 생성
    UJsonCRDTSyncManager* SyncManager = UJsonCRDTBlueprintLibrary::CreateSyncManager(nullptr, "http://localhost:8080/api", "ws://localhost:8080/ws");

    // 문서 생성
    UJsonCRDTDocument* Document = UJsonCRDTBlueprintLibrary::CreateDocument(nullptr, SyncManager, "example-doc");
    Document->SetContentFromString("{\"title\":\"Example Document\",\"content\":\"Initial content\",\"version\":1}");

    // 초기 스냅샷 생성
    FJsonCRDTSnapshot InitialSnapshot = Document->CreateSnapshot();

    // 문서 편집
    TSharedPtr<FJsonObject> JsonObject = Document->GetContent();
    if (JsonObject.IsValid())
    {
        JsonObject->SetStringField("content", "Updated content");
        JsonObject->SetNumberField("version", 2);
        Document->SetContent(JsonObject);
    }

    // 두 번째 스냅샷 생성
    FJsonCRDTSnapshot SecondSnapshot = Document->CreateSnapshot();

    // 문서 편집
    JsonObject = Document->GetContent();
    if (JsonObject.IsValid())
    {
        JsonObject->SetStringField("content", "Final content");
        JsonObject->SetNumberField("version", 3);
        Document->SetContent(JsonObject);
    }

    // 현재 문서 내용 출력
    FString CurrentContent = Document->GetContentAsString();
    UE_LOG(LogTemp, Log, TEXT("Current document content: %s"), *CurrentContent);

    // 초기 스냅샷으로 복원
    Document->RestoreFromSnapshot(InitialSnapshot);

    // 복원된 문서 내용 출력
    FString RestoredContent = Document->GetContentAsString();
    UE_LOG(LogTemp, Log, TEXT("Restored document content: %s"), *RestoredContent);
}
