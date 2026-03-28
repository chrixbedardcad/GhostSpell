#!/usr/bin/env bash
# Build Ghost Voice: fetch whisper.cpp source and compile static libraries.
#
# Usage: ./scripts/build-ghostvoice.sh [--version v1.8.4]
#
# Output: build/whisper/lib/ and build/whisper/include/
# These are referenced by CGo in stt/ghostvoice/engine_cgo.go.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Default whisper.cpp version.
WHISPER_VERSION="${1:-v1.8.4}"
if [[ "$WHISPER_VERSION" == --version ]]; then
    WHISPER_VERSION="${2:-v1.8.4}"
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
    # The extracted directory name varies: whisper.cpp-v1.8.4 or whisper.cpp-1.7.5
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
    -DGGML_OPENMP=ON
    -DGGML_NATIVE=OFF
    -DGGML_AVX=ON
    -DGGML_AVX2=ON
    -DGGML_FMA=ON
    -DGGML_F16C=ON
)

# Platform-specific GPU acceleration.
OS="$(uname -s)"
case "$OS" in
    Darwin)
        CMAKE_ARGS+=(-DGGML_ACCELERATE=ON -DGGML_METAL=ON)
        echo "  Metal GPU acceleration enabled"
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

# --- Step 3: Install via cmake ---
# cmake --install produces a flat, stable layout in WHISPER_OUT regardless
# of whisper.cpp's internal build directory structure.
echo "[3/3] Installing headers + libraries..."
cmake --install "$WHISPER_SRC/build-static" --prefix "$WHISPER_OUT" > /dev/null 2>&1

# Ensure lib* prefix for MinGW linker (cmake install may omit it on Windows).
case "$(uname -s)" in
    MINGW*|MSYS*|CYGWIN*)
        for lib in "$WHISPER_OUT/lib/"*.a; do
            [ -f "$lib" ] || continue
            base=$(basename "$lib")
            [[ "$base" == lib* ]] || mv "$lib" "$WHISPER_OUT/lib/lib$base"
        done
        ;;
esac

# Build ghostvoice binary — GhostSpell's own STT helper (pure C++).
# All link paths use the flat WHISPER_OUT/lib layout from cmake install.
echo "Building ghostvoice..."
GHOSTVOICE_SRC="$PROJECT_ROOT/ghostvoice/main.cpp"
ARCH=$(uname -m)
case "$ARCH" in x86_64|amd64) ARCH="amd64" ;; arm64|aarch64) ARCH="arm64" ;; esac
GHOSTVOICE_OUT="$PROJECT_ROOT/ghostvoice-linux-${ARCH}"
case "$OS" in
    MINGW*|MSYS*|CYGWIN*)
        GHOSTVOICE_OUT="$PROJECT_ROOT/ghostvoice-windows-${ARCH}.exe"
        g++ -O2 -static -fopenmp -o "$GHOSTVOICE_OUT" "$GHOSTVOICE_SRC" \
            -I"$WHISPER_OUT/include" -L"$WHISPER_OUT/lib" \
            -lwhisper -lggml -lggml-cpu -lggml-base \
            -lstdc++ -lm -lpthread -lkernel32
        ;;
    Darwin)
        GHOSTVOICE_OUT="$PROJECT_ROOT/ghostvoice-darwin-${ARCH}"
        g++ -std=c++17 -O2 -o "$GHOSTVOICE_OUT" "$GHOSTVOICE_SRC" \
            -I"$WHISPER_OUT/include" -L"$WHISPER_OUT/lib" \
            -lwhisper -lggml -lggml-cpu -lggml-metal -lggml-blas -lggml-base \
            -lc++ -lm -lpthread \
            -framework Accelerate -framework Metal -framework MetalKit -framework Foundation
        ;;
    *)
        g++ -O2 -fopenmp -o "$GHOSTVOICE_OUT" "$GHOSTVOICE_SRC" \
            -I"$WHISPER_OUT/include" -L"$WHISPER_OUT/lib" \
            -Wl,--start-group -lwhisper -lggml -lggml-cpu -lggml-base -Wl,--end-group \
            -lstdc++ -lm -lpthread
        ;;
esac

# Stage for go:embed — copy to voicebin/ with the platform-correct name.
mkdir -p "$PROJECT_ROOT/voicebin"
case "$OS" in
    MINGW*|MSYS*|CYGWIN*)
        cp "$GHOSTVOICE_OUT" "$PROJECT_ROOT/voicebin/ghostvoice.exe"
        ;;
    *)
        cp "$GHOSTVOICE_OUT" "$PROJECT_ROOT/voicebin/ghostvoice"
        ;;
esac

echo ""
echo "=== Ghost Voice Build Complete ==="
echo "Headers: $WHISPER_OUT/include/"
ls "$WHISPER_OUT/include/" 2>/dev/null
echo "Libraries: $WHISPER_OUT/lib/"
ls "$WHISPER_OUT/lib/" 2>/dev/null
echo "ghostvoice: $GHOSTVOICE_OUT"
echo "Staged for embedding: $PROJECT_ROOT/voicebin/"
ls -lh "$PROJECT_ROOT/voicebin/" 2>/dev/null
