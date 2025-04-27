# Transport System CLI

이 CLI는 이송 시스템을 실행하고 테스트하기 위한 명령줄 인터페이스입니다.

## 사용 방법

### 기본 실행

```bash
go run main.go
```

### 옵션

- `--mongo-uri`: MongoDB 연결 URI (기본값: "mongodb://localhost:27017")
- `--db-name`: 데이터베이스 이름 (기본값: "transport_db")
- `--demo`: 데모 모드로 실행 (샘플 데이터 생성)
- `--env`: .env 파일 경로 (기본값: ".env")

예시:
```bash
go run main.go --mongo-uri="mongodb://localhost:27017" --db-name="my_transport_db" --demo
```

### 환경 변수

다음 환경 변수를 설정하여 설정을 변경할 수 있습니다:

- `MONGO_URI`: MongoDB 연결 URI
- `DB_NAME`: 데이터베이스 이름

환경 변수는 명령줄 인수보다 우선합니다.

## 데모 모드

데모 모드에서는 다음과 같은 작업이 수행됩니다:

1. 광산 설정 생성 (레벨 1-5)
2. 샘플 광산 생성
3. 광산에 금광석 추가
4. 플레이어 생성
5. 이송권 생성
6. 이송 시작 및 참여
7. 이송 약탈 및 방어 시뮬레이션
8. 이송권 구매

## 빌드 방법

```bash
go build -o transport-cli
```

## 실행 방법 (빌드 후)

```bash
./transport-cli --demo
```
