// Copyright 2021 The golang.design Initiative Authors.
// All rights reserved. Use of this source code is governed
// by a MIT license that can be found in the LICENSE file.
//
// Written by Changkun Ou <changkun.de>
//
// Modified: replaced Carbon RegisterEventHotKey with CGEventTap (Quartz).
// CGEventTap + kCFRunLoopCommonModes survives NSMenu modal event loops,
// fixing the hotkey-freeze-after-tray-menu-interaction bug.
// Carbon's InstallApplicationEventHandler only runs in kCFRunLoopDefaultMode
// and gets lost when NSMenu enters NSEventTrackingRunLoopMode.

//go:build darwin

#include <stdint.h>
#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>

extern void keydownCallback(uintptr_t handle);
extern void keyupCallback(uintptr_t handle);

// ---------------------------------------------------------------------------
// Modifier flag mask — ignore caps lock, fn, numpad, device-specific bits.
// ---------------------------------------------------------------------------
#define MODIFIER_FLAGS_MASK (kCGEventFlagMaskCommand | kCGEventFlagMaskShift | \
                             kCGEventFlagMaskAlternate | kCGEventFlagMaskControl)

// ---------------------------------------------------------------------------
// Hotkey table — in-memory registry of active hotkeys.
// Matched in the CGEventTap callback on every keyDown event.
// ---------------------------------------------------------------------------
#define MAX_HOTKEYS 16

typedef struct {
    int          active;    // 1 = registered, 0 = free slot
    CGKeyCode    keycode;   // virtual key code (same values as Carbon kVK_*)
    CGEventFlags flags;     // modifier flags (CGEventFlags, not Carbon)
    uintptr_t    handle;    // Go cgo.Handle for callback routing
} HotkeyEntry;

static HotkeyEntry       sHotkeys[MAX_HOTKEYS];
static CFMachPortRef      sEventTap      = NULL;
static CFRunLoopSourceRef sRunLoopSource = NULL;

// ---------------------------------------------------------------------------
// Convert Carbon modifier bitmask → CGEventFlags.
// Carbon:  cmdKey=0x0100, shiftKey=0x0200, optionKey=0x0800, controlKey=0x1000
// CG:      kCGEventFlagMaskCommand, kCGEventFlagMaskShift, etc.
// The Go layer still uses Carbon constants (Modifier type), so we translate
// here to keep the Go API stable across the Carbon→CGEventTap migration.
// ---------------------------------------------------------------------------
static CGEventFlags carbonModsToCGFlags(int carbonMods) {
    CGEventFlags flags = 0;
    if (carbonMods & 0x0100) flags |= kCGEventFlagMaskCommand;
    if (carbonMods & 0x0200) flags |= kCGEventFlagMaskShift;
    if (carbonMods & 0x0800) flags |= kCGEventFlagMaskAlternate;
    if (carbonMods & 0x1000) flags |= kCGEventFlagMaskControl;
    return flags;
}

// ---------------------------------------------------------------------------
// CGEventTap callback — fires for every keyDown event system-wide.
// Returns NULL to consume matched hotkey events, or the original event
// to let non-matching keys pass through to the focused application.
// ---------------------------------------------------------------------------
static CGEventRef hotkeyTapCallback(
    CGEventTapProxy proxy, CGEventType type, CGEventRef event, void *userInfo) {
    // The system can disable the tap if our callback is too slow or if the
    // user revokes Input Monitoring permission. Re-enable unconditionally.
    if (type == kCGEventTapDisabledByTimeout ||
        type == kCGEventTapDisabledByUserInput) {
        if (sEventTap != NULL) {
            CGEventTapEnable(sEventTap, true);
        }
        return event;
    }

    if (type != kCGEventKeyDown) {
        return event;
    }

    CGKeyCode keycode = (CGKeyCode)CGEventGetIntegerValueField(
        event, kCGKeyboardEventKeycode);
    CGEventFlags flags = CGEventGetFlags(event) & MODIFIER_FLAGS_MASK;

    for (int i = 0; i < MAX_HOTKEYS; i++) {
        if (sHotkeys[i].active &&
            sHotkeys[i].keycode == keycode &&
            sHotkeys[i].flags == flags) {
            keydownCallback(sHotkeys[i].handle);
            return NULL;  // consume the hotkey event
        }
    }

    return event;  // pass through non-matching events
}

// ---------------------------------------------------------------------------
// ensureEventTap installs the CGEventTap on CFRunLoopGetMain() with
// kCFRunLoopCommonModes. Must be called on the main thread (via
// dispatch_sync in registerHotKey).
//
// kCFRunLoopCommonModes is the key to the fix: it includes BOTH
// kCFRunLoopDefaultMode AND NSEventTrackingRunLoopMode, so the tap
// fires even during NSMenu popup tracking. Carbon's event handlers
// only ran in kCFRunLoopDefaultMode, which is why they went dead.
// ---------------------------------------------------------------------------
static int ensureEventTap(void) {
    if (sEventTap != NULL) return 0;

    sEventTap = CGEventTapCreate(
        kCGSessionEventTap,
        kCGHeadInsertEventTap,
        kCGEventTapOptionDefault,          // active filter — can consume events
        CGEventMaskBit(kCGEventKeyDown),
        hotkeyTapCallback,
        NULL
    );
    if (sEventTap == NULL) return -1;

    sRunLoopSource = CFMachPortCreateRunLoopSource(
        kCFAllocatorDefault, sEventTap, 0);
    CFRunLoopAddSource(
        CFRunLoopGetMain(), sRunLoopSource, kCFRunLoopCommonModes);
    CGEventTapEnable(sEventTap, true);
    return 0;
}

// ---------------------------------------------------------------------------
// Public API — called from Go via cgo.
// ---------------------------------------------------------------------------

// registerHotKey installs the event tap (first call) and adds a hotkey entry.
// mod:    Carbon modifier bitmask (0x0100=Cmd, 0x0200=Shift, etc.)
// key:    virtual keycode (kVK_ANSI_G = 5, etc. — same for Carbon and CG)
// handle: Go cgo.Handle for callback routing
// Returns 0 on success, -1 on failure.
int registerHotKey(int mod, int key, uintptr_t handle) {
    __block int result;
    dispatch_sync(dispatch_get_main_queue(), ^{
        if (ensureEventTap() != 0) {
            result = -1;
            return;
        }

        CGEventFlags flags = carbonModsToCGFlags(mod);
        CGKeyCode kc = (CGKeyCode)key;

        // Update existing entry with same key+mods (e.g. re-register).
        for (int i = 0; i < MAX_HOTKEYS; i++) {
            if (sHotkeys[i].active &&
                sHotkeys[i].keycode == kc &&
                sHotkeys[i].flags == flags) {
                sHotkeys[i].handle = handle;
                result = 0;
                return;
            }
        }

        // Allocate a free slot.
        for (int i = 0; i < MAX_HOTKEYS; i++) {
            if (!sHotkeys[i].active) {
                sHotkeys[i].active  = 1;
                sHotkeys[i].keycode = kc;
                sHotkeys[i].flags   = flags;
                sHotkeys[i].handle  = handle;
                result = 0;
                return;
            }
        }

        result = -1;  // table full
    });
    return result;
}

// unregisterHotKey removes a hotkey entry by its cgo.Handle.
int unregisterHotKey(uintptr_t handle) {
    for (int i = 0; i < MAX_HOTKEYS; i++) {
        if (sHotkeys[i].active && sHotkeys[i].handle == handle) {
            sHotkeys[i].active  = 0;
            sHotkeys[i].keycode = 0;
            sHotkeys[i].flags   = 0;
            sHotkeys[i].handle  = 0;
            return 0;
        }
    }
    return -1;
}

// reregisterHotKey re-enables the CGEventTap if the system disabled it.
// With kCFRunLoopCommonModes the tap survives NSMenu modal loops, so this
// is only a safety net for when the system disables the tap (e.g. callback
// timeout, permission revocation, code signing change).
int reregisterHotKey(int mod, int key, uintptr_t handle) {
    if (sEventTap == NULL) return -1;
    if (!CGEventTapIsEnabled(sEventTap)) {
        CGEventTapEnable(sEventTap, true);
    }
    return 0;
}
