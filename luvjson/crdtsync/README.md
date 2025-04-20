# LuvJSON CRDT 동기화

LuvJSON CRDT 동기화 패키지는 LuvJSON CRDT 문서의 분산 노드 간 동기화를 위한 기능을 제공합니다. 이 패키지는 go-ds-crdt의 동기화 메커니즘에서 영감을 받았으며, LuvJSON의 CRDT 구현에 맞게 조정되었습니다.

## 기능

- **분산 노드 간 동기화**: 여러 노드에서 동일한 CRDT 문서를 동기화
- **상태 기반 동기화**: 상태 벡터를 사용한 효율적인 동기화
- **PubSub 기반 브로드캐스팅**: 기존 crdtpubsub 패키지와 통합
- **피어 발견**: Redis 기반 자동 피어 발견
- **패치 저장소**: 패치 저장 및 검색 기능

## 사용법

### 기본 사용법

```go
// CRDT 문서 생성
doc := crdt.NewDocument(common.NewSessionID())

// PubSub 생성
pubsub, err := memory.NewPubSub()
if err != nil {
    log.Fatalf("Failed to create PubSub: %v", err)
}
defer pubsub.Close()

// 브로드캐스터 생성
broadcaster := crdtsync.NewPubSubBroadcaster(
    pubsub,
    "example-doc-patches",
    crdtpubsub.EncodingFormatJSON,
    doc.GetSessionID(),
)

// 패치 저장소 생성
patchStore := crdtsync.NewMemoryPatchStore()

// 상태 벡터 생성
stateVector := crdtsync.NewStateVector()

// 피어 발견 생성
peerDiscovery := crdtsync.NewRedisPeerDiscovery(redisClient, "example-doc", "node-1")
peerDiscovery.Start(ctx)
defer peerDiscovery.Close()

// 싱커 생성
syncer := crdtsync.NewPubSubSyncer(
    pubsub,
    "example-doc-sync",
    "node-1",
    stateVector,
    patchStore,
    crdtpubsub.EncodingFormatJSON,
)

// 동기화 매니저 생성
syncManager := crdtsync.NewSyncManager(doc, broadcaster, syncer, peerDiscovery, patchStore)

// 동기화 매니저 시작
if err := syncManager.Start(ctx); err != nil {
    log.Fatalf("Failed to start sync manager: %v", err)
}
defer syncManager.Stop()

// 패치 적용 및 브로드캐스트
patch := createPatch(doc)
if err := syncManager.ApplyPatch(ctx, patch); err != nil {
    log.Fatalf("Failed to apply patch: %v", err)
}
```

### Redis 기반 동기화

```go
// Redis 클라이언트 생성
redisClient := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

// Redis PubSub 생성
pubsub, err := redispubsub.NewRedisPubSub(redisClient, crdtpubsub.NewOptions())
if err != nil {
    log.Fatalf("Failed to create Redis PubSub: %v", err)
}
defer pubsub.Close()

// Redis 피어 발견 생성
peerDiscovery := crdtsync.NewRedisPeerDiscovery(redisClient, "example-doc", "node-1")
peerDiscovery.Start(ctx)
defer peerDiscovery.Close()

// 나머지는 기본 사용법과 동일
```

### API 모델과 함께 사용

```go
// API 모델 생성
model := api.NewModelWithDocument(doc)

// 문서 수정
model.GetApi().Root(map[string]interface{}{
    "title":    "Example Document",
    "content":  "Initial content",
    "authors":  []string{"user1"},
    "modified": time.Now().Format(time.RFC3339),
})

// 변경사항 플러시 및 브로드캐스트
patch := model.GetApi().Flush()
if err := syncManager.ApplyPatch(ctx, patch); err != nil {
    log.Fatalf("Failed to apply patch: %v", err)
}
```

## 아키텍처

### 주요 인터페이스

- **Broadcaster**: 노드 간 패치 전달을 위한 인터페이스
- **Syncer**: 노드 간 상태 동기화를 위한 인터페이스
- **SyncManager**: CRDT 문서의 동기화를 관리하는 인터페이스
- **PeerDiscovery**: 피어 발견을 위한 인터페이스
- **PatchStore**: 패치 저장소 인터페이스

### 동기화 흐름

1. **로컬 변경**: 로컬 노드에서 CRDT 문서 변경
2. **패치 생성**: 변경사항으로부터 패치 생성
3. **패치 적용**: 로컬 문서에 패치 적용
4. **패치 저장**: 패치 저장소에 패치 저장
5. **패치 브로드캐스트**: 다른 노드에 패치 브로드캐스트
6. **패치 수신**: 다른 노드에서 브로드캐스트된 패치 수신
7. **패치 적용**: 수신된 패치를 로컬 문서에 적용
8. **상태 동기화**: 주기적으로 다른 노드와 상태 동기화

### 상태 동기화 프로토콜

1. **상태 벡터 교환**: 노드 간 상태 벡터 교환
2. **누락된 패치 식별**: 상태 벡터 비교를 통해 누락된 패치 식별
3. **패치 요청**: 누락된 패치 요청
4. **패치 전송**: 요청된 패치 전송
5. **패치 적용**: 수신된 패치를 로컬 문서에 적용

## 예제

`examples/crdtsync/distributed_example.go`에서 전체 예제를 확인할 수 있습니다. 이 예제는 다음 기능을 보여줍니다:

- 메모리 또는 Redis 기반 PubSub 사용
- 문서 생성 및 수정
- 노드 간 동기화
- 사용자 입력 처리

## 향후 개선 사항

- **Merkle DAG 기반 동기화**: 더 효율적인 동기화를 위한 Merkle DAG 구현
- **충돌 해결 전략**: 고급 충돌 해결 전략 구현
- **보안**: 인증 및 암호화 지원
- **성능 최적화**: 대규모 문서 및 많은 노드에 대한 성능 최적화
