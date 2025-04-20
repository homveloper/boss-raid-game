# LuvJSON CRDT Storage

LuvJSON CRDT Storage는 LuvJSON CRDT, CRDTSync, CRDTPubSub 패키지를 통합한 고수준 저장소 API입니다. 이 패키지는 저수준 CRDT 기능을 기반으로 자동 동기화 및 편집 관리를 제공합니다.

## 기능

- **문서 관리**: 문서 생성, 로드, 저장, 삭제 기능
- **자동 동기화**: 여러 노드 간 문서 자동 동기화
- **편집 관리**: 문서 편집 및 변경 추적
- **영구 저장소**: 메모리, 파일, Redis 기반 영구 저장소
- **이벤트 시스템**: 문서 변경 이벤트 처리
- **자동 저장**: 주기적인 문서 자동 저장

## 사용법

### 저장소 생성

```go
// 저장소 옵션 생성
options := crdtstorage.DefaultStorageOptions()
options.PubSubType = "redis"
options.RedisAddr = "localhost:6379"
options.PersistenceType = "redis"
options.AutoSave = true
options.AutoSaveInterval = time.Minute * 5

// 저장소 생성
storage, err := crdtstorage.NewStorage(ctx, options)
if err != nil {
    log.Fatalf("Failed to create storage: %v", err)
}
defer storage.Close()
```

### 문서 생성

```go
// 문서 생성
doc, err := storage.CreateDocument(ctx, "example-doc")
if err != nil {
    log.Fatalf("Failed to create document: %v", err)
}

// 초기 문서 내용 설정
result := doc.Edit(ctx, func(api *api.ModelApi) error {
    api.Root(map[string]interface{}{
        "title":    "Example Document",
        "content":  "Initial content",
        "authors":  []string{"user1"},
        "modified": time.Now().Format(time.RFC3339),
    })
    return nil
})
if !result.Success {
    log.Fatalf("Failed to initialize document: %v", result.Error)
}
```

### 문서 로드

```go
// 문서 로드
doc, err := storage.GetDocument(ctx, "example-doc")
if err != nil {
    log.Fatalf("Failed to load document: %v", err)
}
```

### 수동 동기화

```go
// 특정 문서를 모든 피어와 동기화
if err := doc.Sync(ctx, ""); err != nil {
    log.Fatalf("Failed to sync document: %v", err)
}

// 특정 문서를 특정 피어와 동기화
if err := doc.Sync(ctx, "peer-123"); err != nil {
    log.Fatalf("Failed to sync document with peer: %v", err)
}

// 저장소의 모든 문서를 모든 피어와 동기화
if err := storage.SyncAllDocuments(ctx, ""); err != nil {
    log.Fatalf("Failed to sync all documents: %v", err)
}

// 저장소의 특정 문서를 모든 피어와 동기화
if err := storage.SyncDocument(ctx, "example-doc", ""); err != nil {
    log.Fatalf("Failed to sync document: %v", err)
}
```

### 문서 편집

```go
// 문서 편집
result := doc.Edit(ctx, func(api *api.ModelApi) error {
    // 현재 내용 가져오기
    currentContent, err := api.View()
    if err != nil {
        return fmt.Errorf("failed to get current content: %w", err)
    }

    // 맵으로 변환
    contentMap, ok := currentContent.(map[string]interface{})
    if !ok {
        return fmt.Errorf("content is not a map")
    }

    // 필드 업데이트
    contentMap["title"] = "New Title"
    contentMap["modified"] = time.Now().Format(time.RFC3339)

    // 루트 설정
    return api.Root(contentMap)
})

if !result.Success {
    log.Fatalf("Failed to edit document: %v", result.Error)
}
```

### 문서 내용 가져오기

```go
// 문서 내용 가져오기
var content MyDocumentType
if err := doc.GetContentAs(&content); err != nil {
    log.Fatalf("Failed to get document content: %v", err)
}
fmt.Printf("Title: %s\n", content.Title)
```

### 문서 변경 이벤트 처리

```go
// 문서 변경 콜백 등록
doc.OnChange(func(d *crdtstorage.Document, patch *crdtpatch.Patch) {
    fmt.Println("Document changed")
    // 변경 처리
})
```

### 문서 저장

```go
// 문서 저장
if err := doc.Save(ctx); err != nil {
    log.Fatalf("Failed to save document: %v", err)
}
```

### 문서 삭제

```go
// 문서 삭제
if err := storage.DeleteDocument(ctx, "example-doc"); err != nil {
    log.Fatalf("Failed to delete document: %v", err)
}
```

## 저장소 유형

### 메모리 저장소

메모리 저장소는 문서를 메모리에 저장합니다. 프로그램이 종료되면 모든 데이터가 손실됩니다.

```go
options := crdtstorage.DefaultStorageOptions()
options.PubSubType = "memory"
options.PersistenceType = "memory"
```

### 파일 저장소

파일 저장소는 문서를 파일 시스템에 저장합니다. 프로그램이 종료되어도 데이터가 유지됩니다.

```go
options := crdtstorage.DefaultStorageOptions()
options.PubSubType = "memory"
options.PersistenceType = "file"
options.PersistencePath = "documents"
```

### Redis 저장소

Redis 저장소는 문서를 Redis에 저장합니다. 여러 노드 간에 데이터를 공유할 수 있습니다.

```go
options := crdtstorage.DefaultStorageOptions()
options.PubSubType = "redis"
options.RedisAddr = "localhost:6379"
options.PersistenceType = "redis"
```

## 동기화 메커니즘

CRDTStorage는 다음과 같은 동기화 메커니즘을 사용합니다:

1. **PubSub 기반 브로드캐스팅**: 변경사항을 다른 노드에 브로드캐스트
2. **상태 벡터 기반 동기화**: 상태 벡터를 사용하여 누락된 변경사항 식별
3. **자동 피어 발견**: Redis를 사용하여 자동으로 피어 발견
4. **패치 저장소**: 패치를 저장하고 필요할 때 재전송

## 예제

`examples/crdtstorage` 디렉토리에 다음 예제가 있습니다:

- `simple_example.go`: 기본적인 문서 관리 예제
- `collaborative_example.go`: 여러 노드 간 협업 편집 예제

## 아키텍처

### 주요 인터페이스

- **Storage**: CRDT 문서 저장소 인터페이스
- **Document**: CRDT 문서 인터페이스
- **PersistenceProvider**: 영구 저장소 인터페이스

### 구현체

- **storageImpl**: Storage 인터페이스 구현체
- **MemoryPersistence**: 메모리 기반 영구 저장소
- **FilePersistence**: 파일 기반 영구 저장소
- **RedisPersistence**: Redis 기반 영구 저장소

### 문서 편집 흐름

1. **편집 함수 실행**: 사용자가 제공한 편집 함수 실행
2. **패치 생성**: 변경사항으로부터 패치 생성
3. **패치 적용**: 로컬 문서에 패치 적용
4. **패치 브로드캐스트**: 다른 노드에 패치 브로드캐스트
5. **변경 이벤트 발생**: 문서 변경 이벤트 발생

### 문서 동기화 흐름

1. **패치 수신**: 다른 노드에서 브로드캐스트된 패치 수신
2. **패치 적용**: 수신된 패치를 로컬 문서에 적용
3. **상태 동기화**: 주기적으로 다른 노드와 상태 동기화
4. **누락된 패치 요청**: 필요한 경우 누락된 패치 요청
5. **패치 재전송**: 요청된 패치 재전송
