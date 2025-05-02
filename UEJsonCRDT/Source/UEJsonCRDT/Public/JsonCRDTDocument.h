// Copyright Your Company. All Rights Reserved.

#pragma once

#include "CoreMinimal.h"
#include "UObject/NoExportTypes.h"
#include "Dom/JsonObject.h"
#include "JsonCRDTTypes.h"
#include "JsonCRDTConflictResolver.h"
#include "JsonCRDTLogger.h"
#include "JsonCRDTDocument.generated.h"

class UJsonCRDTSyncManager;

DECLARE_DYNAMIC_MULTICAST_DELEGATE_OneParam(FOnDocumentChanged, const FString&, DocumentID);
DECLARE_DYNAMIC_MULTICAST_DELEGATE_TwoParams(FOnSyncError, const FString&, DocumentID, const FString&, ErrorMessage);
DECLARE_DYNAMIC_MULTICAST_DELEGATE_TwoParams(FOnDocumentRecovered, const FString&, DocumentID, const FString&, RecoverySource);
DECLARE_DYNAMIC_MULTICAST_DELEGATE_OneParam(FOnConflictDetected, const FJsonCRDTConflict&, Conflict);

/**
 * UJsonCRDTDocument - A CRDT-based JSON document that can be synchronized with a server
 */
UCLASS(BlueprintType, Blueprintable)
class UEJSONCRDT_API UJsonCRDTDocument : public UObject
{
	GENERATED_BODY()

public:
	UJsonCRDTDocument();
	virtual ~UJsonCRDTDocument();

	/** Initialize the document with a unique ID */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	void Initialize(const FString& InDocumentID, UJsonCRDTSyncManager* InSyncManager);

	/** Get the document ID */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	FString GetDocumentID() const;

	/** Get the document version */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	int64 GetVersion() const;

	/** Get the document content as a JSON string */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	FString GetContentAsString() const;

	/** Get the document content as a JSON object */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	TSharedPtr<FJsonObject> GetContent() const;

	/** Set the document content from a JSON string */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	bool SetContentFromString(const FString& JsonString);

	/** Set the document content from a JSON object */
	bool SetContent(TSharedPtr<FJsonObject> JsonObject);

	/** Apply a JSON patch to the document */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	bool ApplyPatch(const FJsonCRDTPatch& Patch);

	/** Apply a JSON patch to the document from a JSON string */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	bool ApplyPatchFromString(const FString& PatchString);

	/** Create a snapshot of the current document state */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	FJsonCRDTSnapshot CreateSnapshot() const;

	/** Restore the document from a snapshot */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	bool RestoreFromSnapshot(const FJsonCRDTSnapshot& Snapshot);

	/** Save the document to the server */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	void Save();

	/** Save the document locally */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	bool SaveLocally();

	/** Load the document from local storage */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	bool LoadFromLocal();

	/** Synchronize the document with the server */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	void Sync();

	/** Check if the document has local changes that need to be synchronized */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	bool HasPendingChanges() const;

	/** Get the last error message */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	FString GetLastErrorMessage() const;

	/** Enable or disable automatic local saving */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	void SetAutoLocalSave(bool bEnable);

	/** Check if automatic local saving is enabled */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	bool IsAutoLocalSaveEnabled() const;

	/** Attempt to recover the document from local storage or snapshots */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	bool RecoverDocument();

	/** Event triggered when the document changes */
	UPROPERTY(BlueprintAssignable, Category = "JsonCRDT")
	FOnDocumentChanged OnDocumentChanged;

	/** Event triggered when a sync error occurs */
	UPROPERTY(BlueprintAssignable, Category = "JsonCRDT")
	FOnSyncError OnSyncError;

	/** Event triggered when a document is recovered */
	UPROPERTY(BlueprintAssignable, Category = "JsonCRDT")
	FOnDocumentRecovered OnDocumentRecovered;

	/** Event triggered when a conflict is detected */
	UPROPERTY(BlueprintAssignable, Category = "JsonCRDT")
	FOnConflictDetected OnConflictDetected;

	/** Set the conflict resolver */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	void SetConflictStrategy(EJsonCRDTConflictStrategy Strategy);

	/** Get the current conflict strategy */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	EJsonCRDTConflictStrategy GetConflictStrategy() const;

	/** Set a custom conflict resolver */
	void SetConflictResolver(TSharedPtr<IJsonCRDTConflictResolver> InConflictResolver);

	/** Get the current conflict resolver */
	TSharedPtr<IJsonCRDTConflictResolver> GetConflictResolver() const;

	/** Set the logger */
	void SetLogger(TSharedPtr<IJsonCRDTLogger> InLogger);

	/** Get the current logger */
	TSharedPtr<IJsonCRDTLogger> GetLogger() const;

	/** Enable or disable logging */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	void SetLoggingEnabled(bool bEnable);

	/** Check if logging is enabled */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	bool IsLoggingEnabled() const;

	/** Export logs to file */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	bool ExportLogs(const FString& FilePath);

	/** Visualize document history */
	UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
	bool VisualizeHistory(const FString& FilePath);

private:
	/** The document ID */
	UPROPERTY()
	FString DocumentID;

	/** The document version */
	UPROPERTY()
	int64 Version;

	/** The document content */
	TSharedPtr<FJsonObject> Content;

	/** The sync manager */
	UPROPERTY()
	UJsonCRDTSyncManager* SyncManager;

	/** The operation history */
	TArray<FJsonCRDTPatch> OperationHistory;

	/** The snapshot history */
	TArray<FJsonCRDTSnapshot> SnapshotHistory;

	/** Maximum number of operations to keep in history */
	int32 MaxOperationHistory;

	/** Maximum number of snapshots to keep */
	int32 MaxSnapshotHistory;

	/** Whether to automatically save locally after changes */
	UPROPERTY()
	bool bAutoLocalSave;

	/** The last error message */
	UPROPERTY()
	FString LastErrorMessage;

	/** Pending operations that need to be synchronized */
	TArray<FJsonCRDTPatch> PendingOperations;

	/** Create a new snapshot and add it to the history */
	void CreateAndAddSnapshot();

	/** Notify that the document has changed */
	void NotifyDocumentChanged();

	/** Get the local storage path for this document */
	FString GetLocalStoragePath() const;

	/** Set the last error message */
	void SetLastErrorMessage(const FString& ErrorMessage);

	/** 충돌 해결 전략 */
	UPROPERTY()
	EJsonCRDTConflictStrategy ConflictStrategy;

	/** 충돌 해결 구현체 */
	TSharedPtr<IJsonCRDTConflictResolver> ConflictResolver;

	/** 로거 */
	TSharedPtr<IJsonCRDTLogger> Logger;

	/** 충돌 해결 */
	bool ResolveConflict(FJsonCRDTConflict& Conflict);

	/** 작업 로깅 */
	void LogOperation(const FJsonCRDTOperation& Operation, const FString& OldValue, const FString& NewValue, bool bHadConflict = false, const FJsonCRDTConflict& Conflict = FJsonCRDTConflict());

	/** JSON 경로에서 값 가져오기 */
	TSharedPtr<FJsonValue> GetValueAtPath(TSharedPtr<FJsonObject> JsonObject, const FString& Path) const;

	/** 작업 적용 */
	void ApplyOperation(const FJsonCRDTOperation& Operation);
};
