// Copyright Your Company. All Rights Reserved.

#include "JsonCRDTSyncManager.h"
#include "JsonCRDTDocument.h"
#include "JsonCRDTBlueprintLibrary.h"
#include "JsonCRDTTransport.h"

/**
 * 로컬 저장소 사용 예제
 * 이 예제는 로컬 저장소를 사용하여 문서를 저장하고 복원하는 방법을 보여줍니다.
 */
void LocalStorageExample()
{
    // 동기화 관리자 생성 (Transport 없이)
    UJsonCRDTSyncManager* SyncManager = UJsonCRDTBlueprintLibrary::CreateSyncManager(nullptr);
    
    // 문서 생성
    UJsonCRDTDocument* Document = UJsonCRDTBlueprintLibrary::CreateDocument(nullptr, SyncManager, "local-storage-doc");
    
    // 자동 로컬 저장 활성화
    Document->SetAutoLocalSave(true);
    
    // 문서 내용 설정
    Document->SetContentFromString("{\"title\":\"Local Storage Example\",\"content\":\"This document is saved locally\",\"lastEdited\":\"2023-01-01\"}");
    
    // 문서 저장 (로컬에만 저장됨)
    Document->Save();
    
    // 문서 내용 가져오기
    FString Content = Document->GetContentAsString();
    UE_LOG(LogTemp, Log, TEXT("문서 내용: %s"), *Content);
    
    // 문서 편집
    TSharedPtr<FJsonObject> JsonObject = Document->GetContent();
    if (JsonObject.IsValid())
    {
        JsonObject->SetStringField("content", "This document was edited and saved locally");
        JsonObject->SetStringField("lastEdited", FDateTime::UtcNow().ToString());
        Document->SetContent(JsonObject);
    }
    
    // 문서 저장 (로컬에만 저장됨)
    Document->Save();
    
    // 문서 복구 이벤트 핸들러 등록
    Document->OnDocumentRecovered.AddLambda([](const FString& DocumentID, const FString& RecoverySource) {
        UE_LOG(LogTemp, Log, TEXT("문서 %s가 %s에서 복구되었습니다"), *DocumentID, *RecoverySource);
    });
    
    // 문서 내용 손상 시뮬레이션
    Document->SetContentFromString("{\"title\":\"Corrupted Document\",\"content\":null}");
    
    // 문서 복구 시도
    bool bRecovered = Document->RecoverDocument();
    
    if (bRecovered)
    {
        // 복구된 문서 내용 출력
        FString RecoveredContent = Document->GetContentAsString();
        UE_LOG(LogTemp, Log, TEXT("복구된 문서 내용: %s"), *RecoveredContent);
    }
    else
    {
        // 복구 실패 시 오류 메시지 출력
        UE_LOG(LogTemp, Error, TEXT("문서 복구 실패: %s"), *Document->GetLastErrorMessage());
    }
}
