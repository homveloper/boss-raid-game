// Copyright Your Company. All Rights Reserved.

#include "JsonCRDTSyncManager.h"
#include "JsonCRDTDocument.h"
#include "JsonCRDTTransport.h"
#include "JsonObjectConverter.h"

UJsonCRDTSyncManager::UJsonCRDTSyncManager()
    : DefaultConflictStrategy(EJsonCRDTConflictStrategy::LastWriterWins)
{
    // 기본 로거 설정
    Logger = MakeShared<FJsonCRDTDefaultLogger>();
}

UJsonCRDTSyncManager::~UJsonCRDTSyncManager()
{
    // Transport 객체는 shared_ptr이므로 자동으로 정리됨
}

void UJsonCRDTSyncManager::Initialize(const FString& InServerURL, const FString& InWebSocketURL)
{
    // 기본 Transport 생성 (HTTP 및 WebSocket 사용)
    TSharedPtr<FDefaultJsonCRDTTransport> DefaultTransport = MakeShared<FDefaultJsonCRDTTransport>(InServerURL, InWebSocketURL);
    SetTransport(DefaultTransport);

    // 서버에 연결
    DefaultTransport->Connect();
}

void UJsonCRDTSyncManager::SetTransport(TSharedPtr<IJsonCRDTTransport> InTransport)
{
    Transport = InTransport;

    // 패치 수신 이벤트 등록
    if (Transport.IsValid())
    {
        Transport->RegisterPatchReceived(FOnPatchReceived::CreateUObject(this, &UJsonCRDTSyncManager::OnPatchReceived));
    }
}

void UJsonCRDTSyncManager::CreateDocument(UJsonCRDTDocument* Document)
{
    if (!Document)
    {
        UE_LOG(LogTemp, Error, TEXT("Cannot create null document"));
        return;
    }

    // 문서를 맵에 추가
    Documents.Add(Document->GetDocumentID(), Document);

    // 로거 설정
    if (Logger.IsValid())
    {
        Document->SetLogger(Logger);
    }

    // 충돌 해결 전략 설정
    Document->SetConflictStrategy(DefaultConflictStrategy);

    // 문서 저장
    SaveDocument(Document);
}

void UJsonCRDTSyncManager::LoadDocument(const FString& DocumentID)
{
    // 이미 로드된 문서인지 확인
    if (Documents.Contains(DocumentID))
    {
        UE_LOG(LogTemp, Warning, TEXT("Document %s is already loaded"), *DocumentID);
        return;
    }

    // Transport가 유효한지 확인
    if (!Transport.IsValid())
    {
        UE_LOG(LogTemp, Error, TEXT("Transport is not valid"));
        return;
    }

    // 서버에서 문서 로드
    Transport->LoadDocument(
        DocumentID,
        FOnDocumentLoaded::CreateUObject(this, &UJsonCRDTSyncManager::OnDocumentLoaded),
        FOnTransportError::CreateUObject(this, &UJsonCRDTSyncManager::OnTransportError)
    );
}

void UJsonCRDTSyncManager::SaveDocument(UJsonCRDTDocument* Document)
{
    if (!Document)
    {
        UE_LOG(LogTemp, Error, TEXT("Cannot save null document"));
        return;
    }

    // 항상 로컬에 먼저 저장
    if (Document->SaveLocally())
    {
        UE_LOG(LogTemp, Log, TEXT("Document %s saved locally"), *Document->GetDocumentID());
    }
    else
    {
        UE_LOG(LogTemp, Warning, TEXT("Failed to save document %s locally"), *Document->GetDocumentID());
    }

    // Transport가 유효한지 확인
    if (!Transport.IsValid())
    {
        UE_LOG(LogTemp, Warning, TEXT("Transport is not valid, document %s saved locally only"), *Document->GetDocumentID());
        return;
    }

    // 문서 데이터 생성
    FJsonCRDTDocumentData DocumentData;
    DocumentData.DocumentID = Document->GetDocumentID();
    DocumentData.Version = Document->GetVersion();
    DocumentData.Content = Document->GetContentAsString();
    DocumentData.UpdatedAt = FDateTime::UtcNow();

    // 서버에 문서 저장
    Transport->SaveDocument(
        DocumentData,
        FOnDocumentSaved::CreateUObject(this, &UJsonCRDTSyncManager::OnDocumentSaved),
        FOnTransportError::CreateUObject(this, &UJsonCRDTSyncManager::OnTransportError)
    );
}

void UJsonCRDTSyncManager::SyncDocument(UJsonCRDTDocument* Document)
{
    if (!Document)
    {
        UE_LOG(LogTemp, Error, TEXT("Cannot sync null document"));
        return;
    }

    // Transport가 유효한지 확인
    if (!Transport.IsValid())
    {
        UE_LOG(LogTemp, Warning, TEXT("Transport is not valid, cannot sync document %s"), *Document->GetDocumentID());
        return;
    }

    // 패치 생성 (빈 패치는 동기화 요청을 의미)
    FJsonCRDTPatch SyncPatch;
    SyncPatch.DocumentID = Document->GetDocumentID();
    SyncPatch.BaseVersion = Document->GetVersion();
    SyncPatch.ClientID = TEXT(""); // Transport에서 설정
    SyncPatch.Timestamp = FDateTime::UtcNow();

    // 패치 전송
    Transport->SendPatch(
        SyncPatch,
        FOnPatchSent::CreateLambda([this](const FString& DocumentID) {
            UE_LOG(LogTemp, Log, TEXT("Sync request sent for document %s"), *DocumentID);
        }),
        FOnTransportError::CreateUObject(this, &UJsonCRDTSyncManager::OnTransportError)
    );
}

UJsonCRDTDocument* UJsonCRDTSyncManager::GetDocument(const FString& DocumentID)
{
    UJsonCRDTDocument** Document = Documents.Find(DocumentID);
    return Document ? *Document : nullptr;
}

int32 UJsonCRDTSyncManager::RecoverAllDocuments()
{
    int32 RecoveredCount = 0;

    // 모든 문서에 대해 반복
    for (auto& Pair : Documents)
    {
        UJsonCRDTDocument* Document = Pair.Value;
        if (Document && Document->RecoverDocument())
        {
            RecoveredCount++;
        }
    }

    return RecoveredCount;
}

void UJsonCRDTSyncManager::OnPatchReceived(const FJsonCRDTPatch& Patch)
{
    // 문서 ID로 문서 찾기
    UJsonCRDTDocument* Document = GetDocument(Patch.DocumentID);
    if (!Document)
    {
        UE_LOG(LogTemp, Warning, TEXT("Received patch for unknown document %s"), *Patch.DocumentID);
        return;
    }

    // 패치 적용
    if (Document->ApplyPatch(Patch))
    {
        UE_LOG(LogTemp, Log, TEXT("Applied patch to document %s"), *Patch.DocumentID);

        // 문서 로컬 저장
        Document->SaveLocally();

        // 동기화 완료 이벤트 발생
        OnSyncComplete.Broadcast(Patch.DocumentID);
    }
    else
    {
        UE_LOG(LogTemp, Error, TEXT("Failed to apply patch to document %s"), *Patch.DocumentID);

        // 문서 복구 시도
        if (Document->RecoverDocument())
        {
            UE_LOG(LogTemp, Log, TEXT("Recovered document %s after patch failure"), *Patch.DocumentID);
        }
        else
        {
            UE_LOG(LogTemp, Error, TEXT("Failed to recover document %s after patch failure"), *Patch.DocumentID);
        }
    }
}

void UJsonCRDTSyncManager::OnDocumentLoaded(const FJsonCRDTDocumentData& DocumentData)
{
    // 새 문서 생성
    UJsonCRDTDocument* Document = NewObject<UJsonCRDTDocument>(this);
    Document->Initialize(DocumentData.DocumentID, this);

    // 문서 내용 설정
    Document->SetContentFromString(DocumentData.Content);

    // 문서를 맵에 추가
    Documents.Add(DocumentData.DocumentID, Document);

    // 로거 설정
    if (Logger.IsValid())
    {
        Document->SetLogger(Logger);
    }

    // 충돌 해결 전략 설정
    Document->SetConflictStrategy(DefaultConflictStrategy);

    // 문서 로컬 저장
    Document->SaveLocally();

    UE_LOG(LogTemp, Log, TEXT("Document %s loaded successfully"), *DocumentData.DocumentID);
}

void UJsonCRDTSyncManager::OnDocumentSaved(const FString& DocumentID)
{
    UE_LOG(LogTemp, Log, TEXT("Document %s saved successfully"), *DocumentID);

    // 동기화 완료 이벤트 발생
    OnSyncComplete.Broadcast(DocumentID);
}

void UJsonCRDTSyncManager::OnTransportError(const FString& DocumentID, const FString& ErrorMessage)
{
    UE_LOG(LogTemp, Error, TEXT("Transport error for document %s: %s"), *DocumentID, *ErrorMessage);

    // 문서 저장 오류 이벤트 발생
    OnDocumentSaveError.Broadcast(DocumentID, ErrorMessage);

    // 문서 ID로 문서 찾기
    UJsonCRDTDocument* Document = GetDocument(DocumentID);
    if (Document)
    {
        // 문서 로컬 저장 시도
        if (Document->SaveLocally())
        {
            UE_LOG(LogTemp, Log, TEXT("Document %s saved locally after transport error"), *DocumentID);
        }
    }
}

void UJsonCRDTSyncManager::SaveAllDocumentsLocally()
{
    UE_LOG(LogTemp, Log, TEXT("Saving all documents locally"));

    // 모든 문서에 대해 반복
    for (auto& Pair : Documents)
    {
        UJsonCRDTDocument* Document = Pair.Value;
        if (Document)
        {
            // 문서 로컬 저장
            if (Document->SaveLocally())
            {
                UE_LOG(LogTemp, Log, TEXT("Document %s saved locally"), *Document->GetDocumentID());
            }
            else
            {
                UE_LOG(LogTemp, Warning, TEXT("Failed to save document %s locally"), *Document->GetDocumentID());
            }
        }
    }
}

void UJsonCRDTSyncManager::SetLogger(TSharedPtr<IJsonCRDTLogger> InLogger)
{
    if (InLogger.IsValid())
    {
        Logger = InLogger;

        // 모든 문서에 로거 설정
        for (auto& Pair : Documents)
        {
            UJsonCRDTDocument* Document = Pair.Value;
            if (Document)
            {
                Document->SetLogger(Logger);
            }
        }
    }
}

TSharedPtr<IJsonCRDTLogger> UJsonCRDTSyncManager::GetLogger() const
{
    return Logger;
}

void UJsonCRDTSyncManager::SetLoggingEnabled(bool bEnable)
{
    if (Logger.IsValid())
    {
        Logger->SetLoggingEnabled(bEnable);
    }
}

bool UJsonCRDTSyncManager::IsLoggingEnabled() const
{
    if (Logger.IsValid())
    {
        return Logger->IsLoggingEnabled();
    }
    return false;
}

bool UJsonCRDTSyncManager::ExportAllLogs(const FString& FilePath)
{
    if (!Logger.IsValid())
    {
        UE_LOG(LogTemp, Error, TEXT("Logger is not set"));
        return false;
    }

    return Logger->ExportLogs(FilePath);
}

void UJsonCRDTSyncManager::SetDefaultConflictStrategy(EJsonCRDTConflictStrategy Strategy)
{
    DefaultConflictStrategy = Strategy;

    // 모든 문서에 충돌 해결 전략 설정
    for (auto& Pair : Documents)
    {
        UJsonCRDTDocument* Document = Pair.Value;
        if (Document)
        {
            Document->SetConflictStrategy(Strategy);
        }
    }
}

EJsonCRDTConflictStrategy UJsonCRDTSyncManager::GetDefaultConflictStrategy() const
{
    return DefaultConflictStrategy;
}
