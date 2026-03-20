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

set NPROC=%NUMBER_OF_PROCESSORS%
if "%NPROC%"=="" set NPROC=4

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

set /a WLIBS=0
set WHEADERS_OK=0
if exist "!WHISPER_OUT!\lib" (
    for /f "delims=" %%f in ('dir /b "!WHISPER_OUT!\lib\*.a" 2^>nul') do set /a WLIBS+=1
)
if exist "!WHISPER_OUT!\include\whisper.h" if exist "!WHISPER_OUT!\include\ggml.h" set WHEADERS_OK=1
if !WLIBS! geq 2 if !WHEADERS_OK!==1 (
    echo [1.5] Ghost Voice libraries already built ^(!WLIBS! libs^) — skipping.
    echo.
    goto :skip_ghostvoice
)

echo [1.5] Building Ghost Voice ^(whisper.cpp !WHISPER_VERSION!^)...
echo.

set WNEED=1
if exist "!WHISPER_SRC!\.version" (
    set /p WCACHED=<"!WHISPER_SRC!\.version"
    if "!WCACHED!"=="!WHISPER_VERSION!" (
        echo   Using cached whisper.cpp source
        set WNEED=0
    )
)

if !WNEED!==1 (
    echo   Downloading whisper.cpp !WHISPER_VERSION!...
    curl -fsSL "https://github.com/ggml-org/whisper.cpp/archive/refs/tags/!WHISPER_VERSION!.tar.gz" -o "%BUILD_DIR%\whisper.tar.gz"
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
    echo !WHISPER_VERSION!> "!WHISPER_SRC!\.version"
    del whisper.tar.gz 2>nul
    cd /d "%~dp0"
    echo   Downloaded OK
)

echo   Compiling whisper.cpp static libraries...
set WBUILD=!WHISPER_SRC!\build
:: Clean stale build dir to avoid "ninja: no work to do" with missing output
if exist "!WBUILD!" rmdir /s /q "!WBUILD!"
mkdir "!WBUILD!"
cd /d "!WBUILD!"

set WIN_FLAGS=-D_WIN32_WINNT=0x0A00
cmake .. -G "!GENERATOR_NAME!" ^
    -DCMAKE_BUILD_TYPE=Release ^
    -DCMAKE_C_COMPILER=gcc ^
    -DCMAKE_CXX_COMPILER=g++ ^
    -DCMAKE_C_FLAGS="%WIN_FLAGS%" ^
    -DCMAKE_CXX_FLAGS="%WIN_FLAGS%" ^
    -DBUILD_SHARED_LIBS=OFF ^
    -DWHISPER_BUILD_TESTS=OFF ^
    -DWHISPER_BUILD_EXAMPLES=OFF ^
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

if not exist "!WHISPER_OUT!\include" mkdir "!WHISPER_OUT!\include"
if not exist "!WHISPER_OUT!\lib" mkdir "!WHISPER_OUT!\lib"

:: Headers — copy all .h files from whisper source include dirs
:: NOTE: for /r does not work with delayed-expansion paths (!VAR!), use dir+for /f
echo   Collecting headers...
for /f "delims=" %%p in ('dir /s /b "!WHISPER_SRC!\include\*.h" 2^>nul') do (
    echo     Header: %%~nxp
    copy /y "%%p" "!WHISPER_OUT!\include\" >nul 2>&1
)
:: Also grab ggml headers from ggml/include if present
if exist "!WHISPER_SRC!\ggml\include" (
    for /f "delims=" %%p in ('dir /s /b "!WHISPER_SRC!\ggml\include\*.h" 2^>nul') do (
        echo     Header: %%~nxp
        copy /y "%%p" "!WHISPER_OUT!\include\" >nul 2>&1
    )
)

:: Libraries — collect all .a files from whisper build tree
:: NOTE: for /r does not work with delayed-expansion paths (!VAR!), use dir+for /f
echo   Collecting libraries from !WBUILD!...
for /f "delims=" %%f in ('dir /s /b "!WBUILD!\*.a" 2^>nul') do (
    echo     Found: %%f
    copy /y "%%f" "!WHISPER_OUT!\lib\" >nul
)
for /f "delims=" %%f in ('dir /b "!WHISPER_OUT!\lib\*.a" 2^>nul') do (
    set "wfn=%%f"
    if not "!wfn:~0,3!"=="lib" (
        if not exist "!WHISPER_OUT!\lib\lib%%f" rename "!WHISPER_OUT!\lib\%%f" "lib%%f"
    )
)

set /a WCOUNT=0
for /f "delims=" %%f in ('dir /b "!WHISPER_OUT!\lib\*.a" 2^>nul') do set /a WCOUNT+=1
if !WCOUNT!==0 (
    echo   WARNING: No whisper libraries found — listing build output:
    dir /s /b "!WBUILD!\*.a" "!WBUILD!\*.lib" "!WBUILD!\*.dll" 2>nul
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
set BUILD_TAGS=production
if !GHOSTAI!==1 set BUILD_TAGS=!BUILD_TAGS! ghostai
if !GHOSTVOICE!==1 set BUILD_TAGS=!BUILD_TAGS! ghostvoice

if !GHOSTAI!==1 if !GHOSTVOICE!==1 (
    echo [3] Building GhostSpell with Ghost-AI + Ghost Voice...
) else if !GHOSTAI!==1 (
    echo [3] Building GhostSpell with Ghost-AI ^(local AI^)...
) else (
    echo [2] Building GhostSpell ^(API-only mode^)...
)
set CGO_ENABLED=1

go build -tags "!BUILD_TAGS!" -o ghostspell.exe .
if !errorlevel! neq 0 (
    :: If build failed, retry without ghostai but keep ghostvoice if available
    if !GHOSTAI!==1 (
        echo.
        echo   Ghost-AI link failed — retrying without local AI...
        set GHOSTAI=0
        set RETRY_TAGS=production
        if !GHOSTVOICE!==1 set RETRY_TAGS=!RETRY_TAGS! ghostvoice
        go build -tags "!RETRY_TAGS!" -o ghostspell.exe .
        if !errorlevel! neq 0 (
            echo ERROR: Go build failed
            pause
            exit /b 1
        )
    ) else (
        echo ERROR: Go build failed
        pause
        exit /b 1
    )
)

echo.
echo ============================================
echo   BUILD COMPLETE: ghostspell.exe
if !GHOSTAI!==1 echo   + Ghost-AI ^(local text AI^)
if !GHOSTVOICE!==1 echo   + Ghost Voice ^(local speech-to-text^)
if !GHOSTAI!==0 if !GHOSTVOICE!==0 echo   Mode: API-only
echo ============================================
echo.

:: Clear the app log for a fresh testing session.
set APPDATA_DIR=%APPDATA%\GhostSpell
if exist "%APPDATA_DIR%\ghostspell.log" (
    del /q "%APPDATA_DIR%\ghostspell.log" 2>nul
    echo Cleared %APPDATA_DIR%\ghostspell.log
)
echo.
echo Starting GhostSpell...
start "" ghostspell.exe
echo.
echo Build log saved to: %LOGFILE%
goto :eof
