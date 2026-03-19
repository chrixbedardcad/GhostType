@echo off
echo === GhostSpell Build ===
echo.

cd /d "%~dp0"

echo [1/3] Installing frontend dependencies...
cd gui\frontend
call npm install
if %errorlevel% neq 0 (
    echo ERROR: npm install failed
    pause
    exit /b 1
)

echo.
echo [2/3] Building frontend...
call npm run build
if %errorlevel% neq 0 (
    echo ERROR: frontend build failed
    pause
    exit /b 1
)

cd ..\..

echo.
echo [3/3] Building Go binary...
go build -tags "production ghostai" -o ghostspell.exe .
if %errorlevel% neq 0 (
    echo ERROR: Go build failed
    pause
    exit /b 1
)

echo.
echo === Build complete: ghostspell.exe ===
echo.
echo Starting GhostSpell...
start "" ghostspell.exe
