// Copyright Your Company. All Rights Reserved.

#pragma once

#include "CoreMinimal.h"
#include "Kismet/BlueprintFunctionLibrary.h"
#include "JsonCRDTTypes.h"
#include "JsonCRDTTransport.h"
#include "JsonCRDTBlueprintLibrary.generated.h"

class UJsonCRDTDocument;
class UJsonCRDTSyncManager;

/**
 * UJsonCRDTBlueprintLibrary - Blueprint function library for JSON CRDT operations
 */
UCLASS()
class UEJSONCRDT_API UJsonCRDTBlueprintLibrary : public UBlueprintFunctionLibrary
{
	GENERATED_BODY()

public:
	/**
	 * 기본 Transport를 사용하여 새 CRDT 동기화 관리자 생성
	 * @param WorldContextObject 월드 컨텍스트 객체
	 * @param ServerURL 서버 URL (HTTP API)
	 * @param WebSocketURL WebSocket URL
	 * @return 생성된 동기화 관리자
	 */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	static UJsonCRDTSyncManager* CreateSyncManager(UObject* WorldContextObject, const FString& ServerURL = TEXT(""), const FString& WebSocketURL = TEXT(""));

	/**
	 * 사용자 정의 Transport를 사용하여 새 CRDT 동기화 관리자 생성
	 * @param WorldContextObject 월드 컨텍스트 객체
	 * @param Transport 사용자 정의 Transport
	 * @return 생성된 동기화 관리자
	 */
	static UJsonCRDTSyncManager* CreateSyncManagerWithTransport(UObject* WorldContextObject, TSharedPtr<IJsonCRDTTransport> Transport);

	/** Create a new CRDT document */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	static UJsonCRDTDocument* CreateDocument(UObject* WorldContextObject, UJsonCRDTSyncManager* SyncManager, const FString& DocumentID);

	/** Create a JSON CRDT operation */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	static FJsonCRDTOperation CreateOperation(EJsonCRDTOperationType Type, const FString& Path, const FString& Value, const FString& FromPath = "");

	/** Create a JSON CRDT patch */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	static FJsonCRDTPatch CreatePatch(const FString& DocumentID, int64 BaseVersion, const TArray<FJsonCRDTOperation>& Operations);

	/** Convert a JSON string to a JSON object */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	static bool StringToJsonObject(const FString& JsonString, TSharedPtr<FJsonObject>& OutJsonObject);

	/** Convert a JSON object to a JSON string */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	static bool JsonObjectToString(const TSharedPtr<FJsonObject>& JsonObject, FString& OutJsonString);

	/** Get a value from a JSON object by path */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	static bool GetJsonValueByPath(const TSharedPtr<FJsonObject>& JsonObject, const FString& Path, FString& OutValue);

	/** Set a value in a JSON object by path */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	static bool SetJsonValueByPath(TSharedPtr<FJsonObject>& JsonObject, const FString& Path, const FString& Value);
};
