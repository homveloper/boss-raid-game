// Copyright Your Company. All Rights Reserved.

#pragma once

#include "CoreMinimal.h"
#include "JsonCRDTConflictResolver.h"

/**
 * 기본 충돌 해결 구현체
 */
class UEJSONCRDT_API FJsonCRDTDefaultConflictResolver : public IJsonCRDTConflictResolver
{
public:
    /**
     * 생성자
     * @param InStrategy 충돌 해결 전략
     */
    FJsonCRDTDefaultConflictResolver(EJsonCRDTConflictStrategy InStrategy = EJsonCRDTConflictStrategy::LastWriterWins);
    
    /**
     * 충돌 해결
     * @param Conflict 충돌 정보
     * @return 충돌 해결 여부
     */
    virtual bool ResolveConflict(FJsonCRDTConflict& Conflict) override;
    
    /**
     * 충돌 해결 전략 가져오기
     * @return 충돌 해결 전략
     */
    virtual EJsonCRDTConflictStrategy GetStrategy() const override;
    
    /**
     * 충돌 해결 전략 설정
     * @param InStrategy 충돌 해결 전략
     */
    void SetStrategy(EJsonCRDTConflictStrategy InStrategy);
    
private:
    /** 충돌 해결 전략 */
    EJsonCRDTConflictStrategy Strategy;
    
    /** 마지막 작성자 우선 전략으로 충돌 해결 */
    bool ResolveLastWriterWins(FJsonCRDTConflict& Conflict);
    
    /** 로컬 값 우선 전략으로 충돌 해결 */
    bool ResolveLocalWins(FJsonCRDTConflict& Conflict);
    
    /** 원격 값 우선 전략으로 충돌 해결 */
    bool ResolveRemoteWins(FJsonCRDTConflict& Conflict);
};
