// Copyright Your Company. All Rights Reserved.

#include "JsonCRDTSyncManager.h"
#include "JsonCRDTDocument.h"
#include "JsonCRDTBlueprintLibrary.h"

// 오프라인 지원 예제
void OfflineSupportExample()
{
    // 동기화 관리자 생성
    UJsonCRDTSyncManager* SyncManager = UJsonCRDTBlueprintLibrary::CreateSyncManager(nullptr, "http://localhost:8080/api", "ws://localhost:8080/ws");
    
    // 자동 재연결 설정
    SyncManager->SetAutoReconnect(true);
    SyncManager->SetMaxReconnectAttempts(5);
    SyncManager->SetReconnectDelay(3.0f);
    
    // 네트워크 상태 변경 이벤트 핸들러 등록
    SyncManager->OnNetworkStatusChanged.AddLambda([](bool bIsOnline, const FString& StatusMessage) {
        if (bIsOnline)
        {
            UE_LOG(LogTemp, Log, TEXT("온라인 상태로 변경: %s"), *StatusMessage);
        }
        else
        {
            UE_LOG(LogTemp, Warning, TEXT("오프라인 상태로 변경: %s"), *StatusMessage);
        }
    });
    
    // 서버에 연결
    SyncManager->Connect();
    
    // 문서 생성
    UJsonCRDTDocument* Document = UJsonCRDTBlueprintLibrary::CreateDocument(nullptr, SyncManager, "offline-example-doc");
    
    // 자동 로컬 저장 활성화
    Document->SetAutoLocalSave(true);
    
    // 문서 내용 설정
    Document->SetContentFromString("{\"title\":\"Offline Example\",\"content\":\"This document supports offline editing\",\"lastEdited\":\"2023-01-01\"}");
    
    // 문서 저장 (로컬 및 서버)
    Document->Save();
    
    // 오프라인 모드 시뮬레이션
    SyncManager->SetOfflineMode(true);
    
    // 오프라인 상태에서 문서 편집
    TSharedPtr<FJsonObject> JsonObject = Document->GetContent();
    if (JsonObject.IsValid())
    {
        JsonObject->SetStringField("content", "This document was edited while offline");
        JsonObject->SetStringField("lastEdited", FDateTime::UtcNow().ToString());
        Document->SetContent(JsonObject);
    }
    
    // 오프라인 상태에서 문서 저장 (로컬에만 저장됨)
    Document->Save();
    
    // 오프라인 모드 해제 및 서버와 동기화
    SyncManager->SetOfflineMode(false);
    Document->Sync();
}

// 오류 복구 예제
void ErrorRecoveryExample()
{
    // 동기화 관리자 생성
    UJsonCRDTSyncManager* SyncManager = UJsonCRDTBlueprintLibrary::CreateSyncManager(nullptr, "http://localhost:8080/api", "ws://localhost:8080/ws");
    SyncManager->Connect();
    
    // 문서 생성
    UJsonCRDTDocument* Document = UJsonCRDTBlueprintLibrary::CreateDocument(nullptr, SyncManager, "recovery-example-doc");
    
    // 문서 내용 설정
    Document->SetContentFromString("{\"title\":\"Recovery Example\",\"content\":\"This document demonstrates error recovery\",\"version\":1}");
    
    // 문서 저장 (로컬 및 서버)
    Document->Save();
    
    // 문서 복구 이벤트 핸들러 등록
    Document->OnDocumentRecovered.AddLambda([](const FString& DocumentID, const FString& RecoverySource) {
        UE_LOG(LogTemp, Log, TEXT("문서 %s가 %s에서 복구되었습니다"), *DocumentID, *RecoverySource);
    });
    
    // 오류 발생 시뮬레이션 (문서 내용 손상)
    TSharedPtr<FJsonObject> JsonObject = Document->GetContent();
    if (JsonObject.IsValid())
    {
        // 손상된 내용으로 설정
        Document->SetContentFromString("{\"title\":\"Corrupted Document\",\"content\":null}");
    }
    
    // 문서 복구 시도
    bool bRecovered = Document->RecoverDocument();
    
    if (bRecovered)
    {
        // 복구된 문서 내용 출력
        FString Content = Document->GetContentAsString();
        UE_LOG(LogTemp, Log, TEXT("복구된 문서 내용: %s"), *Content);
    }
    else
    {
        // 복구 실패 시 오류 메시지 출력
        UE_LOG(LogTemp, Error, TEXT("문서 복구 실패: %s"), *Document->GetLastErrorMessage());
    }
}

// 모든 문서 복구 예제
void RecoverAllDocumentsExample()
{
    // 동기화 관리자 생성
    UJsonCRDTSyncManager* SyncManager = UJsonCRDTBlueprintLibrary::CreateSyncManager(nullptr, "http://localhost:8080/api", "ws://localhost:8080/ws");
    
    // 여러 문서 생성
    UJsonCRDTDocument* Document1 = UJsonCRDTBlueprintLibrary::CreateDocument(nullptr, SyncManager, "doc1");
    Document1->SetContentFromString("{\"title\":\"Document 1\",\"content\":\"Content 1\"}");
    Document1->SaveLocally();
    
    UJsonCRDTDocument* Document2 = UJsonCRDTBlueprintLibrary::CreateDocument(nullptr, SyncManager, "doc2");
    Document2->SetContentFromString("{\"title\":\"Document 2\",\"content\":\"Content 2\"}");
    Document2->SaveLocally();
    
    UJsonCRDTDocument* Document3 = UJsonCRDTBlueprintLibrary::CreateDocument(nullptr, SyncManager, "doc3");
    Document3->SetContentFromString("{\"title\":\"Document 3\",\"content\":\"Content 3\"}");
    Document3->SaveLocally();
    
    // 모든 문서 복구 시도
    int32 RecoveredCount = SyncManager->RecoverAllDocuments();
    
    UE_LOG(LogTemp, Log, TEXT("%d개의 문서가 복구되었습니다"), RecoveredCount);
}
