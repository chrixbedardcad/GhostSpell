//go:build darwin

package clipboard

/*
#cgo LDFLAGS: -framework CoreFoundation -framework Carbon

#include <CoreFoundation/CoreFoundation.h>
#include <Carbon/Carbon.h>

// readClipboardUTF8 reads the clipboard content as UTF-8 text.
// Returns a CFDataRef containing the UTF-8 bytes, or NULL on failure/empty.
// The caller must CFRelease the returned data.
CFDataRef readClipboardUTF8(void) {
	PasteboardRef pb;
	if (PasteboardCreate(kPasteboardClipboard, &pb) != noErr) return NULL;
	PasteboardSynchronize(pb);

	ItemCount count;
	if (PasteboardGetItemCount(pb, &count) != noErr || count == 0) {
		CFRelease(pb);
		return NULL;
	}

	PasteboardItemID itemID;
	if (PasteboardGetItemIdentifier(pb, 1, &itemID) != noErr) {
		CFRelease(pb);
		return NULL;
	}

	CFDataRef data = NULL;
	OSStatus err = PasteboardCopyItemFlavorData(pb, itemID,
		CFSTR("public.utf8-plain-text"), &data);
	CFRelease(pb);
	if (err != noErr) return NULL;
	return data;
}

// writeClipboardUTF8 writes UTF-8 text to the clipboard with explicit
// "public.utf8-plain-text" flavor. This avoids encoding ambiguity that
// can occur when shelling out to pbcopy (which may use the process locale).
// Returns 0 on success, -1 on failure.
int writeClipboardUTF8(const void *bytes, int len) {
	PasteboardRef pb;
	if (PasteboardCreate(kPasteboardClipboard, &pb) != noErr) return -1;
	PasteboardClear(pb);
	PasteboardSynchronize(pb);

	CFDataRef data = CFDataCreate(NULL, (const UInt8 *)bytes, len);
	if (!data) {
		CFRelease(pb);
		return -1;
	}

	OSStatus err = PasteboardPutItemFlavor(pb, (PasteboardItemID)1,
		CFSTR("public.utf8-plain-text"), data, kPasteboardFlavorNoFlags);
	CFRelease(data);
	CFRelease(pb);
	return (err == noErr) ? 0 : -1;
}

// clearClipboard clears all clipboard content.
// Returns 0 on success, -1 on failure.
int clearClipboard(void) {
	PasteboardRef pb;
	if (PasteboardCreate(kPasteboardClipboard, &pb) != noErr) return -1;
	PasteboardClear(pb);
	PasteboardSynchronize(pb);
	CFRelease(pb);
	return 0;
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// NewDarwinClipboard creates a Clipboard using the native macOS Pasteboard API.
// Uses PasteboardCreate/PasteboardPutItemFlavor with explicit UTF-8 encoding,
// which avoids encoding issues that can occur when shelling out to pbcopy/pbpaste.
func NewDarwinClipboard() *Clipboard {
	return New(darwinRead, darwinWrite).WithClear(darwinClear)
}

func darwinRead() (string, error) {
	data := C.readClipboardUTF8()
	if data == 0 {
		return "", nil
	}
	defer C.CFRelease(C.CFTypeRef(data))
	length := C.CFDataGetLength(data)
	if length == 0 {
		return "", nil
	}
	ptr := C.CFDataGetBytePtr(data)
	return C.GoStringN((*C.char)(unsafe.Pointer(ptr)), C.int(length)), nil
}

func darwinWrite(text string) error {
	if len(text) == 0 {
		return darwinClear()
	}
	cStr := C.CString(text)
	defer C.free(unsafe.Pointer(cStr))
	if ret := C.writeClipboardUTF8(unsafe.Pointer(cStr), C.int(len(text))); ret != 0 {
		return fmt.Errorf("native clipboard write failed")
	}
	return nil
}

func darwinClear() error {
	if ret := C.clearClipboard(); ret != 0 {
		return fmt.Errorf("native clipboard clear failed")
	}
	return nil
}
