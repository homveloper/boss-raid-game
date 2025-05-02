# UEJsonCRDT - Unreal Engine JSON CRDT Plugin

UEJsonCRDT는 언리얼 엔진에서 CRDT(Conflict-free Replicated Data Type) 기반의 JSON 문서 동기화를 위한 클라이언트 사이드 SDK입니다. 이 플러그인은 CRDT 데이터 병합 및 동기화에 집중하며, 서버-클라이언트 간 통신 방식은 사용자가 자유롭게 구현할 수 있도록 설계되었습니다.

## 주요 기능

- **CRDT 기반 동기화**: 델타 연산을 병합하여 문서 동기화
- **데이터 충돌 해결**: 여러 클라이언트의 동시 편집 충돌 자동 해결
- **스냅샷 관리**: 문서 상태 스냅샷 생성 및 복원
- **로컬 저장소**: 문서를 로컬에 저장하여 오프라인 상태에서도 접근 가능
- **오류 복구**: 문서 손상 시 로컬 저장소나 스냅샷에서 복구
- **확장 가능한 통신 계층**: 사용자 정의 통신 구현체를 통한 유연한 서버 연결
- **블루프린트 지원**: 블루프린트에서도 쉽게 사용 가능

## 설치 방법

1. 이 저장소를 클론하거나 다운로드합니다.
2. `UEJsonCRDT` 폴더를 언리얼 엔진 프로젝트의 `Plugins` 폴더에 복사합니다.
3. 언리얼 엔진 프로젝트를 열고 플러그인을 활성화합니다.

## 아키텍처

UEJsonCRDT는 다음과 같은 주요 컴포넌트로 구성됩니다:

1. **JsonCRDTDocument**: CRDT 문서를 나타내며, 문서 내용 관리 및 로컬 저장을 담당합니다.
2. **JsonCRDTSyncManager**: 문서 동기화 및 CRDT 연산 처리를 담당합니다.
3. **IJsonCRDTTransport**: 서버와의 통신을 추상화한 인터페이스입니다. 사용자는 이 인터페이스를 구현하여 자신만의 통신 방식을 정의할 수 있습니다.

## 사용 방법

### C++에서 사용하기

#### 1. 사용자 정의 Transport 구현

```cpp
#include "JsonCRDTTransport.h"

// 사용자 정의 Transport 구현
class MyCustomTransport : public IJsonCRDTTransport
{
public:
    // 문서 로드 요청
    virtual void LoadDocument(const FString& DocumentID, const FOnDocumentLoaded& OnLoaded, const FOnTransportError& OnError) override
    {
        // 여기에 서버에서 문서를 로드하는 코드 구현
        // 예: HTTP 요청, 소켓 통신 등

        // 로드 성공 시 콜백 호출
        FJsonCRDTDocumentData DocumentData;
        DocumentData.DocumentID = DocumentID;
        DocumentData.Content = "{\"title\":\"Example Document\",\"content\":\"Hello, World!\"}";
        DocumentData.Version = 1;
        OnLoaded.ExecuteIfBound(DocumentData);
    }

    // 문서 저장 요청
    virtual void SaveDocument(const FJsonCRDTDocumentData& DocumentData, const FOnDocumentSaved& OnSaved, const FOnTransportError& OnError) override
    {
        // 여기에 서버에 문서를 저장하는 코드 구현

        // 저장 성공 시 콜백 호출
        OnSaved.ExecuteIfBound(DocumentData.DocumentID);
    }

    // 패치 전송
    virtual void SendPatch(const FJsonCRDTPatch& Patch, const FOnPatchSent& OnSent, const FOnTransportError& OnError) override
    {
        // 여기에 서버에 패치를 전송하는 코드 구현

        // 전송 성공 시 콜백 호출
        OnSent.ExecuteIfBound(Patch.DocumentID);
    }

    // 패치 수신 이벤트 등록
    virtual void RegisterPatchReceived(const FOnPatchReceived& OnPatchReceived) override
    {
        OnPatchReceivedDelegate = OnPatchReceived;
    }

    // 패치 수신 시뮬레이션 (실제 구현에서는 서버로부터 패치를 받았을 때 호출)
    void SimulatePatchReceived(const FJsonCRDTPatch& Patch)
    {
        if (OnPatchReceivedDelegate.IsBound())
        {
            OnPatchReceivedDelegate.Execute(Patch);
        }
    }

private:
    FOnPatchReceived OnPatchReceivedDelegate;
};
```

#### 2. SyncManager 및 Document 사용

```cpp
#include "JsonCRDTSyncManager.h"
#include "JsonCRDTDocument.h"
#include "JsonCRDTBlueprintLibrary.h"

// 사용자 정의 Transport 생성
TSharedPtr<MyCustomTransport> Transport = MakeShared<MyCustomTransport>();

// 동기화 관리자 생성
UJsonCRDTSyncManager* SyncManager = UJsonCRDTBlueprintLibrary::CreateSyncManager(this);
SyncManager->SetTransport(Transport);

// 문서 생성
UJsonCRDTDocument* Document = UJsonCRDTBlueprintLibrary::CreateDocument(this, SyncManager, "document-id");

// 자동 로컬 저장 활성화
Document->SetAutoLocalSave(true);

// 문서 내용 설정
Document->SetContentFromString("{\"title\":\"Example Document\",\"content\":\"Hello, World!\"}");

// 문서 저장 (로컬 및 서버)
Document->Save();

// 문서 내용 가져오기
FString Content = Document->GetContentAsString();

// 문서 편집
TSharedPtr<FJsonObject> JsonObject = Document->GetContent();
if (JsonObject.IsValid())
{
    JsonObject->SetStringField("content", "수정된 내용");
    Document->SetContent(JsonObject);
}

// 문서 저장
Document->Save();

// 문서 복구 이벤트 핸들러 등록
Document->OnDocumentRecovered.AddLambda([](const FString& DocumentID, const FString& RecoverySource) {
    UE_LOG(LogTemp, Log, TEXT("문서 %s가 %s에서 복구되었습니다"), *DocumentID, *RecoverySource);
});

// 문서 복구 시도
bool bRecovered = Document->RecoverDocument();

// 서버로부터 패치 수신 시뮬레이션
FJsonCRDTPatch IncomingPatch;
IncomingPatch.DocumentID = "document-id";
IncomingPatch.BaseVersion = 1;
// 패치에 필요한 작업 추가
Transport->SimulatePatchReceived(IncomingPatch);
```

### 블루프린트에서 사용하기

블루프린트에서도 UEJsonCRDT를 사용할 수 있습니다. 다만, 사용자 정의 Transport는 C++로 구현해야 합니다.

1. 사용자 정의 Transport 구현체 등록
   - C++에서 구현한 Transport 클래스를 블루프린트에서 사용할 수 있도록 등록합니다.
   - `Set Transport` 노드를 사용하여 SyncManager에 Transport를 설정합니다.

2. 동기화 관리자 생성
   - `Create Sync Manager` 노드를 사용하여 동기화 관리자를 생성합니다.

3. 문서 생성 및 설정
   - `Create Document` 노드를 사용하여 문서를 생성합니다.
   - 문서 ID와 동기화 관리자를 설정합니다.
   - `Set Auto Local Save` 노드를 사용하여 자동 로컬 저장을 활성화합니다.
   - `On Document Recovered` 이벤트에 핸들러를 연결하여 문서 복구를 감지합니다.

4. 문서 내용 설정 및 관리
   - `Set Content From String` 노드를 사용하여 문서 내용을 설정합니다.
   - `Save` 노드를 호출하여 문서를 저장합니다.
   - `Save Locally` 노드를 호출하여 문서를 로컬에만 저장합니다.
   - `Get Content As String` 노드를 사용하여 문서 내용을 가져옵니다.

5. 문서 복구
   - `Recover Document` 노드를 호출하여 손상된 문서를 복구합니다.

## 통신 인터페이스

UEJsonCRDT는 `IJsonCRDTTransport` 인터페이스를 통해 서버와의 통신을 추상화합니다. 이 인터페이스는 다음과 같은 주요 메서드를 포함합니다:

```cpp
// 문서 로드
virtual void LoadDocument(const FString& DocumentID, const FOnDocumentLoaded& OnLoaded, const FOnTransportError& OnError) = 0;

// 문서 저장
virtual void SaveDocument(const FJsonCRDTDocumentData& DocumentData, const FOnDocumentSaved& OnSaved, const FOnTransportError& OnError) = 0;

// 패치 전송
virtual void SendPatch(const FJsonCRDTPatch& Patch, const FOnPatchSent& OnSent, const FOnTransportError& OnError) = 0;

// 패치 수신 이벤트 등록
virtual void RegisterPatchReceived(const FOnPatchReceived& OnPatchReceived) = 0;
```

사용자는 이 인터페이스를 구현하여 HTTP, WebSocket, 또는 다른 통신 프로토콜을 사용하여 서버와 통신할 수 있습니다. 플러그인은 기본 구현체로 `FDefaultJsonCRDTTransport`를 제공하지만, 사용자는 자신의 비즈니스 로직에 맞는 구현체를 만들 수 있습니다.

## 예제

`UEJsonCRDT/Examples` 폴더에서 다양한 사용 예제를 확인할 수 있습니다:

- `BasicUsage.cpp`: 기본적인 사용 방법을 보여주는 예제
- `CustomTransport.cpp`: 사용자 정의 Transport 구현 및 사용 방법을 보여주는 예제
- `ApplyPatch.cpp`: 패치 적용 방법을 보여주는 예제
- `SnapshotExample.cpp`: 스냅샷 관리 방법을 보여주는 예제
- `LocalStorage.cpp`: 로컬 저장소 사용 방법을 보여주는 예제

## 확장 가능성

UEJsonCRDT는 다음과 같은 방식으로 확장할 수 있습니다:

1. **사용자 정의 Transport**: `IJsonCRDTTransport` 인터페이스를 구현하여 자신만의 통신 방식을 정의할 수 있습니다.
2. **사용자 정의 저장소**: 로컬 저장소 메커니즘을 확장하여 다른 저장 방식을 구현할 수 있습니다.
3. **사용자 정의 CRDT 알고리즘**: 필요에 따라 다른 CRDT 알고리즘을 구현하여 사용할 수 있습니다.

## 라이선스

이 프로젝트는 MIT 라이선스 하에 배포됩니다. 자세한 내용은 LICENSE 파일을 참조하세요.
