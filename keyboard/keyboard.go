package keyboard

// Simulator provides platform-specific keyboard simulation.
// Used to send Ctrl+A, Ctrl+C, Ctrl+V key sequences to the active window.
type Simulator interface {
	// SelectAll simulates Ctrl+A (or Cmd+A on macOS).
	SelectAll() error

	// Copy simulates Ctrl+C (or Cmd+C on macOS).
	Copy() error

	// Paste simulates Ctrl+V (or Cmd+V on macOS).
	Paste() error

	// WaitForModifierRelease waits for all physical modifier keys to be released.
	// On macOS, this prevents hotkey modifiers (e.g. Ctrl from Ctrl+G) from
	// leaking into subsequent synthetic Cmd+A/C/V events via CGEventPost's
	// HID-level hardware state merging.
	WaitForModifierRelease()
}
