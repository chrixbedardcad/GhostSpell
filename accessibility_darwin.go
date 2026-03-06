//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework ApplicationServices
#include <ApplicationServices/ApplicationServices.h>
#include <dlfcn.h>

int axIsTrusted() {
    return AXIsProcessTrusted();
}

// inputMonitoringGranted checks Input Monitoring (ListenEvent) permission
// using the IOHIDCheckAccess private-but-stable IOKit symbol (macOS 10.15+).
// Returns 1 if granted, 0 if denied, -1 if the API is unavailable.
int inputMonitoringGranted() {
    typedef Boolean (*CheckAccessFunc)(int);
    void *handle = dlopen("/System/Library/Frameworks/IOKit.framework/IOKit", RTLD_LAZY);
    if (!handle) return -1;
    CheckAccessFunc check = (CheckAccessFunc)dlsym(handle, "IOHIDCheckAccess");
    dlclose(handle);
    if (!check) return -1;
    return check(1); // 1 = kIOHIDRequestTypeListenEvent
}
*/
import "C"

import "os/exec"

// checkAccessibility returns true if the process has Accessibility permission.
// On macOS, without this permission hotkey registration crashes (SIGTRAP) and
// keyboard simulation silently fails.
func checkAccessibility() bool {
	return C.axIsTrusted() != 0
}

// checkInputMonitoring returns true if Input Monitoring permission is granted.
// Uses IOHIDCheckAccess (undocumented but stable IOKit symbol used by
// Karabiner-Elements, Hammerspoon, etc.). Returns true if the API is
// unavailable (pre-10.15) so the app falls through gracefully.
func checkInputMonitoring() bool {
	result := C.inputMonitoringGranted()
	return result != 0 // -1 (unavailable) or 1 (granted) → true
}

// openAccessibilitySettings opens the macOS System Settings to the
// Accessibility and Input Monitoring privacy panes so the user can grant
// both permissions GhostType needs.
func openAccessibilitySettings() {
	exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility").Start()
	exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_ListenEvent").Start()
}
