//go:build linux

package main

import (
	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
)

func newClipboard() *clipboard.Clipboard  { return clipboard.NewLinuxClipboard() }
func newKeyboard() keyboard.Simulator     { return keyboard.NewLinuxSimulator() }
func newHotkeyManager() hotkey.Manager    { return hotkey.NewXPlatManager() }

// startMainLoop starts the GTK event loop in a background goroutine first (so
// the wizard window can render if needed), then registers hotkeys (which may
// block waiting for the wizard to complete), then blocks on the hotkey listener.
func startMainLoop(trayRun func() error, registerHotkeys func() error, hk hotkey.Manager) {
	go func() { trayRun() }()
	registerHotkeys()
	hk.Listen()
}
