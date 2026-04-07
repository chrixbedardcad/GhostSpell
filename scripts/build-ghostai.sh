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

# Default llama.cpp version — matches BundledLlamaCppVersion in llm/models_local.go.
LLAMA_VERSION="${1:-b8545}"
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

# --- Step 1.5: Apply Metal compatibility patch (macOS 13) ---
if [ -x "$SCRIPT_DIR/patch-metal.sh" ]; then
    "$SCRIPT_DIR/patch-metal.sh" "$LLAMA_SRC"
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
    -DGGML_OPENMP=ON
    -DLLAMA_BUILD_TESTS=OFF
    -DLLAMA_BUILD_EXAMPLES=OFF
    -DLLAMA_BUILD_SERVER=OFF
    -DBUILD_SHARED_LIBS=OFF
    -DGGML_NATIVE=OFF
    -DGGML_AVX=ON
    -DGGML_AVX2=ON
    -DGGML_AVX512=OFF
    -DGGML_FMA=ON
    -DGGML_F16C=ON
)

# Platform-specific GPU acceleration + compiler setup.
HAS_CUDA=0
case "$(uname -s)" in
    Darwin)
        CMAKE_ARGS+=(-DGGML_ACCELERATE=ON -DGGML_METAL=ON)
        echo "  Metal GPU acceleration enabled"
        ;;
    MINGW*|MSYS*)
        WIN_FLAGS="-D_WIN32_WINNT=0x0A00"
        # CUDA + MSVC: build static libraries (CUDA embedded in ghostai.exe).
        # Non-CUDA: static .a with MinGW GCC.
        if command -v nvcc &>/dev/null && command -v cl &>/dev/null; then
            echo "  CUDA + MSVC detected — building static libraries"
            HAS_CUDA=1
            # Remove MinGW from PATH so cmake finds MSVC, not GCC.
            SAVED_PATH="$PATH"
            export PATH=$(echo "$PATH" | tr ':' '\n' | grep -v '/mingw64/' | grep -v '/msys64/usr/' | tr '\n' ':')
            CMAKE_ARGS=( # Reset — MSVC build uses different flags.
                -G Ninja
                -DCMAKE_BUILD_TYPE=Release
                -DCMAKE_C_FLAGS="$WIN_FLAGS"
                -DCMAKE_CXX_FLAGS="$WIN_FLAGS"
                -DGGML_CUDA=ON
                -DGGML_VULKAN=OFF
                -DGGML_METAL=OFF
                -DGGML_OPENMP=ON
                -DGGML_STATIC=ON
                -DLLAMA_BUILD_TESTS=OFF
                -DLLAMA_BUILD_EXAMPLES=OFF
                -DLLAMA_BUILD_SERVER=OFF
                -DBUILD_SHARED_LIBS=OFF
                -DGGML_NATIVE=OFF
                -DGGML_AVX=ON
                -DGGML_AVX2=ON
                -DGGML_AVX512=OFF
                -DGGML_FMA=ON
                -DGGML_F16C=ON
            )
            # CI has no GPU — use explicit architectures covering ~8 years of GPUs.
            # Local builds with a GPU: cmake auto-detects via GGML_NATIVE.
            if [ -z "${CUDA_PATH:-}" ] || ! nvidia-smi &>/dev/null; then
                CMAKE_ARGS+=(-DCMAKE_CUDA_ARCHITECTURES="60;61;70;75;80;86;89")
                echo "  CUDA architectures: 60;61;70;75;80;86;89 (CI/no GPU)"
            fi
        else
            echo "  MinGW CPU build (no CUDA)"
            CMAKE_ARGS+=(-G Ninja -DCMAKE_C_COMPILER=gcc -DCMAKE_CXX_COMPILER=g++)
            CMAKE_ARGS+=(-DCMAKE_C_FLAGS="$WIN_FLAGS" -DCMAKE_CXX_FLAGS="$WIN_FLAGS")
        fi
        ;;
esac

cd "$LLAMA_BUILD"
cmake .. "${CMAKE_ARGS[@]}" 2>&1 | tail -5

JOBS=$(nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo ${NUMBER_OF_PROCESSORS:-4})
cmake --build . --config Release -j"$JOBS" 2>&1 | tail -5

# Restore MinGW PATH after CUDA build.
if [ "$HAS_CUDA" = "1" ] && [ -n "${SAVED_PATH:-}" ]; then
    export PATH="$SAVED_PATH"
fi

# --- Step 3: Install headers + libraries ---
# Try cmake --install first (flat, stable layout). Fall back to manual copy
# if install rules don't cover all targets (common with static MinGW builds).

echo "[3/3] Installing to $LLAMA_OUT..."
cmake --install "$LLAMA_BUILD" --prefix "$LLAMA_OUT" > /dev/null 2>&1

# Ensure lib* prefix for MinGW linker (cmake install may produce ggml.a instead of libggml.a).
case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*)
        for lib in "$LLAMA_OUT/lib/"*.a; do
            [ -f "$lib" ] || continue
            base=$(basename "$lib")
            [[ "$base" == lib* ]] || mv "$lib" "$LLAMA_OUT/lib/lib$base"
        done
        ;;
esac

# Check if cmake install produced libraries; if not, copy manually.
LIB_COUNT=$(ls "$LLAMA_OUT/lib/" 2>/dev/null | wc -l | tr -d ' ')
if [ "$LIB_COUNT" -eq 0 ]; then
    echo "  cmake install produced no libs — falling back to manual copy"
    mkdir -p "$LLAMA_OUT/include" "$LLAMA_OUT/lib"
    cp "$LLAMA_SRC/include/llama.h" "$LLAMA_OUT/include/" 2>/dev/null || true
    for d in "$LLAMA_SRC/ggml/include" "$LLAMA_SRC/include"; do
        [ -d "$d" ] && cp "$d"/*.h "$LLAMA_OUT/include/" 2>/dev/null || true
    done
    find "$LLAMA_BUILD" -name '*.a' -exec cp {} "$LLAMA_OUT/lib/" \; 2>/dev/null || true
    find "$LLAMA_BUILD" -name '*.lib' -exec cp {} "$LLAMA_OUT/lib/" \; 2>/dev/null || true
    find "$LLAMA_BUILD" -name '*.dll' -exec sh -c 'mkdir -p "$1/bin" && cp "$2" "$1/bin/"' _ "$LLAMA_OUT" {} \; 2>/dev/null || true
    # Ensure lib* prefix for MinGW linker.
    for lib in "$LLAMA_OUT/lib/"*.a; do
        [ -f "$lib" ] || continue
        base=$(basename "$lib")
        [[ "$base" == lib* ]] || mv "$lib" "$LLAMA_OUT/lib/lib$base"
    done
fi

# Windows/MinGW: generate MinGW .a import libs from any DLLs.
# CGo (MinGW) can't link MSVC .lib directly.
case "$(uname -s)" in
    MINGW*|MSYS*)
        for dll in "$LLAMA_OUT/bin/"*.dll; do
            [ -f "$dll" ] || continue
            dllname=$(basename "$dll" .dll)
            gendef "$dll" > /dev/null 2>&1
            if [ -f "${dllname}.def" ]; then
                dlltool -d "${dllname}.def" -l "$LLAMA_OUT/lib/lib${dllname}.a" -D "$(basename "$dll")" > /dev/null 2>&1
                rm -f "${dllname}.def"
            fi
        done
        ;;
esac

# Note: no ggml-cuda stub needed — ghostai is pure C++ and only links
# -lggml-cuda when CUDA is actually built (local MSVC builds).

echo ""
echo "=== Build complete ==="
echo "Headers:   $(ls "$LLAMA_OUT/include/" 2>/dev/null | wc -l | tr -d ' ') files"
echo "Libraries: $(ls "$LLAMA_OUT/lib/" 2>/dev/null | wc -l | tr -d ' ') files"
ls -la "$LLAMA_OUT/lib/"
echo ""
echo "To build GhostSpell with Ghost-AI:"
echo "  go build -tags ghostai ./..."
