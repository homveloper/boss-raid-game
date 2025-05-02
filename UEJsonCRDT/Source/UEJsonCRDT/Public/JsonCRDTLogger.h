// Copyright Your Company. All Rights Reserved.

#pragma once

#include "CoreMinimal.h"
#include "JsonCRDTTypes.h"
#include "JsonCRDTConflictResolver.h"
#include "JsonCRDTLogger.generated.h"

/**
 * 로그 항목 구조체
 */
USTRUCT(BlueprintType)
struct UEJSONCRDT_API FJsonCRDTLogEntry
{
    GENERATED_BODY()
    
    // 로그 ID
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString LogID;
    
    // 문서 ID
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString DocumentID;
    
    // 작업 유형
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString OperationType;
    
    // 작업 경로
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString Path;
    
    // 이전 값
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString OldValue;
    
    // 새 값
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString NewValue;
    
    // 타임스탬프
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FDateTime Timestamp;
    
    // 충돌 여부
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    bool bHadConflict = false;
    
    // 충돌 정보 (충돌이 있는 경우)
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FJsonCRDTConflict Conflict;
    
    // 클라이언트 ID
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString ClientID;
    
    // 작업 소스 (로컬 또는 원격)
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString Source;
};

/**
 * 로그 필터 구조체
 */
USTRUCT(BlueprintType)
struct UEJSONCRDT_API FJsonCRDTLogFilter
{
    GENERATED_BODY()
    
    // 문서 ID (비어있으면 모든 문서)
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString DocumentID;
    
    // 시작 시간
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FDateTime StartTime;
    
    // 종료 시간
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FDateTime EndTime;
    
    // 작업 유형 (비어있으면 모든 유형)
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString OperationType;
    
    // 경로 (비어있으면 모든 경로)
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString Path;
    
    // 충돌만 포함
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    bool bConflictsOnly = false;
    
    // 클라이언트 ID (비어있으면 모든 클라이언트)
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString ClientID;
    
    // 소스 (비어있으면 모든 소스)
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString Source;
};

/**
 * CRDT 로거 인터페이스
 */
class UEJSONCRDT_API IJsonCRDTLogger
{
public:
    virtual ~IJsonCRDTLogger() = default;
    
    /**
     * 작업 로깅
     * @param LogEntry 로그 항목
     */
    virtual void LogOperation(const FJsonCRDTLogEntry& LogEntry) = 0;
    
    /**
     * 로그 내보내기
     * @param FilePath 파일 경로
     * @param Filter 로그 필터
     * @return 내보내기 성공 여부
     */
    virtual bool ExportLogs(const FString& FilePath, const FJsonCRDTLogFilter& Filter = FJsonCRDTLogFilter()) = 0;
    
    /**
     * 로그 가져오기
     * @param Filter 로그 필터
     * @return 로그 항목 배열
     */
    virtual TArray<FJsonCRDTLogEntry> GetLogs(const FJsonCRDTLogFilter& Filter = FJsonCRDTLogFilter()) = 0;
    
    /**
     * 로그 지우기
     * @param Filter 로그 필터
     */
    virtual void ClearLogs(const FJsonCRDTLogFilter& Filter = FJsonCRDTLogFilter()) = 0;
    
    /**
     * 로그 활성화 여부 설정
     * @param bEnable 활성화 여부
     */
    virtual void SetLoggingEnabled(bool bEnable) = 0;
    
    /**
     * 로그 활성화 여부 가져오기
     * @return 활성화 여부
     */
    virtual bool IsLoggingEnabled() const = 0;
};
