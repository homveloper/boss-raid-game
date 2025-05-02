#pragma once

#include "CoreMinimal.h"
#include "LuvJsonCRDT.generated.h"

// Forward declarations
class FLuvJsonDocument;

/**
 * Operation types for CRDT operations
 */
UENUM(BlueprintType)
enum class ELuvJsonOperationType : uint8
{
    Add UMETA(DisplayName = "Add"),
    Remove UMETA(DisplayName = "Remove"),
    Replace UMETA(DisplayName = "Replace")
};

/**
 * A single CRDT operation
 */
USTRUCT(BlueprintType)
struct FLuvJsonOperation
{
    GENERATED_BODY()

    /** The type of operation */
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "LuvJson")
    ELuvJsonOperationType Type;

    /** The path to the target location */
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "LuvJson")
    FString Path;

    /** The value to use for the operation (as a JSON string) */
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "LuvJson")
    FString Value;

    /** The timestamp of the operation */
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "LuvJson")
    int64 Timestamp;

    /** The ID of the client that created the operation */
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "LuvJson")
    FString ClientID;

    FLuvJsonOperation()
        : Type(ELuvJsonOperationType::Add)
        , Timestamp(0)
    {
    }
};

/**
 * A patch containing multiple CRDT operations
 */
USTRUCT(BlueprintType)
struct FLuvJsonPatch
{
    GENERATED_BODY()

    /** The ID of the document this patch applies to */
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "LuvJson")
    FString DocumentID;

    /** The version of the document this patch is based on */
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "LuvJson")
    int64 BaseVersion;

    /** The operations in this patch */
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "LuvJson")
    TArray<FLuvJsonOperation> Operations;

    /** The ID of the client that created the patch */
    UPROPERTY(EditAnywhere, BlueprintReadWrite, Category = "LuvJson")
    FString ClientID;

    FLuvJsonPatch()
        : BaseVersion(0)
    {
    }
};

/**
 * Delegate for document change events
 */
DECLARE_DYNAMIC_MULTICAST_DELEGATE_OneParam(FOnDocumentChanged, const FString&, DocumentID);

/**
 * LuvJsonDocument - A CRDT document that can be synchronized
 */
UCLASS(BlueprintType, Blueprintable)
class LUVJSON_API ULuvJsonDocument : public UObject
{
    GENERATED_BODY()

public:
    ULuvJsonDocument();
    virtual ~ULuvJsonDocument();

    /** Initialize the document with a unique ID */
    UFUNCTION(BlueprintCallable, Category = "LuvJson")
    void Initialize(const FString& InDocumentID);

    /** Get the document ID */
    UFUNCTION(BlueprintCallable, Category = "LuvJson")
    FString GetDocumentID() const;

    /** Get the document version */
    UFUNCTION(BlueprintCallable, Category = "LuvJson")
    int64 GetVersion() const;

    /** Get the document content as a JSON string */
    UFUNCTION(BlueprintCallable, Category = "LuvJson")
    FString GetContentAsString() const;

    /** Set the document content from a JSON string */
    UFUNCTION(BlueprintCallable, Category = "LuvJson")
    bool SetContentFromString(const FString& JsonString);

    /** Apply a patch to the document */
    UFUNCTION(BlueprintCallable, Category = "LuvJson")
    bool ApplyPatch(const FLuvJsonPatch& Patch);

    /** Create an operation */
    UFUNCTION(BlueprintCallable, Category = "LuvJson")
    FLuvJsonOperation CreateOperation(ELuvJsonOperationType Type, const FString& Path, const FString& Value, const FString& ClientID);

    /** Create a patch from operations */
    UFUNCTION(BlueprintCallable, Category = "LuvJson")
    FLuvJsonPatch CreatePatch(const TArray<FLuvJsonOperation>& Operations, const FString& ClientID);

    /** Event triggered when the document changes */
    UPROPERTY(BlueprintAssignable, Category = "LuvJson")
    FOnDocumentChanged OnDocumentChanged;

private:
    /** The native document implementation */
    TSharedPtr<FLuvJsonDocument> NativeDocument;
};

/**
 * LuvJsonClient - Main client for LuvJSON CRDT
 */
UCLASS(BlueprintType, Blueprintable)
class LUVJSON_API ULuvJsonClient : public UObject
{
    GENERATED_BODY()

public:
    ULuvJsonClient();
    virtual ~ULuvJsonClient();

    /** Initialize the client */
    UFUNCTION(BlueprintCallable, Category = "LuvJson")
    void Initialize();

    /** Create a new document */
    UFUNCTION(BlueprintCallable, Category = "LuvJson")
    ULuvJsonDocument* CreateDocument(const FString& DocumentID);

    /** Get a document by ID */
    UFUNCTION(BlueprintCallable, Category = "LuvJson")
    ULuvJsonDocument* GetDocument(const FString& DocumentID);

private:
    /** Map of document ID to document */
    UPROPERTY()
    TMap<FString, ULuvJsonDocument*> Documents;
};
