// Copyright Your Company. All Rights Reserved.

#pragma once

#include "CoreMinimal.h"
#include "JsonCRDTTypes.h"
#include "JsonCRDTConflictResolver.generated.h"

/**
 * 충돌 정보 구조체
 */
USTRUCT(BlueprintType)
struct UEJSONCRDT_API FJsonCRDTConflict
{
    GENERATED_BODY()
    
    // 충돌이 발생한 경로
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString Path;
    
    // 로컬 값
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString LocalValue;
    
    // 원격 값
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString RemoteValue;
    
    // 로컬 작업
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FJsonCRDTOperation LocalOperation;
    
    // 원격 작업
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FJsonCRDTOperation RemoteOperation;
    
    // 충돌 해결 결과 (기본값은 원격 값 우선)
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    FString ResolvedValue;
    
    // 충돌 해결 여부
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
    bool bResolved = false;
};

/**
 * 충돌 해결 전략 열거형
 */
UENUM(BlueprintType)
enum class EJsonCRDTConflictStrategy : uint8
{
    // 마지막 작성자 우선 (Last Writer Wins)
    LastWriterWins UMETA(DisplayName = "Last Writer Wins"),
    
    // 로컬 값 우선
    LocalWins UMETA(DisplayName = "Local Wins"),
    
    // 원격 값 우선
    RemoteWins UMETA(DisplayName = "Remote Wins"),
    
    // 사용자 정의 전략
    Custom UMETA(DisplayName = "Custom")
};

/**
 * 충돌 해결 인터페이스
 */
class UEJSONCRDT_API IJsonCRDTConflictResolver
{
public:
    virtual ~IJsonCRDTConflictResolver() = default;
    
    /**
     * 충돌 해결
     * @param Conflict 충돌 정보
     * @return 충돌 해결 여부
     */
    virtual bool ResolveConflict(FJsonCRDTConflict& Conflict) = 0;
    
    /**
     * 충돌 해결 전략 가져오기
     * @return 충돌 해결 전략
     */
    virtual EJsonCRDTConflictStrategy GetStrategy() const = 0;
};
