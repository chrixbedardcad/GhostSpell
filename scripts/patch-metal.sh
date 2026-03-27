#!/usr/bin/env bash
# patch-metal.sh — Fix Metal GGML_ASSERT crash on macOS 13 (Ventura).
#
# Patches ggml_metal_get_tensor_async to fall back to synchronous memcpy
# when newBufferWithBytesNoCopy returns nil (page-alignment issue).
# Also patches ggml_metal_set_tensor_async for the same issue.
#
# Ref: https://github.com/ggml-org/llama.cpp/issues/16266
#
# Usage: ./scripts/patch-metal.sh <llama-src-dir>

set -euo pipefail

SRC="${1:?Usage: $0 <llama-src-dir>}"
FILE="$SRC/ggml/src/ggml-metal/ggml-metal-context.m"

if [ ! -f "$FILE" ]; then
    echo "[patch-metal] File not found: $FILE — skipping"
    exit 0
fi

if grep -q "GHOSTSPELL_METAL_PATCH" "$FILE" 2>/dev/null; then
    echo "[patch-metal] Already patched — skipping"
    exit 0
fi

echo "[patch-metal] Patching $FILE for macOS 13 compatibility..."

# Replace the GGML_ASSERT(buf_dst) with a fallback to sync memcpy.
python3 - "$FILE" << 'PYEOF'
import sys

path = sys.argv[1]
with open(path, 'r') as f:
    content = f.read()

# Patch ggml_metal_get_tensor_async: replace GGML_ASSERT(buf_dst) with fallback
old_get = '''        GGML_ASSERT(buf_dst);

        struct ggml_metal_buffer_id bid_src = ggml_metal_get_buffer_id(tensor);'''

new_get = '''        // GHOSTSPELL_METAL_PATCH: fallback to sync memcpy if newBufferWithBytesNoCopy
        // fails (page-alignment issue on macOS 13). See ggml-org/llama.cpp#16266.
        if (buf_dst == nil) {
            struct ggml_metal_buffer_id bid_src = ggml_metal_get_buffer_id(tensor);
            if (bid_src.metal == nil) {
                GGML_ABORT("%s: failed to find buffer for tensor '%s'\\n", __func__, tensor->name);
            }
            // Wait for any pending GPU work, then memcpy directly.
            id<MTLCommandQueue> queue = ggml_metal_device_get_queue(ctx->dev);
            id<MTLCommandBuffer> cmd_buf = [queue commandBuffer];
            [cmd_buf commit];
            [cmd_buf waitUntilCompleted];
            const void * src = (const char *)[bid_src.metal contents] + bid_src.offs + offset;
            memcpy(data, src, size);
            return;
        }

        struct ggml_metal_buffer_id bid_src = ggml_metal_get_buffer_id(tensor);'''

if old_get in content:
    content = content.replace(old_get, new_get)
    with open(path, 'w') as f:
        f.write(content)
    print("[patch-metal] Patched ggml_metal_get_tensor_async — OK")
else:
    print("[patch-metal] WARNING: could not find get_tensor_async pattern to patch")

PYEOF

echo "[patch-metal] Done"
