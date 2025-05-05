# Operation-based JSON CRDT 구현 로드맵 (Golang)

## 개요

이 문서는 Operation-based JSON CRDT(Conflict-Free Replicated Data Type)를 Golang으로 구현하기 위한 로드맵을 제시합니다. Operation-based CRDT는 작업(연산)을 전파하여 분산 환경에서 데이터 일관성을 유지하는 방식으로, 네트워크 대역폭을 효율적으로 사용하면서 강력한 최종 일관성(strong eventual consistency)을 보장합니다.

## 1. 기본 데이터 구조 설계 (2-3주)

### 1.1 식별자 및 하이브리드 논리적 시계

```go
// 논리적 타임스탬프 정의 (하이브리드 논리적 시계 사용)
type LogicalTimestamp struct {
    SessionID uint64  // 세션 식별자
    Counter   uint64  // 로컬 카운터
}

// 고유 식별자 정의 (각 노드는 고유한 식별자를 가짐)
type ID struct {
    Timestamp LogicalTimestamp  // 생성 시점의 타임스탬프
}

// 하이브리드 논리적 시계 구현
type HybridClock struct {
    SessionID uint64        // 세션 식별자 (랜덤 생성)
    Counter   uint64        // 논리적 카운터
    LastPhysicalTime int64  // 마지막 물리적 시간 (Unix 나노초)
    mu        sync.Mutex    // 동시성 제어를 위한 뮤텍스
}

// 새 타임스탬프 생성
func (hc *HybridClock) Now() LogicalTimestamp {
    hc.mu.Lock()
    defer hc.mu.Unlock()

    // 현재 물리적 시간 가져오기
    now := time.Now().UnixNano()

    // 물리적 시간이 진행되었으면 카운터 초기화
    if now > hc.LastPhysicalTime {
        hc.LastPhysicalTime = now
        hc.Counter = 0
    } else {
        // 같은 물리적 시간 내에서는 카운터 증가
        hc.Counter++
    }

    return LogicalTimestamp{
        SessionID: hc.SessionID,
        Counter:   hc.Counter,
    }
}

// 타임스탬프 비교 (인과성 확인)
func CompareTimestamps(a, b LogicalTimestamp) int {
    if a.SessionID != b.SessionID {
        if a.SessionID < b.SessionID {
            return -1
        }
        return 1
    }

    if a.Counter < b.Counter {
        return -1
    }
    if a.Counter > b.Counter {
        return 1
    }
    return 0
}

// 타임스탬프 병합 (두 시계 동기화)
func MergeTimestamps(local, remote LogicalTimestamp) LogicalTimestamp {
    if CompareTimestamps(local, remote) < 0 {
        return remote
    }
    return local
}
```

### 1.2 노드 타입 정의

```go
// 노드 타입 열거형
type NodeType string

const (
    ConstantNodeType  NodeType = "con"
    ValueNodeType     NodeType = "val"
    ObjectNodeType    NodeType = "obj"
    VectorNodeType    NodeType = "vec"
    StringNodeType    NodeType = "str"
    BinaryNodeType    NodeType = "bin"
    ArrayNodeType     NodeType = "arr"
)

// 노드 인터페이스 정의
type Node interface {
    GetID() ID
    GetType() NodeType
    Value() interface{}
    ApplyOperation(op Operation) (Node, error)
    // 기타 공통 메서드
}
```

### 1.3 구체적인 노드 구현

```go
// Constant 노드 구현
type ConstantNode struct {
    id    ID
    value interface{}  // 불변 값
}

// LWW-Value 노드 구현
type LWWValueNode struct {
    id    ID
    value ID  // 다른 노드를 가리키는 ID
    timestamp LogicalTimestamp
}

// LWW-Object 노드 구현
type LWWObjectNode struct {
    id    ID
    entries map[string]struct {
        nodeID ID
        timestamp LogicalTimestamp
    }
    tombstones map[string]LogicalTimestamp  // 삭제된 키 추적
}

// RGA-Array 노드 구현
type RGAArrayNode struct {
    id     ID
    elements []struct {
        nodeID ID
        isDeleted bool
        timestamp LogicalTimestamp
    }
}

// 기타 노드 타입 구현...
```

## 2. 작업(Operation) 정의 및 구현 (2-3주)

### 2.1 작업 타입 정의

```go
// 작업 타입 열거형
type OperationType string

const (
    InsertConstant OperationType = "ins_con"
    InsertValue    OperationType = "ins_val"
    InsertObject   OperationType = "ins_obj"
    InsertVector   OperationType = "ins_vec"
    InsertString   OperationType = "ins_str"
    InsertBinary   OperationType = "ins_bin"
    InsertArray    OperationType = "ins_arr"
    Delete         OperationType = "del"
)

// 작업 인터페이스
type Operation interface {
    GetType() OperationType
    GetTargetID() ID
    GetTimestamp() LogicalTimestamp
    // 기타 공통 메서드
}
```

### 2.2 구체적인 작업 구현

```go
// 상수 삽입 작업
type InsertConstantOperation struct {
    targetID  ID
    value     interface{}
    timestamp LogicalTimestamp
}

// 객체 키-값 삽입 작업
type InsertObjectOperation struct {
    targetID  ID
    key       string
    valueID   ID
    timestamp LogicalTimestamp
}

// 배열 요소 삽입 작업
type InsertArrayOperation struct {
    targetID  ID
    afterID   ID  // 이 요소 뒤에 삽입
    valueID   ID
    timestamp LogicalTimestamp
}

// 삭제 작업
type DeleteOperation struct {
    targetID  ID
    position  interface{}  // 객체의 키 또는 배열의 인덱스
    timestamp LogicalTimestamp
}

// 기타 작업 구현...
```

## 3. 문서 및 작업 관리 (2-3주)

### 3.1 문서 구조체와 편집기 인터페이스 분리

```go
// 문서 구조체 - 데이터 저장 및 표현만 담당
type Document struct {
    root      Node
    nodes     map[ID]Node
    clock     *HybridClock
    history   []Operation
}

// 문서 편집기 인터페이스 - 데이터 조작 담당
type DocumentEditor interface {
    // 기본 노드 생성 메서드
    MakeConstant(value interface{}) (ID, error)
    MakeObject() (ID, error)
    MakeArray() (ID, error)

    // 값 조작 메서드
    SetValue(targetID ID, valueID ID) error
    SetObjectKey(targetID ID, key string, valueID ID) error
    DeleteObjectKey(targetID ID, key string) error
    InsertArrayElement(targetID ID, afterID ID, valueID ID) error
    DeleteArrayElement(targetID ID, index int) error

    // 작업 관리 메서드
    GetDocument() *Document
    ApplyOperation(op Operation) error
    GenerateOperation(opType OperationType, params map[string]interface{}) (Operation, error)
}

// 옵저버 인터페이스
type Observer interface {
    OnOperation(op Operation)
}

// 옵저버 관리자 인터페이스
type ObserverManager interface {
    AddObserver(observer Observer)
    RemoveObserver(observer Observer)
    NotifyObservers(op Operation)
}
```

### 3.2 구현 클래스

```go
// 기본 문서 편집기 구현
type DefaultDocumentEditor struct {
    document      *Document
    observers     []Observer
    operationHandler OperationHandler
}

// 작업 핸들러 인터페이스 - 작업 처리 로직 캡슐화
type OperationHandler interface {
    HandleOperation(doc *Document, op Operation) error
    ValidateOperation(doc *Document, op Operation) error
}

// 새 문서 생성
func NewDocument(sessionID uint64) *Document {
    // 하이브리드 시계 초기화
    clock := &HybridClock{
        SessionID: sessionID,
        Counter: 0,
        LastPhysicalTime: time.Now().UnixNano(),
    }

    return &Document{
        nodes: make(map[ID]Node),
        clock: clock,
        history: make([]Operation, 0),
    }
}

// 새 문서 편집기 생성
func NewDocumentEditor(doc *Document) *DefaultDocumentEditor {
    return &DefaultDocumentEditor{
        document: doc,
        observers: make([]Observer, 0),
        operationHandler: NewDefaultOperationHandler(),
    }
}

// DocumentEditor 인터페이스 구현
func (editor *DefaultDocumentEditor) MakeConstant(value interface{}) (ID, error) {
    // 상수 노드 생성 작업 구현
    timestamp := editor.document.clock.Now()
    id := ID{Timestamp: timestamp}

    // 작업 생성
    op, err := editor.GenerateOperation(InsertConstant, map[string]interface{}{
        "value": value,
        "id": id,
    })
    if err != nil {
        return ID{}, err
    }

    // 작업 적용
    if err := editor.ApplyOperation(op); err != nil {
        return ID{}, err
    }

    return id, nil
}

func (editor *DefaultDocumentEditor) SetObjectKey(targetID ID, key string, valueID ID) error {
    // 객체 키 설정 작업 구현
    op, err := editor.GenerateOperation(InsertObject, map[string]interface{}{
        "targetID": targetID,
        "key": key,
        "valueID": valueID,
    })
    if err != nil {
        return err
    }

    return editor.ApplyOperation(op)
}

// 기타 DocumentEditor 메서드 구현...

func (editor *DefaultDocumentEditor) GetDocument() *Document {
    return editor.document
}

func (editor *DefaultDocumentEditor) ApplyOperation(op Operation) error {
    // 1. 작업 유효성 검사
    if err := editor.operationHandler.ValidateOperation(editor.document, op); err != nil {
        return err
    }

    // 2. 작업을 문서에 적용
    if err := editor.operationHandler.HandleOperation(editor.document, op); err != nil {
        return err
    }

    // 3. 히스토리에 작업 추가
    editor.document.history = append(editor.document.history, op)

    // 4. 옵저버에게 작업 알림
    editor.NotifyObservers(op)

    return nil
}

func (editor *DefaultDocumentEditor) GenerateOperation(opType OperationType, params map[string]interface{}) (Operation, error) {
    // 작업 타입에 따라 적절한 작업 객체 생성
    timestamp := editor.document.clock.Now()

    switch opType {
    case InsertConstant:
        value := params["value"]
        id := params["id"].(ID)
        return &InsertConstantOperation{
            targetID: ID{},  // 루트 또는 지정된 타겟
            value: value,
            timestamp: timestamp,
        }, nil
    case InsertObject:
        targetID := params["targetID"].(ID)
        key := params["key"].(string)
        valueID := params["valueID"].(ID)
        return &InsertObjectOperation{
            targetID: targetID,
            key: key,
            valueID: valueID,
            timestamp: timestamp,
        }, nil
    // 기타 작업 타입 처리...
    default:
        return nil, fmt.Errorf("unknown operation type: %v", opType)
    }
}

// ObserverManager 인터페이스 구현
func (editor *DefaultDocumentEditor) AddObserver(observer Observer) {
    editor.observers = append(editor.observers, observer)
}

func (editor *DefaultDocumentEditor) RemoveObserver(observer Observer) {
    for i, obs := range editor.observers {
        if obs == observer {
            editor.observers = append(editor.observers[:i], editor.observers[i+1:]...)
            break
        }
    }
}

func (editor *DefaultDocumentEditor) NotifyObservers(op Operation) {
    for _, observer := range editor.observers {
        observer.OnOperation(op)
    }
}
```

## 4. 동시성 제어 및 충돌 해결 (3-4주)

### 4.1 하이브리드 논리적 시계 활용

```go
// 타임스탬프 생성
func (doc *Document) generateTimestamp() LogicalTimestamp {
    return doc.clock.Now()
}

// 타임스탬프 비교 및 병합
func (doc *Document) handleRemoteTimestamp(remoteTS LogicalTimestamp) {
    // 원격 타임스탬프와 로컬 타임스탬프 비교 및 필요시 동기화
    localTS := doc.clock.Now()
    if CompareTimestamps(remoteTS, localTS) > 0 {
        // 원격 타임스탬프가 더 최신이면 로컬 시계 조정
        doc.clock.LastPhysicalTime = time.Now().UnixNano()
        doc.clock.Counter = remoteTS.Counter + 1
    }
}

// 작업 수신 시 타임스탬프 처리
func (doc *Document) processReceivedOperation(op Operation) {
    // 원격 타임스탬프 처리
    doc.handleRemoteTimestamp(op.GetTimestamp())

    // 작업 적용
    doc.ApplyOperation(op)
}
```

### 4.2 충돌 해결 전략

```go
// LWW-Value 노드의 충돌 해결
func (node *LWWValueNode) ApplyOperation(op Operation) (Node, error) {
    if insOp, ok := op.(InsertValueOperation); ok {
        // 타임스탬프 비교를 통한 충돌 해결
        if CompareTimestamps(insOp.timestamp, node.timestamp) > 0 {
            node.value = insOp.valueID
            node.timestamp = insOp.timestamp
        }
        return node, nil
    }
    return nil, fmt.Errorf("unsupported operation for LWW-Value node")
}

// RGA-Array 노드의 충돌 해결
func (node *RGAArrayNode) ApplyOperation(op Operation) (Node, error) {
    switch op := op.(type) {
    case InsertArrayOperation:
        // 삽입 위치 찾기
        index := node.findPositionAfter(op.afterID)
        // 새 요소 삽입
        element := struct {
            nodeID ID
            isDeleted bool
            timestamp LogicalTimestamp
        }{
            nodeID: op.valueID,
            isDeleted: false,
            timestamp: op.timestamp,
        }
        // 타임스탬프 기반 순서 결정
        node.elements = append(node.elements[:index+1], node.elements[index:]...)
        node.elements[index+1] = element
        return node, nil
    case DeleteOperation:
        // 요소 찾기 및 논리적 삭제
        index := node.findElementIndex(op.position)
        if index >= 0 && CompareTimestamps(op.timestamp, node.elements[index].timestamp) > 0 {
            node.elements[index].isDeleted = true
        }
        return node, nil
    }
    return nil, fmt.Errorf("unsupported operation for RGA-Array node")
}
```

## 5. 네트워크 동기화 (2-3주)

### 5.1 작업 직렬화 및 역직렬화

```go
// 작업 직렬화
func SerializeOperation(op Operation) ([]byte, error) {
    // 구현
}

// 작업 역직렬화
func DeserializeOperation(data []byte) (Operation, error) {
    // 구현
}
```

### 5.2 동기화 관리자

```go
// 동기화 관리자
type SyncManager struct {
    doc *Document
    peers map[string]Peer
    incomingOps chan Operation
    outgoingOps chan Operation
    OnRemoteOperation func(Operation)  // 원격 작업 수신 시 콜백
}

// 피어 인터페이스
type Peer interface {
    SendOperation(op Operation) error
    ReceiveOperation() (Operation, error)
    Close() error
}

// 새 동기화 관리자 생성
func NewSyncManager(doc *Document) *SyncManager {
    return &SyncManager{
        doc: doc,
        peers: make(map[string]Peer),
        incomingOps: make(chan Operation, 100),
        outgoingOps: make(chan Operation, 100),
        OnRemoteOperation: nil,
    }
}

// Observer 인터페이스 구현 - 로컬 작업을 수신하여 전파
func (sm *SyncManager) OnOperation(op Operation) {
    sm.outgoingOps <- op
}

// 동기화 관리자 메서드
func (sm *SyncManager) Start() {
    go sm.processIncoming()
    go sm.processOutgoing()
}

func (sm *SyncManager) processIncoming() {
    for op := range sm.incomingOps {
        // 원격 작업 수신 콜백이 설정된 경우 호출
        if sm.OnRemoteOperation != nil {
            sm.OnRemoteOperation(op)
        }
    }
}

func (sm *SyncManager) processOutgoing() {
    for op := range sm.outgoingOps {
        for _, peer := range sm.peers {
            peer.SendOperation(op)
        }
    }
}

func (sm *SyncManager) AddPeer(id string, peer Peer) {
    sm.peers[id] = peer
    go sm.receivePeerOperations(id, peer)
}

func (sm *SyncManager) receivePeerOperations(id string, peer Peer) {
    for {
        op, err := peer.ReceiveOperation()
        if err != nil {
            // 오류 처리
            break
        }
        sm.incomingOps <- op
    }
}
```

## 6. 영속성 및 스냅샷 (2-3주)

### 6.1 문서 직렬화 및 역직렬화

```go
// 문서 직렬화
func (doc *Document) MarshalJSON() ([]byte, error) {
    // 구현
}

// 문서 역직렬화
func UnmarshalJSON(data []byte) (*Document, error) {
    // 구현
}

// 문서를 JSON으로 변환 (사용자 친화적 형식)
func (doc *Document) ToJSON() ([]byte, error) {
    // CRDT 문서를 일반 JSON으로 변환
    return json.Marshal(doc.ToNativeObject())
}

// 문서를 네이티브 Go 객체로 변환
func (doc *Document) ToNativeObject() interface{} {
    if doc.root == nil {
        return nil
    }

    return doc.nodeToNative(doc.root)
}

// 노드를 네이티브 Go 객체로 변환
func (doc *Document) nodeToNative(node Node) interface{} {
    switch node.GetType() {
    case ConstantNodeType:
        return node.Value()
    case ObjectNodeType:
        if objNode, ok := node.(*LWWObjectNode); ok {
            result := make(map[string]interface{})
            for key, entry := range objNode.entries {
                if childNode, exists := doc.nodes[entry.nodeID]; exists {
                    result[key] = doc.nodeToNative(childNode)
                }
            }
            return result
        }
    case ArrayNodeType:
        if arrNode, ok := node.(*RGAArrayNode); ok {
            var result []interface{}
            for _, elem := range arrNode.elements {
                if !elem.isDeleted {
                    if childNode, exists := doc.nodes[elem.nodeID]; exists {
                        result = append(result, doc.nodeToNative(childNode))
                    }
                }
            }
            return result
        }
    }
    return nil
}
```

### 6.2 스냅샷 및 작업 로그 관리

```go
// 스냅샷 생성
func (doc *Document) CreateSnapshot() ([]byte, error) {
    // 구현
}

// 스냅샷에서 문서 복원
func RestoreFromSnapshot(data []byte) (*Document, error) {
    // 구현
}

// 작업 로그 저장
func (doc *Document) SaveOperationLog(writer io.Writer) error {
    // 구현
}

// 작업 로그 로드 및 적용
func (doc *Document) LoadOperationLog(reader io.Reader) error {
    // 구현
}
```

## 7. 성능 최적화 (2-3주)

### 7.1 메모리 사용량 최적화

```go
// 가비지 컬렉션 - 더 이상 필요하지 않은 작업 및 노드 정리
func (doc *Document) CollectGarbage() {
    // 구현
}

// 작업 압축 - 여러 작업을 단일 작업으로 압축
func CompressOperations(ops []Operation) []Operation {
    // 구현
}
```

### 7.2 연산 최적화

```go
// 인덱싱 - 빠른 노드 검색을 위한 인덱스 구조
type DocumentIndex struct {
    nodesByType map[NodeType]map[ID]Node
    nodesByPath map[string]ID
}

// 인덱스 업데이트
func (idx *DocumentIndex) UpdateIndex(node Node, path string) {
    // 구현
}

// 경로로 노드 찾기
func (idx *DocumentIndex) FindNodeByPath(path string) (Node, bool) {
    // 구현
}
```

## 8. 테스트 및 벤치마킹 (2-3주)

### 8.1 단위 테스트

```go
func TestNodeOperations(t *testing.T) {
    // 구현
}

func TestDocumentOperations(t *testing.T) {
    // 구현
}

func TestConcurrentEdits(t *testing.T) {
    // 구현
}

func TestConflictResolution(t *testing.T) {
    // 구현
}
```

### 8.2 벤치마크

```go
func BenchmarkOperationApplication(b *testing.B) {
    // 구현
}

func BenchmarkSynchronization(b *testing.B) {
    // 구현
}

func BenchmarkSerialization(b *testing.B) {
    // 구현
}
```

## 9. 문서화 및 예제 (1-2주)

### 9.1 API 문서화

- 모든 공개 API에 대한 상세한 문서 작성
- 사용 예제 제공
- 설계 결정 및 알고리즘 설명

### 9.2 예제 애플리케이션

```go
// 간단한 협업 텍스트 에디터
func main() {
    // 문서 생성 (세션 ID는 랜덤하게 생성)
    sessionID := uint64(rand.Int63())
    doc := NewDocument(sessionID)

    // 문서 편집기 생성
    editor := NewDocumentEditor(doc)

    // 동기화 관리자 설정
    syncManager := NewSyncManager(doc)

    // 동기화 관리자를 옵저버로 등록
    editor.AddObserver(syncManager)

    // 피어 연결
    peer := NewWebSocketPeer("ws://example.com/sync")
    syncManager.AddPeer("server", peer)

    // 텍스트 편집
    rootID, _ := editor.MakeConstant("")
    rootObjID, _ := editor.MakeObject()
    editor.SetValue(doc.root.GetID(), rootObjID)

    // 텍스트 추가
    textID, _ := editor.MakeConstant("Hello, ")
    editor.SetObjectKey(rootObjID, "text", textID)

    // 다른 사용자의 편집 적용
    // 원격 작업 수신 시
    syncManager.OnRemoteOperation = func(op Operation) {
        // 원격 작업을 편집기를 통해 적용
        editor.ApplyOperation(op)
    }

    // 문서 상태 확인
    jsonData, _ := doc.ToJSON()
    fmt.Println(string(jsonData))
}
```

## 10. Tombstone 노드 처리 (2주)

### 10.1 Tombstone 구현

```go
// Tombstone 노드 정의
type TombstoneNode struct {
    id           ID
    originalType NodeType
    deletedAt    LogicalTimestamp
}

// Tombstone 생성
func CreateTombstone(node Node, timestamp LogicalTimestamp) *TombstoneNode {
    return &TombstoneNode{
        id:           node.GetID(),
        originalType: node.GetType(),
        deletedAt:    timestamp,
    }
}
```

### 10.2 가비지 컬렉션 메커니즘

```go
// Tombstone 가비지 컬렉션
func (doc *Document) CollectTombstones(minTimestamp LogicalTimestamp) {
    // 모든 복제본이 minTimestamp 이상의 타임스탬프를 가지고 있을 때
    // minTimestamp 이전에 생성된 tombstone을 제거
}

// Tombstone 압축
func CompressTombstones(nodes []Node) []Node {
    // 연속된 tombstone을 단일 범위 tombstone으로 압축
}
```

### 10.3 Tombstone 정책 구성

```go
// Tombstone 정책 구조체
type TombstonePolicy struct {
    RetentionPeriod time.Duration  // tombstone 유지 기간
    CompressionEnabled bool        // 압축 활성화 여부
    MaxTombstoneRatio float64      // 전체 노드 대비 최대 tombstone 비율
}

// 정책 기반 Tombstone 관리
func (doc *Document) ManageTombstones(policy TombstonePolicy) {
    // 정책에 따라 tombstone 관리
}
```

## 11. JSON CRDT Patch 및 Patch Builder (2-3주)

### 11.1 JSON CRDT Patch 정의

```go
// JSON CRDT Patch - 여러 작업을 그룹화하는 컨테이너
type CRDTPatch struct {
    Operations []Operation
    Metadata   map[string]interface{}
    Timestamp  LogicalTimestamp
}

// Patch 메서드
func NewCRDTPatch(timestamp LogicalTimestamp) *CRDTPatch {
    return &CRDTPatch{
        Operations: make([]Operation, 0),
        Metadata:   make(map[string]interface{}),
        Timestamp:  timestamp,
    }
}

// 작업 추가
func (p *CRDTPatch) AddOperation(op Operation) {
    p.Operations = append(p.Operations, op)
}

// 패치 적용
func (p *CRDTPatch) Apply(doc *Document) error {
    for _, op := range p.Operations {
        if err := doc.ApplyOperation(op); err != nil {
            return err
        }
    }
    return nil
}

// 패치 직렬화
func (p *CRDTPatch) MarshalJSON() ([]byte, error) {
    type patchJSON struct {
        Operations []map[string]interface{} `json:"ops"`
        Metadata   map[string]interface{}   `json:"meta,omitempty"`
        Timestamp  LogicalTimestamp         `json:"ts"`
    }

    ops := make([]map[string]interface{}, len(p.Operations))
    for i, op := range p.Operations {
        opMap := map[string]interface{}{
            "type": op.GetType(),
            "target": op.GetTargetID(),
            "ts": op.GetTimestamp(),
        }

        // 작업 타입별 추가 필드
        switch o := op.(type) {
        case *InsertConstantOperation:
            opMap["value"] = o.value
        case *InsertObjectOperation:
            opMap["key"] = o.key
            opMap["valueId"] = o.valueID
        case *InsertArrayOperation:
            opMap["afterId"] = o.afterID
            opMap["valueId"] = o.valueID
        case *DeleteOperation:
            opMap["position"] = o.position
        }

        ops[i] = opMap
    }

    return json.Marshal(patchJSON{
        Operations: ops,
        Metadata:   p.Metadata,
        Timestamp:  p.Timestamp,
    })
}

// 패치 역직렬화
func UnmarshalCRDTPatch(data []byte) (*CRDTPatch, error) {
    // 구현
}
```

### 11.2 Patch Builder 구현

```go
// Patch Builder - 패치 생성을 위한 유연한 인터페이스
type PatchBuilder struct {
    patch       *CRDTPatch
    document    *Document
    editor      *DefaultDocumentEditor
    idCache     map[string]ID  // 경로 -> ID 매핑 캐시
}

// 새 Patch Builder 생성
func NewPatchBuilder(doc *Document, editor *DefaultDocumentEditor) *PatchBuilder {
    return &PatchBuilder{
        patch:    NewCRDTPatch(doc.clock.Now()),
        document: doc,
        editor:   editor,
        idCache:  make(map[string]ID),
    }
}

// 경로로 노드 ID 찾기
func (pb *PatchBuilder) ResolveNodePath(path string) (ID, error) {
    // 캐시에서 ID 확인
    if id, ok := pb.idCache[path]; ok {
        return id, nil
    }

    // 경로 파싱 및 노드 탐색
    parts := strings.Split(path, ".")
    currentNode := pb.document.root

    for _, part := range parts {
        // 배열 인덱스 처리 (예: items[0])
        if idx := strings.Index(part, "["); idx >= 0 {
            key := part[:idx]
            indexStr := part[idx+1 : len(part)-1]
            index, err := strconv.Atoi(indexStr)
            if err != nil {
                return ID{}, fmt.Errorf("invalid array index: %s", indexStr)
            }

            // 객체에서 배열 노드 찾기
            objNode, ok := currentNode.(*LWWObjectNode)
            if !ok {
                return ID{}, fmt.Errorf("not an object node at path: %s", path)
            }

            entry, ok := objNode.entries[key]
            if !ok {
                return ID{}, fmt.Errorf("key not found: %s", key)
            }

            arrayNode, ok := pb.document.nodes[entry.nodeID].(*RGAArrayNode)
            if !ok {
                return ID{}, fmt.Errorf("not an array node at key: %s", key)
            }

            // 배열에서 인덱스로 요소 찾기
            validElements := 0
            for _, elem := range arrayNode.elements {
                if !elem.isDeleted {
                    if validElements == index {
                        currentNode = pb.document.nodes[elem.nodeID]
                        break
                    }
                    validElements++
                }
            }

            if validElements <= index {
                return ID{}, fmt.Errorf("array index out of bounds: %d", index)
            }
        } else {
            // 일반 객체 키 처리
            objNode, ok := currentNode.(*LWWObjectNode)
            if !ok {
                return ID{}, fmt.Errorf("not an object node at path: %s", path)
            }

            entry, ok := objNode.entries[part]
            if !ok {
                return ID{}, fmt.Errorf("key not found: %s", part)
            }

            currentNode = pb.document.nodes[entry.nodeID]
        }
    }

    // 결과 ID 캐싱
    id := currentNode.GetID()
    pb.idCache[path] = id
    return id, nil
}

// 상수 값 설정
func (pb *PatchBuilder) SetConstant(path string, value interface{}) error {
    // 부모 노드 ID 찾기
    parentPath := path[:strings.LastIndex(path, ".")]
    key := path[strings.LastIndex(path, ".")+1:]

    parentID, err := pb.ResolveNodePath(parentPath)
    if err != nil {
        return err
    }

    // 상수 노드 생성
    constID, err := pb.editor.MakeConstant(value)
    if err != nil {
        return err
    }

    // 작업 생성 및 패치에 추가
    op, err := pb.editor.GenerateOperation(InsertObject, map[string]interface{}{
        "targetID": parentID,
        "key": key,
        "valueID": constID,
    })
    if err != nil {
        return err
    }

    pb.patch.AddOperation(op)
    return nil
}

// 객체 키 설정
func (pb *PatchBuilder) SetObjectKey(path string, key string, valuePath string) error {
    // 대상 객체 ID 찾기
    objID, err := pb.ResolveNodePath(path)
    if err != nil {
        return err
    }

    // 값 노드 ID 찾기
    valueID, err := pb.ResolveNodePath(valuePath)
    if err != nil {
        return err
    }

    // 작업 생성 및 패치에 추가
    op, err := pb.editor.GenerateOperation(InsertObject, map[string]interface{}{
        "targetID": objID,
        "key": key,
        "valueID": valueID,
    })
    if err != nil {
        return err
    }

    pb.patch.AddOperation(op)
    return nil
}

// 배열에 요소 추가
func (pb *PatchBuilder) AppendArrayElement(path string, valuePath string) error {
    // 대상 배열 ID 찾기
    arrayID, err := pb.ResolveNodePath(path)
    if err != nil {
        return err
    }

    // 값 노드 ID 찾기
    valueID, err := pb.ResolveNodePath(valuePath)
    if err != nil {
        return err
    }

    // 배열의 마지막 요소 ID 찾기
    arrayNode, ok := pb.document.nodes[arrayID].(*RGAArrayNode)
    if !ok {
        return fmt.Errorf("not an array node at path: %s", path)
    }

    var lastElemID ID
    for i := len(arrayNode.elements) - 1; i >= 0; i-- {
        if !arrayNode.elements[i].isDeleted {
            lastElemID = arrayNode.elements[i].nodeID
            break
        }
    }

    // 작업 생성 및 패치에 추가
    op, err := pb.editor.GenerateOperation(InsertArray, map[string]interface{}{
        "targetID": arrayID,
        "afterID": lastElemID,
        "valueID": valueID,
    })
    if err != nil {
        return err
    }

    pb.patch.AddOperation(op)
    return nil
}

// 노드 삭제
func (pb *PatchBuilder) DeleteNode(path string) error {
    // 경로 파싱
    lastDotIdx := strings.LastIndex(path, ".")
    if lastDotIdx < 0 {
        return fmt.Errorf("invalid path: %s", path)
    }

    parentPath := path[:lastDotIdx]
    key := path[lastDotIdx+1:]

    // 부모 노드 ID 찾기
    parentID, err := pb.ResolveNodePath(parentPath)
    if err != nil {
        return err
    }

    // 작업 생성 및 패치에 추가
    op, err := pb.editor.GenerateOperation(Delete, map[string]interface{}{
        "targetID": parentID,
        "position": key,
    })
    if err != nil {
        return err
    }

    pb.patch.AddOperation(op)
    return nil
}

// 패치 완료 및 반환
func (pb *PatchBuilder) Build() *CRDTPatch {
    return pb.patch
}
```

### 11.3 Patch Builder 사용 예제

```go
// 패치 빌더 사용 예제
func ExamplePatchBuilder() {
    // 문서 및 편집기 생성
    doc := NewDocument(uint64(rand.Int63()))
    editor := NewDocumentEditor(doc)

    // 초기 문서 구조 생성
    rootObjID, _ := editor.MakeObject()
    editor.SetValue(doc.root.GetID(), rootObjID)

    userObjID, _ := editor.MakeObject()
    editor.SetObjectKey(rootObjID, "user", userObjID)

    nameID, _ := editor.MakeConstant("John Doe")
    editor.SetObjectKey(userObjID, "name", nameID)

    itemsArrID, _ := editor.MakeArray()
    editor.SetObjectKey(userObjID, "items", itemsArrID)

    // 패치 빌더 생성
    builder := NewPatchBuilder(doc, editor)

    // 여러 작업을 패치에 추가
    builder.SetConstant("user.age", 30)

    item1ID, _ := editor.MakeObject()
    item1NameID, _ := editor.MakeConstant("Item 1")
    editor.SetObjectKey(item1ID, "name", item1NameID)

    // 패치에 배열 요소 추가 작업 포함
    builder.AppendArrayElement("user.items", "item1")

    // 패치 빌드 및 직렬화
    patch := builder.Build()
    patchJSON, _ := patch.MarshalJSON()

    fmt.Println(string(patchJSON))

    // 패치 적용
    patch.Apply(doc)

    // 결과 확인
    docJSON, _ := doc.ToJSON()
    fmt.Println(string(docJSON))
}
```

## 12. 구현 일정

| 단계 | 기간 | 주요 작업 |
|-----|-----|---------|
| 1. 기본 데이터 구조 설계 | 2-3주 | 노드 타입, 식별자, 기본 구조 구현 |
| 2. 작업 정의 및 구현 | 2-3주 | 작업 타입, 작업 적용 로직 구현 |
| 3. 문서 및 작업 관리 | 2-3주 | 문서 구조체, 작업 관리 메서드 구현 |
| 4. 동시성 제어 및 충돌 해결 | 3-4주 | 벡터 시계, 충돌 해결 전략 구현 |
| 5. 네트워크 동기화 | 2-3주 | 작업 직렬화, 동기화 관리자 구현 |
| 6. 영속성 및 스냅샷 | 2-3주 | 문서 직렬화, 스냅샷 관리 구현 |
| 7. 성능 최적화 | 2-3주 | 메모리 사용량, 연산 최적화 |
| 8. 테스트 및 벤치마킹 | 2-3주 | 단위 테스트, 벤치마크 작성 |
| 9. 문서화 및 예제 | 1-2주 | API 문서, 예제 애플리케이션 작성 |
| 10. Tombstone 노드 처리 | 2주 | Tombstone 구현, 가비지 컬렉션 |
| 11. JSON CRDT Patch 및 Builder | 2-3주 | 패치 구조체, 빌더 패턴 구현 |

## 13. 결론

Operation-based JSON CRDT를 Golang으로 구현하는 것은 복잡한 작업이지만, 위 로드맵을 따라 단계적으로 접근하면 효율적으로 구현할 수 있습니다. 이 구현은 분산 환경에서 실시간 협업 애플리케이션을 개발하는 데 강력한 기반을 제공할 것입니다.

주요 이점:
- 강력한 최종 일관성 보장
- 네트워크 대역폭 효율적 사용
- 오프라인 작업 지원
- 충돌 자동 해결
- 유연한 패치 생성 및 적용

이 구현은 실시간 협업 에디터, 분산 데이터베이스, 오프라인 우선 모바일 애플리케이션, 게임 서버 데이터 동기화 등 다양한 분야에 적용할 수 있습니다.

### 13.1 아키텍처 요약

```
+------------------+     +------------------+     +------------------+
|                  |     |                  |     |                  |
|    Document      |     |  DocumentEditor  |     |   PatchBuilder   |
|                  |     |                  |     |                  |
+------------------+     +------------------+     +------------------+
| - 데이터 저장/표현  |     | - 데이터 조작     |     | - 경로 기반 접근   |
| - 노드 관리        |     | - 작업 생성/적용   |     | - 패치 생성       |
| - 시계 관리        |     | - 옵저버 관리     |     | - 다중 작업 그룹화 |
+------------------+     +------------------+     +------------------+
         ^                        ^                        |
         |                        |                        |
         |                        |                        v
+------------------+     +------------------+     +------------------+
|                  |     |                  |     |                  |
|   SyncManager    |     |    CRDTPatch     |<----| - 작업 그룹       |
|                  |     |                  |     | - 직렬화/역직렬화 |
+------------------+     +------------------+     | - 원자적 적용     |
| - 작업 전파       |     | - 작업 컨테이너   |     +------------------+
| - 피어 관리       |     | - 메타데이터      |
| - 원격 작업 수신   |     | - 타임스탬프      |
+------------------+     +------------------+
```

이 아키텍처는 관심사 분리 원칙을 따르며, 각 컴포넌트가 명확한 책임을 가지고 있습니다. Document는 데이터 저장과 표현을, DocumentEditor는 데이터 조작을, PatchBuilder는 사용자 친화적인 패치 생성을, CRDTPatch는 작업 그룹화를, SyncManager는 네트워크 동기화를 담당합니다. 이러한 구조는 확장성과 유지보수성을 높이며, 다양한 사용 사례에 유연하게 적용할 수 있습니다.
