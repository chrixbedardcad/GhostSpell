package main

import (
	"log/slog"
	"syscall"
	"unsafe"
)

// setAppID sets the Windows AppUserModelID so the app can be pinned
// to the taskbar and has a proper identity in the notification area.
func setAppID() {
	shell32 := syscall.NewLazyDLL("shell32.dll")
	proc := shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	appID, _ := syscall.UTF16PtrFromString("com.ghostspell.app")
	ret, _, _ := proc.Call(uintptr(unsafe.Pointer(appID)))
	if ret != 0 {
		slog.Warn("[win] Failed to set AppUserModelID", "hr", ret)
	}
}
