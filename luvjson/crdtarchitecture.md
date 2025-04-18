---
config:
  theme: mc
  look: neo
---
flowchart TD
 subgraph subGraph0["애플리케이션 레이어"]
        AppCode["애플리케이션 코드"]
        StateObj["게임 상태 객체\n(Go 구조체)"]
        BusinessRules["비즈니스 규칙 엔진"]
  end
 subgraph subGraph1["CRDT 트랜잭션 레이어"]
        TxCoordinator["CRDT TxCoordinator\n(트랜잭션 관리자)"]
        TxSession["트랜잭션 세션"]
        TxLog["트랜잭션 로그"]
        TxController["트랜잭션 제어기"]
        TxObserver["트랜잭션 옵저버"]
        TxSnapshot["트랜잭션 스냅샷"]
        TxTimeoutManager["트랜잭션 타임아웃 관리자"]
        TxMetrics["트랜잭션 메트릭스"]
        TxRetryPolicy["트랜잭션 재시도 정책"]
  end
 subgraph subGraph2["CRDT 코어"]
        OpCRDT["Operation-based CRDT\n(연산 기반 CRDT)"]
        DeltaMgr["델타 매니저"]
        PatchGen["패치 생성기"]
        ConflictResolver["충돌 해결기"]
        OpLog["연산 로그\n(Vector Clock)"]
        RecoveryMgr["복구 매니저"]
        InvariantChecker["불변성 검사기"]
        VectorClockFactory["벡터 클럭 팩토리"]
  end
 subgraph subGraph3["인터페이스 레이어"]
        RuleAdapter["규칙 어댑터"]
        ValidationHook["유효성 검증 훅"]
        RecoveryHook["복구 전략 훅"]
        CustomPropertyHook["커스텀 속성 훅"]
        BeforeTxHook["트랜잭션 시작 전 후크"]
        DuringTxHook["트랜잭션 진행 중 후크"]
        AfterTxHook["트랜잭션 종료 후 후크"]
        TxValidationHook["트랜잭션 유효성 검사 후크"]
        TxErrorHook["트랜잭션 오류 처리 후크"]
  end
 subgraph subGraph4["추적 시스템"]
        Tracker["CRDT Tracker\n(변경 감지)"]
        StructDiff["구조체 비교 엔진"]
        Tracer["CRDT Tracer\n(주기적 스냅샷)"]
        ExceptionMonitor["예외 모니터링"]
        MetricsCollector["메트릭스 수집기"]
        AuditLogger["감사 로깅"]
        PerformanceMonitor["성능 모니터링"]
  end
 subgraph subGraph5["통신 레이어"]
        PubSub["CRDT PUBSUB"]
        HookManager["훅 매니저"]
        Serializer["패치 직렬화"]
        FailureDetector["노드 실패 감지기"]
        TxDistributor["트랜잭션 배포기"]
        AckCollector["확인 수집기"]
        TxPropagator["트랜잭션 전파기"]
        StateSync["상태 동기화기"]
        NetworkRateLimiter["네트워크 속도 제한기"]
        ConnectionPool["연결 풀"]
  end
 subgraph subGraph6["패치 포맷"]
        JsonPatch["JSON Patch"]
        BinaryPatch["Binary Patch"]
        BsonPatch["BSON Patch"]
        MergePatch["Merge Patch"]
        CustomPatch["사용자 정의 패치"]
  end
 subgraph subGraph7["저장 레이어"]
        SnapshotStore["스냅샷 저장소"]
        OpLogStore["연산 로그 저장소"]
        StateStore["상태 저장소"]
        TxLogStore["트랜잭션 로그 저장소"]
        MetadataStore["메타데이터 저장소"]
  end
 subgraph s1["네트워크"]
        Server["서버 노드"]
        Client1["클라이언트 1"]
        Client2["클라이언트 2"]
        Node1["복제 노드 1"]
        Node2["복제 노드 2"]
  end
    AppCode -- 구조체 수정 --> StateObj
    StateObj -- 비즈니스 규칙 검증 --> BusinessRules
    BusinessRules -- 검증 통과 --> StateObj
    AppCode -- 트랜잭션 시작 --> TxCoordinator
    AppCode -- 변경 요청 --> Tracker
    TxCoordinator -- 세션 관리 --> TxSession
    TxCoordinator -- 로그 기록 --> TxLog
    TxCoordinator -- 트랜잭션 제어 --> TxController
    TxCoordinator -- 이벤트 발행 --> TxObserver
    TxCoordinator -- 스냅샷 관리 --> TxSnapshot
    TxCoordinator -- 타임아웃 설정 --> TxTimeoutManager
    TxCoordinator -- 메트릭스 수집 --> TxMetrics
    TxCoordinator -- 재시도 정책 적용 --> TxRetryPolicy
    TxSession -- 벡터 클럭 생성 --> VectorClockFactory
    TxController -- 연산 적용 --> OpCRDT
    TxSnapshot -- 상태 저장 --> SnapshotStore
    TxLog -- 로그 저장 --> TxLogStore
    TxController -- 충돌 해결 요청 --> ConflictResolver
    TxController -- 유효성 검증 요청 --> InvariantChecker
    OpCRDT -- 연산 기록 --> OpLog
    OpCRDT <-- 충돌 확인 --> ConflictResolver
    OpLog -- 로그 저장 --> OpLogStore
    InvariantChecker -- 복구 요청 --> RecoveryMgr
    RecoveryMgr -- 로그 기반 복구 --> OpLog
    VectorClockFactory -- 벡터 클럭 제공 --> OpLog
    StateObj -- 변경 감지 --> Tracker
    Tracker -- 구조체 비교 --> StructDiff
    Tracker -- 델타 생성 --> DeltaMgr
    Tracer -- 스냅샷 생성 --> SnapshotStore
    ExceptionMonitor -- 예외 기록 --> AuditLogger
    PerformanceMonitor -- 메트릭스 수집 --> MetricsCollector
    DeltaMgr -- 패치 생성 --> PatchGen
    PatchGen -- 직렬화 --> Serializer
    Serializer -- 패치 포맷 선택 --> JsonPatch & BinaryPatch & BsonPatch & MergePatch & CustomPatch
    JsonPatch -- 패치 전송 --> PubSub
    BinaryPatch -- 패치 전송 --> PubSub
    BsonPatch -- 패치 전송 --> PubSub
    MergePatch -- 패치 전송 --> PubSub
    CustomPatch -- 패치 전송 --> PubSub
    PubSub -- 훅 실행 --> HookManager
    PubSub -- 노드 상태 확인 --> FailureDetector
    PubSub -- 트랜잭션 배포 --> TxDistributor
    TxDistributor -- 전파 --> TxPropagator
    TxPropagator -- 상태 동기화 --> StateSync
    TxPropagator -- 확인 요청 --> AckCollector
    AckCollector -- 확인 결과 --> TxController
    TxPropagator -- 속도 제한 --> NetworkRateLimiter
    TxPropagator -- 연결 관리 --> ConnectionPool
    BusinessRules -- 규칙 등록 --> RuleAdapter
    RuleAdapter -- 규칙 변환 --> InvariantChecker
    BusinessRules -- 유효성 검증 요청 --> ValidationHook
    ValidationHook -- 검증 적용 --> InvariantChecker
    ExceptionMonitor -- 복구 전략 요청 --> RecoveryHook
    RecoveryHook -- 전략 적용 --> RecoveryMgr
    StateObj -- 커스텀 속성 등록 --> CustomPropertyHook
    CustomPropertyHook -- 속성 변환 --> Tracker
    TxCoordinator -- 시작 전 이벤트 --> BeforeTxHook
    TxCoordinator -- 진행 중 이벤트 --> DuringTxHook
    TxCoordinator -- 종료 후 이벤트 --> AfterTxHook
    TxCoordinator -- 유효성 검사 요청 --> TxValidationHook
    TxCoordinator -- 오류 처리 요청 --> TxErrorHook
    StateStore -- 상태 제공 --> OpCRDT
    SnapshotStore -- 스냅샷 제공 --> TxSnapshot
    OpLogStore -- 로그 제공 --> OpLog
    TxLogStore -- 트랜잭션 로그 제공 --> TxLog
    MetadataStore -- 메타데이터 제공 --> VectorClockFactory
    PubSub -- 배포 --> Server
    Server -- 델타 동기화 --> Client1 & Client2
    Server -- 상태 복제 --> Node1 & Node2
    Client1 -- 로컬 변경 델타 --> Server
    Client2 -- 로컬 변경 델타 --> Server
    Node1 -- 확인 응답 --> AckCollector
    Node2 -- 확인 응답 --> AckCollector
    subGraph2 --> n1["Untitled Node"]


    subgraph "CRDT Monitor"
        Monitor["CRDT Monitor\n(메인 컴포넌트)"]
        DataCollector["데이터 수집기"]
        MetricAnalyzer["메트릭 분석기"]
        AlertManager["알림 관리자"]
        DashboardGenerator["대시보드 생성기"]
        HistoricalStore["이력 저장소"]
        ConsistencyChecker["일관성 검사기"]
        AnomalyDetector["이상 감지기"]
        ReportEngine["리포트 엔진"]
    end

    %% 기존 CRDT 아키텍처와의 연결
    PubSub["CRDT PUBSUB"] -->|"변경 이벤트 구독"| DataCollector
    TxMetrics["트랜잭션 메트릭스"] -->|"성능 데이터 제공"| DataCollector
    OpLog["연산 로그\n(Vector Clock)"] -->|"연산 기록 제공"| ConsistencyChecker
    
    %% CRDT Monitor 내부 연결
    DataCollector -->|"데이터 전달"| MetricAnalyzer
    DataCollector -->|"일관성 검증 요청"| ConsistencyChecker
    DataCollector -->|"이력 저장"| HistoricalStore
    MetricAnalyzer -->|"이상 감지 요청"| AnomalyDetector
    AnomalyDetector -->|"알림 생성"| AlertManager
    MetricAnalyzer -->|"대시보드 데이터"| DashboardGenerator
    HistoricalStore -->|"이력 제공"| ReportEngine
    ConsistencyChecker -->|"일관성 문제 보고"| AlertManager 