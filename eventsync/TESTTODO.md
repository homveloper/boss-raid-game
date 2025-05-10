# EventSync 테스트 계획서

이 문서는 EventSync 패키지의 테스트 계획을 설명합니다. 실제 MongoDB 인스턴스를 사용하여 테스트를 진행하며, 모킹 대신 실제 환경에 가까운 테스트를 수행합니다.

## 테스트 환경 설정

### 필요 조건

- MongoDB 인스턴스 (테스트용 별도 데이터베이스 사용)
- Go 테스트 환경
- 테스트 데이터 생성 스크립트

### 테스트 데이터베이스 설정

```go
func setupTestDB(t *testing.T) (*mongo.Client, *mongo.Database, func()) {
    // 테스트용 MongoDB 연결
    ctx := context.Background()
    client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
    require.NoError(t, err)
    
    // 테스트용 데이터베이스 이름 (고유한 이름 생성)
    dbName := fmt.Sprintf("eventsync_test_%d", time.Now().UnixNano())
    db := client.Database(dbName)
    
    // 정리 함수 반환
    cleanup := func() {
        err := db.Drop(ctx)
        assert.NoError(t, err)
        err = client.Disconnect(ctx)
        assert.NoError(t, err)
    }
    
    return client, db, cleanup
}
```

## 테스트 범주

### 1. 단위 테스트

각 컴포넌트의 개별 기능을 테스트합니다.

#### 1.1 이벤트 저장소 (EventStore)

- **테스트 대상**: `MongoEventStore` 구현체
- **테스트 방법**: 실제 MongoDB 인스턴스 사용
- **테스트 케이스**:
  - 이벤트 저장 및 조회
  - 시퀀스 번호 관리
  - 문서별 이벤트 필터링
  - 시간 범위 기반 이벤트 조회
  - 대량 이벤트 처리 성능

```go
func TestMongoEventStore_StoreEvent(t *testing.T) {
    // 테스트 환경 설정
    client, db, cleanup := setupTestDB(t)
    defer cleanup()
    
    // 이벤트 저장소 생성
    logger, _ := zap.NewDevelopment()
    eventStore, err := NewMongoEventStore(context.Background(), client, db.Name(), "events", logger)
    require.NoError(t, err)
    
    // 테스트 이벤트 생성
    docID := primitive.NewObjectID()
    event := &Event{
        ID:          primitive.NewObjectID(),
        DocumentID:  docID,
        SequenceNum: 1,
        Timestamp:   time.Now(),
        Operation:   "create",
        Diff: &nodestorage.Diff{
            Changes: map[string]interface{}{
                "name": "Test Document",
                "value": 100,
            },
            ChangeType: nodestorage.ChangeTypeCreate,
        },
        ClientID: "test-client",
    }
    
    // 이벤트 저장
    err = eventStore.StoreEvent(context.Background(), event)
    require.NoError(t, err)
    
    // 이벤트 조회
    events, err := eventStore.GetEvents(context.Background(), docID, 0)
    require.NoError(t, err)
    require.Len(t, events, 1)
    assert.Equal(t, event.ID, events[0].ID)
    assert.Equal(t, event.DocumentID, events[0].DocumentID)
    assert.Equal(t, event.Operation, events[0].Operation)
}
```

#### 1.2 스냅샷 저장소 (SnapshotStore)

- **테스트 대상**: `MongoSnapshotStore` 구현체
- **테스트 방법**: 실제 MongoDB 인스턴스 사용
- **테스트 케이스**:
  - 스냅샷 생성 및 조회
  - 최신 스냅샷 조회
  - 시퀀스 번호 기반 스냅샷 조회
  - 오래된 스냅샷 삭제

#### 1.3 상태 벡터 관리자 (StateVectorManager)

- **테스트 대상**: `MongoStateVectorManager` 구현체
- **테스트 방법**: 실제 MongoDB 인스턴스 사용
- **테스트 케이스**:
  - 상태 벡터 저장 및 조회
  - 상태 벡터 업데이트
  - 누락된 이벤트 식별
  - 다중 클라이언트 상태 벡터 관리

#### 1.4 동기화 서비스 (SyncService)

- **테스트 대상**: `SyncService` 구현체
- **테스트 방법**: 실제 MongoDB 인스턴스 사용
- **테스트 케이스**:
  - 클라이언트 상태 벡터 처리
  - 누락된 이벤트 전송
  - 이벤트 구독 및 알림
  - 동시 클라이언트 처리

### 2. 통합 테스트

여러 컴포넌트를 함께 테스트하여 전체 시스템의 동작을 검증합니다.

#### 2.1 nodestorage와 EventSync 통합

- **테스트 대상**: `StorageListener`와 nodestorage 통합
- **테스트 방법**: 실제 MongoDB 인스턴스 사용
- **테스트 케이스**:
  - nodestorage 변경 감지 및 이벤트 생성
  - 다양한 변경 유형 처리 (생성, 업데이트, 삭제)
  - 동시 변경 처리

```go
func TestStorageListenerIntegration(t *testing.T) {
    // 테스트 환경 설정
    client, db, cleanup := setupTestDB(t)
    defer cleanup()
    
    // nodestorage 설정
    ctx := context.Background()
    collection := db.Collection("documents")
    memCache := cache.NewMemoryCache[*TestDocument](nil)
    storageOptions := &nodestorage.Options{
        VersionField:      "version",
        WatchEnabled:      true,
        WatchFullDocument: "updateLookup",
    }
    storage, err := nodestorage.NewStorage[*TestDocument](ctx, collection, memCache, storageOptions)
    require.NoError(t, err)
    
    // 이벤트 저장소 설정
    logger, _ := zap.NewDevelopment()
    eventStore, err := NewMongoEventStore(ctx, client, db.Name(), "events", logger)
    require.NoError(t, err)
    
    // 상태 벡터 관리자 설정
    stateVectorManager, err := NewMongoStateVectorManager(ctx, client, db.Name(), "state_vectors", eventStore, logger)
    require.NoError(t, err)
    
    // 동기화 서비스 설정
    syncService := NewSyncService(eventStore, stateVectorManager, logger)
    
    // 스토리지 리스너 설정 및 시작
    storageListener := NewStorageListener[*TestDocument](storage, syncService, logger)
    storageListener.Start()
    defer storageListener.Stop()
    
    // 테스트 문서 생성
    doc := &TestDocument{
        ID:      primitive.NewObjectID(),
        Name:    "Test Document",
        Value:   100,
        Version: 1,
    }
    
    // 문서 저장
    createdDoc, err := storage.CreateAndGet(ctx, doc)
    require.NoError(t, err)
    
    // 이벤트 생성 확인 (약간의 지연 허용)
    time.Sleep(500 * time.Millisecond)
    
    // 이벤트 조회
    events, err := eventStore.GetEvents(ctx, createdDoc.ID, 0)
    require.NoError(t, err)
    require.NotEmpty(t, events)
    assert.Equal(t, "create", events[0].Operation)
    
    // 문서 업데이트
    updatedDoc, diff, err := storage.FindOneAndUpdate(ctx, createdDoc.ID, func(d *TestDocument) (*TestDocument, error) {
        d.Name = "Updated Document"
        d.Value = 200
        d.Version++
        return d, nil
    })
    require.NoError(t, err)
    require.NotNil(t, diff)
    
    // 업데이트 이벤트 확인
    time.Sleep(500 * time.Millisecond)
    events, err = eventStore.GetEvents(ctx, updatedDoc.ID, events[0].SequenceNum)
    require.NoError(t, err)
    require.NotEmpty(t, events)
    assert.Equal(t, "update", events[0].Operation)
}
```

#### 2.2 전체 동기화 흐름

- **테스트 대상**: 전체 동기화 프로세스
- **테스트 방법**: 실제 MongoDB 인스턴스 사용
- **테스트 케이스**:
  - 초기 동기화
  - 실시간 업데이트
  - 재연결 시 동기화
  - 다중 클라이언트 동기화

### 3. 성능 테스트

시스템의 성능과 확장성을 테스트합니다.

#### 3.1 대량 이벤트 처리

- **테스트 대상**: 이벤트 저장소 및 동기화 서비스
- **테스트 방법**: 실제 MongoDB 인스턴스 사용
- **테스트 케이스**:
  - 대량 이벤트 생성 및 저장 (10,000+)
  - 대량 이벤트 조회 성능
  - 메모리 사용량 모니터링

#### 3.2 동시성 테스트

- **테스트 대상**: 전체 시스템
- **테스트 방법**: 실제 MongoDB 인스턴스 사용
- **테스트 케이스**:
  - 다중 클라이언트 동시 연결 (100+)
  - 동시 문서 수정
  - 동시 동기화 요청

#### 3.3 장기 실행 테스트

- **테스트 대상**: 전체 시스템
- **테스트 방법**: 실제 MongoDB 인스턴스 사용
- **테스트 케이스**:
  - 장시간 실행 (24시간+)
  - 메모리 누수 검사
  - 성능 저하 모니터링

### 4. 장애 복구 테스트

시스템의 복원력과 장애 처리 능력을 테스트합니다.

#### 4.1 네트워크 장애 테스트

- **테스트 대상**: 동기화 서비스
- **테스트 방법**: 네트워크 장애 시뮬레이션
- **테스트 케이스**:
  - 일시적 연결 끊김
  - 장시간 연결 끊김
  - 불안정한 네트워크 환경

#### 4.2 데이터베이스 장애 테스트

- **테스트 대상**: 이벤트 저장소 및 동기화 서비스
- **테스트 방법**: 데이터베이스 장애 시뮬레이션
- **테스트 케이스**:
  - 데이터베이스 연결 끊김
  - 데이터베이스 재시작
  - 읽기/쓰기 지연

## 테스트 구현 계획

### 1단계: 기본 단위 테스트 구현

- 각 컴포넌트의 핵심 기능에 대한 단위 테스트 작성
- 테스트 유틸리티 및 헬퍼 함수 구현
- 테스트 데이터 생성 스크립트 작성

### 2단계: 통합 테스트 구현

- nodestorage와 EventSync 통합 테스트 작성
- 전체 동기화 흐름 테스트 작성
- 다양한 시나리오에 대한 테스트 케이스 추가

### 3단계: 성능 및 장애 복구 테스트 구현

- 대량 이벤트 처리 테스트 작성
- 동시성 테스트 작성
- 장애 복구 테스트 작성

## 테스트 실행 방법

### 일반 테스트 실행

```bash
# 모든 테스트 실행
go test ./eventsync/... -v

# 특정 패키지 테스트 실행
go test ./eventsync/eventstore -v

# 특정 테스트 실행
go test ./eventsync/... -run TestMongoEventStore_StoreEvent -v
```

### 성능 테스트 실행

```bash
# 벤치마크 테스트 실행
go test ./eventsync/... -bench=. -benchmem

# 특정 벤치마크 실행
go test ./eventsync/... -bench=BenchmarkEventStore_StoreEvent -benchmem
```

### 장기 실행 테스트

```bash
# 장기 실행 테스트 (별도 스크립트 사용)
./scripts/run_long_tests.sh
```

## 테스트 데이터 구조

### TestDocument 구조체

```go
type TestDocument struct {
    ID      primitive.ObjectID `bson:"_id" json:"id"`
    Name    string             `bson:"name" json:"name"`
    Value   int                `bson:"value" json:"value"`
    Tags    []string           `bson:"tags,omitempty" json:"tags,omitempty"`
    Created time.Time          `bson:"created" json:"created"`
    Updated time.Time          `bson:"updated" json:"updated"`
    Version int64              `bson:"version" json:"version"`
}

// Copy는 TestDocument의 복사본을 반환합니다.
func (d *TestDocument) Copy() *TestDocument {
    if d == nil {
        return nil
    }
    
    copy := &TestDocument{
        ID:      d.ID,
        Name:    d.Name,
        Value:   d.Value,
        Created: d.Created,
        Updated: d.Updated,
        Version: d.Version,
    }
    
    if d.Tags != nil {
        copy.Tags = make([]string, len(d.Tags))
        for i, tag := range d.Tags {
            copy.Tags[i] = tag
        }
    }
    
    return copy
}
```

## 결론

이 테스트 계획은 EventSync 패키지의 품질과 안정성을 보장하기 위한 종합적인 접근 방식을 제공합니다. 실제 MongoDB 인스턴스를 사용하여 테스트함으로써 실제 환경에 가까운 조건에서 시스템의 동작을 검증할 수 있습니다.

테스트는 단위 테스트부터 시작하여 통합 테스트, 성능 테스트, 장애 복구 테스트로 확장됩니다. 이를 통해 EventSync 패키지의 모든 측면을 철저히 검증할 수 있습니다.
