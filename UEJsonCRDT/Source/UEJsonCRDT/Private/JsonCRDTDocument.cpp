// Copyright Your Company. All Rights Reserved.

#include "JsonCRDTDocument.h"
#include "JsonCRDTSyncManager.h"
#include "JsonObjectConverter.h"
#include "JsonCRDTDefaultConflictResolver.h"
#include "JsonCRDTDefaultLogger.h"
#include "JsonCRDTVisualizer.h"
#include "Serialization/JsonReader.h"
#include "Serialization/JsonSerializer.h"
#include "Misc/Paths.h"
#include "Misc/FileHelper.h"
#include "HAL/PlatformFilemanager.h"
#include "Misc/Guid.h"

UJsonCRDTDocument::UJsonCRDTDocument()
	: Version(1)
	, SyncManager(nullptr)
	, MaxOperationHistory(100)
	, MaxSnapshotHistory(10)
	, bAutoLocalSave(false)
	, ConflictStrategy(EJsonCRDTConflictStrategy::LastWriterWins)
{
	Content = MakeShared<FJsonObject>();

	// 기본 충돌 해결 전략 설정
	ConflictResolver = MakeShared<FJsonCRDTDefaultConflictResolver>(ConflictStrategy);

	// 기본 로거 설정
	Logger = MakeShared<FJsonCRDTDefaultLogger>();
}

UJsonCRDTDocument::~UJsonCRDTDocument()
{
}

void UJsonCRDTDocument::Initialize(const FString& InDocumentID, UJsonCRDTSyncManager* InSyncManager)
{
	DocumentID = InDocumentID;
	SyncManager = InSyncManager;

	// Create initial snapshot
	CreateAndAddSnapshot();
}

FString UJsonCRDTDocument::GetDocumentID() const
{
	return DocumentID;
}

int64 UJsonCRDTDocument::GetVersion() const
{
	return Version;
}

FString UJsonCRDTDocument::GetContentAsString() const
{
	FString OutputString;
	TSharedRef<TJsonWriter<>> Writer = TJsonWriterFactory<>::Create(&OutputString);
	FJsonSerializer::Serialize(Content.ToSharedRef(), Writer);
	return OutputString;
}

TSharedPtr<FJsonObject> UJsonCRDTDocument::GetContent() const
{
	return Content;
}

bool UJsonCRDTDocument::SetContentFromString(const FString& JsonString)
{
	TSharedPtr<FJsonObject> JsonObject;
	TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(JsonString);
	if (FJsonSerializer::Deserialize(Reader, JsonObject) && JsonObject.IsValid())
	{
		return SetContent(JsonObject);
	}
	return false;
}

bool UJsonCRDTDocument::SetContent(TSharedPtr<FJsonObject> JsonObject)
{
	if (!JsonObject.IsValid())
	{
		return false;
	}

	Content = JsonObject;
	Version++;

	// Create a snapshot after content change
	CreateAndAddSnapshot();

	// Notify that the document has changed
	NotifyDocumentChanged();

	return true;
}

bool UJsonCRDTDocument::ApplyPatch(const FJsonCRDTPatch& Patch)
{
	// Validate the patch
	if (Patch.DocumentID != DocumentID)
	{
		UE_LOG(LogTemp, Error, TEXT("Patch document ID does not match: %s != %s"), *Patch.DocumentID, *DocumentID);
		return false;
	}

	// Apply the operations in the patch
	for (const FJsonCRDTOperation& Operation : Patch.Operations)
	{
		// 현재 값 가져오기 (로깅 및 충돌 해결용)
		FString OldValue;
		if (Operation.Type == EJsonCRDTOperationType::Replace ||
			Operation.Type == EJsonCRDTOperationType::Remove ||
			Operation.Type == EJsonCRDTOperationType::Test)
		{
			// 경로에서 현재 값 가져오기
			TSharedPtr<FJsonValue> CurrentValue = GetValueAtPath(Content, Operation.Path);
			if (CurrentValue.IsValid())
			{
				// 값을 문자열로 변환
				TSharedRef<TJsonWriter<>> Writer = TJsonWriterFactory<>::Create(&OldValue);
				FJsonSerializer::Serialize(CurrentValue.ToSharedRef(), Writer);
			}
		}

		// 충돌 감지 및 해결
		bool bHadConflict = false;
		FJsonCRDTConflict Conflict;

		if (Operation.Type == EJsonCRDTOperationType::Replace)
		{
			// 같은 경로에 대한 로컬 작업이 있는지 확인
			for (int32 i = OperationHistory.Num() - 1; i >= 0; --i)
			{
				const FJsonCRDTOperation& LocalOperation = OperationHistory[i];

				// 같은 경로에 대한 Replace 작업인 경우 충돌 가능성 있음
				if (LocalOperation.Path == Operation.Path &&
					LocalOperation.Type == EJsonCRDTOperationType::Replace &&
					LocalOperation.Value != Operation.Value)
				{
					// 충돌 정보 설정
					Conflict.Path = Operation.Path;
					Conflict.LocalValue = LocalOperation.Value;
					Conflict.RemoteValue = Operation.Value;
					Conflict.LocalOperation = LocalOperation;
					Conflict.RemoteOperation = Operation;

					// 충돌 해결
					bHadConflict = true;

					// 충돌 해결 시도
					if (ResolveConflict(Conflict))
					{
						// 충돌이 해결되었으면 해결된 값으로 작업 수정
						FJsonCRDTOperation ResolvedOperation = Operation;
						ResolvedOperation.Value = Conflict.ResolvedValue;

						// 해결된 작업 적용
						ApplyOperation(ResolvedOperation);

						// 충돌 이벤트 발생
						OnConflictDetected.Broadcast(Conflict);

						// 작업 로깅
						LogOperation(ResolvedOperation, OldValue, Conflict.ResolvedValue, true, Conflict);

						// 작업 히스토리에 추가
						OperationHistory.Add(ResolvedOperation);
						if (OperationHistory.Num() > MaxOperationHistory)
						{
							OperationHistory.RemoveAt(0);
						}

						// 다음 작업으로 넘어감
						continue;
					}
				}
			}
		}

		// 충돌이 없거나 해결되지 않은 경우 일반적인 작업 적용
		if (!bHadConflict)
		{
			// 새 값 가져오기 (로깅용)
			FString NewValue = Operation.Value;

			// 작업 적용
			ApplyOperation(Operation);

			// 작업 로깅
			LogOperation(Operation, OldValue, NewValue);

			// 작업 히스토리에 추가
			OperationHistory.Add(Operation);
			if (OperationHistory.Num() > MaxOperationHistory)
			{
				OperationHistory.RemoveAt(0);
			}
		}
	}

	// Update the version
	Version++;

	// Create a snapshot after applying a patch
	CreateAndAddSnapshot();

	// Notify that the document has changed
	NotifyDocumentChanged();

	return true;
}

void UJsonCRDTDocument::ApplyOperation(const FJsonCRDTOperation& Operation)
{
	// Apply the operation based on its type
	switch (Operation.Type)
	{
	case EJsonCRDTOperationType::Add:
		// Add a value at the specified path
		// Implementation depends on the path format and structure
		break;

	case EJsonCRDTOperationType::Remove:
		// Remove a value at the specified path
		break;

	case EJsonCRDTOperationType::Replace:
		// Replace a value at the specified path
		break;

	case EJsonCRDTOperationType::Move:
		// Move a value from one path to another
		break;

	case EJsonCRDTOperationType::Copy:
		// Copy a value from one path to another
		break;

	case EJsonCRDTOperationType::Test:
		// Test if a value at the specified path equals the given value
		break;

	default:
		UE_LOG(LogTemp, Error, TEXT("Unknown operation type: %d"), (int32)Operation.Type);
		break;
	}
}

bool UJsonCRDTDocument::ApplyPatchFromString(const FString& PatchString)
{
	FJsonCRDTPatch Patch;
	if (FJsonObjectConverter::JsonObjectStringToUStruct(PatchString, &Patch, 0, 0))
	{
		return ApplyPatch(Patch);
	}
	return false;
}

FJsonCRDTSnapshot UJsonCRDTDocument::CreateSnapshot() const
{
	FJsonCRDTSnapshot Snapshot;
	Snapshot.DocumentID = DocumentID;
	Snapshot.Version = Version;
	Snapshot.Timestamp = FDateTime::UtcNow();
	Snapshot.Content = GetContentAsString();
	return Snapshot;
}

bool UJsonCRDTDocument::RestoreFromSnapshot(const FJsonCRDTSnapshot& Snapshot)
{
	// Validate the snapshot
	if (Snapshot.DocumentID != DocumentID)
	{
		UE_LOG(LogTemp, Error, TEXT("Snapshot document ID does not match: %s != %s"), *Snapshot.DocumentID, *DocumentID);
		return false;
	}

	// Restore the content from the snapshot
	if (!SetContentFromString(Snapshot.Content))
	{
		UE_LOG(LogTemp, Error, TEXT("Failed to restore content from snapshot"));
		return false;
	}

	// Update the version
	Version = Snapshot.Version;

	// Notify that the document has changed
	NotifyDocumentChanged();

	return true;
}

void UJsonCRDTDocument::Save()
{
	if (SyncManager)
	{
		SyncManager->SaveDocument(this);
	}
}

void UJsonCRDTDocument::Sync()
{
	if (SyncManager)
	{
		SyncManager->SyncDocument(this);
	}
}

void UJsonCRDTDocument::CreateAndAddSnapshot()
{
	FJsonCRDTSnapshot Snapshot = CreateSnapshot();
	SnapshotHistory.Add(Snapshot);

	// Trim the snapshot history if needed
	if (SnapshotHistory.Num() > MaxSnapshotHistory)
	{
		SnapshotHistory.RemoveAt(0, SnapshotHistory.Num() - MaxSnapshotHistory);
	}
}

void UJsonCRDTDocument::NotifyDocumentChanged()
{
	OnDocumentChanged.Broadcast(DocumentID);

	// Auto-save locally if enabled
	if (bAutoLocalSave)
	{
		SaveLocally();
	}
}

bool UJsonCRDTDocument::SaveLocally()
{
	// Get the local storage path
	FString FilePath = GetLocalStoragePath();

	// Create the directory if it doesn't exist
	FString DirectoryPath = FPaths::GetPath(FilePath);
	IPlatformFile& PlatformFile = FPlatformFileManager::Get().GetPlatformFile();
	if (!PlatformFile.DirectoryExists(*DirectoryPath))
	{
		if (!PlatformFile.CreateDirectoryTree(*DirectoryPath))
		{
			SetLastErrorMessage(FString::Printf(TEXT("Failed to create directory: %s"), *DirectoryPath));
			UE_LOG(LogTemp, Error, TEXT("%s"), *LastErrorMessage);
			return false;
		}
	}

	// Create a JSON object to store the document data
	TSharedPtr<FJsonObject> SaveData = MakeShared<FJsonObject>();
	SaveData->SetStringField(TEXT("documentId"), DocumentID);
	SaveData->SetNumberField(TEXT("version"), Version);
	SaveData->SetStringField(TEXT("timestamp"), FDateTime::UtcNow().ToString());
	SaveData->SetObjectField(TEXT("content"), Content);

	// Add the latest snapshot
	if (SnapshotHistory.Num() > 0)
	{
		TSharedPtr<FJsonObject> SnapshotObject = MakeShared<FJsonObject>();
		SnapshotObject->SetStringField(TEXT("documentId"), SnapshotHistory.Last().DocumentID);
		SnapshotObject->SetNumberField(TEXT("version"), SnapshotHistory.Last().Version);
		SnapshotObject->SetStringField(TEXT("timestamp"), SnapshotHistory.Last().Timestamp.ToString());
		SnapshotObject->SetStringField(TEXT("content"), SnapshotHistory.Last().Content);
		SaveData->SetObjectField(TEXT("latestSnapshot"), SnapshotObject);
	}

	// Serialize the JSON object to a string
	FString SaveString;
	TSharedRef<TJsonWriter<>> Writer = TJsonWriterFactory<>::Create(&SaveString);
	if (!FJsonSerializer::Serialize(SaveData.ToSharedRef(), Writer))
	{
		SetLastErrorMessage(TEXT("Failed to serialize document data"));
		UE_LOG(LogTemp, Error, TEXT("%s"), *LastErrorMessage);
		return false;
	}

	// Save the string to a file
	if (!FFileHelper::SaveStringToFile(SaveString, *FilePath))
	{
		SetLastErrorMessage(FString::Printf(TEXT("Failed to save document to file: %s"), *FilePath));
		UE_LOG(LogTemp, Error, TEXT("%s"), *LastErrorMessage);
		return false;
	}

	UE_LOG(LogTemp, Log, TEXT("Document %s saved locally to %s"), *DocumentID, *FilePath);
	return true;
}

bool UJsonCRDTDocument::LoadFromLocal()
{
	// Get the local storage path
	FString FilePath = GetLocalStoragePath();

	// Check if the file exists
	IPlatformFile& PlatformFile = FPlatformFileManager::Get().GetPlatformFile();
	if (!PlatformFile.FileExists(*FilePath))
	{
		SetLastErrorMessage(FString::Printf(TEXT("Local file does not exist: %s"), *FilePath));
		UE_LOG(LogTemp, Warning, TEXT("%s"), *LastErrorMessage);
		return false;
	}

	// Load the string from the file
	FString LoadString;
	if (!FFileHelper::LoadFileToString(LoadString, *FilePath))
	{
		SetLastErrorMessage(FString::Printf(TEXT("Failed to load document from file: %s"), *FilePath));
		UE_LOG(LogTemp, Error, TEXT("%s"), *LastErrorMessage);
		return false;
	}

	// Deserialize the JSON object
	TSharedPtr<FJsonObject> LoadData;
	TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(LoadString);
	if (!FJsonSerializer::Deserialize(Reader, LoadData) || !LoadData.IsValid())
	{
		SetLastErrorMessage(TEXT("Failed to deserialize document data"));
		UE_LOG(LogTemp, Error, TEXT("%s"), *LastErrorMessage);
		return false;
	}

	// Validate the document ID
	FString LoadedDocumentID;
	if (!LoadData->TryGetStringField(TEXT("documentId"), LoadedDocumentID) || LoadedDocumentID != DocumentID)
	{
		SetLastErrorMessage(FString::Printf(TEXT("Document ID mismatch: %s != %s"), *LoadedDocumentID, *DocumentID));
		UE_LOG(LogTemp, Error, TEXT("%s"), *LastErrorMessage);
		return false;
	}

	// Get the version
	int64 LoadedVersion;
	if (LoadData->TryGetNumberField(TEXT("version"), LoadedVersion))
	{
		Version = LoadedVersion;
	}

	// Get the content
	const TSharedPtr<FJsonObject>* ContentObject;
	if (LoadData->TryGetObjectField(TEXT("content"), ContentObject))
	{
		Content = *ContentObject;
	}
	else
	{
		SetLastErrorMessage(TEXT("Failed to get content from loaded data"));
		UE_LOG(LogTemp, Error, TEXT("%s"), *LastErrorMessage);
		return false;
	}

	// Load the latest snapshot if available
	const TSharedPtr<FJsonObject>* SnapshotObject;
	if (LoadData->TryGetObjectField(TEXT("latestSnapshot"), SnapshotObject))
	{
		FJsonCRDTSnapshot Snapshot;

		// Get the snapshot document ID
		if (!(*SnapshotObject)->TryGetStringField(TEXT("documentId"), Snapshot.DocumentID) || Snapshot.DocumentID != DocumentID)
		{
			UE_LOG(LogTemp, Warning, TEXT("Snapshot document ID mismatch, ignoring snapshot"));
		}
		else
		{
			// Get the snapshot version
			(*SnapshotObject)->TryGetNumberField(TEXT("version"), Snapshot.Version);

			// Get the snapshot timestamp
			FString TimestampString;
			if ((*SnapshotObject)->TryGetStringField(TEXT("timestamp"), TimestampString))
			{
				FDateTime::Parse(TimestampString, Snapshot.Timestamp);
			}

			// Get the snapshot content
			(*SnapshotObject)->TryGetStringField(TEXT("content"), Snapshot.Content);

			// Add the snapshot to the history
			SnapshotHistory.Add(Snapshot);
		}
	}

	UE_LOG(LogTemp, Log, TEXT("Document %s loaded from local storage"), *DocumentID);

	// Notify that the document has changed
	NotifyDocumentChanged();

	return true;
}

bool UJsonCRDTDocument::HasPendingChanges() const
{
	return PendingOperations.Num() > 0;
}

FString UJsonCRDTDocument::GetLastErrorMessage() const
{
	return LastErrorMessage;
}

void UJsonCRDTDocument::SetAutoLocalSave(bool bEnable)
{
	bAutoLocalSave = bEnable;

	// If enabling auto-save, save the current state
	if (bAutoLocalSave)
	{
		SaveLocally();
	}
}

bool UJsonCRDTDocument::IsAutoLocalSaveEnabled() const
{
	return bAutoLocalSave;
}

bool UJsonCRDTDocument::RecoverDocument()
{
	// First try to load from local storage
	if (LoadFromLocal())
	{
		OnDocumentRecovered.Broadcast(DocumentID, TEXT("LocalStorage"));
		return true;
	}

	// If that fails, try to restore from the latest snapshot
	if (SnapshotHistory.Num() > 0)
	{
		if (RestoreFromSnapshot(SnapshotHistory.Last()))
		{
			OnDocumentRecovered.Broadcast(DocumentID, TEXT("Snapshot"));
			return true;
		}
	}

	// If all recovery methods fail
	SetLastErrorMessage(TEXT("Failed to recover document from any source"));
	UE_LOG(LogTemp, Error, TEXT("%s"), *LastErrorMessage);
	return false;
}

FString UJsonCRDTDocument::GetLocalStoragePath() const
{
	return FPaths::ProjectSavedDir() / TEXT("JsonCRDT") / DocumentID + TEXT(".json");
}

void UJsonCRDTDocument::SetLastErrorMessage(const FString& ErrorMessage)
{
	LastErrorMessage = ErrorMessage;
}

TSharedPtr<FJsonValue> UJsonCRDTDocument::GetValueAtPath(TSharedPtr<FJsonObject> JsonObject, const FString& Path) const
{
	if (!JsonObject.IsValid() || Path.IsEmpty())
	{
		return nullptr;
	}

	// 경로 파싱 (JSON Pointer 형식: /path/to/value)
	TArray<FString> PathParts;
	FString PathCopy = Path;

	// 첫 번째 '/'가 있으면 제거
	if (PathCopy.StartsWith(TEXT("/")))
	{
		PathCopy.RemoveAt(0, 1);
	}

	// 경로를 '/'로 분할
	PathCopy.ParseIntoArray(PathParts, TEXT("/"));

	// 빈 경로인 경우 전체 객체 반환
	if (PathParts.Num() == 0)
	{
		return MakeShared<FJsonValueObject>(JsonObject);
	}

	// 경로를 따라 값 찾기
	TSharedPtr<FJsonValue> CurrentValue = MakeShared<FJsonValueObject>(JsonObject);
	for (int32 i = 0; i < PathParts.Num(); ++i)
	{
		FString& Part = PathParts[i];

		// ~로 시작하는 이스케이프된 문자 처리
		Part.ReplaceInline(TEXT("~1"), TEXT("/"));
		Part.ReplaceInline(TEXT("~0"), TEXT("~"));

		// 현재 값이 객체인 경우
		if (CurrentValue->Type == EJson::Object)
		{
			TSharedPtr<FJsonObject> CurrentObject = CurrentValue->AsObject();
			if (!CurrentObject->HasField(Part))
			{
				return nullptr;
			}

			CurrentValue = CurrentObject->GetField<EJson::None>(Part);
		}
		// 현재 값이 배열인 경우
		else if (CurrentValue->Type == EJson::Array)
		{
			TArray<TSharedPtr<FJsonValue>> Array = CurrentValue->AsArray();
			int32 Index = FCString::Atoi(*Part);

			if (Index < 0 || Index >= Array.Num())
			{
				return nullptr;
			}

			CurrentValue = Array[Index];
		}
		// 다른 타입인 경우 (경로가 더 남아있으면 오류)
		else if (i < PathParts.Num() - 1)
		{
			return nullptr;
		}
	}

	return CurrentValue;
}

void UJsonCRDTDocument::SetConflictStrategy(EJsonCRDTConflictStrategy Strategy)
{
	ConflictStrategy = Strategy;

	// 기본 충돌 해결 전략 업데이트
	if (ConflictResolver.IsValid() && ConflictResolver->GetStrategy() != Strategy)
	{
		TSharedPtr<FJsonCRDTDefaultConflictResolver> DefaultResolver = StaticCastSharedPtr<FJsonCRDTDefaultConflictResolver>(ConflictResolver);
		if (DefaultResolver.IsValid())
		{
			DefaultResolver->SetStrategy(Strategy);
		}
		else
		{
			// 기본 충돌 해결 전략으로 새로 생성
			ConflictResolver = MakeShared<FJsonCRDTDefaultConflictResolver>(Strategy);
		}
	}
}

EJsonCRDTConflictStrategy UJsonCRDTDocument::GetConflictStrategy() const
{
	return ConflictStrategy;
}

void UJsonCRDTDocument::SetConflictResolver(TSharedPtr<IJsonCRDTConflictResolver> InConflictResolver)
{
	if (InConflictResolver.IsValid())
	{
		ConflictResolver = InConflictResolver;
		ConflictStrategy = ConflictResolver->GetStrategy();
	}
}

TSharedPtr<IJsonCRDTConflictResolver> UJsonCRDTDocument::GetConflictResolver() const
{
	return ConflictResolver;
}

void UJsonCRDTDocument::SetLogger(TSharedPtr<IJsonCRDTLogger> InLogger)
{
	Logger = InLogger;
}

TSharedPtr<IJsonCRDTLogger> UJsonCRDTDocument::GetLogger() const
{
	return Logger;
}

void UJsonCRDTDocument::SetLoggingEnabled(bool bEnable)
{
	if (Logger.IsValid())
	{
		Logger->SetLoggingEnabled(bEnable);
	}
}

bool UJsonCRDTDocument::IsLoggingEnabled() const
{
	if (Logger.IsValid())
	{
		return Logger->IsLoggingEnabled();
	}
	return false;
}

bool UJsonCRDTDocument::ExportLogs(const FString& FilePath)
{
	if (!Logger.IsValid())
	{
		SetLastErrorMessage(TEXT("Logger is not set"));
		return false;
	}

	// 현재 문서에 대한 로그만 필터링
	FJsonCRDTLogFilter Filter;
	Filter.DocumentID = DocumentID;

	return Logger->ExportLogs(FilePath, Filter);
}

bool UJsonCRDTDocument::VisualizeHistory(const FString& FilePath)
{
	if (!Logger.IsValid())
	{
		SetLastErrorMessage(TEXT("Logger is not set"));
		return false;
	}

	// 현재 문서에 대한 로그만 필터링
	FJsonCRDTLogFilter Filter;
	Filter.DocumentID = DocumentID;

	// 로그 가져오기
	TArray<FJsonCRDTLogEntry> Logs = Logger->GetLogs(Filter);

	// 시각화 도구 생성
	UJsonCRDTVisualizer* Visualizer = NewObject<UJsonCRDTVisualizer>();

	// 문서 히스토리 시각화
	return Visualizer->VisualizeDocumentHistory(Logs, FilePath);
}

bool UJsonCRDTDocument::ResolveConflict(FJsonCRDTConflict& Conflict)
{
	if (!ConflictResolver.IsValid())
	{
		UE_LOG(LogTemp, Warning, TEXT("No conflict resolver set, using default strategy"));
		ConflictResolver = MakeShared<FJsonCRDTDefaultConflictResolver>(ConflictStrategy);
	}

	return ConflictResolver->ResolveConflict(Conflict);
}

void UJsonCRDTDocument::LogOperation(const FJsonCRDTOperation& Operation, const FString& OldValue, const FString& NewValue, bool bHadConflict, const FJsonCRDTConflict& Conflict)
{
	if (!Logger.IsValid() || !Logger->IsLoggingEnabled())
	{
		return;
	}

	// 로그 항목 생성
	FJsonCRDTLogEntry LogEntry;
	LogEntry.LogID = FGuid::NewGuid().ToString();
	LogEntry.DocumentID = DocumentID;
	LogEntry.Path = Operation.Path;
	LogEntry.OldValue = OldValue;
	LogEntry.NewValue = NewValue;
	LogEntry.Timestamp = FDateTime::UtcNow();
	LogEntry.bHadConflict = bHadConflict;
	LogEntry.ClientID = Operation.ClientID;
	LogEntry.Source = TEXT("Remote");

	// 작업 유형 설정
	switch (Operation.Type)
	{
	case EJsonCRDTOperationType::Add:
		LogEntry.OperationType = TEXT("Add");
		break;
	case EJsonCRDTOperationType::Remove:
		LogEntry.OperationType = TEXT("Remove");
		break;
	case EJsonCRDTOperationType::Replace:
		LogEntry.OperationType = TEXT("Replace");
		break;
	case EJsonCRDTOperationType::Move:
		LogEntry.OperationType = TEXT("Move");
		break;
	case EJsonCRDTOperationType::Copy:
		LogEntry.OperationType = TEXT("Copy");
		break;
	case EJsonCRDTOperationType::Test:
		LogEntry.OperationType = TEXT("Test");
		break;
	default:
		LogEntry.OperationType = TEXT("Unknown");
		break;
	}

	// 충돌 정보 설정
	if (bHadConflict)
	{
		LogEntry.Conflict = Conflict;
	}

	// 로그 기록
	Logger->LogOperation(LogEntry);
}
