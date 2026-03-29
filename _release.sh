#!/usr/bin/env bash
# GhostSpell Release — macOS
#
# Builds everything, creates .dmg with code signing + notarization,
# and uploads to a GitHub Release. The in-app update checker picks
# up the new version automatically.
#
# Prerequisites: gh (GitHub CLI) authenticated, git clean working tree,
#                Apple Developer signing identity for code signing.
#
# Usage:
#   ./_release.sh              # build + sign + upload
#   ./_release.sh --skip-build # upload existing DMG (already built)

set -uo pipefail
cd "$(dirname "$0")"

info()  { printf '\033[1;34m→ %s\033[0m\n' "$*"; }
ok()    { printf '\033[1;32m✓ %s\033[0m\n' "$*"; }
warn()  { printf '\033[1;33m⚠ %s\033[0m\n' "$*"; }
fail()  { printf '\033[1;31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

echo "============================================"
echo "      GhostSpell Release — macOS"
echo "============================================"
echo ""

# --- Pre-flight checks ---
command -v gh &>/dev/null || fail "gh (GitHub CLI) not found. Install: brew install gh"

# Read version from source of truth.
VER=$(grep 'const Version' internal/version/version.go | sed 's/.*"\(.*\)"/\1/')
[ -n "$VER" ] || fail "Could not read version from internal/version/version.go"
TAG="v$VER"
ARCH=$(uname -m)
case "$ARCH" in x86_64|amd64) ARCH="amd64" ;; arm64|aarch64) ARCH="arm64" ;; esac

echo "Version: $VER"
echo "Tag:     $TAG"
echo "Arch:    $ARCH"
echo ""

# Check for uncommitted changes.
if ! git diff --quiet 2>/dev/null; then
    warn "You have uncommitted changes:"
    git status --short
    echo ""
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo ""
    [[ $REPLY =~ ^[Yy]$ ]] || exit 1
fi

# --- Build (unless --skip-build) ---
if [ "${1:-}" = "--skip-build" ]; then
    echo "Skipping build (--skip-build)"
    echo ""
else
    info "[1/4] Building..."
    echo ""
    chmod +x _build.sh
    ./_build.sh
    # _build.sh launches ghostspell in background — kill it so we can package.
    sleep 2
    for proc in ghostspell ghostai ghostvoice; do
        pkill -x "$proc" 2>/dev/null || true
    done
    pkill -f "GhostSpell.app" 2>/dev/null || true
    sleep 1
    echo ""
fi

# --- Verify artifacts exist ---
info "[2/4] Checking artifacts..."
GHOSTSPELL_BIN=""
for f in ghostspell ghostspell-darwin-*; do
    [ -f "$f" ] && GHOSTSPELL_BIN="$f" && break
done
[ -n "$GHOSTSPELL_BIN" ] || fail "MISSING: ghostspell binary"

GHOSTAI_BIN=""
for f in ghostai ghostai-darwin-*; do
    [ -f "$f" ] && GHOSTAI_BIN="$f" && break
done

GHOST_BIN=""
for f in ghost ghost-darwin-*; do
    [ -f "$f" ] && GHOST_BIN="$f" && break
done

echo "  ghostspell: $GHOSTSPELL_BIN"
echo "  ghostai:    ${GHOSTAI_BIN:-MISSING (API-only mode)}"
echo "  ghost:      ${GHOST_BIN:-MISSING}"
echo ""

# --- Bundle .app + .dmg ---
info "[3/4] Bundling macOS .app + .dmg..."

# The bundle script expects the binary at a specific path.
cp "$GHOSTSPELL_BIN" "ghostspell-darwin-${ARCH}"
chmod +x scripts/bundle-macos.sh
./scripts/bundle-macos.sh "ghostspell-darwin-${ARCH}" "$ARCH"

DMG="GhostSpell-darwin-${ARCH}.dmg"
[ -f "$DMG" ] || fail "DMG not created: $DMG"
ok "DMG created: $DMG"
echo ""

# --- Package release artifacts ---
mkdir -p release
cp "$DMG" "release/"

# Also include standalone binaries for non-DMG installs.
if [ -n "$GHOSTAI_BIN" ]; then
    cp "$GHOSTAI_BIN" "release/ghostai-darwin-${ARCH}"
fi
if [ -n "$GHOST_BIN" ]; then
    cp "$GHOST_BIN" "release/ghost-darwin-${ARCH}"
fi

# --- Create tag + upload ---
info "[4/4] Uploading to GitHub Release $TAG..."
echo ""

# Create tag if it doesn't exist.
git tag "$TAG" 2>/dev/null || true
git push origin "$TAG" 2>/dev/null || true

# Create or update the release.
if gh release create "$TAG" release/* --title "$TAG" --generate-release-notes; then
    ok "Release created"
else
    echo ""
    echo "Release may already exist. Uploading assets to existing release..."
    for f in release/*; do
        gh release upload "$TAG" "$f" --clobber
    done
    ok "Assets uploaded"
fi

echo ""
echo "============================================"
echo "  RELEASE COMPLETE: $TAG"
echo "  https://github.com/chrixbedardcad/GhostSpell/releases/tag/$TAG"
echo "============================================"
echo ""
