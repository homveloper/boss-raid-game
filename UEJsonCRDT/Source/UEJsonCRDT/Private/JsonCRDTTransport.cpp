// Copyright Your Company. All Rights Reserved.

#include "JsonCRDTTransport.h"
#include "HttpModule.h"
#include "Interfaces/IHttpResponse.h"
#include "JsonObjectConverter.h"
#include "Serialization/JsonReader.h"
#include "Serialization/JsonSerializer.h"
#include "WebSocketsModule.h"
#include "Misc/Guid.h"

FDefaultJsonCRDTTransport::FDefaultJsonCRDTTransport(const FString& InServerURL, const FString& InWebSocketURL)
    : ServerURL(InServerURL)
    , WebSocketURL(InWebSocketURL)
{
    // 고유 클라이언트 ID 생성
    ClientID = GenerateClientID();
}

FDefaultJsonCRDTTransport::~FDefaultJsonCRDTTransport()
{
    Disconnect();
}

void FDefaultJsonCRDTTransport::LoadDocument(const FString& DocumentID, const FOnDocumentLoaded& OnLoaded, const FOnTransportError& OnError)
{
    // HTTP 요청 생성
    TSharedRef<IHttpRequest, ESPMode::ThreadSafe> HttpRequest = FHttpModule::Get().CreateRequest();
    HttpRequest->SetURL(FString::Printf(TEXT("%s/documents/%s"), *ServerURL, *DocumentID));
    HttpRequest->SetVerb(TEXT("GET"));
    HttpRequest->SetHeader(TEXT("Content-Type"), TEXT("application/json"));

    // 응답 처리 콜백 설정
    HttpRequest->OnProcessRequestComplete().BindLambda([DocumentID, OnLoaded, OnError](FHttpRequestPtr Request, FHttpResponsePtr Response, bool bSucceeded)
    {
        if (!bSucceeded || !Response.IsValid())
        {
            OnError.ExecuteIfBound(DocumentID, TEXT("No response from server"));
            return;
        }

        if (Response->GetResponseCode() != 200)
        {
            OnError.ExecuteIfBound(DocumentID, FString::Printf(TEXT("Server error: %d, %s"), Response->GetResponseCode(), *Response->GetContentAsString()));
            return;
        }

        // 응답 파싱
        TSharedPtr<FJsonObject> JsonObject;
        TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(Response->GetContentAsString());
        if (!FJsonSerializer::Deserialize(Reader, JsonObject) || !JsonObject.IsValid())
        {
            OnError.ExecuteIfBound(DocumentID, TEXT("Failed to parse response"));
            return;
        }

        // 문서 데이터 생성
        FJsonCRDTDocumentData DocumentData;
        DocumentData.DocumentID = DocumentID;
        
        // 문서 내용 가져오기
        FString Content;
        if (JsonObject->TryGetStringField(TEXT("content"), Content))
        {
            DocumentData.Content = Content;
        }
        
        // 문서 버전 가져오기
        int64 Version = 1;
        if (JsonObject->TryGetNumberField(TEXT("version"), Version))
        {
            DocumentData.Version = Version;
        }
        
        // 생성 시간 가져오기
        FString CreatedAtStr;
        if (JsonObject->TryGetStringField(TEXT("createdAt"), CreatedAtStr))
        {
            FDateTime::Parse(CreatedAtStr, DocumentData.CreatedAt);
        }
        
        // 수정 시간 가져오기
        FString UpdatedAtStr;
        if (JsonObject->TryGetStringField(TEXT("updatedAt"), UpdatedAtStr))
        {
            FDateTime::Parse(UpdatedAtStr, DocumentData.UpdatedAt);
        }

        // 로드 완료 콜백 호출
        OnLoaded.ExecuteIfBound(DocumentData);
    });

    // 요청 전송
    HttpRequest->ProcessRequest();
}

void FDefaultJsonCRDTTransport::SaveDocument(const FJsonCRDTDocumentData& DocumentData, const FOnDocumentSaved& OnSaved, const FOnTransportError& OnError)
{
    // HTTP 요청 생성
    TSharedRef<IHttpRequest, ESPMode::ThreadSafe> HttpRequest = FHttpModule::Get().CreateRequest();
    HttpRequest->SetURL(FString::Printf(TEXT("%s/documents/%s"), *ServerURL, *DocumentData.DocumentID));
    HttpRequest->SetVerb(TEXT("PUT"));
    HttpRequest->SetHeader(TEXT("Content-Type"), TEXT("application/json"));

    // 요청 본문 생성
    TSharedPtr<FJsonObject> RequestBody = MakeShared<FJsonObject>();
    RequestBody->SetStringField(TEXT("clientId"), ClientID);
    RequestBody->SetStringField(TEXT("content"), DocumentData.Content);
    RequestBody->SetNumberField(TEXT("version"), DocumentData.Version);
    RequestBody->SetStringField(TEXT("updatedAt"), DocumentData.UpdatedAt.ToString());

    FString RequestBodyString;
    TSharedRef<TJsonWriter<>> Writer = TJsonWriterFactory<>::Create(&RequestBodyString);
    FJsonSerializer::Serialize(RequestBody.ToSharedRef(), Writer);

    HttpRequest->SetContentAsString(RequestBodyString);

    // 응답 처리 콜백 설정
    HttpRequest->OnProcessRequestComplete().BindLambda([DocumentData, OnSaved, OnError](FHttpRequestPtr Request, FHttpResponsePtr Response, bool bSucceeded)
    {
        if (!bSucceeded || !Response.IsValid())
        {
            OnError.ExecuteIfBound(DocumentData.DocumentID, TEXT("No response from server"));
            return;
        }

        if (Response->GetResponseCode() != 200)
        {
            OnError.ExecuteIfBound(DocumentData.DocumentID, FString::Printf(TEXT("Server error: %d, %s"), Response->GetResponseCode(), *Response->GetContentAsString()));
            return;
        }

        // 저장 완료 콜백 호출
        OnSaved.ExecuteIfBound(DocumentData.DocumentID);
    });

    // 요청 전송
    HttpRequest->ProcessRequest();
}

void FDefaultJsonCRDTTransport::SendPatch(const FJsonCRDTPatch& Patch, const FOnPatchSent& OnSent, const FOnTransportError& OnError)
{
    if (!IsConnected())
    {
        OnError.ExecuteIfBound(Patch.DocumentID, TEXT("Not connected to server"));
        return;
    }

    // 패치 메시지 생성
    TSharedPtr<FJsonObject> PatchMessage = MakeShared<FJsonObject>();
    PatchMessage->SetStringField(TEXT("type"), TEXT("patch"));
    PatchMessage->SetStringField(TEXT("documentId"), Patch.DocumentID);
    PatchMessage->SetStringField(TEXT("clientId"), ClientID);
    PatchMessage->SetNumberField(TEXT("baseVersion"), Patch.BaseVersion);
    
    // 패치 작업 배열 생성
    TArray<TSharedPtr<FJsonValue>> OperationsArray;
    for (const FJsonCRDTOperation& Operation : Patch.Operations)
    {
        TSharedPtr<FJsonObject> OperationObject = MakeShared<FJsonObject>();
        
        // 작업 타입 설정
        FString OperationType;
        switch (Operation.Type)
        {
        case EJsonCRDTOperationType::Add:
            OperationType = TEXT("add");
            break;
        case EJsonCRDTOperationType::Remove:
            OperationType = TEXT("remove");
            break;
        case EJsonCRDTOperationType::Replace:
            OperationType = TEXT("replace");
            break;
        case EJsonCRDTOperationType::Move:
            OperationType = TEXT("move");
            break;
        case EJsonCRDTOperationType::Copy:
            OperationType = TEXT("copy");
            break;
        case EJsonCRDTOperationType::Test:
            OperationType = TEXT("test");
            break;
        default:
            OperationType = TEXT("unknown");
            break;
        }
        
        OperationObject->SetStringField(TEXT("op"), OperationType);
        OperationObject->SetStringField(TEXT("path"), Operation.Path);
        
        // from 필드는 move와 copy 작업에만 필요
        if (Operation.Type == EJsonCRDTOperationType::Move || Operation.Type == EJsonCRDTOperationType::Copy)
        {
            OperationObject->SetStringField(TEXT("from"), Operation.FromPath);
        }
        
        // value 필드는 add, replace, test 작업에만 필요
        if (Operation.Type == EJsonCRDTOperationType::Add || 
            Operation.Type == EJsonCRDTOperationType::Replace || 
            Operation.Type == EJsonCRDTOperationType::Test)
        {
            // value는 이미 JSON 문자열이므로 파싱하여 추가
            TSharedPtr<FJsonValue> ValueJsonValue;
            TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(Operation.Value);
            if (FJsonSerializer::Deserialize(Reader, ValueJsonValue))
            {
                OperationObject->SetField(TEXT("value"), ValueJsonValue);
            }
            else
            {
                // 파싱 실패 시 문자열로 추가
                OperationObject->SetStringField(TEXT("value"), Operation.Value);
            }
        }
        
        OperationsArray.Add(MakeShared<FJsonValueObject>(OperationObject));
    }
    
    PatchMessage->SetArrayField(TEXT("operations"), OperationsArray);
    
    // 패치 메시지 직렬화
    FString PatchMessageString;
    TSharedRef<TJsonWriter<>> Writer = TJsonWriterFactory<>::Create(&PatchMessageString);
    FJsonSerializer::Serialize(PatchMessage.ToSharedRef(), Writer);
    
    // WebSocket으로 패치 전송
    WebSocket->Send(PatchMessageString);
    
    // 전송 완료 콜백 호출
    OnSent.ExecuteIfBound(Patch.DocumentID);
}

void FDefaultJsonCRDTTransport::RegisterPatchReceived(const FOnPatchReceived& OnPatchReceived)
{
    OnPatchReceivedDelegate = OnPatchReceived;
}

bool FDefaultJsonCRDTTransport::Connect()
{
    if (WebSocket.IsValid() && WebSocket->IsConnected())
    {
        return true;
    }

    if (!FModuleManager::Get().IsModuleLoaded("WebSockets"))
    {
        FModuleManager::Get().LoadModule("WebSockets");
    }

    WebSocket = FWebSocketsModule::Get().CreateWebSocket(WebSocketURL);

    WebSocket->OnConnected().AddRaw(this, &FDefaultJsonCRDTTransport::OnWebSocketConnected);
    WebSocket->OnConnectionError().AddRaw(this, &FDefaultJsonCRDTTransport::OnWebSocketConnectionError);
    WebSocket->OnMessage().AddRaw(this, &FDefaultJsonCRDTTransport::OnWebSocketMessage);
    WebSocket->OnClosed().AddRaw(this, &FDefaultJsonCRDTTransport::OnWebSocketClosed);

    WebSocket->Connect();
    return true;
}

void FDefaultJsonCRDTTransport::Disconnect()
{
    if (WebSocket.IsValid() && WebSocket->IsConnected())
    {
        WebSocket->Close();
        WebSocket = nullptr;
    }
}

bool FDefaultJsonCRDTTransport::IsConnected() const
{
    return WebSocket.IsValid() && WebSocket->IsConnected();
}

void FDefaultJsonCRDTTransport::OnWebSocketConnected()
{
    UE_LOG(LogTemp, Log, TEXT("Connected to WebSocket server"));
    
    // 인증 메시지 전송
    TSharedPtr<FJsonObject> AuthMessage = MakeShared<FJsonObject>();
    AuthMessage->SetStringField(TEXT("type"), TEXT("auth"));
    AuthMessage->SetStringField(TEXT("clientId"), ClientID);

    FString AuthMessageString;
    TSharedRef<TJsonWriter<>> Writer = TJsonWriterFactory<>::Create(&AuthMessageString);
    FJsonSerializer::Serialize(AuthMessage.ToSharedRef(), Writer);

    WebSocket->Send(AuthMessageString);
}

void FDefaultJsonCRDTTransport::OnWebSocketConnectionError(const FString& Error)
{
    UE_LOG(LogTemp, Error, TEXT("WebSocket connection error: %s"), *Error);
}

void FDefaultJsonCRDTTransport::OnWebSocketMessage(const FString& Message)
{
    // 메시지 파싱
    TSharedPtr<FJsonObject> JsonObject;
    TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(Message);
    if (!FJsonSerializer::Deserialize(Reader, JsonObject) || !JsonObject.IsValid())
    {
        UE_LOG(LogTemp, Error, TEXT("Failed to parse WebSocket message: %s"), *Message);
        return;
    }

    // 메시지 타입 가져오기
    FString MessageType;
    if (!JsonObject->TryGetStringField(TEXT("type"), MessageType))
    {
        UE_LOG(LogTemp, Error, TEXT("WebSocket message missing 'type' field: %s"), *Message);
        return;
    }

    // 패치 메시지 처리
    if (MessageType == TEXT("patch"))
    {
        // 패치 파싱
        FJsonCRDTPatch Patch;
        if (FJsonObjectConverter::JsonObjectToUStruct(JsonObject.ToSharedRef(), &Patch, 0, 0))
        {
            // 패치 수신 콜백 호출
            if (OnPatchReceivedDelegate.IsBound())
            {
                OnPatchReceivedDelegate.Execute(Patch);
            }
        }
        else
        {
            UE_LOG(LogTemp, Error, TEXT("Failed to parse patch from WebSocket message: %s"), *Message);
        }
    }
}

void FDefaultJsonCRDTTransport::OnWebSocketClosed(int32 StatusCode, const FString& Reason, bool bWasClean)
{
    UE_LOG(LogTemp, Log, TEXT("WebSocket closed: %d, %s, %s"), StatusCode, *Reason, bWasClean ? TEXT("clean") : TEXT("not clean"));
}

FString FDefaultJsonCRDTTransport::GenerateClientID()
{
    return FGuid::NewGuid().ToString();
}
