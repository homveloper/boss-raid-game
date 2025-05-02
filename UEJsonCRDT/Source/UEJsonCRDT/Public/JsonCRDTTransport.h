// Copyright Your Company. All Rights Reserved.

#pragma once

#include "CoreMinimal.h"
#include "JsonCRDTTypes.h"
#include "JsonCRDTTransport.generated.h"

// 문서 로드 완료 시 호출되는 델리게이트
DECLARE_DELEGATE_OneParam(FOnDocumentLoaded, const FJsonCRDTDocumentData&);

// 문서 저장 완료 시 호출되는 델리게이트
DECLARE_DELEGATE_OneParam(FOnDocumentSaved, const FString& /* DocumentID */);

// 패치 전송 완료 시 호출되는 델리게이트
DECLARE_DELEGATE_OneParam(FOnPatchSent, const FString& /* DocumentID */);

// 패치 수신 시 호출되는 델리게이트
DECLARE_DELEGATE_OneParam(FOnPatchReceived, const FJsonCRDTPatch&);

// 전송 오류 발생 시 호출되는 델리게이트
DECLARE_DELEGATE_TwoParams(FOnTransportError, const FString& /* DocumentID */, const FString& /* ErrorMessage */);

/**
 * 문서 데이터 구조체
 */
USTRUCT(BlueprintType)
struct UEJSONCRDT_API FJsonCRDTDocumentData
{
    GENERATED_BODY()

    /** 문서 ID */
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString DocumentID;

    /** 문서 버전 */
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    int64 Version = 1;

    /** 문서 내용 (JSON 문자열) */
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString Content;

    /** 문서 생성 시간 */
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FDateTime CreatedAt = FDateTime::UtcNow();

    /** 문서 수정 시간 */
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FDateTime UpdatedAt = FDateTime::UtcNow();
};

/**
 * JsonCRDT 전송 인터페이스
 * 서버와의 통신을 추상화하는 인터페이스입니다.
 * 사용자는 이 인터페이스를 구현하여 자신만의 통신 방식을 정의할 수 있습니다.
 */
class UEJSONCRDT_API IJsonCRDTTransport
{
public:
    virtual ~IJsonCRDTTransport() = default;

    /**
     * 문서 로드 요청
     * @param DocumentID 로드할 문서 ID
     * @param OnLoaded 로드 완료 시 호출될 콜백
     * @param OnError 오류 발생 시 호출될 콜백
     */
    virtual void LoadDocument(const FString& DocumentID, const FOnDocumentLoaded& OnLoaded, const FOnTransportError& OnError) = 0;

    /**
     * 문서 저장 요청
     * @param DocumentData 저장할 문서 데이터
     * @param OnSaved 저장 완료 시 호출될 콜백
     * @param OnError 오류 발생 시 호출될 콜백
     */
    virtual void SaveDocument(const FJsonCRDTDocumentData& DocumentData, const FOnDocumentSaved& OnSaved, const FOnTransportError& OnError) = 0;

    /**
     * 패치 전송
     * @param Patch 전송할 패치
     * @param OnSent 전송 완료 시 호출될 콜백
     * @param OnError 오류 발생 시 호출될 콜백
     */
    virtual void SendPatch(const FJsonCRDTPatch& Patch, const FOnPatchSent& OnSent, const FOnTransportError& OnError) = 0;

    /**
     * 패치 수신 이벤트 등록
     * @param OnPatchReceived 패치 수신 시 호출될 콜백
     */
    virtual void RegisterPatchReceived(const FOnPatchReceived& OnPatchReceived) = 0;
};

/**
 * 기본 JsonCRDT 전송 구현체
 * 기본적인 HTTP 및 WebSocket 통신을 구현합니다.
 */
class UEJSONCRDT_API FDefaultJsonCRDTTransport : public IJsonCRDTTransport
{
public:
    /**
     * 생성자
     * @param InServerURL 서버 URL (HTTP API)
     * @param InWebSocketURL WebSocket URL
     */
    FDefaultJsonCRDTTransport(const FString& InServerURL, const FString& InWebSocketURL);
    virtual ~FDefaultJsonCRDTTransport();

    // IJsonCRDTTransport 인터페이스 구현
    virtual void LoadDocument(const FString& DocumentID, const FOnDocumentLoaded& OnLoaded, const FOnTransportError& OnError) override;
    virtual void SaveDocument(const FJsonCRDTDocumentData& DocumentData, const FOnDocumentSaved& OnSaved, const FOnTransportError& OnError) override;
    virtual void SendPatch(const FJsonCRDTPatch& Patch, const FOnPatchSent& OnSent, const FOnTransportError& OnError) override;
    virtual void RegisterPatchReceived(const FOnPatchReceived& OnPatchReceived) override;

    /**
     * 서버에 연결
     * @return 연결 성공 여부
     */
    bool Connect();

    /**
     * 서버와의 연결 해제
     */
    void Disconnect();

    /**
     * 서버와의 연결 상태 확인
     * @return 연결 상태
     */
    bool IsConnected() const;

private:
    /** 서버 URL (HTTP API) */
    FString ServerURL;

    /** WebSocket URL */
    FString WebSocketURL;

    /** WebSocket 연결 */
    TSharedPtr<class IWebSocket> WebSocket;

    /** 클라이언트 ID */
    FString ClientID;

    /** 패치 수신 콜백 */
    FOnPatchReceived OnPatchReceivedDelegate;

    /** WebSocket 연결 이벤트 핸들러 */
    void OnWebSocketConnected();
    void OnWebSocketConnectionError(const FString& Error);
    void OnWebSocketMessage(const FString& Message);
    void OnWebSocketClosed(int32 StatusCode, const FString& Reason, bool bWasClean);

    /** 고유 클라이언트 ID 생성 */
    FString GenerateClientID();
};
