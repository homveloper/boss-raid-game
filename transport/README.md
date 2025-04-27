# 이송 (Transport) System

이송 시스템은 각 광산에서 채광을 진행한 금광석을 이송하여 각각의 연합원이 획득하는 컨텐츠입니다.

## 주요 기능

### 광산 및 금광석 관리
- 연합별 광산 생성 및 관리
- 광산 레벨에 따른 설정 (최소/최대 이송량, 이송 시간, 최대 참여 인원)
- 금광석 추가 및 제거

### 이송권 관리
- 매일(UTC+0 00:00) 이송권 충전
- 이송권 구매 (첫 구매 300보옥, 이후 100보옥씩 증가)
- 하루가 지나면 구매 가격 초기화

### 이송 프로세스
- 이송 시작 (30분 준비 시간)
- 다른 연합원 이송 참여
- 이송 시간 계산 (광산 레벨에 따라 다름)
- 이송 완료 및 금광석 획득

### 약탈 및 방어
- 이송 중인 수레 약탈
- 30분 약탈 방어 시간
- 방어 성공/실패에 따른 금광석 손실

## 데이터 모델

### Mine (광산)
- 광산 ID, 이름, 레벨
- 현재 보유 금광석 양
- 상태 (활성/비활성)

### Transport (이송)
- 이송 ID, 광산 정보
- 상태 (준비 중/진행 중/완료/약탈됨)
- 금광석 양, 참여자 목록
- 준비 시간, 이송 시간
- 약탈 상태

### TransportTicket (이송권)
- 플레이어 ID, 연합 ID
- 현재 이송권 수, 최대 이송권 수
- 마지막 충전 시간
- 구매 횟수, 마지막 구매 시간

### MineConfig (광산 설정)
- 광산 레벨
- 최소/최대 이송량
- 이송 시간
- 최대 참여 인원

## 서비스

### MineService
- 광산 생성 및 관리
- 금광석 추가/제거
- 광산 설정 관리

### TicketService
- 이송권 생성 및 관리
- 이송권 사용
- 이송권 구매

### TransportService
- 이송 시작
- 이송 참여
- 이송 완료 처리
- 약탈 및 방어 처리

## 사용 예시

```go
// 서비스 생성
mineService := NewMineService(mineStorage, mineConfigStorage)
ticketService := NewTicketService(ticketStorage)
transportService := NewTransportService(transportStorage, mineService, ticketService)

// 광산 생성
mine, err := mineService.CreateMine(ctx, allianceID, "Gold Mine Alpha", 1)

// 이송권 확인
ticket, err := ticketService.GetOrCreateTickets(ctx, playerID, allianceID, 5)

// 이송 시작
transport, err := transportService.StartTransport(ctx, playerID, playerName, mineID, 200)

// 이송 참여
transport, err = transportService.JoinTransport(ctx, transportID, playerID, playerName, 150)

// 약탈 시도
transport, err = transportService.RaidTransport(ctx, transportID, raiderID, raiderName)

// 방어
transport, err = transportService.DefendTransport(ctx, transportID, defenderID, defenderName, true)
```

## 구현 세부사항

### 낙관적 동시성 제어
- 모든 문서는 `VectorClock` 필드를 통해 버전 관리
- 동시 업데이트 충돌 감지 및 해결

### 실시간 변경 감지
- MongoDB 변경 스트림을 활용한 실시간 이벤트 처리
- 이송 상태 변경, 약탈 시도 등의 이벤트 모니터링

### 캐싱
- 자주 접근하는 데이터 캐싱으로 성능 향상
- 핫 데이터 감지 및 자동 캐싱

### 비동기 처리
- 이송 시작, 완료, 약탈 방어 시간 등의 비동기 처리
- 고루틴을 활용한 백그라운드 작업
