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
