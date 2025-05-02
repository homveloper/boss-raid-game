// Copyright Your Company. All Rights Reserved.

#pragma once

#include "CoreMinimal.h"
#include "JsonCRDTLogger.h"

/**
 * 기본 CRDT 로거 구현체
 */
class UEJSONCRDT_API FJsonCRDTDefaultLogger : public IJsonCRDTLogger
{
public:
    /**
     * 생성자
     * @param InMaxLogEntries 최대 로그 항목 수
     */
    FJsonCRDTDefaultLogger(int32 InMaxLogEntries = 1000);
    
    /**
     * 작업 로깅
     * @param LogEntry 로그 항목
     */
    virtual void LogOperation(const FJsonCRDTLogEntry& LogEntry) override;
    
    /**
     * 로그 내보내기
     * @param FilePath 파일 경로
     * @param Filter 로그 필터
     * @return 내보내기 성공 여부
     */
    virtual bool ExportLogs(const FString& FilePath, const FJsonCRDTLogFilter& Filter = FJsonCRDTLogFilter()) override;
    
    /**
     * 로그 가져오기
     * @param Filter 로그 필터
     * @return 로그 항목 배열
     */
    virtual TArray<FJsonCRDTLogEntry> GetLogs(const FJsonCRDTLogFilter& Filter = FJsonCRDTLogFilter()) override;
    
    /**
     * 로그 지우기
     * @param Filter 로그 필터
     */
    virtual void ClearLogs(const FJsonCRDTLogFilter& Filter = FJsonCRDTLogFilter()) override;
    
    /**
     * 로그 활성화 여부 설정
     * @param bEnable 활성화 여부
     */
    virtual void SetLoggingEnabled(bool bEnable) override;
    
    /**
     * 로그 활성화 여부 가져오기
     * @return 활성화 여부
     */
    virtual bool IsLoggingEnabled() const override;
    
    /**
     * 최대 로그 항목 수 설정
     * @param InMaxLogEntries 최대 로그 항목 수
     */
    void SetMaxLogEntries(int32 InMaxLogEntries);
    
    /**
     * 최대 로그 항목 수 가져오기
     * @return 최대 로그 항목 수
     */
    int32 GetMaxLogEntries() const;
    
private:
    /** 로그 항목 배열 */
    TArray<FJsonCRDTLogEntry> LogEntries;
    
    /** 로그 항목 최대 개수 */
    int32 MaxLogEntries;
    
    /** 로깅 활성화 여부 */
    bool bLoggingEnabled;
    
    /** 로그 필터 적용 */
    bool ApplyFilter(const FJsonCRDTLogEntry& LogEntry, const FJsonCRDTLogFilter& Filter) const;
    
    /** 로그를 JSON 형식으로 변환 */
    TSharedPtr<FJsonObject> LogEntryToJson(const FJsonCRDTLogEntry& LogEntry) const;
    
    /** JSON을 로그 형식으로 변환 */
    bool JsonToLogEntry(const TSharedPtr<FJsonObject>& JsonObject, FJsonCRDTLogEntry& OutLogEntry) const;
};
