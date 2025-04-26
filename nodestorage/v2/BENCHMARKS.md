# nodestorage/v2 성능 벤치마크

이 문서는 `nodestorage/v2` 패키지의 성능 벤치마크 실행 및 분석 방법을 설명합니다.

## 벤치마크 실행 방법

### 필요 조건

- Go 1.18 이상
- MongoDB 서버 (테스트용)
- Redis 서버 (Redis 캐시 테스트용, 선택 사항)

### 환경 변수 설정

다음 환경 변수를 설정하여 벤치마크 환경을 구성할 수 있습니다:

- `MONGODB_URI`: MongoDB 연결 문자열 (기본값: `mongodb://localhost:27017`)
- `REDIS_ADDR`: Redis 서버 주소 (기본값: `localhost:6379`)

### 벤치마크 실행

Windows:
```
run_benchmarks.bat
```

Linux/macOS:
```
chmod +x run_benchmarks.sh
./run_benchmarks.sh
```

또는 직접 Go 벤치마크 명령어 실행:

```
# 캐시 벤치마크 실행
go test -bench=BenchmarkCacheComparison -benchmem -benchtime=1s ./cache -json > benchmark_results/cache_benchmark.json

# 스토리지 벤치마크 실행
go test -bench=BenchmarkStorageCacheComparison -benchmem -benchtime=1s . -json > benchmark_results/storage_benchmark.json

# 벤치마크 분석 실행
go run cmd/benchmark_analysis/main.go
```

## 벤치마크 구성

### 캐시 벤치마크

다음 캐시 구현체의 성능을 측정합니다:

- **메모리 캐시**: 메모리 기반 캐시
- **BadgerDB 캐시**: 디스크 기반 키-값 저장소
- **Redis 캐시**: 분산 캐시 (Redis 서버 필요)

다음 작업의 성능을 측정합니다:

- **set**: 문서 저장
- **get-hit**: 캐시에 있는 문서 조회
- **get-miss**: 캐시에 없는 문서 조회
- **delete**: 문서 삭제
- **set-get**: 문서 저장 후 조회

다양한 문서 크기(100B, 1KB, 10KB)에 대한 성능을 측정합니다.

### 스토리지 벤치마크

다음 스토리지 구성의 성능을 측정합니다:

- **캐시 없음**: 캐시 없이 MongoDB만 사용
- **메모리 캐시**: 메모리 캐시와 MongoDB 사용
- **BadgerDB 캐시**: BadgerDB 캐시와 MongoDB 사용
- **Redis 캐시**: Redis 캐시와 MongoDB 사용 (Redis 서버 필요)

다음 작업의 성능을 측정합니다:

- **find-one**: 문서 조회
- **find-many**: 여러 문서 조회
- **find-one-and-upsert**: 문서 생성 또는 기존 문서 반환
- **find-one-and-update**: 낙관적 동시성 제어를 통한 문서 편집
- **update-one**: MongoDB 업데이트 연산자를 사용한 문서 업데이트
- **update-section**: 문서 내 특정 섹션 업데이트
- **delete-one**: 문서 삭제

다양한 문서 크기(100B, 1KB, 10KB)에 대한 성능을 측정합니다.

## 벤치마크 결과 분석

벤치마크 실행 후 `benchmark_results` 디렉토리에 결과가 저장됩니다:

- `benchmark_results/cache_benchmark.json`: 캐시 벤치마크 결과
- `benchmark_results/storage_benchmark.json`: 스토리지 벤치마크 결과
- `benchmark_results/reports/`: 분석 보고서
  - `benchmark_results/reports/summary.md`: 요약 보고서
  - `benchmark_results/reports/cache/`: 캐시 벤치마크 분석 보고서
  - `benchmark_results/reports/storage/`: 스토리지 벤치마크 분석 보고서

분석 보고서는 다음 정보를 제공합니다:

- 각 작업별 성능 비교
- 문서 크기별 성능 비교
- 캐시 구현체별 성능 비교
- 성능 차트 및 시각화
- 권장 사항 및 분석

## 벤치마크 커스터마이징

### 벤치마크 매개변수 조정

`benchmark_test.go` 파일에서 다음 매개변수를 조정할 수 있습니다:

- 문서 크기
- 작업 유형
- 벤치마크 반복 횟수
- 병렬 처리 수준

### 새로운 벤치마크 추가

새로운 벤치마크를 추가하려면 다음 단계를 따르세요:

1. `benchmark_test.go` 파일에 새로운 벤치마크 함수 추가
2. `run_benchmarks.bat` 또는 `run_benchmarks.sh` 파일에 새로운 벤치마크 명령 추가
3. `benchmark_analysis.go` 파일에 새로운 벤치마크 결과 분석 로직 추가

## 벤치마크 결과 해석

### 주요 지표

- **ns/op**: 작업당 소요 시간(나노초) - 낮을수록 좋음
- **B/op**: 작업당 메모리 할당량(바이트) - 낮을수록 좋음
- **allocs/op**: 작업당 메모리 할당 횟수 - 낮을수록 좋음
- **MB/s**: 초당 처리량(메가바이트) - 높을수록 좋음

### 일반적인 권장 사항

- **메모리 캐시**: 가장 빠른 접근 시간을 제공하지만 사용 가능한 메모리에 제한됩니다.
  - 작은 데이터셋과 빈번한 읽기 작업에 적합합니다.
  - 분산 환경이나 영속성이 필요한 경우에는 적합하지 않습니다.

- **BadgerDB 캐시**: 성능과 영속성 사이의 균형을 제공합니다.
  - 영속성이 필요한 중간 크기의 데이터셋에 적합합니다.
  - 단일 노드 애플리케이션에 적합합니다.

- **Redis 캐시**: 캐시 공유가 필요한 분산 환경에 적합합니다.
  - 분산 애플리케이션에 적합합니다.
  - 중간 정도의 읽기/쓰기 비율을 가진 애플리케이션에 적합합니다.

- **캐시 없음**: 빈번한 쓰기 작업과 드문 읽기 작업이 있는 워크로드에 적합합니다.
  - 캐시 무효화가 빈번한 쓰기 위주 애플리케이션에 적합합니다.
  - 읽기 성능보다 데이터 일관성이 더 중요한 애플리케이션에 적합합니다.
