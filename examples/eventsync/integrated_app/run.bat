@echo off
echo EventSync 통합 예제 애플리케이션 실행

REM MongoDB 실행 확인
echo MongoDB가 실행 중인지 확인하세요.
echo MongoDB가 실행 중이 아니라면, 별도의 터미널에서 MongoDB를 실행하세요.
echo.

REM 애플리케이션 실행
echo 애플리케이션을 실행합니다...
go run main.go

echo 애플리케이션이 종료되었습니다.
