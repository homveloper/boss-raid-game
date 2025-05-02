// Copyright Your Company. All Rights Reserved.

#pragma once

#include "CoreMinimal.h"
#include "UObject/NoExportTypes.h"
#include "JsonCRDTTypes.h"
#include "JsonCRDTTransport.h"
#include "JsonCRDTLogger.h"
#include "JsonCRDTConflictResolver.h"
#include "JsonCRDTSyncManager.generated.h"

class UJsonCRDTDocument;

DECLARE_DYNAMIC_MULTICAST_DELEGATE_OneParam(FOnSyncComplete, const FString&, DocumentID);
DECLARE_DYNAMIC_MULTICAST_DELEGATE_TwoParams(FOnDocumentSaveError, const FString&, DocumentID, const FString&, ErrorMessage);

/**
 * UJsonCRDTSyncManager - Manages synchronization of CRDT documents
 *
 * 이 클래스는 CRDT 문서의 동기화를 관리합니다.
 * 서버와의 통신은 IJsonCRDTTransport 인터페이스를 통해 추상화됩니다.
 */
UCLASS(BlueprintType, Blueprintable)
class UEJSONCRDT_API UJsonCRDTSyncManager : public UObject
{
	GENERATED_BODY()

public:
	UJsonCRDTSyncManager();
	virtual ~UJsonCRDTSyncManager();

	/**
	 * 기본 Transport로 초기화 (HTTP 및 WebSocket 사용)
	 * @param InServerURL 서버 URL (HTTP API)
	 * @param InWebSocketURL WebSocket URL
	 */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	void Initialize(const FString& InServerURL, const FString& InWebSocketURL);

	/**
	 * Transport 설정
	 * @param InTransport 사용자 정의 Transport
	 */
	void SetTransport(TSharedPtr<IJsonCRDTTransport> InTransport);

	/**
	 * 새 문서 생성
	 * @param Document 생성할 문서
	 */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	void CreateDocument(UJsonCRDTDocument* Document);

	/**
	 * 서버에서 문서 로드
	 * @param DocumentID 로드할 문서 ID
	 */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	void LoadDocument(const FString& DocumentID);

	/**
	 * 문서 저장
	 * @param Document 저장할 문서
	 */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	void SaveDocument(UJsonCRDTDocument* Document);

	/**
	 * 문서 동기화
	 * @param Document 동기화할 문서
	 */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	void SyncDocument(UJsonCRDTDocument* Document);

	/**
	 * ID로 문서 가져오기
	 * @param DocumentID 문서 ID
	 * @return 문서 객체
	 */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	UJsonCRDTDocument* GetDocument(const FString& DocumentID);

	/**
	 * 모든 문서 복구 시도
	 * @return 복구된 문서 수
	 */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	int32 RecoverAllDocuments();

	/**
	 * 로거 설정
	 * @param InLogger 로거
	 */
	void SetLogger(TSharedPtr<IJsonCRDTLogger> InLogger);

	/**
	 * 로거 가져오기
	 * @return 로거
	 */
	TSharedPtr<IJsonCRDTLogger> GetLogger() const;

	/**
	 * 로깅 활성화 여부 설정
	 * @param bEnable 활성화 여부
	 */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	void SetLoggingEnabled(bool bEnable);

	/**
	 * 로깅 활성화 여부 가져오기
	 * @return 활성화 여부
	 */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	bool IsLoggingEnabled() const;

	/**
	 * 모든 문서의 로그 내보내기
	 * @param FilePath 파일 경로
	 * @return 내보내기 성공 여부
	 */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	bool ExportAllLogs(const FString& FilePath);

	/**
	 * 기본 충돌 해결 전략 설정
	 * @param Strategy 충돌 해결 전략
	 */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	void SetDefaultConflictStrategy(EJsonCRDTConflictStrategy Strategy);

	/**
	 * 기본 충돌 해결 전략 가져오기
	 * @return 충돌 해결 전략
	 */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	EJsonCRDTConflictStrategy GetDefaultConflictStrategy() const;

	/** 문서 동기화 완료 시 이벤트 */
	UPROPERTY(BlueprintAssignable, Category = "JsonCRDT")
	FOnSyncComplete OnSyncComplete;

	/** 문서 저장 오류 발생 시 이벤트 */
	UPROPERTY(BlueprintAssignable, Category = "JsonCRDT")
	FOnDocumentSaveError OnDocumentSaveError;

private:
	/** Transport 인터페이스 */
	TSharedPtr<IJsonCRDTTransport> Transport;

	/** 문서 ID에서 문서 객체로의 맵 */
	UPROPERTY()
	TMap<FString, UJsonCRDTDocument*> Documents;

	/** 패치 수신 처리 */
	void OnPatchReceived(const FJsonCRDTPatch& Patch);

	/** 문서 로드 완료 처리 */
	void OnDocumentLoaded(const FJsonCRDTDocumentData& DocumentData);

	/** 문서 저장 완료 처리 */
	void OnDocumentSaved(const FString& DocumentID);

	/** 전송 오류 처리 */
	void OnTransportError(const FString& DocumentID, const FString& ErrorMessage);

	/** 모든 문서 로컬 저장 */
	void SaveAllDocumentsLocally();

	/** 로거 */
	TSharedPtr<IJsonCRDTLogger> Logger;

	/** 기본 충돌 해결 전략 */
	EJsonCRDTConflictStrategy DefaultConflictStrategy;
};
