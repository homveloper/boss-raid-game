@echo off
REM CRDT 서버 실행 스크립트

REM 기본 설정
set PORT=8080
set GAME_PORT=8081
set REDIS_ADDR=localhost:6379
set REDIS_PASSWORD=
set REDIS_DB=0
set TOPIC=crdt-sync
set NAMESPACE=/crdt-data
set DEBUG=false
set USE_IPFS_LITE=false
set ENABLE_GAME=false
set CLIENT_DIR=.\client
set BOOTSTRAP=

REM 명령줄 인수 파싱
:parse_args
if "%~1"=="" goto :end_parse_args

set ARG=%~1
if "%ARG:~0,7%"=="--port=" (
    set PORT=%ARG:~7%
    goto :next_arg
)
if "%ARG:~0,12%"=="--game-port=" (
    set GAME_PORT=%ARG:~12%
    goto :next_arg
)
if "%ARG:~0,8%"=="--redis=" (
    set REDIS_ADDR=%ARG:~8%
    goto :next_arg
)
if "%ARG:~0,17%"=="--redis-password=" (
    set REDIS_PASSWORD=%ARG:~17%
    goto :next_arg
)
if "%ARG:~0,11%"=="--redis-db=" (
    set REDIS_DB=%ARG:~11%
    goto :next_arg
)
if "%ARG:~0,8%"=="--topic=" (
    set TOPIC=%ARG:~8%
    goto :next_arg
)
if "%ARG:~0,12%"=="--namespace=" (
    set NAMESPACE=%ARG:~12%
    goto :next_arg
)
if "%ARG:~0,12%"=="--bootstrap=" (
    set BOOTSTRAP=%ARG:~12%
    goto :next_arg
)
if "%ARG%"=="--debug" (
    set DEBUG=true
    goto :next_arg
)
if "%ARG%"=="--ipfs-lite" (
    set USE_IPFS_LITE=true
    goto :next_arg
)
if "%ARG%"=="--enable-game" (
    set ENABLE_GAME=true
    goto :next_arg
)
if "%ARG:~0,12%"=="--client-dir=" (
    set CLIENT_DIR=%ARG:~12%
    goto :next_arg
)

echo 알 수 없는 옵션: %ARG%
exit /b 1

:next_arg
shift
goto :parse_args

:end_parse_args

REM 실행 명령 구성
set CMD=crdtserver.exe --port=%PORT% --game-port=%GAME_PORT% --redis=%REDIS_ADDR% --redis-db=%REDIS_DB% --topic=%TOPIC% --namespace=%NAMESPACE% --client-dir=%CLIENT_DIR%

if not "%REDIS_PASSWORD%"=="" (
    set CMD=%CMD% --redis-password=%REDIS_PASSWORD%
)

if not "%BOOTSTRAP%"=="" (
    set CMD=%CMD% --bootstrap=%BOOTSTRAP%
)

if "%DEBUG%"=="true" (
    set CMD=%CMD% --debug
)

if "%USE_IPFS_LITE%"=="true" (
    set CMD=%CMD% --ipfs-lite
)

if "%ENABLE_GAME%"=="true" (
    set CMD=%CMD% --enable-game
)

REM 실행 정보 출력
echo CRDT 서버 시작 중...
echo 포트: %PORT%
echo 게임 포트: %GAME_PORT%
echo Redis: %REDIS_ADDR%
echo 토픽: %TOPIC%
echo 네임스페이스: %NAMESPACE%
echo 디버그 모드: %DEBUG%
echo IPFS-Lite 사용: %USE_IPFS_LITE%
echo 게임 서버 활성화: %ENABLE_GAME%
echo 클라이언트 디렉토리: %CLIENT_DIR%

if not "%BOOTSTRAP%"=="" (
    echo 부트스트랩 피어: %BOOTSTRAP%
)

REM 서버 실행
echo 명령: %CMD%
%CMD%
