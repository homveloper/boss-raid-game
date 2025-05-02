// Copyright Your Company. All Rights Reserved.

#pragma once

#include "CoreMinimal.h"
#include "JsonCRDTLogger.h"
#include "JsonCRDTVisualizer.generated.h"

/**
 * CRDT 시각화 도구
 */
UCLASS(BlueprintType)
class UEJSONCRDT_API UJsonCRDTVisualizer : public UObject
{
    GENERATED_BODY()
    
public:
    /** 로그 데이터를 HTML 형식으로 내보내기 */
    UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
    bool ExportToHTML(const TArray<FJsonCRDTLogEntry>& LogEntries, const FString& FilePath);
    
    /** 로그 데이터를 CSV 형식으로 내보내기 */
    UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
    bool ExportToCSV(const TArray<FJsonCRDTLogEntry>& LogEntries, const FString& FilePath);
    
    /** 문서 변경 히스토리 시각화 */
    UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
    bool VisualizeDocumentHistory(const TArray<FJsonCRDTLogEntry>& LogEntries, const FString& FilePath);
    
    /** 충돌 시각화 */
    UFUNCTION(BlueprintCallable, Category = "JsonCRDT")
    bool VisualizeConflicts(const TArray<FJsonCRDTLogEntry>& LogEntries, const FString& FilePath);
    
private:
    /** HTML 헤더 생성 */
    FString GenerateHTMLHeader(const FString& Title);
    
    /** HTML 푸터 생성 */
    FString GenerateHTMLFooter();
    
    /** 로그 항목을 HTML 테이블 행으로 변환 */
    FString LogEntryToHTMLRow(const FJsonCRDTLogEntry& LogEntry);
    
    /** 로그 항목을 CSV 행으로 변환 */
    FString LogEntryToCSVRow(const FJsonCRDTLogEntry& LogEntry);
    
    /** 충돌을 HTML로 시각화 */
    FString ConflictToHTML(const FJsonCRDTConflict& Conflict);
    
    /** 문서 변경 히스토리를 HTML로 시각화 */
    FString DocumentHistoryToHTML(const TArray<FJsonCRDTLogEntry>& LogEntries);
};
