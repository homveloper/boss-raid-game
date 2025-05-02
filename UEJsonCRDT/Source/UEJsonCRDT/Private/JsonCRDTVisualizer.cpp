// Copyright Your Company. All Rights Reserved.

#include "JsonCRDTVisualizer.h"
#include "Misc/FileHelper.h"
#include "Misc/Paths.h"
#include "HAL/PlatformFilemanager.h"

bool UJsonCRDTVisualizer::ExportToHTML(const TArray<FJsonCRDTLogEntry>& LogEntries, const FString& FilePath)
{
    // HTML 문서 생성
    FString HtmlContent = GenerateHTMLHeader(TEXT("CRDT Log Visualization"));
    
    // 테이블 시작
    HtmlContent += TEXT("<table class=\"table table-striped table-hover\">\n");
    HtmlContent += TEXT("<thead>\n");
    HtmlContent += TEXT("<tr>\n");
    HtmlContent += TEXT("<th>Timestamp</th>\n");
    HtmlContent += TEXT("<th>Document ID</th>\n");
    HtmlContent += TEXT("<th>Operation</th>\n");
    HtmlContent += TEXT("<th>Path</th>\n");
    HtmlContent += TEXT("<th>Old Value</th>\n");
    HtmlContent += TEXT("<th>New Value</th>\n");
    HtmlContent += TEXT("<th>Client ID</th>\n");
    HtmlContent += TEXT("<th>Source</th>\n");
    HtmlContent += TEXT("<th>Conflict</th>\n");
    HtmlContent += TEXT("</tr>\n");
    HtmlContent += TEXT("</thead>\n");
    HtmlContent += TEXT("<tbody>\n");
    
    // 각 로그 항목을 테이블 행으로 추가
    for (const FJsonCRDTLogEntry& LogEntry : LogEntries)
    {
        HtmlContent += LogEntryToHTMLRow(LogEntry);
    }
    
    // 테이블 종료
    HtmlContent += TEXT("</tbody>\n");
    HtmlContent += TEXT("</table>\n");
    
    // HTML 문서 종료
    HtmlContent += GenerateHTMLFooter();
    
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
    if (!FFileHelper::SaveStringToFile(HtmlContent, *FilePath))
    {
        UE_LOG(LogTemp, Error, TEXT("Failed to save HTML to file: %s"), *FilePath);
        return false;
    }
    
    UE_LOG(LogTemp, Log, TEXT("Exported %d log entries to HTML file: %s"), LogEntries.Num(), *FilePath);
    return true;
}

bool UJsonCRDTVisualizer::ExportToCSV(const TArray<FJsonCRDTLogEntry>& LogEntries, const FString& FilePath)
{
    // CSV 헤더
    FString CsvContent = TEXT("Timestamp,Document ID,Operation,Path,Old Value,New Value,Client ID,Source,Had Conflict\n");
    
    // 각 로그 항목을 CSV 행으로 추가
    for (const FJsonCRDTLogEntry& LogEntry : LogEntries)
    {
        CsvContent += LogEntryToCSVRow(LogEntry);
    }
    
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
    if (!FFileHelper::SaveStringToFile(CsvContent, *FilePath))
    {
        UE_LOG(LogTemp, Error, TEXT("Failed to save CSV to file: %s"), *FilePath);
        return false;
    }
    
    UE_LOG(LogTemp, Log, TEXT("Exported %d log entries to CSV file: %s"), LogEntries.Num(), *FilePath);
    return true;
}

bool UJsonCRDTVisualizer::VisualizeDocumentHistory(const TArray<FJsonCRDTLogEntry>& LogEntries, const FString& FilePath)
{
    // HTML 문서 생성
    FString HtmlContent = GenerateHTMLHeader(TEXT("Document History Visualization"));
    
    // 문서 변경 히스토리 시각화
    HtmlContent += DocumentHistoryToHTML(LogEntries);
    
    // HTML 문서 종료
    HtmlContent += GenerateHTMLFooter();
    
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
    if (!FFileHelper::SaveStringToFile(HtmlContent, *FilePath))
    {
        UE_LOG(LogTemp, Error, TEXT("Failed to save HTML to file: %s"), *FilePath);
        return false;
    }
    
    UE_LOG(LogTemp, Log, TEXT("Exported document history visualization to HTML file: %s"), *FilePath);
    return true;
}

bool UJsonCRDTVisualizer::VisualizeConflicts(const TArray<FJsonCRDTLogEntry>& LogEntries, const FString& FilePath)
{
    // 충돌이 있는 로그 항목만 필터링
    TArray<FJsonCRDTLogEntry> ConflictLogs;
    for (const FJsonCRDTLogEntry& LogEntry : LogEntries)
    {
        if (LogEntry.bHadConflict)
        {
            ConflictLogs.Add(LogEntry);
        }
    }
    
    // 충돌이 없으면 빈 파일 생성
    if (ConflictLogs.Num() == 0)
    {
        UE_LOG(LogTemp, Warning, TEXT("No conflicts found in the log entries"));
        
        // HTML 문서 생성
        FString HtmlContent = GenerateHTMLHeader(TEXT("Conflict Visualization"));
        HtmlContent += TEXT("<div class=\"alert alert-info\">No conflicts found in the log entries.</div>\n");
        HtmlContent += GenerateHTMLFooter();
        
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
        if (!FFileHelper::SaveStringToFile(HtmlContent, *FilePath))
        {
            UE_LOG(LogTemp, Error, TEXT("Failed to save HTML to file: %s"), *FilePath);
            return false;
        }
        
        return true;
    }
    
    // HTML 문서 생성
    FString HtmlContent = GenerateHTMLHeader(TEXT("Conflict Visualization"));
    
    // 충돌 개요
    HtmlContent += TEXT("<div class=\"alert alert-warning\">\n");
    HtmlContent += FString::Printf(TEXT("<h4>Found %d conflicts</h4>\n"), ConflictLogs.Num());
    HtmlContent += TEXT("</div>\n");
    
    // 각 충돌 시각화
    for (int32 i = 0; i < ConflictLogs.Num(); ++i)
    {
        const FJsonCRDTLogEntry& LogEntry = ConflictLogs[i];
        
        HtmlContent += TEXT("<div class=\"card mb-4\">\n");
        HtmlContent += TEXT("<div class=\"card-header\">\n");
        HtmlContent += FString::Printf(TEXT("<h5>Conflict #%d - %s</h5>\n"), i + 1, *LogEntry.Timestamp.ToString());
        HtmlContent += TEXT("</div>\n");
        HtmlContent += TEXT("<div class=\"card-body\">\n");
        
        // 충돌 정보
        HtmlContent += TEXT("<dl class=\"row\">\n");
        HtmlContent += FString::Printf(TEXT("<dt class=\"col-sm-3\">Document ID</dt><dd class=\"col-sm-9\">%s</dd>\n"), *LogEntry.DocumentID);
        HtmlContent += FString::Printf(TEXT("<dt class=\"col-sm-3\">Path</dt><dd class=\"col-sm-9\">%s</dd>\n"), *LogEntry.Path);
        HtmlContent += FString::Printf(TEXT("<dt class=\"col-sm-3\">Operation</dt><dd class=\"col-sm-9\">%s</dd>\n"), *LogEntry.OperationType);
        HtmlContent += FString::Printf(TEXT("<dt class=\"col-sm-3\">Client ID</dt><dd class=\"col-sm-9\">%s</dd>\n"), *LogEntry.ClientID);
        HtmlContent += TEXT("</dl>\n");
        
        // 충돌 시각화
        HtmlContent += ConflictToHTML(LogEntry.Conflict);
        
        HtmlContent += TEXT("</div>\n");
        HtmlContent += TEXT("</div>\n");
    }
    
    // HTML 문서 종료
    HtmlContent += GenerateHTMLFooter();
    
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
    if (!FFileHelper::SaveStringToFile(HtmlContent, *FilePath))
    {
        UE_LOG(LogTemp, Error, TEXT("Failed to save HTML to file: %s"), *FilePath);
        return false;
    }
    
    UE_LOG(LogTemp, Log, TEXT("Exported conflict visualization to HTML file: %s"), *FilePath);
    return true;
}

FString UJsonCRDTVisualizer::GenerateHTMLHeader(const FString& Title)
{
    FString Header = TEXT("<!DOCTYPE html>\n");
    Header += TEXT("<html lang=\"en\">\n");
    Header += TEXT("<head>\n");
    Header += TEXT("<meta charset=\"UTF-8\">\n");
    Header += TEXT("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n");
    Header += FString::Printf(TEXT("<title>%s</title>\n"), *Title);
    Header += TEXT("<link href=\"https://cdn.jsdelivr.net/npm/bootstrap@5.3.0-alpha1/dist/css/bootstrap.min.css\" rel=\"stylesheet\">\n");
    Header += TEXT("<style>\n");
    Header += TEXT("body { padding: 20px; }\n");
    Header += TEXT(".conflict-container { display: flex; justify-content: space-between; margin-bottom: 20px; }\n");
    Header += TEXT(".conflict-side { flex: 1; padding: 10px; border: 1px solid #ddd; border-radius: 5px; margin: 0 10px; }\n");
    Header += TEXT(".conflict-local { background-color: #f8f9fa; }\n");
    Header += TEXT(".conflict-remote { background-color: #f8f9fa; }\n");
    Header += TEXT(".conflict-resolved { background-color: #d1e7dd; }\n");
    Header += TEXT(".timeline { position: relative; margin: 20px 0; padding-left: 30px; }\n");
    Header += TEXT(".timeline-item { position: relative; margin-bottom: 20px; }\n");
    Header += TEXT(".timeline-item:before { content: ''; position: absolute; left: -30px; top: 0; width: 2px; height: 100%; background-color: #ddd; }\n");
    Header += TEXT(".timeline-item:after { content: ''; position: absolute; left: -36px; top: 0; width: 14px; height: 14px; border-radius: 50%; background-color: #007bff; }\n");
    Header += TEXT(".timeline-content { padding: 10px; border: 1px solid #ddd; border-radius: 5px; }\n");
    Header += TEXT("</style>\n");
    Header += TEXT("</head>\n");
    Header += TEXT("<body>\n");
    Header += TEXT("<div class=\"container\">\n");
    Header += FString::Printf(TEXT("<h1>%s</h1>\n"), *Title);
    Header += TEXT("<hr>\n");
    
    return Header;
}

FString UJsonCRDTVisualizer::GenerateHTMLFooter()
{
    FString Footer = TEXT("</div>\n");
    Footer += TEXT("<script src=\"https://cdn.jsdelivr.net/npm/bootstrap@5.3.0-alpha1/dist/js/bootstrap.bundle.min.js\"></script>\n");
    Footer += TEXT("</body>\n");
    Footer += TEXT("</html>\n");
    
    return Footer;
}

FString UJsonCRDTVisualizer::LogEntryToHTMLRow(const FJsonCRDTLogEntry& LogEntry)
{
    FString Row = TEXT("<tr");
    
    // 충돌이 있는 경우 행 강조
    if (LogEntry.bHadConflict)
    {
        Row += TEXT(" class=\"table-warning\"");
    }
    
    Row += TEXT(">\n");
    
    // 타임스탬프
    Row += FString::Printf(TEXT("<td>%s</td>\n"), *LogEntry.Timestamp.ToString());
    
    // 문서 ID
    Row += FString::Printf(TEXT("<td>%s</td>\n"), *LogEntry.DocumentID);
    
    // 작업 유형
    Row += FString::Printf(TEXT("<td>%s</td>\n"), *LogEntry.OperationType);
    
    // 경로
    Row += FString::Printf(TEXT("<td>%s</td>\n"), *LogEntry.Path);
    
    // 이전 값
    Row += FString::Printf(TEXT("<td><code>%s</code></td>\n"), *LogEntry.OldValue);
    
    // 새 값
    Row += FString::Printf(TEXT("<td><code>%s</code></td>\n"), *LogEntry.NewValue);
    
    // 클라이언트 ID
    Row += FString::Printf(TEXT("<td>%s</td>\n"), *LogEntry.ClientID);
    
    // 소스
    Row += FString::Printf(TEXT("<td>%s</td>\n"), *LogEntry.Source);
    
    // 충돌
    if (LogEntry.bHadConflict)
    {
        Row += TEXT("<td><span class=\"badge bg-warning\">Conflict</span></td>\n");
    }
    else
    {
        Row += TEXT("<td></td>\n");
    }
    
    Row += TEXT("</tr>\n");
    
    return Row;
}

FString UJsonCRDTVisualizer::LogEntryToCSVRow(const FJsonCRDTLogEntry& LogEntry)
{
    // CSV 필드 이스케이프
    auto EscapeCSV = [](const FString& Field) -> FString
    {
        FString Result = Field;
        Result.ReplaceInline(TEXT("\""), TEXT("\"\""));
        if (Result.Contains(TEXT(",")) || Result.Contains(TEXT("\"")) || Result.Contains(TEXT("\n")))
        {
            Result = FString::Printf(TEXT("\"%s\""), *Result);
        }
        return Result;
    };
    
    FString Row;
    
    // 타임스탬프
    Row += EscapeCSV(LogEntry.Timestamp.ToString()) + TEXT(",");
    
    // 문서 ID
    Row += EscapeCSV(LogEntry.DocumentID) + TEXT(",");
    
    // 작업 유형
    Row += EscapeCSV(LogEntry.OperationType) + TEXT(",");
    
    // 경로
    Row += EscapeCSV(LogEntry.Path) + TEXT(",");
    
    // 이전 값
    Row += EscapeCSV(LogEntry.OldValue) + TEXT(",");
    
    // 새 값
    Row += EscapeCSV(LogEntry.NewValue) + TEXT(",");
    
    // 클라이언트 ID
    Row += EscapeCSV(LogEntry.ClientID) + TEXT(",");
    
    // 소스
    Row += EscapeCSV(LogEntry.Source) + TEXT(",");
    
    // 충돌 여부
    Row += LogEntry.bHadConflict ? TEXT("Yes") : TEXT("No");
    
    Row += TEXT("\n");
    
    return Row;
}

FString UJsonCRDTVisualizer::ConflictToHTML(const FJsonCRDTConflict& Conflict)
{
    FString Html = TEXT("<div class=\"conflict-container\">\n");
    
    // 로컬 측
    Html += TEXT("<div class=\"conflict-side conflict-local\">\n");
    Html += TEXT("<h5>Local</h5>\n");
    Html += TEXT("<dl class=\"row\">\n");
    Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">Value</dt><dd class=\"col-sm-9\"><code>%s</code></dd>\n"), *Conflict.LocalValue);
    Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">Operation</dt><dd class=\"col-sm-9\">%s</dd>\n"), *FString::FromInt(static_cast<int32>(Conflict.LocalOperation.Type)));
    Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">Path</dt><dd class=\"col-sm-9\">%s</dd>\n"), *Conflict.LocalOperation.Path);
    Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">Timestamp</dt><dd class=\"col-sm-9\">%s</dd>\n"), *Conflict.LocalOperation.Timestamp.ToString());
    Html += TEXT("</dl>\n");
    Html += TEXT("</div>\n");
    
    // 원격 측
    Html += TEXT("<div class=\"conflict-side conflict-remote\">\n");
    Html += TEXT("<h5>Remote</h5>\n");
    Html += TEXT("<dl class=\"row\">\n");
    Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">Value</dt><dd class=\"col-sm-9\"><code>%s</code></dd>\n"), *Conflict.RemoteValue);
    Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">Operation</dt><dd class=\"col-sm-9\">%s</dd>\n"), *FString::FromInt(static_cast<int32>(Conflict.RemoteOperation.Type)));
    Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">Path</dt><dd class=\"col-sm-9\">%s</dd>\n"), *Conflict.RemoteOperation.Path);
    Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">Timestamp</dt><dd class=\"col-sm-9\">%s</dd>\n"), *Conflict.RemoteOperation.Timestamp.ToString());
    Html += TEXT("</dl>\n");
    Html += TEXT("</div>\n");
    
    // 해결된 값
    Html += TEXT("<div class=\"conflict-side conflict-resolved\">\n");
    Html += TEXT("<h5>Resolved</h5>\n");
    Html += TEXT("<dl class=\"row\">\n");
    Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">Value</dt><dd class=\"col-sm-9\"><code>%s</code></dd>\n"), *Conflict.ResolvedValue);
    Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">Resolved</dt><dd class=\"col-sm-9\">%s</dd>\n"), Conflict.bResolved ? TEXT("Yes") : TEXT("No"));
    Html += TEXT("</dl>\n");
    Html += TEXT("</div>\n");
    
    Html += TEXT("</div>\n");
    
    return Html;
}

FString UJsonCRDTVisualizer::DocumentHistoryToHTML(const TArray<FJsonCRDTLogEntry>& LogEntries)
{
    // 문서 ID별로 로그 항목 그룹화
    TMap<FString, TArray<FJsonCRDTLogEntry>> DocumentLogs;
    for (const FJsonCRDTLogEntry& LogEntry : LogEntries)
    {
        TArray<FJsonCRDTLogEntry>& Logs = DocumentLogs.FindOrAdd(LogEntry.DocumentID);
        Logs.Add(LogEntry);
    }
    
    FString Html;
    
    // 각 문서에 대한 타임라인 생성
    for (const auto& Pair : DocumentLogs)
    {
        const FString& DocumentID = Pair.Key;
        const TArray<FJsonCRDTLogEntry>& Logs = Pair.Value;
        
        Html += FString::Printf(TEXT("<h2>Document: %s</h2>\n"), *DocumentID);
        Html += TEXT("<div class=\"timeline\">\n");
        
        // 시간순으로 정렬된 로그 항목
        TArray<FJsonCRDTLogEntry> SortedLogs = Logs;
        SortedLogs.Sort([](const FJsonCRDTLogEntry& A, const FJsonCRDTLogEntry& B) {
            return A.Timestamp < B.Timestamp;
        });
        
        // 각 로그 항목을 타임라인 항목으로 추가
        for (const FJsonCRDTLogEntry& LogEntry : SortedLogs)
        {
            Html += TEXT("<div class=\"timeline-item\">\n");
            Html += FString::Printf(TEXT("<div class=\"timeline-date\">%s</div>\n"), *LogEntry.Timestamp.ToString());
            
            Html += TEXT("<div class=\"timeline-content\">\n");
            
            // 작업 정보
            Html += FString::Printf(TEXT("<h5>%s</h5>\n"), *LogEntry.OperationType);
            Html += TEXT("<dl class=\"row\">\n");
            Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">Path</dt><dd class=\"col-sm-9\">%s</dd>\n"), *LogEntry.Path);
            Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">Old Value</dt><dd class=\"col-sm-9\"><code>%s</code></dd>\n"), *LogEntry.OldValue);
            Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">New Value</dt><dd class=\"col-sm-9\"><code>%s</code></dd>\n"), *LogEntry.NewValue);
            Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">Client ID</dt><dd class=\"col-sm-9\">%s</dd>\n"), *LogEntry.ClientID);
            Html += FString::Printf(TEXT("<dt class=\"col-sm-3\">Source</dt><dd class=\"col-sm-9\">%s</dd>\n"), *LogEntry.Source);
            Html += TEXT("</dl>\n");
            
            // 충돌 정보
            if (LogEntry.bHadConflict)
            {
                Html += TEXT("<div class=\"alert alert-warning\">\n");
                Html += TEXT("<h6>Conflict Detected</h6>\n");
                Html += ConflictToHTML(LogEntry.Conflict);
                Html += TEXT("</div>\n");
            }
            
            Html += TEXT("</div>\n");
            Html += TEXT("</div>\n");
        }
        
        Html += TEXT("</div>\n");
    }
    
    return Html;
}
