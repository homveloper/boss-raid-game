# JSON-RPC 기반 이송 시스템 API

이 문서는 JSON-RPC 기반 이송 시스템 API의 사용 방법을 설명합니다.

## 개요

이 API는 JSON-RPC 2.0 프로토콜을 사용하여 이송 시스템의 기능을 제공합니다. 단일 엔드포인트를 통해 여러 요청을 배치로 처리할 수 있어 네트워크 효율성을 높입니다.

## 엔드포인트

```
POST /rpc
```

## 요청 형식

### 단일 요청

```json
{
  "jsonrpc": "2.0",
  "method": "methodName",
  "params": {
    // 메서드별 파라미터
  },
  "id": 1
}
```

### 배치 요청

```json
[
  {
    "jsonrpc": "2.0",
    "method": "method1",
    "params": {
      // 메서드1 파라미터
    },
    "id": 1
  },
  {
    "jsonrpc": "2.0",
    "method": "method2",
    "params": {
      // 메서드2 파라미터
    },
    "id": 2
  }
]
```

## 응답 형식

### 단일 응답

```json
{
  "jsonrpc": "2.0",
  "result": {
    // 메서드별 결과
  },
  "id": 1
}
```

### 에러 응답

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32000,
    "message": "에러 메시지",
    "data": "추가 에러 정보"
  },
  "id": 1
}
```

### 배치 응답

```json
[
  {
    "jsonrpc": "2.0",
    "result": {
      // 메서드1 결과
    },
    "id": 1
  },
  {
    "jsonrpc": "2.0",
    "error": {
      "code": -32000,
      "message": "에러 메시지",
      "data": "추가 에러 정보"
    },
    "id": 2
  }
]
```

## 에러 코드

| 코드 | 메시지 | 설명 |
|------|--------|------|
| -32700 | Parse error | 잘못된 JSON 형식 |
| -32600 | Invalid Request | 잘못된 요청 형식 |
| -32601 | Method not found | 존재하지 않는 메서드 |
| -32602 | Invalid params | 잘못된 파라미터 |
| -32000 | Server error | 서버 내부 오류 |
| -32001 | Validation error | 유효성 검증 실패 |
| -32002 | Not found | 리소스를 찾을 수 없음 |
| -32003 | Conflict | 리소스 충돌 |

## 사용 가능한 메서드

### createTransport

이송을 생성합니다.

**파라미터**:
```json
{
  "alliance_id": "alliance1",
  "player_id": "player1",
  "player_name": "Player One",
  "mine_id": "mine1",
  "mine_name": "Gold Mine",
  "mine_level": 1,
  "general_id": "general1",
  "gold_amount": 100,
  "max_participants": 5,
  "prep_time": 30,
  "transport_time": 60
}
```

**결과**:
```json
{
  "id": "1234567890"
}
```

### joinTransport

이송에 참가합니다.

**파라미터**:
```json
{
  "transport_id": "1234567890",
  "player_id": "player2",
  "player_name": "Player Two",
  "gold_amount": 50
}
```

**결과**:
```json
{
  "status": "joined",
  "id": "1234567890"
}
```

### startTransport

이송을 시작합니다.

**파라미터**:
```json
{
  "transport_id": "1234567890"
}
```

**결과**:
```json
{
  "status": "started"
}
```

### getTransport

이송 정보를 조회합니다.

**파라미터**:
```json
{
  "transport_id": "1234567890"
}
```

**결과**:
```json
{
  "id": "1234567890",
  "alliance_id": "alliance1",
  "player_id": "player1",
  "mine_id": "mine1",
  "mine_name": "Gold Mine",
  "mine_level": 1,
  "general_id": "general1",
  "status": "PREPARING",
  "gold_amount": 100,
  "max_participants": 5,
  "participants": [
    {
      "player_id": "player1",
      "player_name": "Player One",
      "gold_amount": 100,
      "joined_at": "2023-01-01T12:00:00Z"
    }
  ],
  "prep_start_time": "2023-01-01T12:00:00Z",
  "prep_end_time": "2023-01-01T12:30:00Z",
  "created_at": "2023-01-01T12:00:00Z",
  "updated_at": "2023-01-01T12:00:00Z",
  "version": 1
}
```

### getActiveTransports

활성 이송 목록을 조회합니다.

**파라미터**:
```json
{
  "alliance_id": "alliance1"
}
```

**결과**:
```json
[
  {
    "id": "1234567890",
    "alliance_id": "alliance1",
    "player_id": "player1",
    "status": "PREPARING",
    // 기타 필드...
  },
  {
    "id": "1234567891",
    "alliance_id": "alliance1",
    "player_id": "player3",
    "status": "IN_TRANSIT",
    // 기타 필드...
  }
]
```

### raidTransport

이송을 약탈합니다.

**파라미터**:
```json
{
  "transport_id": "1234567890",
  "raider_id": "raider1",
  "raider_name": "Raider One"
}
```

**결과**:
```json
{
  "status": "raiding",
  "raid_id": "9876543210"
}
```

### defendTransport

이송을 방어합니다.

**파라미터**:
```json
{
  "transport_id": "1234567890",
  "defender_id": "defender1",
  "defender_name": "Defender One",
  "successful": true
}
```

**결과**:
```json
{
  "status": "defended"
}
```

## 배치 요청 예시

여러 작업을 한 번의 요청으로 처리하는 예시입니다:

```json
[
  {
    "jsonrpc": "2.0",
    "method": "createTransport",
    "params": {
      "alliance_id": "alliance1",
      "player_id": "player1",
      "player_name": "Player One",
      "mine_id": "mine1",
      "mine_name": "Gold Mine",
      "mine_level": 1,
      "general_id": "general1",
      "gold_amount": 100,
      "max_participants": 5,
      "prep_time": 30,
      "transport_time": 60
    },
    "id": 1
  },
  {
    "jsonrpc": "2.0",
    "method": "getActiveTransports",
    "params": {
      "alliance_id": "alliance1"
    },
    "id": 2
  }
]
```

응답:

```json
[
  {
    "jsonrpc": "2.0",
    "result": {
      "id": "1234567890"
    },
    "id": 1
  },
  {
    "jsonrpc": "2.0",
    "result": [
      {
        "id": "1234567890",
        "alliance_id": "alliance1",
        "player_id": "player1",
        "status": "PREPARING",
        // 기타 필드...
      }
    ],
    "id": 2
  }
]
```

## 실행 방법

```bash
cd transport/cmd/cqrs_demo
go run main.go
```

## curl을 이용한 테스트 방법

아래 명령어를 복사하여 터미널에서 실행하면 API를 테스트할 수 있습니다.

### 1. 이송 생성 (단일 요청)

```bash
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "createTransport",
    "params": {
      "alliance_id": "alliance1",
      "player_id": "player1",
      "player_name": "Player One",
      "mine_id": "mine1",
      "mine_name": "Gold Mine",
      "mine_level": 1,
      "general_id": "general1",
      "gold_amount": 100,
      "max_participants": 5,
      "prep_time": 30,
      "transport_time": 60
    },
    "id": 1
  }'
```

### 2. 활성 이송 목록 조회

```bash
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "getActiveTransports",
    "params": {
      "alliance_id": "alliance1"
    },
    "id": 1
  }'
```

```cmd
curl -X POST http://localhost:8080/rpc -H "Content-Type: application/json" -d '{"jsonrpc": "2.0", "method": "getActiveTransports", "params": { "alliance_id": "alliance1"}, "id": 1}'
```

### 3. 이송 참가

```bash
# 먼저 이송 ID를 가져옵니다 (이전 응답에서 얻은 ID로 대체하세요)
TRANSPORT_ID="1234567890"

curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "joinTransport",
    "params": {
      "transport_id": "'$TRANSPORT_ID'",
      "player_id": "player2",
      "player_name": "Player Two",
      "gold_amount": 50
    },
    "id": 1
  }'
```

### 4. 이송 시작

```bash
# 이송 ID를 가져옵니다 (이전 응답에서 얻은 ID로 대체하세요)
TRANSPORT_ID="1234567890"

curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "startTransport",
    "params": {
      "transport_id": "'$TRANSPORT_ID'"
    },
    "id": 1
  }'
```

### 5. 이송 조회

```bash
# 이송 ID를 가져옵니다 (이전 응답에서 얻은 ID로 대체하세요)
TRANSPORT_ID="1234567890"

curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "getTransport",
    "params": {
      "transport_id": "'$TRANSPORT_ID'"
    },
    "id": 1
  }'
```

### 6. 이송 약탈

```bash
# 이송 ID를 가져옵니다 (이전 응답에서 얻은 ID로 대체하세요)
TRANSPORT_ID="1234567890"

curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "raidTransport",
    "params": {
      "transport_id": "'$TRANSPORT_ID'",
      "raider_id": "raider1",
      "raider_name": "Raider One"
    },
    "id": 1
  }'
```

### 7. 이송 방어

```bash
# 이송 ID를 가져옵니다 (이전 응답에서 얻은 ID로 대체하세요)
TRANSPORT_ID="1234567890"

curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "defendTransport",
    "params": {
      "transport_id": "'$TRANSPORT_ID'",
      "defender_id": "defender1",
      "defender_name": "Defender One",
      "successful": true
    },
    "id": 1
  }'
```

### 8. 배치 요청 (여러 작업 한 번에 처리)

```bash
curl -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '[
    {
      "jsonrpc": "2.0",
      "method": "createTransport",
      "params": {
        "alliance_id": "alliance1",
        "player_id": "player1",
        "player_name": "Player One",
        "mine_id": "mine1",
        "mine_name": "Gold Mine",
        "mine_level": 1,
        "general_id": "general1",
        "gold_amount": 100,
        "max_participants": 5,
        "prep_time": 30,
        "transport_time": 60
      },
      "id": 1
    },
    {
      "jsonrpc": "2.0",
      "method": "getActiveTransports",
      "params": {
        "alliance_id": "alliance1"
      },
      "id": 2
    }
  ]'
```

### 9. 전체 워크플로우 테스트 스크립트

아래 스크립트를 `.sh` 파일로 저장하여 실행하면 전체 워크플로우를 테스트할 수 있습니다:

```bash
#!/bin/bash

# 1. 이송 생성
echo "1. 이송 생성"
RESPONSE=$(curl -s -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "createTransport",
    "params": {
      "alliance_id": "alliance1",
      "player_id": "player1",
      "player_name": "Player One",
      "mine_id": "mine1",
      "mine_name": "Gold Mine",
      "mine_level": 1,
      "general_id": "general1",
      "gold_amount": 100,
      "max_participants": 5,
      "prep_time": 30,
      "transport_time": 60
    },
    "id": 1
  }')

echo $RESPONSE
TRANSPORT_ID=$(echo $RESPONSE | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
echo "생성된 이송 ID: $TRANSPORT_ID"
echo ""

# 2. 이송 참가
echo "2. 이송 참가"
curl -s -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "joinTransport",
    "params": {
      "transport_id": "'$TRANSPORT_ID'",
      "player_id": "player2",
      "player_name": "Player Two",
      "gold_amount": 50
    },
    "id": 1
  }'
echo ""

# 3. 이송 시작
echo "3. 이송 시작"
curl -s -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "startTransport",
    "params": {
      "transport_id": "'$TRANSPORT_ID'"
    },
    "id": 1
  }'
echo ""

# 4. 이송 조회
echo "4. 이송 조회"
curl -s -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "getTransport",
    "params": {
      "transport_id": "'$TRANSPORT_ID'"
    },
    "id": 1
  }'
echo ""

# 5. 이송 약탈
echo "5. 이송 약탈"
RAID_RESPONSE=$(curl -s -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "raidTransport",
    "params": {
      "transport_id": "'$TRANSPORT_ID'",
      "raider_id": "raider1",
      "raider_name": "Raider One"
    },
    "id": 1
  }')
echo $RAID_RESPONSE
RAID_ID=$(echo $RAID_RESPONSE | grep -o '"raid_id":"[^"]*"' | cut -d'"' -f4)
echo "생성된 약탈 ID: $RAID_ID"
echo ""

# 6. 이송 방어
echo "6. 이송 방어"
curl -s -X POST http://localhost:8080/rpc \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "defendTransport",
    "params": {
      "transport_id": "'$TRANSPORT_ID'",
      "defender_id": "defender1",
      "defender_name": "Defender One",
      "successful": true
    },
    "id": 1
  }'
echo ""

echo "테스트 완료!"
```
