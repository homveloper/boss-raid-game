# IdleDungeon (아이들 던전)

IdleDungeon은 `nodestorage/v2`와 `eventsync` 패키지를 통합하여 실시간 상태 동기화가 가능한 멀티플레이어 게임을 구현한 데모 애플리케이션입니다.

## 주요 기능

- 여러 클라이언트 간 게임 상태 실시간 동기화
- 몬스터와 플레이어가 있는 2D 오픈 필드
- 전투 및 이동 동기화
- 실시간 업데이트를 위한 Server-Sent Events (SSE)
- 영구 저장소로 MongoDB 사용
- nodestorage를 사용한 낙관적 동시성 제어
- eventsync를 사용한 이벤트 소싱 및 상태 벡터 동기화

## 프로젝트 구조

```
idledungeon/
├── cmd/
│   └── server/
│       └── main.go         # 메인 서버 진입점
├── internal/
│   ├── model/
│   │   ├── game.go         # 게임 상태 모델
│   │   ├── unit.go         # 유닛 모델 (플레이어, 몬스터)
│   │   └── world.go        # 월드 모델
│   ├── server/
│   │   ├── handler.go      # HTTP 핸들러
│   │   ├── sse.go          # SSE 구현
│   │   └── server.go       # 서버 구현
│   └── storage/
│       └── storage.go      # nodestorage를 사용한 스토리지 구현
├── client/
│   ├── index.html          # 메인 HTML 파일
│   ├── css/
│   │   └── style.css       # CSS 스타일
│   └── js/
│       ├── app.js          # 메인 애플리케이션 로직
│       ├── game.js         # 게임 렌더링 및 로직
│       └── sync.js         # 서버와의 동기화
└── go.mod                  # Go 모듈 파일
```

## 사전 요구 사항

- Go 1.21 이상
- MongoDB 4.4 이상
- 최신 웹 브라우저 (Chrome, Firefox, Edge 등)

## 시작하기

### 1. MongoDB 시작

로컬 머신에서 MongoDB가 실행 중인지 확인하세요:

```bash
# Docker 사용
docker run -d -p 27017:27017 --name mongodb mongo:latest
```

### 2. 서버 빌드 및 실행

```bash
cd idledungeon
go run cmd/server/main.go
```

서버는 기본적으로 8080 포트에서 시작됩니다. 명령줄 플래그를 사용하여 포트 및 기타 설정을 사용자 지정할 수 있습니다:

```bash
go run cmd/server/main.go --port=8080 --mongo=mongodb://localhost:27017 --db=idledungeon --debug
```

### 3. 클라이언트 열기

웹 브라우저를 열고 다음 주소로 이동하세요:

```
http://localhost:8080
```

## 사용 방법

1. "연결" 버튼을 클릭하여 서버에 연결합니다
2. "새 게임 생성" 버튼을 클릭하여 새 게임 인스턴스를 생성합니다
3. 플레이어 이름을 입력하여 게임에 참가합니다
4. 마우스를 사용하여 게임 월드를 탐색합니다:
   - 드래그하여 뷰를 이동합니다
   - 스크롤하여 확대/축소합니다
   - 유닛을 클릭하여 선택합니다

## 작동 방식

### 서버 측

1. 서버는 `nodestorage/v2`를 사용하여 MongoDB에 게임 상태를 저장하고 관리합니다
2. 클라이언트가 변경(예: 플레이어 이동)을 수행하면 서버는 낙관적 동시성 제어를 사용하여 게임 상태를 업데이트합니다
3. `eventsync` 패키지는 이러한 변경 사항을 캡처하여 이벤트로 변환합니다
4. 서버는 클라이언트 상태 벡터를 추적하여 각 클라이언트가 필요로 하는 이벤트를 결정합니다

### 클라이언트 측

1. 클라이언트는 Server-Sent Events (SSE)를 사용하여 서버에 연결합니다
2. 클라이언트는 수신한 이벤트를 추적하기 위해 상태 벡터를 유지합니다
3. 클라이언트가 이벤트를 수신하면 로컬 게임 상태를 업데이트합니다
4. 클라이언트는 캔버스에 게임 상태를 렌더링합니다

## 개발

### 디버그 모드에서 실행

더 자세한 로깅으로 서버를 디버그 모드에서 실행하려면:

```bash
go run cmd/server/main.go --debug
```

### 프로덕션용 빌드

```bash
go build -o idledungeon-server cmd/server/main.go
```

## 라이선스

이 프로젝트는 MIT 라이선스에 따라 라이선스가 부여됩니다 - 자세한 내용은 LICENSE 파일을 참조하세요.

## 감사의 말

- 낙관적 동시성 제어를 위한 `nodestorage/v2` 패키지
- 이벤트 소싱 및 상태 벡터 동기화를 위한 `eventsync` 패키지
