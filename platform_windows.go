//go:build windows

package main

import (
	"os"
	"runtime"

	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
)

func newClipboard() *clipboard.Clipboard  { return clipboard.NewWindowsClipboard() }
func newKeyboard() keyboard.Simulator     { return keyboard.NewWindowsSimulator() }
func newHotkeyManager() hotkey.Manager    { return hotkey.NewWindowsManager() }

// startMainLoop runs the Wails event loop on the main thread (required because
// COM/CoInitializeEx was called on the main thread during init — WebView2 needs
// it). Hotkeys are registered in a background goroutine (which blocks on
// wizardDone so the wizard can render first), then that goroutine runs the
// Windows message loop for RegisterHotKey + GetMessageW.
func startMainLoop(trayRun func() error, registerHotkeys func() error, hk hotkey.Manager) {
	go func() {
		runtime.LockOSThread()
		if err := registerHotkeys(); err != nil {
			os.Exit(1)
		}
		hk.Listen()
	}()
	// Wails event loop on main thread — blocks until app quits.
	// COM was initialized here by go-webview2's init().
	trayRun()
	hk.Stop()
}
