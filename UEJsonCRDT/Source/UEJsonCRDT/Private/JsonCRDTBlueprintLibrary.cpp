// Copyright Your Company. All Rights Reserved.

#include "JsonCRDTBlueprintLibrary.h"
#include "JsonCRDTDocument.h"
#include "JsonCRDTSyncManager.h"
#include "JsonCRDTTransport.h"
#include "JsonObjectConverter.h"
#include "Serialization/JsonReader.h"
#include "Serialization/JsonSerializer.h"

UJsonCRDTSyncManager* UJsonCRDTBlueprintLibrary::CreateSyncManager(UObject* WorldContextObject, const FString& ServerURL, const FString& WebSocketURL)
{
	UJsonCRDTSyncManager* SyncManager = NewObject<UJsonCRDTSyncManager>(WorldContextObject);

	// ServerURL과 WebSocketURL이 모두 비어있지 않은 경우에만 초기화
	if (!ServerURL.IsEmpty() && !WebSocketURL.IsEmpty())
	{
		SyncManager->Initialize(ServerURL, WebSocketURL);
	}

	return SyncManager;
}

UJsonCRDTSyncManager* UJsonCRDTBlueprintLibrary::CreateSyncManagerWithTransport(UObject* WorldContextObject, TSharedPtr<IJsonCRDTTransport> Transport)
{
	UJsonCRDTSyncManager* SyncManager = NewObject<UJsonCRDTSyncManager>(WorldContextObject);

	// Transport가 유효한 경우에만 설정
	if (Transport.IsValid())
	{
		SyncManager->SetTransport(Transport);
	}

	return SyncManager;
}

UJsonCRDTDocument* UJsonCRDTBlueprintLibrary::CreateDocument(UObject* WorldContextObject, UJsonCRDTSyncManager* SyncManager, const FString& DocumentID)
{
	if (!SyncManager)
	{
		UE_LOG(LogTemp, Error, TEXT("Cannot create document with null sync manager"));
		return nullptr;
	}

	UJsonCRDTDocument* Document = NewObject<UJsonCRDTDocument>(WorldContextObject);
	Document->Initialize(DocumentID, SyncManager);
	return Document;
}

FJsonCRDTOperation UJsonCRDTBlueprintLibrary::CreateOperation(EJsonCRDTOperationType Type, const FString& Path, const FString& Value, const FString& FromPath)
{
	FJsonCRDTOperation Operation;
	Operation.Type = Type;
	Operation.Path = Path;
	Operation.Value = Value;
	Operation.FromPath = FromPath;
	Operation.Timestamp = FDateTime::UtcNow();
	return Operation;
}

FJsonCRDTPatch UJsonCRDTBlueprintLibrary::CreatePatch(const FString& DocumentID, int64 BaseVersion, const TArray<FJsonCRDTOperation>& Operations)
{
	FJsonCRDTPatch Patch;
	Patch.DocumentID = DocumentID;
	Patch.BaseVersion = BaseVersion;
	Patch.Operations = Operations;
	Patch.Timestamp = FDateTime::UtcNow();
	return Patch;
}

bool UJsonCRDTBlueprintLibrary::StringToJsonObject(const FString& JsonString, TSharedPtr<FJsonObject>& OutJsonObject)
{
	TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(JsonString);
	return FJsonSerializer::Deserialize(Reader, OutJsonObject);
}

bool UJsonCRDTBlueprintLibrary::JsonObjectToString(const TSharedPtr<FJsonObject>& JsonObject, FString& OutJsonString)
{
	if (!JsonObject.IsValid())
	{
		return false;
	}

	TSharedRef<TJsonWriter<>> Writer = TJsonWriterFactory<>::Create(&OutJsonString);
	return FJsonSerializer::Serialize(JsonObject.ToSharedRef(), Writer);
}

bool UJsonCRDTBlueprintLibrary::GetJsonValueByPath(const TSharedPtr<FJsonObject>& JsonObject, const FString& Path, FString& OutValue)
{
	if (!JsonObject.IsValid())
	{
		return false;
	}

	// Split the path into segments
	TArray<FString> PathSegments;
	Path.ParseIntoArray(PathSegments, TEXT("/"), true);

	// Remove empty segments
	for (int32 i = PathSegments.Num() - 1; i >= 0; --i)
	{
		if (PathSegments[i].IsEmpty())
		{
			PathSegments.RemoveAt(i);
		}
	}

	if (PathSegments.Num() == 0)
	{
		// Return the entire object as a string
		return JsonObjectToString(JsonObject, OutValue);
	}

	// Navigate through the path
	TSharedPtr<FJsonObject> CurrentObject = JsonObject;
	for (int32 i = 0; i < PathSegments.Num() - 1; ++i)
	{
		const FString& Segment = PathSegments[i];

		// Check if the segment is an array index
		int32 ArrayIndex = -1;
		if (Segment.IsNumeric() && Segment.IsNumeric())
		{
			ArrayIndex = FCString::Atoi(*Segment);
		}

		if (ArrayIndex >= 0)
		{
			// Get the array
			const TArray<TSharedPtr<FJsonValue>>* JsonArray;
			if (!CurrentObject->TryGetArrayField(Segment, JsonArray))
			{
				return false;
			}

			// Check if the index is valid
			if (ArrayIndex >= JsonArray->Num())
			{
				return false;
			}

			// Get the object at the index
			const TSharedPtr<FJsonValue>& JsonValue = (*JsonArray)[ArrayIndex];
			if (JsonValue->Type != EJson::Object)
			{
				return false;
			}

			CurrentObject = JsonValue->AsObject();
		}
		else
		{
			// Get the object field
			const TSharedPtr<FJsonObject>* NextObject;
			if (!CurrentObject->TryGetObjectField(Segment, NextObject))
			{
				return false;
			}

			CurrentObject = *NextObject;
		}
	}

	// Get the value at the final segment
	const FString& FinalSegment = PathSegments.Last();

	// Check if the final segment is an array index
	int32 ArrayIndex = -1;
	if (FinalSegment.IsNumeric())
	{
		ArrayIndex = FCString::Atoi(*FinalSegment);
	}

	if (ArrayIndex >= 0)
	{
		// Get the array
		const TArray<TSharedPtr<FJsonValue>>* JsonArray;
		if (!CurrentObject->TryGetArrayField(FinalSegment, JsonArray))
		{
			return false;
		}

		// Check if the index is valid
		if (ArrayIndex >= JsonArray->Num())
		{
			return false;
		}

		// Get the value at the index
		const TSharedPtr<FJsonValue>& JsonValue = (*JsonArray)[ArrayIndex];

		// Convert the value to a string
		if (JsonValue->Type == EJson::String)
		{
			OutValue = JsonValue->AsString();
		}
		else if (JsonValue->Type == EJson::Number)
		{
			OutValue = FString::Printf(TEXT("%f"), JsonValue->AsNumber());
		}
		else if (JsonValue->Type == EJson::Boolean)
		{
			OutValue = JsonValue->AsBool() ? TEXT("true") : TEXT("false");
		}
		else if (JsonValue->Type == EJson::Null)
		{
			OutValue = TEXT("null");
		}
		else
		{
			// Convert the value to a JSON string
			TSharedRef<TJsonWriter<>> Writer = TJsonWriterFactory<>::Create(&OutValue);
			FJsonSerializer::Serialize(JsonValue, Writer);
		}

		return true;
	}
	else
	{
		// Get the field value
		const TSharedPtr<FJsonValue>* JsonValue;
		if (!CurrentObject->TryGetField(FinalSegment, JsonValue))
		{
			return false;
		}

		// Convert the value to a string
		if ((*JsonValue)->Type == EJson::String)
		{
			OutValue = (*JsonValue)->AsString();
		}
		else if ((*JsonValue)->Type == EJson::Number)
		{
			OutValue = FString::Printf(TEXT("%f"), (*JsonValue)->AsNumber());
		}
		else if ((*JsonValue)->Type == EJson::Boolean)
		{
			OutValue = (*JsonValue)->AsBool() ? TEXT("true") : TEXT("false");
		}
		else if ((*JsonValue)->Type == EJson::Null)
		{
			OutValue = TEXT("null");
		}
		else
		{
			// Convert the value to a JSON string
			TSharedRef<TJsonWriter<>> Writer = TJsonWriterFactory<>::Create(&OutValue);
			FJsonSerializer::Serialize(*JsonValue, Writer);
		}

		return true;
	}
}

bool UJsonCRDTBlueprintLibrary::SetJsonValueByPath(TSharedPtr<FJsonObject>& JsonObject, const FString& Path, const FString& Value)
{
	if (!JsonObject.IsValid())
	{
		return false;
	}

	// Split the path into segments
	TArray<FString> PathSegments;
	Path.ParseIntoArray(PathSegments, TEXT("/"), true);

	// Remove empty segments
	for (int32 i = PathSegments.Num() - 1; i >= 0; --i)
	{
		if (PathSegments[i].IsEmpty())
		{
			PathSegments.RemoveAt(i);
		}
	}

	if (PathSegments.Num() == 0)
	{
		// Cannot set the entire object
		return false;
	}

	// Parse the value as JSON
	TSharedPtr<FJsonValue> JsonValue;
	TSharedRef<TJsonReader<>> Reader = TJsonReaderFactory<>::Create(Value);
	if (FJsonSerializer::Deserialize(Reader, JsonValue))
	{
		// Navigate through the path
		TSharedPtr<FJsonObject> CurrentObject = JsonObject;
		for (int32 i = 0; i < PathSegments.Num() - 1; ++i)
		{
			const FString& Segment = PathSegments[i];

			// Check if the segment is an array index
			int32 ArrayIndex = -1;
			if (Segment.IsNumeric())
			{
				ArrayIndex = FCString::Atoi(*Segment);
			}

			if (ArrayIndex >= 0)
			{
				// Get the array
				TArray<TSharedPtr<FJsonValue>>* JsonArray;
				if (!CurrentObject->TryGetArrayField(Segment, JsonArray))
				{
					// Create the array if it doesn't exist
					JsonArray = &CurrentObject->SetArrayField(Segment);
				}

				// Ensure the array is large enough
				while (JsonArray->Num() <= ArrayIndex)
				{
					JsonArray->Add(MakeShared<FJsonValueObject>(MakeShared<FJsonObject>()));
				}

				// Get the object at the index
				TSharedPtr<FJsonValue>& ArrayValue = (*JsonArray)[ArrayIndex];
				if (ArrayValue->Type != EJson::Object)
				{
					// Replace with an object
					ArrayValue = MakeShared<FJsonValueObject>(MakeShared<FJsonObject>());
				}

				CurrentObject = ArrayValue->AsObject();
			}
			else
			{
				// Get the object field
				TSharedPtr<FJsonObject>* NextObject;
				if (!CurrentObject->TryGetObjectField(Segment, NextObject))
				{
					// Create the object if it doesn't exist
					NextObject = &CurrentObject->SetObjectField(Segment, MakeShared<FJsonObject>());
				}

				CurrentObject = *NextObject;
			}
		}

		// Set the value at the final segment
		const FString& FinalSegment = PathSegments.Last();

		// Check if the final segment is an array index
		int32 ArrayIndex = -1;
		if (FinalSegment.IsNumeric())
		{
			ArrayIndex = FCString::Atoi(*FinalSegment);
		}

		if (ArrayIndex >= 0)
		{
			// Get the array
			TArray<TSharedPtr<FJsonValue>>* JsonArray;
			if (!CurrentObject->TryGetArrayField(FinalSegment, JsonArray))
			{
				// Create the array if it doesn't exist
				JsonArray = &CurrentObject->SetArrayField(FinalSegment);
			}

			// Ensure the array is large enough
			while (JsonArray->Num() <= ArrayIndex)
			{
				JsonArray->Add(MakeShared<FJsonValueNull>());
			}

			// Set the value at the index
			(*JsonArray)[ArrayIndex] = JsonValue;
		}
		else
		{
			// Set the field value
			CurrentObject->SetField(FinalSegment, JsonValue);
		}

		return true;
	}
	else
	{
		// Treat the value as a string
		// Navigate through the path
		TSharedPtr<FJsonObject> CurrentObject = JsonObject;
		for (int32 i = 0; i < PathSegments.Num() - 1; ++i)
		{
			const FString& Segment = PathSegments[i];

			// Check if the segment is an array index
			int32 ArrayIndex = -1;
			if (Segment.IsNumeric())
			{
				ArrayIndex = FCString::Atoi(*Segment);
			}

			if (ArrayIndex >= 0)
			{
				// Get the array
				TArray<TSharedPtr<FJsonValue>>* JsonArray;
				if (!CurrentObject->TryGetArrayField(Segment, JsonArray))
				{
					// Create the array if it doesn't exist
					JsonArray = &CurrentObject->SetArrayField(Segment);
				}

				// Ensure the array is large enough
				while (JsonArray->Num() <= ArrayIndex)
				{
					JsonArray->Add(MakeShared<FJsonValueObject>(MakeShared<FJsonObject>()));
				}

				// Get the object at the index
				TSharedPtr<FJsonValue>& ArrayValue = (*JsonArray)[ArrayIndex];
				if (ArrayValue->Type != EJson::Object)
				{
					// Replace with an object
					ArrayValue = MakeShared<FJsonValueObject>(MakeShared<FJsonObject>());
				}

				CurrentObject = ArrayValue->AsObject();
			}
			else
			{
				// Get the object field
				TSharedPtr<FJsonObject>* NextObject;
				if (!CurrentObject->TryGetObjectField(Segment, NextObject))
				{
					// Create the object if it doesn't exist
					NextObject = &CurrentObject->SetObjectField(Segment, MakeShared<FJsonObject>());
				}

				CurrentObject = *NextObject;
			}
		}

		// Set the value at the final segment
		const FString& FinalSegment = PathSegments.Last();

		// Check if the final segment is an array index
		int32 ArrayIndex = -1;
		if (FinalSegment.IsNumeric())
		{
			ArrayIndex = FCString::Atoi(*FinalSegment);
		}

		if (ArrayIndex >= 0)
		{
			// Get the array
			TArray<TSharedPtr<FJsonValue>>* JsonArray;
			if (!CurrentObject->TryGetArrayField(FinalSegment, JsonArray))
			{
				// Create the array if it doesn't exist
				JsonArray = &CurrentObject->SetArrayField(FinalSegment);
			}

			// Ensure the array is large enough
			while (JsonArray->Num() <= ArrayIndex)
			{
				JsonArray->Add(MakeShared<FJsonValueNull>());
			}

			// Set the value at the index
			(*JsonArray)[ArrayIndex] = MakeShared<FJsonValueString>(Value);
		}
		else
		{
			// Set the field value
			CurrentObject->SetStringField(FinalSegment, Value);
		}

		return true;
	}
}
