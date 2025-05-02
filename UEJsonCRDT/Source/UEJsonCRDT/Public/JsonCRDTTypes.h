// Copyright Your Company. All Rights Reserved.

#pragma once

#include "CoreMinimal.h"
#include "JsonCRDTTypes.generated.h"

/**
 * Operation types for JSON CRDT
 */
UENUM(BlueprintType)
enum class EJsonCRDTOperationType : uint8
{
	Add UMETA(DisplayName = "Add"),
	Remove UMETA(DisplayName = "Remove"),
	Replace UMETA(DisplayName = "Replace"),
	Move UMETA(DisplayName = "Move"),
	Copy UMETA(DisplayName = "Copy"),
	Test UMETA(DisplayName = "Test")
};

/**
 * A single CRDT operation
 */
USTRUCT(BlueprintType)
struct UEJSONCRDT_API FJsonCRDTOperation
{
	GENERATED_BODY()

	/** The type of operation */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	EJsonCRDTOperationType Type;

	/** The path to the target location */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	FString Path;

	/** The path to the source location (for Move and Copy operations) */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	FString FromPath;

	/** The value to use for the operation (as a JSON string) */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	FString Value;

	/** The timestamp of the operation */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	FDateTime Timestamp;

	/** The ID of the client that created the operation */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	FString ClientID;

	FJsonCRDTOperation()
		: Type(EJsonCRDTOperationType::Add)
		, Timestamp(FDateTime::UtcNow())
	{
	}
};

/**
 * A patch containing multiple CRDT operations
 */
USTRUCT(BlueprintType)
struct UEJSONCRDT_API FJsonCRDTPatch
{
	GENERATED_BODY()

	/** The ID of the document this patch applies to */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	FString DocumentID;

	/** The version of the document this patch is based on */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	int64 BaseVersion;

	/** The operations in this patch */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	TArray<FJsonCRDTOperation> Operations;

	/** The timestamp of the patch */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	FDateTime Timestamp;

	/** The ID of the client that created the patch */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	FString ClientID;

	FJsonCRDTPatch()
		: BaseVersion(0)
		, Timestamp(FDateTime::UtcNow())
	{
	}
};

/**
 * A snapshot of a document at a specific point in time
 */
USTRUCT(BlueprintType)
struct UEJSONCRDT_API FJsonCRDTSnapshot
{
	GENERATED_BODY()

	/** The ID of the document */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	FString DocumentID;

	/** The version of the document */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	int64 Version;

	/** The timestamp of the snapshot */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	FDateTime Timestamp;

	/** The content of the document (as a JSON string) */
	UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "JsonCRDT")
	FString Content;

	FJsonCRDTSnapshot()
		: Version(0)
		, Timestamp(FDateTime::UtcNow())
	{
	}
};
