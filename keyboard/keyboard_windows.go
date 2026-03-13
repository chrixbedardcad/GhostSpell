//go:build windows

package keyboard

import (
	"fmt"
	"log/slog"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                    = syscall.NewLazyDLL("user32.dll")
	procSendInput             = user32.NewProc("SendInput")
	procGetAsyncKeyState      = user32.NewProc("GetAsyncKeyState")
	procGetForegroundWindow   = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW        = user32.NewProc("GetWindowTextW")
)

const (
	inputKeyboard = 1
	keyEventUp    = 0x0002

	vkControl = 0x11
	vkShift   = 0x10
	vkMenu    = 0x12 // Alt
	vkLWin    = 0x5B
	vkRWin    = 0x5C
	vkA       = 0x41
	vkC       = 0x43
	vkV       = 0x56
	vkRight   = 0x27
)

// keybdInput is the KEYBDINPUT structure for SendInput.
type keybdInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	dwExtraInfo uintptr
}

// input is the INPUT structure for SendInput.
type input struct {
	inputType uint32
	ki        keybdInput
	padding   [8]byte
}

// WindowsSimulator implements keyboard simulation on Windows using SendInput.
type WindowsSimulator struct{}

// NewWindowsSimulator creates a new Windows keyboard simulator.
func NewWindowsSimulator() *WindowsSimulator {
	return &WindowsSimulator{}
}

func makeInput(vk uint16, down bool) input {
	var flags uint32
	if !down {
		flags = keyEventUp
	}
	return input{
		inputType: inputKeyboard,
		ki: keybdInput{
			wVk:     vk,
			dwFlags: flags,
		},
	}
}

func sendKey(vk uint16, down bool) error {
	inp := makeInput(vk, down)
	ret, _, _ := procSendInput.Call(1, uintptr(unsafe.Pointer(&inp)), unsafe.Sizeof(inp))
	if ret == 0 {
		action := "keydown"
		if !down {
			action = "keyup"
		}
		return fmt.Errorf("SendInput failed for vk=0x%02X %s", vk, action)
	}
	return nil
}

// sendKeyComboAtomic sends modifier+key as a single atomic SendInput call.
// This prevents other processes from injecting input between our keystrokes.
func sendKeyComboAtomic(modifier, key uint16) error {
	inputs := [4]input{
		makeInput(modifier, true),
		makeInput(key, true),
		makeInput(key, false),
		makeInput(modifier, false),
	}
	ret, _, err := procSendInput.Call(
		4,
		uintptr(unsafe.Pointer(&inputs[0])),
		unsafe.Sizeof(inputs[0]),
	)
	if ret != 4 {
		return fmt.Errorf("SendInput: expected 4 events injected, got %d (err=%v)", ret, err)
	}
	slog.Debug("SendInput: keystroke sent", "modifier", fmt.Sprintf("0x%02X", modifier), "key", fmt.Sprintf("0x%02X", key), "injected", ret)
	return nil
}

// WaitForModifierRelease polls GetAsyncKeyState until all modifier keys
// (Ctrl, Shift, Alt, Win) are physically released. This prevents our
// synthetic Ctrl+C/V from colliding with the user's hotkey modifiers.
func (s *WindowsSimulator) WaitForModifierRelease() {
	modKeys := []uint16{vkControl, vkShift, vkMenu, vkLWin, vkRWin}
	const maxWait = 500 * time.Millisecond
	const pollInterval = 5 * time.Millisecond
	start := time.Now()
	deadline := start.Add(maxWait)

	for time.Now().Before(deadline) {
		anyPressed := false
		for _, vk := range modKeys {
			ret, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
			if ret&0x8000 != 0 { // high bit = currently pressed
				anyPressed = true
				break
			}
		}
		if !anyPressed {
			waited := time.Since(start)
			if waited > 10*time.Millisecond {
				slog.Debug("WaitForModifierRelease: modifiers released", "waited_ms", waited.Milliseconds())
			}
			// Small settle delay — let the OS fully process the key release
			// before we inject new keystrokes.
			time.Sleep(30 * time.Millisecond)
			return
		}
		time.Sleep(pollInterval)
	}
	slog.Warn("WaitForModifierRelease: timed out after 500ms, proceeding anyway")
}

func (s *WindowsSimulator) ReadSelectedText() string    { return "" }
func (s *WindowsSimulator) ReadAllText() string          { return "" }
func (s *WindowsSimulator) WriteSelectedText(string) bool { return false }
func (s *WindowsSimulator) WriteAllText(string) bool      { return false }

// FrontAppName returns the title of the foreground window.
func (s *WindowsSimulator) FrontAppName() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}
	var buf [256]uint16
	ret, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if ret == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:ret])
}

func (s *WindowsSimulator) SelectAllAX() error     { return fmt.Errorf("not supported") }
func (s *WindowsSimulator) CopyAX() error          { return fmt.Errorf("not supported") }
func (s *WindowsSimulator) PasteAX() error          { return fmt.Errorf("not supported") }
func (s *WindowsSimulator) SelectAllScript() error  { return fmt.Errorf("not supported") }
func (s *WindowsSimulator) CopyScript() error        { return fmt.Errorf("not supported") }
func (s *WindowsSimulator) PasteScript() error       { return fmt.Errorf("not supported") }

func (s *WindowsSimulator) SelectAll() error {
	slog.Debug("Windows SelectAll: sending Ctrl+A", "foreground", s.FrontAppName())
	return sendKeyComboAtomic(vkControl, vkA)
}

func (s *WindowsSimulator) Copy() error {
	slog.Debug("Windows Copy: sending Ctrl+C", "foreground", s.FrontAppName())
	return sendKeyComboAtomic(vkControl, vkC)
}

func (s *WindowsSimulator) Paste() error {
	return sendKeyComboAtomic(vkControl, vkV)
}

func (s *WindowsSimulator) PressRight() error {
	if err := sendKey(vkRight, true); err != nil {
		return err
	}
	time.Sleep(10 * time.Millisecond)
	return sendKey(vkRight, false)
}
