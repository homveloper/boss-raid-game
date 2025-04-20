#!/bin/bash

# Redis 서버가 실행 중인지 확인
redis-cli ping > /dev/null 2>&1
if [ $? -ne 0 ]; then
    echo "Redis 서버가 실행 중이지 않습니다. Redis를 먼저 시작하세요."
    exit 1
fi

# 필요한 패키지 설치
go mod tidy

# 서버 빌드
go build -o crdtserver .

# 서버 실행
./crdtserver "$@"
