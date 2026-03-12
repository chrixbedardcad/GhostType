#!/usr/bin/env bash
# Build Ghost-AI: fetch llama.cpp source and compile static libraries.
#
# Usage: ./scripts/build-ghostai.sh [--version b8281]
#
# Output: build/llama/lib/ and build/llama/include/
# These are referenced by CGo in llm/ghostai/engine_cgo.go.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Default llama.cpp version — matches BundledLlamaCppVersion in llm/local.go.
LLAMA_VERSION="${1:-b8281}"
if [[ "$LLAMA_VERSION" == --version ]]; then
    LLAMA_VERSION="${2:-b8281}"
fi

BUILD_DIR="$PROJECT_ROOT/build"
LLAMA_SRC="$BUILD_DIR/llama-src"
LLAMA_OUT="$BUILD_DIR/llama"

echo "=== Ghost-AI Build ==="
echo "llama.cpp version: $LLAMA_VERSION"
echo "Build dir:         $BUILD_DIR"
echo ""

# --- Step 1: Download llama.cpp source ---

if [ -d "$LLAMA_SRC" ] && [ -f "$LLAMA_SRC/.version" ]; then
    CACHED_VER=$(cat "$LLAMA_SRC/.version")
    if [ "$CACHED_VER" = "$LLAMA_VERSION" ]; then
        echo "[1/3] Using cached llama.cpp source ($CACHED_VER)"
    else
        echo "[1/3] Version changed ($CACHED_VER -> $LLAMA_VERSION), re-downloading..."
        rm -rf "$LLAMA_SRC"
    fi
fi

if [ ! -d "$LLAMA_SRC" ]; then
    echo "[1/3] Downloading llama.cpp $LLAMA_VERSION..."
    mkdir -p "$BUILD_DIR"
    TARBALL_URL="https://github.com/ggml-org/llama.cpp/archive/refs/tags/${LLAMA_VERSION}.tar.gz"
    curl -fsSL "$TARBALL_URL" | tar xz -C "$BUILD_DIR"
    mv "$BUILD_DIR/llama.cpp-${LLAMA_VERSION}" "$LLAMA_SRC"
    echo "$LLAMA_VERSION" > "$LLAMA_SRC/.version"
    echo "    Downloaded to $LLAMA_SRC"
fi

# --- Step 2: Build static libraries with CMake ---

echo "[2/3] Building static libraries..."

LLAMA_BUILD="$LLAMA_SRC/build"
mkdir -p "$LLAMA_BUILD"

CMAKE_ARGS=(
    -DCMAKE_BUILD_TYPE=Release
    -DGGML_STATIC=ON
    -DGGML_CUDA=OFF
    -DGGML_VULKAN=OFF
    -DGGML_METAL=OFF
    -DGGML_OPENMP=OFF
    -DLLAMA_BUILD_TESTS=OFF
    -DLLAMA_BUILD_EXAMPLES=OFF
    -DLLAMA_BUILD_SERVER=OFF
    -DBUILD_SHARED_LIBS=OFF
)

# macOS: enable Accelerate framework for faster BLAS operations.
if [ "$(uname)" = "Darwin" ]; then
    CMAKE_ARGS+=(-DGGML_ACCELERATE=ON)
    # Metal can be enabled for GPU but we keep CPU-only for now.
    # CMAKE_ARGS+=(-DGGML_METAL=ON)
fi

cd "$LLAMA_BUILD"
cmake .. "${CMAKE_ARGS[@]}" 2>&1 | tail -5
cmake --build . --config Release -j"$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)" 2>&1 | tail -3

# --- Step 3: Copy headers and libraries to output ---

echo "[3/3] Installing to $LLAMA_OUT..."

mkdir -p "$LLAMA_OUT/include" "$LLAMA_OUT/lib"

# Headers.
cp "$LLAMA_SRC/include/llama.h" "$LLAMA_OUT/include/"
# ggml headers may be in different locations depending on version.
for header_dir in "$LLAMA_SRC/ggml/include" "$LLAMA_SRC/include"; do
    if [ -d "$header_dir" ]; then
        cp "$header_dir"/*.h "$LLAMA_OUT/include/" 2>/dev/null || true
    fi
done

# Static libraries — search for them in the build tree.
find "$LLAMA_BUILD" -name '*.a' -exec cp {} "$LLAMA_OUT/lib/" \; 2>/dev/null || true
# On Windows (MinGW), look for .lib files too.
find "$LLAMA_BUILD" -name '*.lib' -exec cp {} "$LLAMA_OUT/lib/" \; 2>/dev/null || true

echo ""
echo "=== Build complete ==="
echo "Headers:   $(ls "$LLAMA_OUT/include/" 2>/dev/null | wc -l | tr -d ' ') files"
echo "Libraries: $(ls "$LLAMA_OUT/lib/" 2>/dev/null | wc -l | tr -d ' ') files"
ls -la "$LLAMA_OUT/lib/"
echo ""
echo "To build GhostSpell with Ghost-AI:"
echo "  go build -tags ghostai ./..."
