@echo off
setlocal enabledelayedexpansion

:: All output goes to both console and build.log via PowerShell Tee-Object.
set "LOGFILE=%~dp0build.log"
if "%~1"=="__INNER__" goto :main
powershell -NoProfile -Command "& { cmd /c '\"%~f0\" __INNER__' 2>&1 | Tee-Object -FilePath '%LOGFILE%' }"
exit /b %errorlevel%

:main

echo ============================================
echo          GhostSpell Full Build
echo ============================================
echo.

cd /d "%~dp0"

:: --clean flag: delete build cache and force full rebuild.
if "%~1"=="--clean" (
    echo [clean] Deleting build cache...
    if exist "build\llama" rmdir /s /q "build\llama"
    if exist "build\whisper" rmdir /s /q "build\whisper"
    echo [clean] Done — rebuilding from scratch.
    echo.
)
if "%~2"=="--clean" (
    if exist "build\llama" rmdir /s /q "build\llama"
    if exist "build\whisper" rmdir /s /q "build\whisper"
)

:: Auto-detect MSYS2 MinGW64 toolchain and add to PATH if present.
if exist "C:\msys64\mingw64\bin\gcc.exe" (
    set "PATH=C:\msys64\mingw64\bin;%PATH%"
)

set LLAMA_VERSION=b8281
set BUILD_DIR=%~dp0build
set LLAMA_SRC=%BUILD_DIR%\llama-src
set LLAMA_OUT=%BUILD_DIR%\llama
set GHOSTAI=0

:: ============================================================
:: Step 0 — Check prerequisites
:: ============================================================
echo [0] Checking prerequisites...
echo.

set MISSING=0

where go >nul 2>&1
if %errorlevel% neq 0 (
    echo   ERROR: 'go' not found. Install Go from https://go.dev/dl/
    set MISSING=1
) else (
    for /f "delims=" %%v in ('go version 2^>^&1') do echo   go ......... OK ^(%%v^)
)

where node >nul 2>&1
if %errorlevel% neq 0 (
    echo   ERROR: 'node' not found. Install Node.js from https://nodejs.org/
    set MISSING=1
) else (
    for /f "delims=" %%v in ('node --version 2^>^&1') do echo   node ....... OK ^(%%v^)
)

where npm >nul 2>&1
if %errorlevel% neq 0 (
    echo   ERROR: 'npm' not found. Install Node.js from https://nodejs.org/
    set MISSING=1
) else (
    for /f "delims=" %%v in ('npm --version 2^>^&1') do echo   npm ........ OK ^(%%v^)
)

if %MISSING%==1 (
    echo.
    echo   Install the missing tools above and try again.
    pause
    exit /b 1
)

:: Ghost-AI toolchain (optional — build falls back to API-only mode without these)
set HAS_CMAKE=0
set HAS_GCC=0
set HAS_GENERATOR=0
set GENERATOR_NAME=

where cmake >nul 2>&1
if !errorlevel!==0 set HAS_CMAKE=1

where gcc >nul 2>&1
if !errorlevel!==0 set HAS_GCC=1

where ninja >nul 2>&1
if !errorlevel!==0 (
    set HAS_GENERATOR=1
    set GENERATOR_NAME=Ninja
)
if !HAS_GENERATOR!==0 (
    where mingw32-make >nul 2>&1
    if !errorlevel!==0 (
        set HAS_GENERATOR=1
        set GENERATOR_NAME=MinGW Makefiles
    )
)

if !HAS_CMAKE!==1 if !HAS_GCC!==1 if !HAS_GENERATOR!==1 (
    for /f "delims=" %%v in ('cmake --version 2^>^&1 ^| findstr /n "." ^| findstr "^1:"') do (
        set "cmake_ver=%%v"
        set "cmake_ver=!cmake_ver:~2!"
        echo   cmake ...... OK ^(!cmake_ver!^)
    )
    for /f "delims=" %%v in ('gcc --version 2^>^&1 ^| findstr /n "." ^| findstr "^1:"') do (
        set "gcc_ver=%%v"
        set "gcc_ver=!gcc_ver:~2!"
        echo   gcc ........ OK ^(!gcc_ver!^)
    )
    echo   generator .. OK ^(!GENERATOR_NAME!^)
    set GHOSTAI=1
)

if !GHOSTAI!==0 (
    echo.
    echo   NOTE: Ghost-AI toolchain not found ^(cmake / gcc / ninja^).
    echo         Building WITHOUT local AI — you can still use API providers
    echo         ^(OpenAI, Anthropic, etc.^) via Settings.
    echo.
    echo         To enable local AI, install MSYS2 ^(https://www.msys2.org^) then run:
    echo           pacman -S mingw-w64-x86_64-toolchain mingw-w64-x86_64-cmake mingw-w64-x86_64-ninja
    echo         and re-run this script.
)

echo.

:: CPU count for parallel builds
set NPROC=%NUMBER_OF_PROCESSORS%
if "%NPROC%"=="" set NPROC=4

:: ============================================================
:: Step 1 — Build Ghost-AI static libraries (if toolchain found)
:: ============================================================
if !GHOSTAI!==0 goto :skip_ghostai

:: Skip if libraries already built
set /a EXISTING_LIBS=0
if exist "%LLAMA_OUT%\lib" (
    for %%f in ("%LLAMA_OUT%\lib\*.a") do set /a EXISTING_LIBS+=1
)
if !EXISTING_LIBS! geq 3 (
    echo [1] Ghost-AI libraries already built ^(!EXISTING_LIBS! libs^) — skipping.
    echo     To rebuild: delete the build\llama folder and re-run.
    echo.
    goto :skip_ghostai
)

echo [1] Building Ghost-AI ^(llama.cpp %LLAMA_VERSION%^)...
echo.

:: --- Download llama.cpp source ---
set NEED_DOWNLOAD=1
if exist "%LLAMA_SRC%\.version" (
    set /p CACHED_VER=<"%LLAMA_SRC%\.version"
    if "!CACHED_VER!"=="%LLAMA_VERSION%" (
        echo   Using cached llama.cpp source ^(%LLAMA_VERSION%^)
        set NEED_DOWNLOAD=0
    ) else (
        echo   Version changed ^(!CACHED_VER! -^> %LLAMA_VERSION%^), re-downloading...
        rmdir /s /q "%LLAMA_SRC%" 2>nul
    )
)

if !NEED_DOWNLOAD!==1 (
    echo   Downloading llama.cpp %LLAMA_VERSION%...
    if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"
    curl -fsSL "https://github.com/ggml-org/llama.cpp/archive/refs/tags/%LLAMA_VERSION%.tar.gz" -o "%BUILD_DIR%\llama.tar.gz"
    if !errorlevel! neq 0 (
        echo   ERROR: Download failed — falling back to API-only build
        set GHOSTAI=0
        goto :skip_ghostai
    )
    cd /d "%BUILD_DIR%"
    tar xzf llama.tar.gz
    if !errorlevel! neq 0 (
        echo   ERROR: Extract failed — falling back to API-only build
        cd /d "%~dp0"
        set GHOSTAI=0
        goto :skip_ghostai
    )
    if exist "llama-src" rmdir /s /q "llama-src"
    rename "llama.cpp-%LLAMA_VERSION%" llama-src
    echo %LLAMA_VERSION%> "%LLAMA_SRC%\.version"
    del llama.tar.gz 2>nul
    cd /d "%~dp0"
    echo   Downloaded OK
)

:: --- Build static libraries with CMake ---
echo   Compiling static libraries ^(this may take a few minutes^)...

set LLAMA_BUILD=%LLAMA_SRC%\build
if not exist "%LLAMA_BUILD%" mkdir "%LLAMA_BUILD%"

set WIN_FLAGS=-D_WIN32_WINNT=0x0A00

cd /d "%LLAMA_BUILD%"
cmake .. -G "!GENERATOR_NAME!" ^
    -DCMAKE_BUILD_TYPE=Release ^
    -DCMAKE_C_COMPILER=gcc ^
    -DCMAKE_CXX_COMPILER=g++ ^
    -DCMAKE_C_FLAGS="%WIN_FLAGS%" ^
    -DCMAKE_CXX_FLAGS="%WIN_FLAGS%" ^
    -DGGML_STATIC=ON ^
    -DGGML_CUDA=OFF ^
    -DGGML_VULKAN=OFF ^
    -DGGML_METAL=OFF ^
    -DGGML_OPENMP=OFF ^
    -DLLAMA_BUILD_TESTS=OFF ^
    -DLLAMA_BUILD_EXAMPLES=OFF ^
    -DLLAMA_BUILD_SERVER=OFF ^
    -DBUILD_SHARED_LIBS=OFF ^
    -DGGML_NATIVE=OFF ^
    -DGGML_AVX=ON ^
    -DGGML_AVX2=OFF ^
    -DGGML_AVX512=OFF ^
    -DGGML_FMA=OFF ^
    -DGGML_F16C=OFF
if !errorlevel! neq 0 (
    echo   ERROR: CMake configure failed — falling back to API-only build
    cd /d "%~dp0"
    set GHOSTAI=0
    goto :skip_ghostai
)

cmake --build . --config Release -j %NPROC%
if !errorlevel! neq 0 (
    echo   ERROR: Compile failed — falling back to API-only build
    cd /d "%~dp0"
    set GHOSTAI=0
    goto :skip_ghostai
)
cd /d "%~dp0"

:: --- Install headers + libraries ---
if not exist "%LLAMA_OUT%\include" mkdir "%LLAMA_OUT%\include"
if not exist "%LLAMA_OUT%\lib" mkdir "%LLAMA_OUT%\lib"

:: Headers
copy /y "%LLAMA_SRC%\include\llama.h" "%LLAMA_OUT%\include\" >nul 2>&1
if exist "%LLAMA_SRC%\ggml\include" (
    copy /y "%LLAMA_SRC%\ggml\include\*.h" "%LLAMA_OUT%\include\" >nul 2>&1
)
if exist "%LLAMA_SRC%\include\ggml*.h" (
    copy /y "%LLAMA_SRC%\include\ggml*.h" "%LLAMA_OUT%\include\" >nul 2>&1
)

:: Static libraries — collect all .a from build tree
for /r "%LLAMA_BUILD%" %%f in (*.a) do (
    copy /y "%%f" "%LLAMA_OUT%\lib\" >nul 2>&1
)

:: Ensure lib* prefix for MinGW linker
for %%f in ("%LLAMA_OUT%\lib\*.a") do (
    set "fname=%%~nxf"
    if not "!fname:~0,3!"=="lib" (
        rename "%%f" "lib!fname!"
    )
)

:: Verify we got libraries
set /a LCOUNT=0
for %%f in ("%LLAMA_OUT%\lib\*.a") do set /a LCOUNT+=1
if !LCOUNT!==0 (
    echo   WARNING: No static libraries found — falling back to API-only build
    set GHOSTAI=0
) else (
    echo   Ghost-AI ready: !LCOUNT! libraries installed
)
echo.

:skip_ghostai

:: ============================================================
:: Step 1.5 — Build Ghost Voice (whisper.cpp) if toolchain found
:: ============================================================
set GHOSTVOICE=0
if !HAS_CMAKE!==1 if !HAS_GCC!==1 if !HAS_GENERATOR!==1 set GHOSTVOICE=1

if !GHOSTVOICE!==0 goto :skip_ghostvoice

set WHISPER_VERSION=v1.7.5
set WHISPER_SRC=%BUILD_DIR%\whisper-src
set WHISPER_OUT=%BUILD_DIR%\whisper

:: Skip if whisper-cli already built
if exist "%~dp0whisper-cli.exe" (
    echo [1.5] Ghost Voice already built ^(whisper-cli.exe found^) — skipping.
    echo     To rebuild: delete whisper-cli.exe and the build\whisper folder, then re-run.
    echo.
    goto :skip_ghostvoice
)

echo [1.5] Building Ghost Voice ^(whisper.cpp %WHISPER_VERSION%^)...
echo.

:: --- Download whisper.cpp source ---
set WNEED=1
if exist "%WHISPER_SRC%\.version" (
    set /p WCACHED=<"%WHISPER_SRC%\.version"
    if "!WCACHED!"=="%WHISPER_VERSION%" (
        echo   Using cached whisper.cpp source
        set WNEED=0
    )
)

if !WNEED!==1 (
    echo   Downloading whisper.cpp %WHISPER_VERSION%...
    curl -fsSL "https://github.com/ggml-org/whisper.cpp/archive/refs/tags/%WHISPER_VERSION%.tar.gz" -o "%BUILD_DIR%\whisper.tar.gz"
    if !errorlevel! neq 0 (
        echo   WARNING: Download failed — skipping Ghost Voice
        set GHOSTVOICE=0
        goto :skip_ghostvoice
    )
    cd /d "%BUILD_DIR%"
    tar xzf whisper.tar.gz
    for /d %%d in (whisper.cpp-*) do (
        if exist "whisper-src" rmdir /s /q "whisper-src"
        rename "%%d" whisper-src
    )
    echo %WHISPER_VERSION%> "%WHISPER_SRC%\.version"
    del whisper.tar.gz 2>nul
    cd /d "%~dp0"
    echo   Downloaded OK
)

:: --- Build static libraries with CMake ---
echo   Compiling whisper.cpp static libraries...

set WHISPER_BUILD=%WHISPER_SRC%\build
:: Clean stale build dir to force a fresh build
if exist "%WHISPER_BUILD%" rmdir /s /q "%WHISPER_BUILD%"
mkdir "%WHISPER_BUILD%"

set WIN_FLAGS=-D_WIN32_WINNT=0x0A00

cd /d "%WHISPER_BUILD%"
cmake .. -G "!GENERATOR_NAME!" ^
    -DCMAKE_BUILD_TYPE=Release ^
    -DCMAKE_C_COMPILER=gcc ^
    -DCMAKE_CXX_COMPILER=g++ ^
    -DCMAKE_C_FLAGS="%WIN_FLAGS%" ^
    -DCMAKE_CXX_FLAGS="%WIN_FLAGS%" ^
    -DBUILD_SHARED_LIBS=OFF ^
    -DWHISPER_BUILD_TESTS=OFF ^
    -DWHISPER_BUILD_EXAMPLES=ON ^
    -DGGML_STATIC=ON ^
    -DGGML_CUDA=OFF ^
    -DGGML_VULKAN=OFF ^
    -DGGML_METAL=OFF ^
    -DGGML_OPENMP=OFF ^
    -DGGML_NATIVE=OFF ^
    -DGGML_AVX=ON ^
    -DGGML_AVX2=OFF ^
    -DGGML_AVX512=OFF ^
    -DGGML_FMA=OFF ^
    -DGGML_F16C=OFF
if !errorlevel! neq 0 (
    echo   WARNING: CMake failed — skipping Ghost Voice
    cd /d "%~dp0"
    set GHOSTVOICE=0
    goto :skip_ghostvoice
)

cmake --build . --config Release -j %NPROC%
if !errorlevel! neq 0 (
    echo   WARNING: Build failed — skipping Ghost Voice
    cd /d "%~dp0"
    set GHOSTVOICE=0
    goto :skip_ghostvoice
)
cd /d "%~dp0"

:: --- Install headers + libraries (same pattern as Ghost-AI) ---
:: Wipe output dir to prevent stale headers/libs from previous failed builds
if exist "%WHISPER_OUT%" rmdir /s /q "%WHISPER_OUT%"
mkdir "%WHISPER_OUT%\include"
mkdir "%WHISPER_OUT%\lib"

:: Headers — copy all .h from whisper's include dirs
echo   Collecting headers...
copy /y "%WHISPER_SRC%\include\*.h" "%WHISPER_OUT%\include\" >nul 2>&1
if exist "%WHISPER_SRC%\ggml\include" (
    copy /y "%WHISPER_SRC%\ggml\include\*.h" "%WHISPER_OUT%\include\" >nul 2>&1
)

:: Copy whisper-cli for standalone testing (bypasses all Go/CGo code).
for /r "%WHISPER_BUILD%" %%f in (whisper-cli.exe) do (
    copy /y "%%f" "%~dp0whisper-cli.exe" >nul 2>&1
    echo   whisper-cli.exe copied for standalone testing
)

:: Static libraries — collect all .a from build tree
echo   Collecting libraries...
for /r "%WHISPER_BUILD%" %%f in (*.a) do (
    echo     Found: %%~nxf
    copy /y "%%f" "%WHISPER_OUT%\lib\" >nul 2>&1
)

:: Ensure lib* prefix for MinGW linker
for %%f in ("%WHISPER_OUT%\lib\*.a") do (
    set "wfn=%%~nxf"
    if not "!wfn:~0,3!"=="lib" (
        rename "%%f" "lib!wfn!"
    )
)


:: Verify we got libraries
set /a WCOUNT=0
for %%f in ("%WHISPER_OUT%\lib\*.a") do set /a WCOUNT+=1
if !WCOUNT!==0 (
    echo   WARNING: No whisper libraries found — falling back without Ghost Voice
    set GHOSTVOICE=0
) else (
    echo   Ghost Voice ready: !WCOUNT! libraries installed
)
echo.

:skip_ghostvoice

:: ============================================================
:: Step 2 — Build frontend
:: ============================================================
if !GHOSTAI!==1 (
    echo [2] Building frontend...
) else (
    echo [1] Building frontend...
)
echo.

cd /d "%~dp0gui\frontend"
call npm install
if !errorlevel! neq 0 (
    echo ERROR: npm install failed
    cd /d "%~dp0"
    pause
    exit /b 1
)

echo.
call npm run build
if !errorlevel! neq 0 (
    echo ERROR: frontend build failed
    cd /d "%~dp0"
    pause
    exit /b 1
)
cd /d "%~dp0"
echo.

:: ============================================================
:: Step 3 — Build Go binary
:: ============================================================
:: ghostspell.exe links Ghost-AI (llama.cpp) only.
:: Voice (whisper.cpp) runs via whisper-cli.exe subprocess — no CGo needed.

set MAIN_TAGS=production
if !GHOSTAI!==1 set MAIN_TAGS=!MAIN_TAGS! ghostai

if !GHOSTAI!==1 (
    echo [3] Building ghostspell.exe with Ghost-AI...
) else (
    echo [2] Building ghostspell.exe ^(API-only mode^)...
)
set CGO_ENABLED=1

go build -tags "!MAIN_TAGS!" -o ghostspell.exe .
if !errorlevel! neq 0 (
    if !GHOSTAI!==1 (
        echo.
        echo   Build failed — retrying without Ghost-AI...
        set GHOSTAI=0
        go build -tags "production" -o ghostspell.exe .
    )
)
if !errorlevel! neq 0 (
    echo ERROR: Go build failed
    pause
    exit /b 1
)

echo.
echo ============================================
echo   BUILD COMPLETE: ghostspell.exe
if !GHOSTAI!==1 echo   + Ghost-AI ^(local text AI^)
if !GHOSTVOICE!==1 echo   + whisper-cli.exe ^(local speech-to-text^)
if !GHOSTAI!==0 if !GHOSTVOICE!==0 echo   Mode: API-only
echo ============================================
echo.

:: Clear logs for a fresh testing session.
set APPDATA_DIR=%APPDATA%\GhostSpell
if exist "%APPDATA_DIR%\ghostspell.log" (
    del /q "%APPDATA_DIR%\ghostspell.log" 2>nul
    echo Cleared %APPDATA_DIR%\ghostspell.log
)
if exist "%APPDATA_DIR%\ghostvoice.log" (
    del /q "%APPDATA_DIR%\ghostvoice.log" 2>nul
    echo Cleared %APPDATA_DIR%\ghostvoice.log
)
echo.
echo Starting GhostSpell...
start "" ghostspell.exe
echo.
echo Build log saved to: %LOGFILE%
goto :eof
