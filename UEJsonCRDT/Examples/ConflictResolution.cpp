// Copyright Your Company. All Rights Reserved.

#include "JsonCRDTSyncManager.h"
#include "JsonCRDTDocument.h"
#include "JsonCRDTBlueprintLibrary.h"
#include "JsonCRDTConflictResolver.h"
#include "JsonCRDTDefaultConflictResolver.h"

/**
 * 사용자 정의 충돌 해결 전략 구현 예제
 */
class FMyCustomConflictResolver : public IJsonCRDTConflictResolver
{
public:
    virtual bool ResolveConflict(FJsonCRDTConflict& Conflict) override
    {
        // 경로에 따라 다른 전략 적용
        if (Conflict.Path.Contains(TEXT("/priority")))
        {
            // 우선순위 필드는 항상 높은 값 선택
            float LocalPriority = FCString::Atof(*Conflict.LocalValue);
            float RemotePriority = FCString::Atof(*Conflict.RemoteValue);
            
            if (LocalPriority >= RemotePriority)
            {
                Conflict.ResolvedValue = Conflict.LocalValue;
            }
            else
            {
                Conflict.ResolvedValue = Conflict.RemoteValue;
            }
        }
        else if (Conflict.Path.Contains(TEXT("/lastModified")))
        {
            // 마지막 수정 시간은 항상 최신 값 선택
            FDateTime LocalTime, RemoteTime;
            FDateTime::Parse(Conflict.LocalValue, LocalTime);
            FDateTime::Parse(Conflict.RemoteValue, RemoteTime);
            
            if (LocalTime >= RemoteTime)
            {
                Conflict.ResolvedValue = Conflict.LocalValue;
            }
            else
            {
                Conflict.ResolvedValue = Conflict.RemoteValue;
            }
        }
        else if (Conflict.Path.Contains(TEXT("/name")))
        {
            // 이름 필드는 항상 로컬 값 선택
            Conflict.ResolvedValue = Conflict.LocalValue;
        }
        else
        {
            // 기타 필드는 두 값을 합침 (문자열인 경우)
            if (Conflict.LocalValue.StartsWith(TEXT("\"")) && Conflict.RemoteValue.StartsWith(TEXT("\"")))
            {
                // 따옴표 제거
                FString LocalValueWithoutQuotes = Conflict.LocalValue.Mid(1, Conflict.LocalValue.Len() - 2);
                FString RemoteValueWithoutQuotes = Conflict.RemoteValue.Mid(1, Conflict.RemoteValue.Len() - 2);
                
                // 값 합치기
                FString MergedValue = LocalValueWithoutQuotes + TEXT(" + ") + RemoteValueWithoutQuotes;
                
                // 따옴표 추가
                Conflict.ResolvedValue = FString::Printf(TEXT("\"%s\""), *MergedValue);
            }
            else
            {
                // 문자열이 아닌 경우 원격 값 선택
                Conflict.ResolvedValue = Conflict.RemoteValue;
            }
        }
        
        Conflict.bResolved = true;
        return true;
    }
    
    virtual EJsonCRDTConflictStrategy GetStrategy() const override
    {
        return EJsonCRDTConflictStrategy::Custom;
    }
};

/**
 * 충돌 해결 예제
 */
void ConflictResolutionExample()
{
    // 동기화 관리자 생성
    UJsonCRDTSyncManager* SyncManager = UJsonCRDTBlueprintLibrary::CreateSyncManager(nullptr);
    
    // 문서 생성
    UJsonCRDTDocument* Document = UJsonCRDTBlueprintLibrary::CreateDocument(nullptr, SyncManager, "conflict-example-doc");
    
    // 충돌 감지 이벤트 핸들러 등록
    Document->OnConflictDetected.AddLambda([](const FJsonCRDTConflict& Conflict) {
        UE_LOG(LogTemp, Warning, TEXT("충돌 감지: %s"), *Conflict.Path);
        UE_LOG(LogTemp, Warning, TEXT("  로컬 값: %s"), *Conflict.LocalValue);
        UE_LOG(LogTemp, Warning, TEXT("  원격 값: %s"), *Conflict.RemoteValue);
        UE_LOG(LogTemp, Warning, TEXT("  해결된 값: %s"), *Conflict.ResolvedValue);
    });
    
    // 문서 내용 설정
    Document->SetContentFromString("{\"title\":\"Conflict Example\",\"content\":\"This is a test\",\"priority\":5,\"lastModified\":\"2023-01-01T00:00:00Z\",\"name\":\"Test Document\"}");
    
    // 기본 충돌 해결 전략 설정 (마지막 작성자 우선)
    Document->SetConflictStrategy(EJsonCRDTConflictStrategy::LastWriterWins);
    
    // 로컬 작업 시뮬레이션
    TSharedPtr<FJsonObject> JsonObject = Document->GetContent();
    if (JsonObject.IsValid())
    {
        JsonObject->SetStringField("content", "This is a local edit");
        JsonObject->SetNumberField("priority", 10);
        JsonObject->SetStringField("lastModified", FDateTime::UtcNow().ToString());
        JsonObject->SetStringField("name", "Local Name");
        Document->SetContent(JsonObject);
    }
    
    // 원격 작업 시뮬레이션 (충돌 발생)
    FJsonCRDTPatch RemotePatch;
    RemotePatch.DocumentID = "conflict-example-doc";
    RemotePatch.BaseVersion = 1;
    RemotePatch.ClientID = "remote-client";
    RemotePatch.Timestamp = FDateTime::UtcNow() - FTimespan::FromMinutes(5); // 5분 전 (로컬보다 이전)
    
    // 원격 작업 추가
    FJsonCRDTOperation Operation1;
    Operation1.Type = EJsonCRDTOperationType::Replace;
    Operation1.Path = "/content";
    Operation1.Value = "\"This is a remote edit\"";
    Operation1.Timestamp = RemotePatch.Timestamp;
    RemotePatch.Operations.Add(Operation1);
    
    FJsonCRDTOperation Operation2;
    Operation2.Type = EJsonCRDTOperationType::Replace;
    Operation2.Path = "/priority";
    Operation2.Value = "8";
    Operation2.Timestamp = RemotePatch.Timestamp;
    RemotePatch.Operations.Add(Operation2);
    
    FJsonCRDTOperation Operation3;
    Operation3.Type = EJsonCRDTOperationType::Replace;
    Operation3.Path = "/lastModified";
    Operation3.Value = "\"2023-01-02T00:00:00Z\"";
    Operation3.Timestamp = RemotePatch.Timestamp;
    RemotePatch.Operations.Add(Operation3);
    
    FJsonCRDTOperation Operation4;
    Operation4.Type = EJsonCRDTOperationType::Replace;
    Operation4.Path = "/name";
    Operation4.Value = "\"Remote Name\"";
    Operation4.Timestamp = RemotePatch.Timestamp;
    RemotePatch.Operations.Add(Operation4);
    
    // 패치 적용 (충돌 발생 및 해결)
    Document->ApplyPatch(RemotePatch);
    
    // 문서 내용 확인
    FString Content = Document->GetContentAsString();
    UE_LOG(LogTemp, Log, TEXT("LWW 전략 적용 후 문서 내용: %s"), *Content);
    
    // 사용자 정의 충돌 해결 전략 설정
    TSharedPtr<FMyCustomConflictResolver> CustomResolver = MakeShared<FMyCustomConflictResolver>();
    Document->SetConflictResolver(CustomResolver);
    
    // 문서 내용 재설정
    Document->SetContentFromString("{\"title\":\"Conflict Example\",\"content\":\"This is a test\",\"priority\":5,\"lastModified\":\"2023-01-01T00:00:00Z\",\"name\":\"Test Document\"}");
    
    // 로컬 작업 시뮬레이션
    JsonObject = Document->GetContent();
    if (JsonObject.IsValid())
    {
        JsonObject->SetStringField("content", "This is a local edit");
        JsonObject->SetNumberField("priority", 10);
        JsonObject->SetStringField("lastModified", FDateTime::UtcNow().ToString());
        JsonObject->SetStringField("name", "Local Name");
        Document->SetContent(JsonObject);
    }
    
    // 패치 적용 (사용자 정의 충돌 해결 전략 사용)
    Document->ApplyPatch(RemotePatch);
    
    // 문서 내용 확인
    Content = Document->GetContentAsString();
    UE_LOG(LogTemp, Log, TEXT("사용자 정의 전략 적용 후 문서 내용: %s"), *Content);
}
