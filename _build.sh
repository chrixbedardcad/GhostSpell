#!/usr/bin/env bash
# GhostSpell Full Build — macOS
#
# Equivalent of _build.bat for Windows.
# Checks prerequisites, builds Ghost-AI + Ghost Voice + frontend + Go binary,
# then launches GhostSpell.
#
# Usage:
#   ./_build.sh           # full build + launch
#   ./_build.sh --clean   # delete build cache and rebuild from scratch
#
# All output goes to both console and build.log.

set -uo pipefail
LOGFILE="$(cd "$(dirname "$0")" && pwd)/build.log"
exec > >(tee "$LOGFILE") 2>&1

cd "$(dirname "$0")"

info()  { printf '\033[1;34m→ %s\033[0m\n' "$*"; }
ok()    { printf '\033[1;32m✓ %s\033[0m\n' "$*"; }
warn()  { printf '\033[1;33m⚠ %s\033[0m\n' "$*"; }
fail()  { printf '\033[1;31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

echo "============================================"
echo "         GhostSpell Full Build"
echo "============================================"
echo ""

# Kill any running GhostSpell processes before building.
# On macOS, ghostspell spawns ghostai and ghostvoice subprocesses —
# they must be stopped or the compiler can't overwrite the binaries.
for proc in ghostspell ghostai ghostvoice; do
    if pgrep -x "$proc" >/dev/null 2>&1; then
        echo "[pre-build] Stopping $proc..."
        pkill -x "$proc" 2>/dev/null || true
    fi
done
# Also kill the .app bundle process (macOS).
if pgrep -f "GhostSpell.app" >/dev/null 2>&1; then
    echo "[pre-build] Stopping GhostSpell.app..."
    pkill -f "GhostSpell.app" 2>/dev/null || true
fi
sleep 0.5

# --clean flag: delete build cache, source, and binaries — full rebuild from scratch.
if [[ "${1:-}" == "--clean" ]]; then
    echo "[clean] Deleting build cache, sources, and binaries..."
    rm -rf build/llama build/llama-src build/whisper build/whisper-src
    rm -f ghostai ghostspell ghost ghostvoice-darwin-* ghostvoice-linux-*
    echo "[clean] Done — rebuilding everything from scratch."
    echo ""
fi

# ============================================================
# Step 0 — Check prerequisites
# ============================================================
info "Checking prerequisites..."
echo ""

MISSING=0

if command -v go &>/dev/null; then
    echo "  go ......... OK ($(go version))"
else
    echo "  ERROR: 'go' not found. Install: brew install go"
    MISSING=1
fi

if command -v node &>/dev/null; then
    echo "  node ....... OK ($(node --version))"
else
    echo "  ERROR: 'node' not found. Install: brew install node"
    MISSING=1
fi

if command -v npm &>/dev/null; then
    echo "  npm ........ OK ($(npm --version))"
else
    echo "  ERROR: 'npm' not found. Install: brew install node"
    MISSING=1
fi

if [ "$MISSING" -eq 1 ]; then
    echo ""
    echo "  Install the missing tools above and try again."
    exit 1
fi

# Ghost-AI toolchain — auto-install missing pieces on macOS
GHOSTAI=0

# Xcode Command Line Tools (provides clang/clang++)
if ! command -v clang &>/dev/null && ! command -v gcc &>/dev/null; then
    info "Installing Xcode Command Line Tools (may prompt for password)..."
    xcode-select --install 2>/dev/null || true
    # Wait for installation to complete (user must click Install in the dialog).
    until command -v clang &>/dev/null || command -v gcc &>/dev/null; do
        sleep 5
    done
    ok "Xcode CLT installed"
fi

# CMake (auto-install via Homebrew if missing)
if ! command -v cmake &>/dev/null; then
    if command -v brew &>/dev/null; then
        info "Installing CMake via Homebrew..."
        brew install cmake
    else
        warn "CMake not found and Homebrew not available — skipping Ghost-AI/Ghost Voice"
    fi
fi

if command -v cmake &>/dev/null && (command -v clang &>/dev/null || command -v gcc &>/dev/null); then
    echo "  cmake ...... OK ($(cmake --version | head -1))"
    if command -v clang &>/dev/null; then
        echo "  clang ...... OK ($(clang --version | head -1))"
    else
        echo "  gcc ........ OK ($(gcc --version | head -1))"
    fi
    GHOSTAI=1
else
    echo ""
    warn "Ghost-AI toolchain not fully available — building in API-only mode."
    echo "        You can still use cloud providers (OpenAI, Anthropic, etc.)."
fi

echo ""

NPROC=$(sysctl -n hw.ncpu 2>/dev/null || echo 4)
PROJECT_ROOT="$(pwd)"
LLAMA_OUT="$PROJECT_ROOT/build/llama"

# ============================================================
# Step 1 — Build Ghost-AI static libraries (if toolchain found)
# ============================================================
if [ "$GHOSTAI" -eq 1 ]; then
    # Skip if libraries already built AND version matches build-ghostai.sh.
    EXPECTED_VER=$(grep '^LLAMA_VERSION=' scripts/build-ghostai.sh | head -1 | sed 's/.*:-\(.*\)}.*/\1/')
    CACHED_VER=""
    [ -f build/llama-src/.version ] && CACHED_VER=$(cat build/llama-src/.version)
    EXISTING_LIBS=$(ls build/llama/lib/*.a 2>/dev/null | wc -l | tr -d ' ')
    if [ "$EXISTING_LIBS" -ge 3 ] && [ "$CACHED_VER" = "$EXPECTED_VER" ]; then
        echo "[1] Ghost-AI libraries already built ($EXISTING_LIBS libs, $CACHED_VER) — skipping."
        echo "    To rebuild: delete the build/llama folder and re-run."
        echo ""
    elif [ "$EXISTING_LIBS" -ge 3 ] && [ "$CACHED_VER" != "$EXPECTED_VER" ]; then
        info "[1] llama.cpp version changed ($CACHED_VER → $EXPECTED_VER) — rebuilding..."
        rm -rf build/llama
        chmod +x scripts/build-ghostai.sh
        if ! ./scripts/build-ghostai.sh; then
            warn "Ghost-AI build failed — falling back to API-only build"
            GHOSTAI=0
        else
            ok "Ghost-AI built ($EXPECTED_VER)"
        fi
        echo ""
    else
        info "[1] Building Ghost-AI (llama.cpp)..."
        chmod +x scripts/build-ghostai.sh
        if ! ./scripts/build-ghostai.sh; then
            warn "Ghost-AI build failed — falling back to API-only build"
            GHOSTAI=0
        else
            ok "Ghost-AI built"
        fi
        echo ""
    fi
fi

# ============================================================
# Step 1.5 — Build Ghost Voice (whisper.cpp) if toolchain found
# ============================================================
GHOSTVOICE=0
if command -v cmake &>/dev/null && (command -v clang &>/dev/null || command -v gcc &>/dev/null); then
    GHOSTVOICE=1
fi

if [ "$GHOSTVOICE" -eq 1 ]; then
    # Skip if ghostvoice binary already exists and supports daemon mode
    ARCH=$(uname -m)
    case "$ARCH" in x86_64|amd64) ARCH="amd64" ;; arm64|aarch64) ARCH="arm64" ;; esac
    GHOSTVOICE_BIN=""
    for f in ghostvoice-darwin-* ghostvoice_bin ghostvoice; do
        [ -f "$f" ] && GHOSTVOICE_BIN="$f" && break
    done

    if [ -n "$GHOSTVOICE_BIN" ] && grep -q "daemon" "$GHOSTVOICE_BIN" 2>/dev/null; then
        echo "[1.5] Ghost Voice already built ($GHOSTVOICE_BIN) — skipping."
        echo "     To rebuild: delete $GHOSTVOICE_BIN and the build/whisper folder, then re-run."
        echo ""
    else
        info "[1.5] Building Ghost Voice (whisper.cpp)..."
        chmod +x scripts/build-ghostvoice.sh
        if ! ./scripts/build-ghostvoice.sh; then
            warn "Ghost Voice build failed — skipping"
            GHOSTVOICE=0
        else
            ok "Ghost Voice built"
        fi
        echo ""
    fi
fi

# ============================================================
# Step 2 — Build frontend
# ============================================================
if [ "$GHOSTAI" -eq 1 ]; then
    info "[2] Building frontend..."
else
    info "[1] Building frontend..."
fi
echo ""

cd gui/frontend
npm install --silent 2>&1 || npm install
if [ $? -ne 0 ]; then
    fail "npm install failed"
fi

echo ""
npm run build
if [ $? -ne 0 ]; then
    fail "frontend build failed"
fi
cd ../..
echo ""

# ============================================================
# Step 3 — Build Go binary
# ============================================================
MAIN_TAGS="production"
if [ "$GHOSTVOICE" -eq 1 ] && [ -f voicebin/ghostvoice ]; then
    MAIN_TAGS="$MAIN_TAGS ghostvoice"
fi
if [ "$GHOSTAI" -eq 1 ]; then
    info "[3] Building ghostspell + ghostai..."
else
    info "[2] Building ghostspell (API-only mode)..."
fi

export CGO_ENABLED=1
if ! go build -tags "$MAIN_TAGS" -o ghostspell .; then
    fail "Go build failed"
fi

# Build ghostai binary (pure C++ — links llama.cpp directly, same pattern as ghostvoice).
if [ "$GHOSTAI" -eq 1 ]; then
    echo "  Building ghostai (LLM server, C++)..."
    GHOSTAI_SRC="$PROJECT_ROOT/ghostai/main.cpp"
    ARCH_TAG=$(uname -m)
    case "$ARCH_TAG" in x86_64|amd64) ARCH_TAG="amd64" ;; arm64|aarch64) ARCH_TAG="arm64" ;; esac
    GHOSTAI_OUT="ghostai-darwin-${ARCH_TAG}"
    case "$(uname -s)" in
        Darwin)
            g++ -std=c++17 -O2 -o "$GHOSTAI_OUT" "$GHOSTAI_SRC" \
                -I"$LLAMA_OUT/include" -L"$LLAMA_OUT/lib" \
                -lllama -lggml -lggml-cpu -lggml-metal -lggml-blas -lggml-base \
                -lc++ -lm -lpthread \
                -framework Accelerate -framework Metal -framework MetalKit -framework Foundation
            ;;
        MINGW*|MSYS*|CYGWIN*)
            g++ -O2 -static -fopenmp -o "$GHOSTAI_OUT" "$GHOSTAI_SRC" \
                -I"$LLAMA_OUT/include" -L"$LLAMA_OUT/lib" \
                -lllama -lggml -lggml-cpu -lggml-base \
                -lstdc++ -lm -lpthread -lkernel32
            ;;
        *)
            g++ -O2 -fopenmp -o "$GHOSTAI_OUT" "$GHOSTAI_SRC" \
                -I"$LLAMA_OUT/include" -L"$LLAMA_OUT/lib" \
                -Wl,--start-group -lllama -lggml -lggml-cpu -lggml-base -Wl,--end-group \
                -lstdc++ -lm -lpthread
            ;;
    esac
    if [ $? -eq 0 ]; then
        echo "  ghostai built OK"
    else
        warn "ghostai build failed — local AI will not be available"
        GHOSTAI=0
    fi
fi

# Build ghost CLI (pure Go, no CGo needed).
echo "  Building ghost (CLI)..."
if go build -tags "production" -o ghost ./cmd/ghost; then
    echo "  ghost built OK"
else
    echo "  WARNING: ghost build failed — CLI will not be available"
fi

echo ""
echo "============================================"
echo "  BUILD COMPLETE: ghostspell + ghost"
[ "$GHOSTAI" -eq 1 ] && echo "  + ghostai (local LLM server)"
[ "$GHOSTVOICE" -eq 1 ] && echo "  + ghostvoice (local speech-to-text)"
[ "$GHOSTAI" -eq 0 ] && [ "$GHOSTVOICE" -eq 0 ] && echo "  Mode: API-only"
echo "============================================"
echo ""

# Clear logs for a fresh testing session.
APPDATA_DIR="$HOME/Library/Application Support/GhostSpell"
if [ -f "$APPDATA_DIR/ghostspell.log" ]; then
    rm -f "$APPDATA_DIR/ghostspell.log"
    echo "Cleared $APPDATA_DIR/ghostspell.log"
fi
if [ -f "$APPDATA_DIR/ghostvoice.log" ]; then
    rm -f "$APPDATA_DIR/ghostvoice.log"
    echo "Cleared $APPDATA_DIR/ghostvoice.log"
fi
if [ -f "$APPDATA_DIR/ghostspell_crash.log" ]; then
    rm -f "$APPDATA_DIR/ghostspell_crash.log"
    echo "Cleared $APPDATA_DIR/ghostspell_crash.log"
fi
if [ -f "$APPDATA_DIR/ghost-server.log" ]; then
    rm -f "$APPDATA_DIR/ghost-server.log"
    echo "Cleared $APPDATA_DIR/ghost-server.log"
fi
if [ -f "$APPDATA_DIR/ghostai.log" ]; then
    rm -f "$APPDATA_DIR/ghostai.log"
    echo "Cleared $APPDATA_DIR/ghostai.log"
fi
echo ""

if [ -n "${GHOSTSPELL_NO_LAUNCH:-}" ]; then
    echo "Build complete. Skipping launch (GHOSTSPELL_NO_LAUNCH set)."
else
    echo "Starting GhostSpell..."
    ./ghostspell &
fi
echo ""
echo "Build log saved to: $LOGFILE"
