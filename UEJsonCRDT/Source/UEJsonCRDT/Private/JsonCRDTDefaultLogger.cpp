// Copyright Your Company. All Rights Reserved.

#include "JsonCRDTDefaultLogger.h"
#include "Misc/FileHelper.h"
#include "Serialization/JsonSerializer.h"
#include "Serialization/JsonWriter.h"
#include "Serialization/JsonReader.h"
#include "Misc/Paths.h"
#include "HAL/PlatformFilemanager.h"

FJsonCRDTDefaultLogger::FJsonCRDTDefaultLogger(int32 InMaxLogEntries)
    : MaxLogEntries(InMaxLogEntries)
    , bLoggingEnabled(true)
{
}

void FJsonCRDTDefaultLogger::LogOperation(const FJsonCRDTLogEntry& LogEntry)
{
    if (!bLoggingEnabled)
    {
        return;
    }
    
    // 로그 항목 추가
    LogEntries.Add(LogEntry);
    
    // 최대 개수 초과 시 가장 오래된 항목 제거
    if (LogEntries.Num() > MaxLogEntries)
    {
        LogEntries.RemoveAt(0);
    }
}

bool FJsonCRDTDefaultLogger::ExportLogs(const FString& FilePath, const FJsonCRDTLogFilter& Filter)
{
    // 필터링된 로그 항목 가져오기
    TArray<FJsonCRDTLogEntry> FilteredLogs = GetLogs(Filter);
    
    // JSON 배열 생성
    TArray<TSharedPtr<FJsonValue>> JsonArray;
    for (const FJsonCRDTLogEntry& LogEntry : FilteredLogs)
    {
        TSharedPtr<FJsonObject> JsonObject = LogEntryToJson(LogEntry);
        JsonArray.Add(MakeShared<FJsonValueObject>(JsonObject));
    }
    
    // JSON 문자열로 변환
    FString JsonString;
    TSharedRef<TJsonWriter<>> Writer = TJsonWriterFactory<>::Create(&JsonString);
    FJsonSerializer::Serialize(JsonArray, Writer);
    
    // 디렉토리 생성
    FString Directory = FPaths::GetPath(FilePath);
    IPlatformFile& PlatformFile = FPlatformFileManager::Get().GetPlatformFile();
    if (!PlatformFile.DirectoryExists(*Directory))
    {
        if (!PlatformFile.CreateDirectoryTree(*Directory))
        {
            UE_LOG(LogTemp, Error, TEXT("Failed to create directory: %s"), *Directory);
            return false;
        }
    }
    
    // 파일에 저장
    if (!FFileHelper::SaveStringToFile(JsonString, *FilePath))
    {
        UE_LOG(LogTemp, Error, TEXT("Failed to save logs to file: %s"), *FilePath);
        return false;
    }
    
    UE_LOG(LogTemp, Log, TEXT("Exported %d log entries to %s"), FilteredLogs.Num(), *FilePath);
    return true;
}

TArray<FJsonCRDTLogEntry> FJsonCRDTDefaultLogger::GetLogs(const FJsonCRDTLogFilter& Filter)
{
    TArray<FJsonCRDTLogEntry> FilteredLogs;
    
    // 필터 적용
    for (const FJsonCRDTLogEntry& LogEntry : LogEntries)
    {
        if (ApplyFilter(LogEntry, Filter))
        {
            FilteredLogs.Add(LogEntry);
        }
    }
    
    return FilteredLogs;
}

void FJsonCRDTDefaultLogger::ClearLogs(const FJsonCRDTLogFilter& Filter)
{
    // 필터가 비어있으면 모든 로그 지우기
    if (Filter.DocumentID.IsEmpty() && 
        Filter.OperationType.IsEmpty() && 
        Filter.Path.IsEmpty() && 
        Filter.ClientID.IsEmpty() && 
        Filter.Source.IsEmpty() && 
        !Filter.bConflictsOnly &&
        Filter.StartTime.GetTicks() == 0 &&
        Filter.EndTime.GetTicks() == 0)
    {
        LogEntries.Empty();
        return;
    }
    
    // 필터에 맞지 않는 항목만 유지
    for (int32 i = LogEntries.Num() - 1; i >= 0; --i)
    {
        if (ApplyFilter(LogEntries[i], Filter))
        {
            LogEntries.RemoveAt(i);
        }
    }
}

void FJsonCRDTDefaultLogger::SetLoggingEnabled(bool bEnable)
{
    bLoggingEnabled = bEnable;
}

bool FJsonCRDTDefaultLogger::IsLoggingEnabled() const
{
    return bLoggingEnabled;
}

void FJsonCRDTDefaultLogger::SetMaxLogEntries(int32 InMaxLogEntries)
{
    MaxLogEntries = InMaxLogEntries;
    
    // 최대 개수 초과 시 가장 오래된 항목 제거
    while (LogEntries.Num() > MaxLogEntries)
    {
        LogEntries.RemoveAt(0);
    }
}

int32 FJsonCRDTDefaultLogger::GetMaxLogEntries() const
{
    return MaxLogEntries;
}

bool FJsonCRDTDefaultLogger::ApplyFilter(const FJsonCRDTLogEntry& LogEntry, const FJsonCRDTLogFilter& Filter) const
{
    // 문서 ID 필터
    if (!Filter.DocumentID.IsEmpty() && LogEntry.DocumentID != Filter.DocumentID)
    {
        return false;
    }
    
    // 시작 시간 필터
    if (Filter.StartTime.GetTicks() > 0 && LogEntry.Timestamp < Filter.StartTime)
    {
        return false;
    }
    
    // 종료 시간 필터
    if (Filter.EndTime.GetTicks() > 0 && LogEntry.Timestamp > Filter.EndTime)
    {
        return false;
    }
    
    // 작업 유형 필터
    if (!Filter.OperationType.IsEmpty() && LogEntry.OperationType != Filter.OperationType)
    {
        return false;
    }
    
    // 경로 필터
    if (!Filter.Path.IsEmpty() && !LogEntry.Path.Contains(Filter.Path))
    {
        return false;
    }
    
    // 충돌 필터
    if (Filter.bConflictsOnly && !LogEntry.bHadConflict)
    {
        return false;
    }
    
    // 클라이언트 ID 필터
    if (!Filter.ClientID.IsEmpty() && LogEntry.ClientID != Filter.ClientID)
    {
        return false;
    }
    
    // 소스 필터
    if (!Filter.Source.IsEmpty() && LogEntry.Source != Filter.Source)
    {
        return false;
    }
    
    return true;
}

TSharedPtr<FJsonObject> FJsonCRDTDefaultLogger::LogEntryToJson(const FJsonCRDTLogEntry& LogEntry) const
{
    TSharedPtr<FJsonObject> JsonObject = MakeShared<FJsonObject>();
    
    JsonObject->SetStringField(TEXT("logId"), LogEntry.LogID);
    JsonObject->SetStringField(TEXT("documentId"), LogEntry.DocumentID);
    JsonObject->SetStringField(TEXT("operationType"), LogEntry.OperationType);
    JsonObject->SetStringField(TEXT("path"), LogEntry.Path);
    JsonObject->SetStringField(TEXT("oldValue"), LogEntry.OldValue);
    JsonObject->SetStringField(TEXT("newValue"), LogEntry.NewValue);
    JsonObject->SetStringField(TEXT("timestamp"), LogEntry.Timestamp.ToString());
    JsonObject->SetBoolField(TEXT("hadConflict"), LogEntry.bHadConflict);
    JsonObject->SetStringField(TEXT("clientId"), LogEntry.ClientID);
    JsonObject->SetStringField(TEXT("source"), LogEntry.Source);
    
    // 충돌 정보가 있는 경우
    if (LogEntry.bHadConflict)
    {
        TSharedPtr<FJsonObject> ConflictObject = MakeShared<FJsonObject>();
        
        ConflictObject->SetStringField(TEXT("path"), LogEntry.Conflict.Path);
        ConflictObject->SetStringField(TEXT("localValue"), LogEntry.Conflict.LocalValue);
        ConflictObject->SetStringField(TEXT("remoteValue"), LogEntry.Conflict.RemoteValue);
        ConflictObject->SetStringField(TEXT("resolvedValue"), LogEntry.Conflict.ResolvedValue);
        ConflictObject->SetBoolField(TEXT("resolved"), LogEntry.Conflict.bResolved);
        
        // 로컬 작업 정보
        TSharedPtr<FJsonObject> LocalOperationObject = MakeShared<FJsonObject>();
        LocalOperationObject->SetStringField(TEXT("type"), FString::FromInt(static_cast<int32>(LogEntry.Conflict.LocalOperation.Type)));
        LocalOperationObject->SetStringField(TEXT("path"), LogEntry.Conflict.LocalOperation.Path);
        LocalOperationObject->SetStringField(TEXT("value"), LogEntry.Conflict.LocalOperation.Value);
        LocalOperationObject->SetStringField(TEXT("fromPath"), LogEntry.Conflict.LocalOperation.FromPath);
        LocalOperationObject->SetStringField(TEXT("timestamp"), LogEntry.Conflict.LocalOperation.Timestamp.ToString());
        
        // 원격 작업 정보
        TSharedPtr<FJsonObject> RemoteOperationObject = MakeShared<FJsonObject>();
        RemoteOperationObject->SetStringField(TEXT("type"), FString::FromInt(static_cast<int32>(LogEntry.Conflict.RemoteOperation.Type)));
        RemoteOperationObject->SetStringField(TEXT("path"), LogEntry.Conflict.RemoteOperation.Path);
        RemoteOperationObject->SetStringField(TEXT("value"), LogEntry.Conflict.RemoteOperation.Value);
        RemoteOperationObject->SetStringField(TEXT("fromPath"), LogEntry.Conflict.RemoteOperation.FromPath);
        RemoteOperationObject->SetStringField(TEXT("timestamp"), LogEntry.Conflict.RemoteOperation.Timestamp.ToString());
        
        ConflictObject->SetObjectField(TEXT("localOperation"), LocalOperationObject);
        ConflictObject->SetObjectField(TEXT("remoteOperation"), RemoteOperationObject);
        
        JsonObject->SetObjectField(TEXT("conflict"), ConflictObject);
    }
    
    return JsonObject;
}

bool FJsonCRDTDefaultLogger::JsonToLogEntry(const TSharedPtr<FJsonObject>& JsonObject, FJsonCRDTLogEntry& OutLogEntry) const
{
    if (!JsonObject.IsValid())
    {
        return false;
    }
    
    // 기본 필드 파싱
    JsonObject->TryGetStringField(TEXT("logId"), OutLogEntry.LogID);
    JsonObject->TryGetStringField(TEXT("documentId"), OutLogEntry.DocumentID);
    JsonObject->TryGetStringField(TEXT("operationType"), OutLogEntry.OperationType);
    JsonObject->TryGetStringField(TEXT("path"), OutLogEntry.Path);
    JsonObject->TryGetStringField(TEXT("oldValue"), OutLogEntry.OldValue);
    JsonObject->TryGetStringField(TEXT("newValue"), OutLogEntry.NewValue);
    
    // 타임스탬프 파싱
    FString TimestampString;
    if (JsonObject->TryGetStringField(TEXT("timestamp"), TimestampString))
    {
        FDateTime::Parse(TimestampString, OutLogEntry.Timestamp);
    }
    
    JsonObject->TryGetBoolField(TEXT("hadConflict"), OutLogEntry.bHadConflict);
    JsonObject->TryGetStringField(TEXT("clientId"), OutLogEntry.ClientID);
    JsonObject->TryGetStringField(TEXT("source"), OutLogEntry.Source);
    
    // 충돌 정보 파싱
    if (OutLogEntry.bHadConflict)
    {
        const TSharedPtr<FJsonObject>* ConflictObject;
        if (JsonObject->TryGetObjectField(TEXT("conflict"), ConflictObject))
        {
            (*ConflictObject)->TryGetStringField(TEXT("path"), OutLogEntry.Conflict.Path);
            (*ConflictObject)->TryGetStringField(TEXT("localValue"), OutLogEntry.Conflict.LocalValue);
            (*ConflictObject)->TryGetStringField(TEXT("remoteValue"), OutLogEntry.Conflict.RemoteValue);
            (*ConflictObject)->TryGetStringField(TEXT("resolvedValue"), OutLogEntry.Conflict.ResolvedValue);
            (*ConflictObject)->TryGetBoolField(TEXT("resolved"), OutLogEntry.Conflict.bResolved);
            
            // 로컬 작업 정보 파싱
            const TSharedPtr<FJsonObject>* LocalOperationObject;
            if ((*ConflictObject)->TryGetObjectField(TEXT("localOperation"), LocalOperationObject))
            {
                FString TypeString;
                if ((*LocalOperationObject)->TryGetStringField(TEXT("type"), TypeString))
                {
                    OutLogEntry.Conflict.LocalOperation.Type = static_cast<EJsonCRDTOperationType>(FCString::Atoi(*TypeString));
                }
                
                (*LocalOperationObject)->TryGetStringField(TEXT("path"), OutLogEntry.Conflict.LocalOperation.Path);
                (*LocalOperationObject)->TryGetStringField(TEXT("value"), OutLogEntry.Conflict.LocalOperation.Value);
                (*LocalOperationObject)->TryGetStringField(TEXT("fromPath"), OutLogEntry.Conflict.LocalOperation.FromPath);
                
                FString OperationTimestampString;
                if ((*LocalOperationObject)->TryGetStringField(TEXT("timestamp"), OperationTimestampString))
                {
                    FDateTime::Parse(OperationTimestampString, OutLogEntry.Conflict.LocalOperation.Timestamp);
                }
            }
            
            // 원격 작업 정보 파싱
            const TSharedPtr<FJsonObject>* RemoteOperationObject;
            if ((*ConflictObject)->TryGetObjectField(TEXT("remoteOperation"), RemoteOperationObject))
            {
                FString TypeString;
                if ((*RemoteOperationObject)->TryGetStringField(TEXT("type"), TypeString))
                {
                    OutLogEntry.Conflict.RemoteOperation.Type = static_cast<EJsonCRDTOperationType>(FCString::Atoi(*TypeString));
                }
                
                (*RemoteOperationObject)->TryGetStringField(TEXT("path"), OutLogEntry.Conflict.RemoteOperation.Path);
                (*RemoteOperationObject)->TryGetStringField(TEXT("value"), OutLogEntry.Conflict.RemoteOperation.Value);
                (*RemoteOperationObject)->TryGetStringField(TEXT("fromPath"), OutLogEntry.Conflict.RemoteOperation.FromPath);
                
                FString OperationTimestampString;
                if ((*RemoteOperationObject)->TryGetStringField(TEXT("timestamp"), OperationTimestampString))
                {
                    FDateTime::Parse(OperationTimestampString, OutLogEntry.Conflict.RemoteOperation.Timestamp);
                }
            }
        }
    }
    
    return true;
}
