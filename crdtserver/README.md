# CRDT 서버

이 프로젝트는 go-ds-crdt와 go-ds-redis를 사용하여 구현된 분산 키-값 저장소 서버입니다.

## 기능

- Merkle CRDT 기반 분산 키-값 저장소
- Redis를 영구 저장소로 사용
- Redis PubSub을 통한 노드 간 동기화
- Redis 기반 자동 부트스트랩 피어 관리
- RESTful API를 통한 데이터 접근

## 요구사항

- Go 1.19 이상
- Redis 서버

## 설치 및 실행

### 1. 의존성 설치

```bash
go mod tidy
```

### 2. 빌드

```bash
go build -o crdtserver .
```

### 3. 실행

```bash
# 기본 설정으로 실행
./crdtserver

# 사용자 정의 설정으로 실행
./crdtserver --port=8080 --redis=localhost:6379 --topic=crdt-sync --namespace=/crdt-data
```

또는 제공된 스크립트를 사용하여 실행:

```bash
chmod +x run.sh
./run.sh
```

## 명령줄 옵션

- `--port`: HTTP 서버 포트 (기본값: 8080)
- `--redis`: Redis 서버 주소 (기본값: localhost:6379)
- `--redis-password`: Redis 비밀번호 (기본값: 없음)
- `--redis-db`: Redis 데이터베이스 번호 (기본값: 0)
- `--topic`: PubSub 토픽 (기본값: crdt-sync)
- `--bootstrap`: 부트스트랩 피어 목록 (쉼표로 구분)
- `--namespace`: CRDT 데이터 네임스페이스 (기본값: /crdt-data)
- `--debug`: 디버그 로깅 활성화 (기본값: false)

## API 엔드포인트

### 상태 확인

```
GET /health
```

### 피어 정보

```
GET /peers
```

### 데이터 조회

```
GET /api/data/:key
```

### 데이터 저장

```
POST /api/data/:key
```

요청 본문에 저장할 데이터를 포함합니다.

### 데이터 삭제

```
DELETE /api/data/:key
```

### 데이터 목록 조회

```
GET /api/data?prefix=<접두사>
```

### 실시간 업데이트 (Server-Sent Events)

```
GET /events
```

이 엔드포인트는 Server-Sent Events(SSE) 프로토콜을 통해 데이터 변경사항을 실시간으로 수신할 수 있습니다.

이벤트 형식:
```json
{
  "event": "put|get|delete",  // 이벤트 유형
  "key": "key-name",         // 데이터 키
  "value": "data-value"      // 데이터 값 (선택적)
}
```

클라이언트 예제 (JavaScript):
```javascript
const eventSource = new EventSource('/events');

eventSource.onmessage = (event) => {
  const data = JSON.parse(event.data);
  console.log('Received update:', data);
};

eventSource.onerror = (error) => {
  console.error('SSE error:', error);
  eventSource.close();
};
```

## 다중 서버 설정

여러 서버 인스턴스를 실행하여 분산 환경을 구성할 수 있습니다:

```bash
# 첫 번째 서버 (포트 8080)
./crdtserver --port=8080

# 두 번째 서버 (포트 8081)
./crdtserver --port=8081
```

Redis를 통해 자동으로 피어를 발견하고 연결합니다.

## 라이선스

MIT
