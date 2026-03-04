//go:build windows

package main

import (
	"runtime"

	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
)

func newClipboard() *clipboard.Clipboard  { return clipboard.NewWindowsClipboard() }
func newKeyboard() keyboard.Simulator     { return keyboard.NewWindowsSimulator() }
func newHotkeyManager() hotkey.Manager    { return hotkey.NewWindowsManager() }

// startMainLoop starts the Wails event loop in a background goroutine first
// (so the wizard window can render if needed), then registers hotkeys on the
// current thread (which may block waiting for the wizard), then locks this
// thread for the Windows message loop (RegisterHotKey + GetMessageW).
func startMainLoop(trayRun func() error, registerHotkeys func() error, hk hotkey.Manager) {
	go func() { trayRun() }()
	registerHotkeys()
	runtime.LockOSThread()
	hk.Listen()
}
