@echo off
:: Full clean rebuild — deletes all cached C libraries and rebuilds from scratch.
:: Use _build.bat for fast incremental builds.

echo ============================================
echo       GhostSpell FULL CLEAN BUILD
echo ============================================
echo.

cd /d "%~dp0"

echo Cleaning build cache...
if exist "build\llama" rmdir /s /q "build\llama"
if exist "build\whisper" rmdir /s /q "build\whisper"
echo Done.
echo.

call "%~dp0_build.bat"
