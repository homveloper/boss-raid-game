# CRDT Todo 앱 예제

이 예제는 luvjson-client-sdk를 사용하여 만든 실시간 동기화 Todo 앱입니다. 서버와 클라이언트 간에 CRDT(Conflict-free Replicated Data Type)를 사용하여 데이터를 동기화합니다.

## 기능

- 작업 추가
- 작업 내용 수정
- 작업 제거
- 완료 처리
- 진행 중 목록 조회
- 완료 목록 조회
- 실시간 동기화

## 실행 방법

### 필요 조건

- Go 1.16 이상
- gorilla/websocket 패키지

### 서버 실행

1. gorilla/websocket 패키지 설치:
   ```bash
   go get github.com/gorilla/websocket
   ```

2. 서버 디렉토리로 이동:
   ```bash
   cd examples/todo/server
   ```

3. 서버 실행:
   ```bash
   go run main.go
   ```

4. 서버가 http://localhost:8080 에서 실행됩니다.

### 클라이언트 접속

웹 브라우저에서 http://localhost:8080 주소로 접속하면 Todo 앱을 사용할 수 있습니다.

## 구조

### 서버 사이드

- `main.go`: 서버 코드
  - WebSocket을 통한 실시간 통신
  - Todo 항목 관리
  - 클라이언트 간 동기화

### 클라이언트 사이드

- `index.html`: HTML 구조
- `styles.css`: 스타일시트
- `app.js`: 클라이언트 로직
  - WebSocket 연결
  - UI 업데이트
  - 사용자 상호작용 처리

## 동작 방식

1. 클라이언트가 서버에 WebSocket으로 연결합니다.
2. 서버는 현재 Todo 목록을 클라이언트에 전송합니다.
3. 클라이언트가 Todo 항목을 추가, 수정, 삭제하면 해당 작업이 서버로 전송됩니다.
4. 서버는 작업을 처리하고 다른 모든 클라이언트에게 변경사항을 브로드캐스트합니다.
5. 각 클라이언트는 받은 변경사항을 로컬 상태에 적용합니다.

이 방식으로 모든 클라이언트가 항상 최신 상태를 유지하며, 네트워크 지연이나 일시적인 연결 끊김이 있어도 데이터 일관성이 유지됩니다.
