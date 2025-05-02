// Copyright Your Company. All Rights Reserved.

#include "JsonCRDTSyncManager.h"
#include "JsonCRDTDocument.h"
#include "JsonCRDTBlueprintLibrary.h"
#include "JsonCRDTLogger.h"
#include "JsonCRDTDefaultLogger.h"
#include "JsonCRDTVisualizer.h"

/**
 * 로깅 및 시각화 예제
 */
void LoggingAndVisualizationExample()
{
    // 동기화 관리자 생성
    UJsonCRDTSyncManager* SyncManager = UJsonCRDTBlueprintLibrary::CreateSyncManager(nullptr);
    
    // 로깅 활성화
    SyncManager->SetLoggingEnabled(true);
    
    // 문서 생성
    UJsonCRDTDocument* Document = UJsonCRDTBlueprintLibrary::CreateDocument(nullptr, SyncManager, "logging-example-doc");
    
    // 문서 내용 설정
    Document->SetContentFromString("{\"title\":\"Logging Example\",\"content\":\"This is a test\",\"tags\":[\"logging\",\"example\"]}");
    
    // 문서 편집 (여러 번)
    for (int32 i = 0; i < 5; ++i)
    {
        TSharedPtr<FJsonObject> JsonObject = Document->GetContent();
        if (JsonObject.IsValid())
        {
            // 내용 수정
            JsonObject->SetStringField("content", FString::Printf(TEXT("Edit #%d"), i + 1));
            
            // 태그 추가
            TArray<TSharedPtr<FJsonValue>>* Tags;
            if (JsonObject->TryGetArrayField("tags", Tags))
            {
                Tags->Add(MakeShared<FJsonValueString>(FString::Printf(TEXT("tag%d"), i + 1)));
            }
            
            // 타임스탬프 추가
            JsonObject->SetStringField("lastModified", FDateTime::UtcNow().ToString());
            
            // 수정된 내용 설정
            Document->SetContent(JsonObject);
        }
        
        // 잠시 대기 (로그 타임스탬프 차이를 위해)
        FPlatformProcess::Sleep(0.5f);
    }
    
    // 원격 작업 시뮬레이션 (충돌 발생)
    FJsonCRDTPatch RemotePatch;
    RemotePatch.DocumentID = "logging-example-doc";
    RemotePatch.BaseVersion = 1;
    RemotePatch.ClientID = "remote-client";
    RemotePatch.Timestamp = FDateTime::UtcNow();
    
    // 원격 작업 추가
    FJsonCRDTOperation Operation;
    Operation.Type = EJsonCRDTOperationType::Replace;
    Operation.Path = "/content";
    Operation.Value = "\"Remote edit\"";
    Operation.Timestamp = RemotePatch.Timestamp;
    RemotePatch.Operations.Add(Operation);
    
    // 패치 적용 (충돌 발생 및 해결)
    Document->ApplyPatch(RemotePatch);
    
    // 로그 내보내기 (JSON 형식)
    FString LogFilePath = FPaths::ProjectSavedDir() / TEXT("JsonCRDT") / TEXT("logs.json");
    if (SyncManager->ExportAllLogs(LogFilePath))
    {
        UE_LOG(LogTemp, Log, TEXT("로그를 %s에 내보냈습니다"), *LogFilePath);
    }
    else
    {
        UE_LOG(LogTemp, Error, TEXT("로그 내보내기 실패"));
    }
    
    // 문서 히스토리 시각화 (HTML 형식)
    FString HistoryFilePath = FPaths::ProjectSavedDir() / TEXT("JsonCRDT") / TEXT("history.html");
    if (Document->VisualizeHistory(HistoryFilePath))
    {
        UE_LOG(LogTemp, Log, TEXT("문서 히스토리를 %s에 내보냈습니다"), *HistoryFilePath);
    }
    else
    {
        UE_LOG(LogTemp, Error, TEXT("문서 히스토리 내보내기 실패"));
    }
    
    // 시각화 도구를 사용하여 충돌 시각화
    UJsonCRDTVisualizer* Visualizer = NewObject<UJsonCRDTVisualizer>();
    
    // 로그 가져오기
    TSharedPtr<IJsonCRDTLogger> Logger = SyncManager->GetLogger();
    if (Logger.IsValid())
    {
        // 충돌만 필터링
        FJsonCRDTLogFilter Filter;
        Filter.DocumentID = Document->GetDocumentID();
        Filter.bConflictsOnly = true;
        
        TArray<FJsonCRDTLogEntry> ConflictLogs = Logger->GetLogs(Filter);
        
        // 충돌 시각화
        FString ConflictFilePath = FPaths::ProjectSavedDir() / TEXT("JsonCRDT") / TEXT("conflicts.html");
        if (Visualizer->VisualizeConflicts(ConflictLogs, ConflictFilePath))
        {
            UE_LOG(LogTemp, Log, TEXT("충돌을 %s에 내보냈습니다"), *ConflictFilePath);
        }
        else
        {
            UE_LOG(LogTemp, Error, TEXT("충돌 내보내기 실패"));
        }
        
        // CSV 형식으로 내보내기
        FString CsvFilePath = FPaths::ProjectSavedDir() / TEXT("JsonCRDT") / TEXT("logs.csv");
        if (Visualizer->ExportToCSV(Logger->GetLogs(FJsonCRDTLogFilter()), CsvFilePath))
        {
            UE_LOG(LogTemp, Log, TEXT("로그를 CSV 형식으로 %s에 내보냈습니다"), *CsvFilePath);
        }
        else
        {
            UE_LOG(LogTemp, Error, TEXT("CSV 내보내기 실패"));
        }
    }
}
