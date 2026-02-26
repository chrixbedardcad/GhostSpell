//go:build windows

package tray

import (
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/chrixbedardcad/GhostType/internal/version"
)

// Win32 constants.
const (
	wmDestroy  = 0x0002
	wmCommand  = 0x0111
	wmUser     = 0x0400
	wmTrayIcon = wmUser + 1

	wmLButtonUp = 0x0202
	wmRButtonUp = 0x0205

	nimAdd    = 0x00000000
	nimModify = 0x00000001
	nimDelete = 0x00000002

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	csOwnDC = 0x0020

	mfString    = 0x00000000
	mfSeparator = 0x00000800
	mfGrayed    = 0x00000001

	tpmLeftAlign   = 0x0000
	tpmBottomAlign = 0x0020

	hwndMessage = ^uintptr(2) // HWND_MESSAGE = (HWND)-3

	idiApplication = 32512
)

// Menu item ID ranges.
const (
	idModeCorrect   = 2001
	idModeTranslate = 2002
	idModeRewrite   = 2003
	idExit          = 2099
	idLangBase      = 2100 // + language index
	idTemplBase     = 2200 // + template index
)

// Win32 structs.
type wndClassExW struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

type notifyIconData struct {
	size            uint32
	hwnd            uintptr
	id              uint32
	flags           uint32
	callbackMessage uint32
	icon            uintptr
	tip             [128]uint16
}

type point struct {
	x, y int32
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

// Win32 DLL procs.
var (
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procCheckMenuRadioItem  = user32.NewProc("CheckMenuRadioItem")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")

	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

// Config holds tray configuration and callbacks.
type Config struct {
	// Callbacks — called on the tray's OS thread.
	OnModeChange  func(modeName string) // "correct", "translate", "rewrite"
	OnLangSelect  func(idx int)
	OnTemplSelect func(idx int)
	OnExit        func()

	// State readers — called to build the menu each time.
	GetActiveMode  func() string // returns "correct", "translate", or "rewrite"
	GetLangIdx     func() int
	GetTemplateIdx func() int

	// Static data for building menu items.
	Languages     []string // language codes
	LanguageNames []string // display names, parallel to Languages
	TemplateNames []string // rewrite template display names
}

// trayState holds the runtime state of the tray icon.
type trayState struct {
	cfg    Config
	hwnd   uintptr
	nid    notifyIconData
	wg     sync.WaitGroup
	stopCh chan struct{}
}

// global state — only one tray per process.
var (
	globalTray   *trayState
	globalTrayMu sync.Mutex
)

// Start launches the system tray icon in a background goroutine.
// Returns a stop function that removes the icon and stops the message loop.
func Start(cfg Config) (stop func()) {
	globalTrayMu.Lock()
	defer globalTrayMu.Unlock()

	ts := &trayState{
		cfg:    cfg,
		stopCh: make(chan struct{}),
	}
	globalTray = ts

	ts.wg.Add(1)
	go ts.run()

	return func() {
		select {
		case <-ts.stopCh:
			return // already stopped
		default:
		}
		// Post WM_DESTROY to break the message loop.
		if ts.hwnd != 0 {
			procPostMessageW.Call(ts.hwnd, wmDestroy, 0, 0)
		}
		ts.wg.Wait()
	}
}

func (ts *trayState) run() {
	defer ts.wg.Done()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInstance, _, _ := procGetModuleHandleW.Call(0)

	// Load default application icon.
	hIcon, _, _ := procLoadIconW.Call(0, uintptr(idiApplication))

	// Register window class.
	className := utf16Ptr("GhostTypeTray")
	wc := wndClassExW{
		size:      uint32(unsafe.Sizeof(wndClassExW{})),
		style:     csOwnDC,
		wndProc:   wndProcCallback(),
		instance:  hInstance,
		icon:      hIcon,
		iconSm:    hIcon,
		className: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	// Create message-only window.
	ts.hwnd, _, _ = procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16Ptr("GhostType"))),
		0, 0, 0, 0, 0,
		hwndMessage, 0, hInstance, 0,
	)

	// Add tray icon.
	ts.nid = notifyIconData{
		size:            uint32(unsafe.Sizeof(notifyIconData{})),
		hwnd:            ts.hwnd,
		id:              1,
		flags:           nifMessage | nifIcon | nifTip,
		callbackMessage: wmTrayIcon,
		icon:            hIcon,
	}
	ts.setTooltip()
	procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&ts.nid)))

	// Message loop.
	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&m)),
			0, 0, 0,
		)
		// GetMessageW returns 0 for WM_QUIT, -1 for error.
		if ret == 0 || int32(ret) == -1 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	// Cleanup.
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&ts.nid)))
	procDestroyWindow.Call(ts.hwnd)
	close(ts.stopCh)
}

func (ts *trayState) setTooltip() {
	tip := fmt.Sprintf("GhostType v%s - %s", version.Version, ts.cfg.GetActiveMode())
	tipUTF16 := utf16Slice(tip)
	n := len(tipUTF16)
	if n > 127 {
		n = 127
	}
	copy(ts.nid.tip[:n], tipUTF16[:n])
	ts.nid.tip[n] = 0
}

func (ts *trayState) updateTooltip() {
	ts.setTooltip()
	ts.nid.flags = nifTip
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&ts.nid)))
	ts.nid.flags = nifMessage | nifIcon | nifTip
}

func (ts *trayState) showMenu() {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}

	// Version header (grayed).
	header := fmt.Sprintf("GhostType v%s", version.Version)
	procAppendMenuW.Call(hMenu, mfString|mfGrayed, 0, uintptr(unsafe.Pointer(utf16Ptr(header))))
	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)

	// Mode items.
	activeMode := ts.cfg.GetActiveMode()
	procAppendMenuW.Call(hMenu, mfString, idModeCorrect, uintptr(unsafe.Pointer(utf16Ptr("Correct"))))
	procAppendMenuW.Call(hMenu, mfString, idModeTranslate, uintptr(unsafe.Pointer(utf16Ptr("Translate"))))
	procAppendMenuW.Call(hMenu, mfString, idModeRewrite, uintptr(unsafe.Pointer(utf16Ptr("Rewrite"))))

	// Radio-check the active mode.
	activeID := uint32(idModeCorrect)
	switch activeMode {
	case "translate":
		activeID = idModeTranslate
	case "rewrite":
		activeID = idModeRewrite
	}
	procCheckMenuRadioItem.Call(hMenu,
		uintptr(idModeCorrect), uintptr(idModeRewrite),
		uintptr(activeID), 0) // 0 = MF_BYCOMMAND

	// Language section.
	if len(ts.cfg.Languages) > 0 {
		procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
		procAppendMenuW.Call(hMenu, mfString|mfGrayed, 0, uintptr(unsafe.Pointer(utf16Ptr("Language:"))))

		langIdx := ts.cfg.GetLangIdx()
		for i, name := range ts.cfg.LanguageNames {
			label := "  " + name
			procAppendMenuW.Call(hMenu, mfString, uintptr(idLangBase+i), uintptr(unsafe.Pointer(utf16Ptr(label))))
		}
		if len(ts.cfg.LanguageNames) > 0 {
			procCheckMenuRadioItem.Call(hMenu,
				uintptr(idLangBase), uintptr(idLangBase+len(ts.cfg.LanguageNames)-1),
				uintptr(idLangBase+langIdx), 0)
		}
	}

	// Template section.
	if len(ts.cfg.TemplateNames) > 0 {
		procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
		procAppendMenuW.Call(hMenu, mfString|mfGrayed, 0, uintptr(unsafe.Pointer(utf16Ptr("Template:"))))

		templIdx := ts.cfg.GetTemplateIdx()
		for i, name := range ts.cfg.TemplateNames {
			label := "  " + name
			procAppendMenuW.Call(hMenu, mfString, uintptr(idTemplBase+i), uintptr(unsafe.Pointer(utf16Ptr(label))))
		}
		procCheckMenuRadioItem.Call(hMenu,
			uintptr(idTemplBase), uintptr(idTemplBase+len(ts.cfg.TemplateNames)-1),
			uintptr(idTemplBase+templIdx), 0)
	}

	// Exit.
	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
	procAppendMenuW.Call(hMenu, mfString, idExit, uintptr(unsafe.Pointer(utf16Ptr("Exit"))))

	// Show menu at cursor.
	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(ts.hwnd)
	procTrackPopupMenu.Call(hMenu, tpmLeftAlign|tpmBottomAlign, uintptr(pt.x), uintptr(pt.y), 0, ts.hwnd, 0)

	procDestroyMenu.Call(hMenu)
}

func (ts *trayState) handleMenuCommand(id int) {
	switch {
	case id >= idLangBase && id < idTemplBase:
		idx := id - idLangBase
		if ts.cfg.OnLangSelect != nil {
			ts.cfg.OnLangSelect(idx)
		}
		ts.updateTooltip()

	case id >= idTemplBase:
		idx := id - idTemplBase
		if ts.cfg.OnTemplSelect != nil {
			ts.cfg.OnTemplSelect(idx)
		}
		ts.updateTooltip()

	case id == idModeCorrect:
		if ts.cfg.OnModeChange != nil {
			ts.cfg.OnModeChange("correct")
		}
		ts.updateTooltip()

	case id == idModeTranslate:
		if ts.cfg.OnModeChange != nil {
			ts.cfg.OnModeChange("translate")
		}
		ts.updateTooltip()

	case id == idModeRewrite:
		if ts.cfg.OnModeChange != nil {
			ts.cfg.OnModeChange("rewrite")
		}
		ts.updateTooltip()

	case id == idExit:
		if ts.cfg.OnExit != nil {
			ts.cfg.OnExit()
		}
	}
}

// wndProcCallback returns a syscall callback for the window procedure.
func wndProcCallback() uintptr {
	return syscall.NewCallback(func(hwnd, umsg, wParam, lParam uintptr) uintptr {
		globalTrayMu.Lock()
		ts := globalTray
		globalTrayMu.Unlock()

		switch umsg {
		case wmTrayIcon:
			switch lParam {
			case wmRButtonUp, wmLButtonUp:
				if ts != nil {
					ts.showMenu()
				}
			}
			return 0

		case wmCommand:
			id := int(wParam & 0xFFFF)
			if ts != nil {
				ts.handleMenuCommand(id)
			}
			return 0

		case wmDestroy:
			procPostQuitMessage.Call(0)
			return 0
		}

		ret, _, _ := procDefWindowProcW.Call(hwnd, umsg, wParam, lParam)
		return ret
	})
}

// utf16Ptr converts a Go string to a null-terminated UTF-16 pointer.
func utf16Ptr(s string) *uint16 {
	u := utf16Slice(s)
	u = append(u, 0)
	return &u[0]
}

// utf16Slice converts a Go string to a UTF-16 slice (no null terminator).
func utf16Slice(s string) []uint16 {
	result := make([]uint16, 0, len(s)+1)
	for _, r := range s {
		if r <= 0xFFFF {
			result = append(result, uint16(r))
		} else {
			r -= 0x10000
			result = append(result, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
		}
	}
	return result
}
