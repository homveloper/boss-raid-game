// Copyright Your Company. All Rights Reserved.

#include "JsonCRDTSyncManager.h"
#include "JsonCRDTDocument.h"
#include "JsonCRDTBlueprintLibrary.h"
#include "JsonCRDTTransport.h"

/**
 * 사용자 정의 Transport 구현 예제
 * 이 예제는 사용자 정의 통신 방식을 구현하는 방법을 보여줍니다.
 */
class FMyCustomTransport : public IJsonCRDTTransport
{
public:
    FMyCustomTransport()
    {
        // 고유 클라이언트 ID 생성
        ClientID = FGuid::NewGuid().ToString();
    }
    
    virtual ~FMyCustomTransport() = default;
    
    // 문서 로드 요청
    virtual void LoadDocument(const FString& DocumentID, const FOnDocumentLoaded& OnLoaded, const FOnTransportError& OnError) override
    {
        UE_LOG(LogTemp, Log, TEXT("사용자 정의 Transport: 문서 %s 로드 요청"), *DocumentID);
        
        // 여기에 실제 서버 통신 코드 구현
        // 예시: 게임 내 통신 시스템, REST API, 소켓 통신 등
        
        // 예제를 위한 더미 데이터 생성
        FJsonCRDTDocumentData DocumentData;
        DocumentData.DocumentID = DocumentID;
        DocumentData.Version = 1;
        DocumentData.Content = FString::Printf(TEXT("{\"title\":\"Custom Document %s\",\"content\":\"This document was loaded using a custom transport\"}"), *DocumentID);
        DocumentData.CreatedAt = FDateTime::UtcNow();
        DocumentData.UpdatedAt = FDateTime::UtcNow();
        
        // 로드 성공 콜백 호출
        OnLoaded.ExecuteIfBound(DocumentData);
    }
    
    // 문서 저장 요청
    virtual void SaveDocument(const FJsonCRDTDocumentData& DocumentData, const FOnDocumentSaved& OnSaved, const FOnTransportError& OnError) override
    {
        UE_LOG(LogTemp, Log, TEXT("사용자 정의 Transport: 문서 %s 저장 요청"), *DocumentData.DocumentID);
        
        // 여기에 실제 서버 통신 코드 구현
        
        // 저장 성공 콜백 호출
        OnSaved.ExecuteIfBound(DocumentData.DocumentID);
    }
    
    // 패치 전송
    virtual void SendPatch(const FJsonCRDTPatch& Patch, const FOnPatchSent& OnSent, const FOnTransportError& OnError) override
    {
        UE_LOG(LogTemp, Log, TEXT("사용자 정의 Transport: 문서 %s 패치 전송 요청"), *Patch.DocumentID);
        
        // 여기에 실제 서버 통신 코드 구현
        
        // 전송 성공 콜백 호출
        OnSent.ExecuteIfBound(Patch.DocumentID);
    }
    
    // 패치 수신 이벤트 등록
    virtual void RegisterPatchReceived(const FOnPatchReceived& OnPatchReceived) override
    {
        OnPatchReceivedDelegate = OnPatchReceived;
    }
    
    // 패치 수신 시뮬레이션 (실제 구현에서는 서버로부터 패치를 받았을 때 호출)
    void SimulatePatchReceived(const FJsonCRDTPatch& Patch)
    {
        if (OnPatchReceivedDelegate.IsBound())
        {
            OnPatchReceivedDelegate.Execute(Patch);
        }
    }
    
private:
    FString ClientID;
    FOnPatchReceived OnPatchReceivedDelegate;
};

// 사용자 정의 Transport 사용 예제
void CustomTransportExample()
{
    // 사용자 정의 Transport 생성
    TSharedPtr<FMyCustomTransport> CustomTransport = MakeShared<FMyCustomTransport>();
    
    // 동기화 관리자 생성
    UJsonCRDTSyncManager* SyncManager = UJsonCRDTBlueprintLibrary::CreateSyncManager(nullptr);
    
    // 사용자 정의 Transport 설정
    SyncManager->SetTransport(CustomTransport);
    
    // 문서 생성
    UJsonCRDTDocument* Document = UJsonCRDTBlueprintLibrary::CreateDocument(nullptr, SyncManager, "custom-transport-doc");
    
    // 자동 로컬 저장 활성화
    Document->SetAutoLocalSave(true);
    
    // 문서 내용 설정
    Document->SetContentFromString("{\"title\":\"Custom Transport Example\",\"content\":\"This document uses a custom transport implementation\"}");
    
    // 문서 저장
    Document->Save();
    
    // 문서 동기화
    Document->Sync();
    
    // 서버로부터 패치 수신 시뮬레이션
    FJsonCRDTPatch IncomingPatch;
    IncomingPatch.DocumentID = "custom-transport-doc";
    IncomingPatch.BaseVersion = 1;
    IncomingPatch.ClientID = "server";
    IncomingPatch.Timestamp = FDateTime::UtcNow();
    
    // 패치에 작업 추가
    FJsonCRDTOperation Operation;
    Operation.Type = EJsonCRDTOperationType::Replace;
    Operation.Path = "/content";
    Operation.Value = "\"This content was updated by the server\"";
    IncomingPatch.Operations.Add(Operation);
    
    // 패치 수신 시뮬레이션
    CustomTransport->SimulatePatchReceived(IncomingPatch);
    
    // 문서 내용 확인
    FString Content = Document->GetContentAsString();
    UE_LOG(LogTemp, Log, TEXT("문서 내용: %s"), *Content);
}
