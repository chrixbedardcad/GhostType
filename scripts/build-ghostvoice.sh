#!/usr/bin/env bash
# Build Ghost Voice: fetch whisper.cpp source and compile static libraries.
#
# Usage: ./scripts/build-ghostvoice.sh [--version v1.7.5]
#
# Output: build/whisper/lib/ and build/whisper/include/
# These are referenced by CGo in stt/ghostvoice/engine_cgo.go.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Default whisper.cpp version.
WHISPER_VERSION="${1:-v1.7.5}"
if [[ "$WHISPER_VERSION" == --version ]]; then
    WHISPER_VERSION="${2:-v1.7.5}"
fi

BUILD_DIR="$PROJECT_ROOT/build"
WHISPER_SRC="$BUILD_DIR/whisper-src"
WHISPER_OUT="$BUILD_DIR/whisper"

echo "=== Ghost Voice Build ==="
echo "whisper.cpp version: $WHISPER_VERSION"
echo "Build dir:           $BUILD_DIR"
echo ""

# --- Step 1: Download whisper.cpp source ---

if [ -d "$WHISPER_SRC" ] && [ -f "$WHISPER_SRC/.version" ]; then
    CACHED_VER=$(cat "$WHISPER_SRC/.version")
    if [ "$CACHED_VER" = "$WHISPER_VERSION" ]; then
        echo "[1/3] Using cached whisper.cpp source ($CACHED_VER)"
    else
        echo "[1/3] Version changed ($CACHED_VER -> $WHISPER_VERSION), re-downloading..."
        rm -rf "$WHISPER_SRC"
    fi
fi

if [ ! -d "$WHISPER_SRC" ]; then
    echo "[1/3] Downloading whisper.cpp $WHISPER_VERSION..."
    mkdir -p "$BUILD_DIR"
    TARBALL_URL="https://github.com/ggml-org/whisper.cpp/archive/refs/tags/${WHISPER_VERSION}.tar.gz"
    curl -fsSL "$TARBALL_URL" | tar xz -C "$BUILD_DIR"
    # The extracted directory name varies: whisper.cpp-v1.7.5 or whisper.cpp-1.7.5
    EXTRACTED=$(ls -d "$BUILD_DIR"/whisper.cpp-* 2>/dev/null | head -1)
    if [ -z "$EXTRACTED" ]; then
        echo "ERROR: Failed to find extracted whisper.cpp directory"
        exit 1
    fi
    mv "$EXTRACTED" "$WHISPER_SRC"
    echo "$WHISPER_VERSION" > "$WHISPER_SRC/.version"
    echo "    Downloaded to $WHISPER_SRC"
fi

# --- Step 2: Build static libraries ---

echo "[2/3] Building whisper.cpp static libraries..."
mkdir -p "$WHISPER_SRC/build-static"
cd "$WHISPER_SRC/build-static"

CMAKE_ARGS=(
    -DCMAKE_BUILD_TYPE=Release
    -DBUILD_SHARED_LIBS=OFF
    -DWHISPER_BUILD_TESTS=OFF
    -DWHISPER_BUILD_EXAMPLES=ON
    -DGGML_STATIC=ON
    -DGGML_CUDA=OFF
    -DGGML_VULKAN=OFF
    -DGGML_METAL=OFF
    -DGGML_OPENMP=OFF
    -DGGML_NATIVE=OFF
    -DGGML_AVX=ON
    -DGGML_AVX2=OFF
    -DGGML_FMA=OFF
    -DGGML_F16C=OFF
)

# Platform-specific flags.
OS="$(uname -s)"
case "$OS" in
    Darwin)
        CMAKE_ARGS+=(-DGGML_ACCELERATE=ON)
        ;;
    MINGW*|MSYS*|CYGWIN*)
        CMAKE_ARGS+=(
            -G "MinGW Makefiles"
            -DCMAKE_C_FLAGS="-D_WIN32_WINNT=0x0A00"
            -DCMAKE_CXX_FLAGS="-D_WIN32_WINNT=0x0A00"
        )
        ;;
esac

cmake "$WHISPER_SRC" "${CMAKE_ARGS[@]}"

# Parallel build.
JOBS=$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)
cmake --build . --config Release -j "$JOBS"

# --- Step 3: Copy outputs ---

echo "[3/3] Copying headers, libraries, and whisper-cli..."
mkdir -p "$WHISPER_OUT/include" "$WHISPER_OUT/lib" "$WHISPER_OUT/bin"

# Headers.
cp "$WHISPER_SRC/include/whisper.h" "$WHISPER_OUT/include/" 2>/dev/null || true
cp "$WHISPER_SRC/whisper.h" "$WHISPER_OUT/include/" 2>/dev/null || true
# Also copy ggml headers if present.
for h in ggml.h ggml-alloc.h ggml-backend.h; do
    find "$WHISPER_SRC" -maxdepth 3 -name "$h" -exec cp {} "$WHISPER_OUT/include/" \; 2>/dev/null || true
done

# Libraries — find all .a files from the build.
find "$WHISPER_SRC/build-static" -name "*.a" -exec cp {} "$WHISPER_OUT/lib/" \;

# On Windows/MinGW, rename libXXX.dll.a → libXXX.a if needed.
for f in "$WHISPER_OUT/lib"/*.dll.a; do
    [ -f "$f" ] && mv "$f" "${f%.dll.a}.a"
done

# Build ghostvoice binary — GhostSpell's own STT helper (pure C++).
echo "Building ghostvoice..."
GHOSTVOICE_SRC="$PROJECT_ROOT/ghostvoice/main.cpp"
GHOSTVOICE_OUT="$PROJECT_ROOT/ghostvoice_bin"
case "$OS" in
    MINGW*|MSYS*|CYGWIN*)
        GHOSTVOICE_OUT="$PROJECT_ROOT/ghostvoice.exe"
        g++ -O2 -static -o "$GHOSTVOICE_OUT" "$GHOSTVOICE_SRC" \
            -I"$WHISPER_SRC/include" -I"$WHISPER_SRC/ggml/include" \
            -L"$WHISPER_SRC/build-static/src" -L"$WHISPER_SRC/build-static/ggml/src" \
            -l:libwhisper.a -l:ggml.a -l:ggml-cpu.a -l:ggml-base.a \
            -lstdc++ -lm -lpthread -lkernel32
        ;;
    Darwin)
        g++ -O2 -o "$GHOSTVOICE_OUT" "$GHOSTVOICE_SRC" \
            -I"$WHISPER_OUT/include" -L"$WHISPER_OUT/lib" \
            -lwhisper -lggml -lggml-cpu -lggml-base \
            -lc++ -lm -lpthread -framework Accelerate
        ;;
    *)
        g++ -O2 -o "$GHOSTVOICE_OUT" "$GHOSTVOICE_SRC" \
            -I"$WHISPER_OUT/include" -L"$WHISPER_OUT/lib" \
            -Wl,--start-group -lwhisper -lggml -lggml-cpu -lggml-base -Wl,--end-group \
            -lstdc++ -lm -lpthread
        ;;
esac

echo ""
echo "=== Ghost Voice Build Complete ==="
echo "Headers: $WHISPER_OUT/include/"
ls "$WHISPER_OUT/include/" 2>/dev/null
echo "Libraries: $WHISPER_OUT/lib/"
ls "$WHISPER_OUT/lib/" 2>/dev/null
echo "ghostvoice: $GHOSTVOICE_OUT"
echo ""
echo "Place ghostvoice next to ghostspell, then run GhostSpell."
