# LuvJSON Client SDK

LuvJSON Client SDK는 Go로 작성된 CRDT 기반 JSON 문서 동기화 라이브러리입니다. 이 SDK는 WebAssembly를 통해 JavaScript와 언리얼 엔진에서 사용할 수 있습니다.

## 특징

- **CRDT 기반 동기화**: 충돌 없는 데이터 동기화
- **크로스 플랫폼**: JavaScript와 언리얼 엔진 지원
- **경량**: 최소한의 의존성으로 설계
- **확장 가능**: 다양한 환경에 맞게 확장 가능

## 구조

```
luvjson-client-sdk/
├── src/                  # 소스 코드
│   ├── core/             # 핵심 CRDT 구현
│   ├── bindings/         # WASM 바인딩
│   ├── js/               # JavaScript 래퍼
│   └── ue/               # 언리얼 엔진 래퍼
├── dist/                 # 빌드 결과물
│   ├── wasm/             # WASM 모듈
│   ├── js/               # JavaScript 라이브러리
│   └── ue/               # 언리얼 엔진 플러그인
├── examples/             # 예제 코드
│   ├── js/               # JavaScript 예제
│   └── ue/               # 언리얼 엔진 예제
├── docs/                 # 문서
├── tests/                # 테스트
└── build.sh              # 빌드 스크립트
```

## 빌드 방법

### 요구 사항

- Go 1.16 이상
- 언리얼 엔진 4.26 이상 (언리얼 엔진 플러그인 사용 시)

### 빌드 명령

```bash
# 모든 플랫폼용 빌드
./build.sh

# JavaScript용 빌드
GOOS=js GOARCH=wasm go build -o ./dist/wasm/luvjson.wasm ./src/bindings
```

## JavaScript에서 사용하기

```html
<!DOCTYPE html>
<html>
<head>
    <title>LuvJSON Example</title>
    <script src="wasm_exec.js"></script>
    <script src="luvjson.js"></script>
    <script>
        async function init() {
            const client = new LuvJsonClient({
                wasmUrl: '/wasm/luvjson.wasm'
            });
            
            // 문서 생성
            const result = await client.createDocument('doc1');
            if (result.success) {
                console.log('Document created:', result.id);
            }
            
            // 작업 생성
            const opResult = await client.createOperation('add', 'title', 'Hello, World!', 'client1');
            if (opResult.success) {
                console.log('Operation created:', opResult.operation);
                
                // 패치 생성
                const patchResult = await client.createPatch('doc1', [opResult.operation], 'client1');
                if (patchResult.success) {
                    console.log('Patch created:', patchResult.patch);
                    
                    // 패치 적용
                    const applyResult = await client.applyPatch('doc1', patchResult.patch);
                    if (applyResult.success) {
                        console.log('Patch applied, new version:', applyResult.version);
                        
                        // 문서 내용 가져오기
                        const contentResult = await client.getDocumentContent('doc1');
                        if (contentResult.success) {
                            console.log('Document content:', contentResult.content);
                        }
                    }
                }
            }
        }
    </script>
</head>
<body onload="init()">
    <h1>LuvJSON Example</h1>
</body>
</html>
```

## 언리얼 엔진에서 사용하기

1. `dist/ue/LuvJsonCRDT` 폴더를 언리얼 엔진 프로젝트의 `Plugins` 폴더에 복사합니다.
2. 언리얼 에디터에서 플러그인을 활성화합니다.
3. 블루프린트 또는 C++에서 LuvJSON API를 사용합니다.

### C++ 예제

```cpp
#include "LuvJsonCRDT.h"

void YourClass::Example()
{
    // 클라이언트 생성
    ULuvJsonClient* Client = NewObject<ULuvJsonClient>();
    Client->Initialize();
    
    // 문서 생성
    ULuvJsonDocument* Document = Client->CreateDocument("doc1");
    
    // 문서 내용 설정
    Document->SetContentFromString("{\"title\":\"Hello, World!\"}");
    
    // 작업 생성
    FLuvJsonOperation Operation = Document->CreateOperation(
        ELuvJsonOperationType::Add,
        "description",
        "\"A sample document\"",
        "client1"
    );
    
    // 패치 생성 및 적용
    TArray<FLuvJsonOperation> Operations;
    Operations.Add(Operation);
    
    FLuvJsonPatch Patch = Document->CreatePatch(Operations, "client1");
    Document->ApplyPatch(Patch);
    
    // 문서 내용 가져오기
    FString Content = Document->GetContentAsString();
    UE_LOG(LogTemp, Log, TEXT("Document content: %s"), *Content);
}
```

## 라이선스

MIT 라이선스
