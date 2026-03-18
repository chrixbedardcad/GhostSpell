//go:build darwin

package screenshot

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation -framework ImageIO
#include <CoreGraphics/CoreGraphics.h>
#include <CoreFoundation/CoreFoundation.h>
#include <ImageIO/ImageIO.h>
#include <stdlib.h>

// captureActiveWindowPNG captures the frontmost on-screen window as PNG data.
// Returns a malloc'd buffer and its length, or NULL on failure.
static void* captureActiveWindowPNG(size_t *outLen) {
    *outLen = 0;

    // Get the list of on-screen windows, front-to-back order.
    CFArrayRef windowList = CGWindowListCopyWindowInfo(
        kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements,
        kCGNullWindowID);
    if (!windowList) return NULL;

    CGWindowID targetWID = 0;
    CFIndex count = CFArrayGetCount(windowList);

    // Find the first window that belongs to a non-system app and has a non-zero
    // area. This is the frontmost user window (the one behind GhostSpell's
    // indicator, if it's showing).
    for (CFIndex i = 0; i < count; i++) {
        CFDictionaryRef info = (CFDictionaryRef)CFArrayGetValueAtIndex(windowList, i);

        // Skip windows with layer != 0 (menu bar, dock, system overlays).
        CFNumberRef layerRef;
        int layer = 0;
        if (CFDictionaryGetValueIfPresent(info, kCGWindowLayer, (const void **)&layerRef)) {
            CFNumberGetValue(layerRef, kCFNumberIntType, &layer);
        }
        if (layer != 0) continue;

        // Skip windows owned by our own process.
        CFNumberRef pidRef;
        int pid = 0;
        if (CFDictionaryGetValueIfPresent(info, kCGWindowOwnerPID, (const void **)&pidRef)) {
            CFNumberGetValue(pidRef, kCFNumberIntType, &pid);
        }
        if (pid == getpid()) continue;

        // Require nonzero size.
        CGRect bounds;
        CFDictionaryRef boundsDict;
        if (!CFDictionaryGetValueIfPresent(info, kCGWindowBounds, (const void **)&boundsDict)) continue;
        if (!CGRectMakeWithDictionaryRepresentation(boundsDict, &bounds)) continue;
        if (bounds.size.width < 10 || bounds.size.height < 10) continue;

        CFNumberRef widRef;
        if (CFDictionaryGetValueIfPresent(info, kCGWindowNumber, (const void **)&widRef)) {
            int wid;
            CFNumberGetValue(widRef, kCFNumberIntType, &wid);
            targetWID = (CGWindowID)wid;
        }
        break;
    }
    CFRelease(windowList);

    if (targetWID == 0) return NULL;

    // Capture the window image.
    CGImageRef image = CGWindowListCreateImage(
        CGRectNull,
        kCGWindowListOptionIncludingWindow,
        targetWID,
        kCGWindowImageBoundsIgnoreFraming | kCGWindowImageNominalResolution);
    if (!image) return NULL;

    // Encode to PNG into a CFMutableData.
    CFMutableDataRef pngData = CFDataCreateMutable(kCFAllocatorDefault, 0);
    if (!pngData) { CGImageRelease(image); return NULL; }

    CGImageDestinationRef dest = CGImageDestinationCreateWithData(pngData, kUTTypePNG, 1, NULL);
    if (!dest) { CFRelease(pngData); CGImageRelease(image); return NULL; }

    CGImageDestinationAddImage(dest, image, NULL);
    bool ok = CGImageDestinationFinalize(dest);
    CFRelease(dest);
    CGImageRelease(image);

    if (!ok) { CFRelease(pngData); return NULL; }

    // Copy to a Go-friendly malloc'd buffer.
    CFIndex len = CFDataGetLength(pngData);
    void *buf = malloc((size_t)len);
    if (!buf) { CFRelease(pngData); return NULL; }
    CFDataGetBytes(pngData, CFRangeMake(0, len), (UInt8 *)buf);
    CFRelease(pngData);

    *outLen = (size_t)len;
    return buf;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// CaptureActiveWindow captures the frontmost window as PNG bytes.
func CaptureActiveWindow() ([]byte, error) {
	var length C.size_t
	ptr := C.captureActiveWindowPNG(&length)
	if ptr == nil {
		return nil, fmt.Errorf("failed to capture active window screenshot")
	}
	defer C.free(ptr)

	data := C.GoBytes(ptr, C.int(length))
	return data, nil
}
