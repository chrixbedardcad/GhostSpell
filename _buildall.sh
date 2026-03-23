#!/usr/bin/env bash
# Full clean rebuild — deletes all cached C libraries and rebuilds from scratch.
# Use _build.sh for fast incremental builds.

echo "============================================"
echo "      GhostSpell FULL CLEAN BUILD"
echo "============================================"
echo ""

cd "$(dirname "$0")"

echo "Cleaning build cache..."
rm -rf build/llama build/whisper
echo "Done."
echo ""

exec ./_build.sh
