@echo off
:: Windows-specific build script for the Text-Based Boss Raid Game

setlocal enabledelayedexpansion

:: Default values
set "OS=all"
set "ARCH=all"
set "OUTPUT=.\bin"
set "APP_NAME=boss-raid-game"

:: Parse command line arguments
:parse_args
if "%~1"=="" goto :end_parse_args
if /i "%~1"=="-o" (
    set "OS=%~2"
    shift
    shift
    goto :parse_args
)
if /i "%~1"=="--os" (
    set "OS=%~2"
    shift
    shift
    goto :parse_args
)
if /i "%~1"=="-a" (
    set "ARCH=%~2"
    shift
    shift
    goto :parse_args
)
if /i "%~1"=="--arch" (
    set "ARCH=%~2"
    shift
    shift
    goto :parse_args
)
if /i "%~1"=="-d" (
    set "OUTPUT=%~2"
    shift
    shift
    goto :parse_args
)
if /i "%~1"=="--output" (
    set "OUTPUT=%~2"
    shift
    shift
    goto :parse_args
)
if /i "%~1"=="-c" (
    set "CLEAN=yes"
    shift
    goto :parse_args
)
if /i "%~1"=="--clean" (
    set "CLEAN=yes"
    shift
    goto :parse_args
)
if /i "%~1"=="-h" (
    goto :show_help
)
if /i "%~1"=="--help" (
    goto :show_help
)
echo Unknown option: %~1
goto :show_help

:end_parse_args

:: Clean if requested
if defined CLEAN (
    echo Cleaning...
    if exist "%OUTPUT%" (
        rmdir /s /q "%OUTPUT%"
        echo Removed output directory: %OUTPUT%
    )
    go clean
    echo Cleaned!
)

:: Create output directory if it doesn't exist
if not exist "%OUTPUT%" (
    echo Creating output directory: %OUTPUT%
    mkdir "%OUTPUT%"
)

:: Build for specified platforms
if /i "%OS%"=="all" (
    call :build_windows
    call :build_darwin
    call :build_linux
) else if /i "%OS%"=="windows" (
    call :build_windows
) else if /i "%OS%"=="darwin" (
    call :build_darwin
) else if /i "%OS%"=="linux" (
    call :build_linux
) else (
    echo Operating system %OS% is not supported
    exit /b 1
)

echo Build process completed!
exit /b 0

:build_windows
echo Building for Windows...
if /i "%ARCH%"=="all" (
    call :build_binary windows 386
    call :build_binary windows amd64
    call :build_binary windows arm
    call :build_binary windows arm64
) else (
    call :build_binary windows %ARCH%
)
exit /b 0

:build_darwin
echo Building for macOS...
if /i "%ARCH%"=="all" (
    call :build_binary darwin amd64
    call :build_binary darwin arm64
) else (
    call :build_binary darwin %ARCH%
)
exit /b 0

:build_linux
echo Building for Linux...
if /i "%ARCH%"=="all" (
    call :build_binary linux 386
    call :build_binary linux amd64
    call :build_binary linux arm
    call :build_binary linux arm64
) else (
    call :build_binary linux %ARCH%
)
exit /b 0

:build_binary
set "target_os=%~1"
set "target_arch=%~2"

:: Set extension based on OS
set "extension="
if "%target_os%"=="windows" set "extension=.exe"

set "output_file=%OUTPUT%\%APP_NAME%_%target_os%_%target_arch%%extension%"

echo Building for %target_os%/%target_arch%...

:: Set environment variables for cross-compilation
set "GOOS=%target_os%"
set "GOARCH=%target_arch%"

:: For ARM, set GOARM to 7 (ARMv7) as a default
if "%target_arch%"=="arm" set "GOARM=7"

:: Build the binary
go build -o "%output_file%" -ldflags="-s -w" .

if %ERRORLEVEL% equ 0 (
    echo Successfully built: %output_file%
) else (
    echo Failed to build for %target_os%/%target_arch%
)

exit /b 0

:show_help
echo Usage: %~nx0 [options]
echo Options:
echo   -o, --os OS        Operating system to build for (windows, darwin, linux, all)
echo   -a, --arch ARCH    Architecture to build for (386, amd64, arm, arm64, all)
echo   -d, --output DIR   Output directory for binaries (default: .\bin)
echo   -c, --clean        Clean before building
echo   -h, --help         Show this help message
echo.
echo Examples:
echo   %~nx0              Build for all supported platforms and architectures
echo   %~nx0 -o windows -a amd64  Build only for Windows 64-bit
echo   %~nx0 -c           Clean and build for all platforms
exit /b 0
