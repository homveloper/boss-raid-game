// Copyright Your Company. All Rights Reserved.

#include "JsonCRDTDefaultConflictResolver.h"

FJsonCRDTDefaultConflictResolver::FJsonCRDTDefaultConflictResolver(EJsonCRDTConflictStrategy InStrategy)
    : Strategy(InStrategy)
{
}

bool FJsonCRDTDefaultConflictResolver::ResolveConflict(FJsonCRDTConflict& Conflict)
{
    switch (Strategy)
    {
        case EJsonCRDTConflictStrategy::LastWriterWins:
            return ResolveLastWriterWins(Conflict);
        
        case EJsonCRDTConflictStrategy::LocalWins:
            return ResolveLocalWins(Conflict);
        
        case EJsonCRDTConflictStrategy::RemoteWins:
            return ResolveRemoteWins(Conflict);
        
        case EJsonCRDTConflictStrategy::Custom:
            // 사용자 정의 전략은 기본 구현체에서 지원하지 않음
            UE_LOG(LogTemp, Warning, TEXT("Custom conflict resolution strategy not implemented in default resolver"));
            return false;
        
        default:
            UE_LOG(LogTemp, Error, TEXT("Unknown conflict resolution strategy"));
            return false;
    }
}

EJsonCRDTConflictStrategy FJsonCRDTDefaultConflictResolver::GetStrategy() const
{
    return Strategy;
}

void FJsonCRDTDefaultConflictResolver::SetStrategy(EJsonCRDTConflictStrategy InStrategy)
{
    Strategy = InStrategy;
}

bool FJsonCRDTDefaultConflictResolver::ResolveLastWriterWins(FJsonCRDTConflict& Conflict)
{
    // 타임스탬프 비교
    if (Conflict.LocalOperation.Timestamp > Conflict.RemoteOperation.Timestamp)
    {
        // 로컬 작업이 더 최신
        Conflict.ResolvedValue = Conflict.LocalValue;
    }
    else
    {
        // 원격 작업이 더 최신 또는 같은 시간
        Conflict.ResolvedValue = Conflict.RemoteValue;
    }
    
    Conflict.bResolved = true;
    return true;
}

bool FJsonCRDTDefaultConflictResolver::ResolveLocalWins(FJsonCRDTConflict& Conflict)
{
    // 항상 로컬 값 사용
    Conflict.ResolvedValue = Conflict.LocalValue;
    Conflict.bResolved = true;
    return true;
}

bool FJsonCRDTDefaultConflictResolver::ResolveRemoteWins(FJsonCRDTConflict& Conflict)
{
    // 항상 원격 값 사용
    Conflict.ResolvedValue = Conflict.RemoteValue;
    Conflict.bResolved = true;
    return true;
}
