//go:build darwin

package keyboard

/*
#cgo LDFLAGS: -framework CoreGraphics -framework CoreFoundation
#include <CoreGraphics/CoreGraphics.h>

void sendKeyCombo(CGKeyCode modifier, CGKeyCode key) {
	CGEventRef modDown = CGEventCreateKeyboardEvent(NULL, modifier, true);
	CGEventRef keyDown = CGEventCreateKeyboardEvent(NULL, key, true);
	CGEventRef keyUp   = CGEventCreateKeyboardEvent(NULL, key, false);
	CGEventRef modUp   = CGEventCreateKeyboardEvent(NULL, modifier, false);

	CGEventSetFlags(keyDown, CGEventGetFlags(modDown));
	CGEventSetFlags(keyUp, CGEventGetFlags(modDown));

	CGEventPost(kCGHIDEventTap, modDown);
	CGEventPost(kCGHIDEventTap, keyDown);
	CGEventPost(kCGHIDEventTap, keyUp);
	CGEventPost(kCGHIDEventTap, modUp);

	CFRelease(modDown);
	CFRelease(keyDown);
	CFRelease(keyUp);
	CFRelease(modUp);
}
*/
import "C"
import "time"

// macOS virtual key codes (from Events.h)
const (
	kVK_Command = 0x37
	kVK_ANSI_A  = 0x00
	kVK_ANSI_C  = 0x08
	kVK_ANSI_V  = 0x09
)

// DarwinSimulator implements keyboard simulation on macOS using CGEvent.
// Requires Accessibility permission (System Preferences → Privacy → Accessibility).
type DarwinSimulator struct{}

// NewDarwinSimulator creates a new macOS keyboard simulator.
func NewDarwinSimulator() *DarwinSimulator {
	return &DarwinSimulator{}
}

func (s *DarwinSimulator) SelectAll() error {
	C.sendKeyCombo(C.CGKeyCode(kVK_Command), C.CGKeyCode(kVK_ANSI_A))
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (s *DarwinSimulator) Copy() error {
	C.sendKeyCombo(C.CGKeyCode(kVK_Command), C.CGKeyCode(kVK_ANSI_C))
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (s *DarwinSimulator) Paste() error {
	C.sendKeyCombo(C.CGKeyCode(kVK_Command), C.CGKeyCode(kVK_ANSI_V))
	time.Sleep(10 * time.Millisecond)
	return nil
}
