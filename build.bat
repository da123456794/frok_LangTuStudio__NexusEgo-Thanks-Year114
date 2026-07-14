@echo off
setlocal EnableExtensions EnableDelayedExpansion
chcp 65001 >nul

cd /d "%~dp0"

set "DIST_DIR=NexusEgo_Storage\dist"

if exist "%DIST_DIR%" rmdir /s /q "%DIST_DIR%"
mkdir "%DIST_DIR%" 2>nul

echo ============================
echo   NexusEgo multi-platform build
echo ============================

call :build_target android arm64
call :build_target linux amd64
call :build_target linux arm64
call :build_target windows amd64 .exe
call :build_target windows arm64 .exe
call :build_target darwin amd64
call :build_target darwin arm64

echo.
echo =====================================
echo Build finished. Generated files:
echo =====================================
dir "%DIST_DIR%"
echo.
pause
exit /b 0

:build_target
set "TARGET_OS=%~1"
set "TARGET_ARCH=%~2"
set "TARGET_EXT=%~3"
set "OUTPUT=%DIST_DIR%\NexusEgo_!TARGET_OS!_!TARGET_ARCH!!TARGET_EXT!"

echo.
echo Building !TARGET_OS! !TARGET_ARCH!...

set "GOOS=!TARGET_OS!"
set "GOARCH=!TARGET_ARCH!"
set "CGO_ENABLED=0"

go build -ldflags="-s -w" -o "!OUTPUT!" ./cmd/nexusego
if errorlevel 1 (
    echo   build failed
    goto :eof
)

echo   build succeeded
upx -9 "!OUTPUT!" >nul 2>&1
if errorlevel 1 (
    echo   skipped UPX compression
) else (
    echo   UPX compression succeeded
)

goto :eof
