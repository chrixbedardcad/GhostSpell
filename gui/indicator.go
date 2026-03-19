package gui

import (
	"log/slog"
	"net/url"
	"runtime"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	indicatorWin *application.WebviewWindow
	indicatorApp *application.App
	indicatorMu  sync.Mutex
)

func CreateIndicator(app *application.App) {
	indicatorMu.Lock()
	defer indicatorMu.Unlock()
	indicatorApp = app
	slog.Info("[gui] Indicator lazy-init registered (window created on first use)")
}

func ensureIndicatorWindow() {
	if indicatorWin != nil || indicatorApp == nil {
		return
	}

	bgType := application.BackgroundTypeTransparent
	ignoreMouse := false
	if runtime.GOOS == "windows" {
		bgType = application.BackgroundTypeTranslucent
	}

	x, y := getDefaultIndicatorPos()
	if indicatorSavedX > 0 || indicatorSavedY > 0 {
		x, y = indicatorSavedX, indicatorSavedY
	}

	indicatorWin = indicatorApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:              "ghostspell-indicator",
		Title:             "",
		X:                 x,
		Y:                 y,
		Width:             320,
		Height:            400,
		Frameless:         true,
		AlwaysOnTop:       true,
		BackgroundType:    bgType,
		BackgroundColour:  application.RGBA{Red: 0, Green: 0, Blue: 0, Alpha: 0},
		DisableResize:     true,
		Hidden:            false,
		IgnoreMouseEvents: ignoreMouse,
		URL:               "/dist/react.html?window=indicator",
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                  true,
			DisableFramelessWindowDecorations: true,
		},
		Mac: application.MacWindow{
			TitleBar:    application.MacTitleBar{Hide: true},
			Backdrop:    application.MacBackdropTransparent,
			WindowLevel: application.MacWindowLevelFloating,
		},
	})
	slog.Info("[gui] Indicator window created (React, fixed 320x400)", "x", x, "y", y)
}

var indicatorPos = "top-right"
var indicatorMode = "processing"
var indicatorSavedX, indicatorSavedY int

func SetIndicatorPosition(pos string) {
	indicatorMu.Lock()
	indicatorPos = pos
	indicatorMu.Unlock()
}

func SetIndicatorMode(mode string) {
	indicatorMu.Lock()
	indicatorMode = mode
	indicatorMu.Unlock()
}

func SetIndicatorSavedPosition(x, y int) {
	indicatorMu.Lock()
	indicatorSavedX = x
	indicatorSavedY = y
	indicatorMu.Unlock()
}

func emitIndicatorEvent(data map[string]any) {
	app := application.Get()
	if app != nil {
		app.Event.Emit("indicatorState", data)
	}
}

func getDefaultIndicatorPos() (int, int) {
	app := application.Get()
	if app == nil {
		return 100, 100
	}
	screen := app.Screen.GetPrimary()
	if screen == nil {
		return 100, 100
	}
	pos := indicatorPos
	switch pos {
	case "top-left":
		return screen.WorkArea.X + 20, screen.WorkArea.Y + 20
	case "top-right":
		return screen.WorkArea.X + screen.WorkArea.Width - 68, screen.WorkArea.Y + 20
	case "bottom-left":
		return screen.WorkArea.X + 20, screen.WorkArea.Y + screen.WorkArea.Height - 68
	case "bottom-right":
		return screen.WorkArea.X + screen.WorkArea.Width - 68, screen.WorkArea.Y + screen.WorkArea.Height - 68
	default:
		return screen.WorkArea.X + (screen.WorkArea.Width-48)/2, screen.WorkArea.Y + screen.WorkArea.Height/3
	}
}

// Legacy position helpers — used by settings HTML for preview.
func getIndicatorPositionForSize(w, h int) (int, int) {
	app := application.Get()
	if app == nil {
		return 100, 100
	}
	screen := app.Screen.GetPrimary()
	if screen == nil {
		return 100, 100
	}
	indicatorMu.Lock()
	pos := indicatorPos
	indicatorMu.Unlock()
	switch pos {
	case "top-left":
		return screen.WorkArea.X + 20, screen.WorkArea.Y + 20
	case "top-right":
		return screen.WorkArea.X + screen.WorkArea.Width - w - 20, screen.WorkArea.Y + 20
	case "bottom-left":
		return screen.WorkArea.X + 20, screen.WorkArea.Y + screen.WorkArea.Height - h - 20
	case "bottom-right":
		return screen.WorkArea.X + screen.WorkArea.Width - w - 20, screen.WorkArea.Y + screen.WorkArea.Height - h - 20
	default:
		return screen.WorkArea.X + (screen.WorkArea.Width-w)/2, screen.WorkArea.Y + screen.WorkArea.Height/3
	}
}

func getIndicatorPosition() (int, int) { return getIndicatorPositionForSize(260, 52) }
func getIdlePosition() (int, int)      { return getIndicatorPositionForSize(48, 48) }

func PreviewIndicatorPosition() {
	indicatorMu.Lock()
	pos := indicatorPos
	if pos == "hidden" {
		indicatorMu.Unlock()
		return
	}
	ensureIndicatorWindow()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}
	x, y := getDefaultIndicatorPos()
	win.SetPosition(x, y)
	emitIndicatorEvent(map[string]any{"state": "pop", "icon": "✏️", "name": "Preview"})
	go func() {
		time.Sleep(2 * time.Second)
		HideIndicator()
	}()
}

func ShowIdle() {
	indicatorMu.Lock()
	mode := indicatorMode
	if mode != "always" {
		indicatorMu.Unlock()
		return
	}
	ensureIndicatorWindow()
	indicatorMu.Unlock()
	slog.Debug("[indicator] ShowIdle: displaying idle ghost")
	emitIndicatorEvent(map[string]any{"state": "idle"})
}

func ShowIndicator(promptIcon, promptName, modelLabel string) {
	slog.Debug("[indicator] ShowIndicator called", "prompt", promptName, "icon", promptIcon, "model", modelLabel)
	indicatorMu.Lock()
	pos := indicatorPos
	if pos == "hidden" {
		indicatorMu.Unlock()
		return
	}
	ensureIndicatorWindow()
	indicatorMu.Unlock()
	emitIndicatorEvent(map[string]any{
		"state": "processing", "icon": promptIcon, "name": promptName, "model": modelLabel,
	})
}

func HideIndicator() {
	indicatorMu.Lock()
	mode := indicatorMode
	indicatorMu.Unlock()
	slog.Debug("[indicator] HideIndicator called", "mode", mode)
	if mode == "always" {
		emitIndicatorEvent(map[string]any{"state": "idle"})
		return
	}
	emitIndicatorEvent(map[string]any{"state": "hidden"})
}

func PopIndicator(promptIcon, promptName string) {
	slog.Debug("[indicator] PopIndicator called", "prompt", promptName, "icon", promptIcon)
	indicatorMu.Lock()
	ensureIndicatorWindow()
	indicatorMu.Unlock()
	emitIndicatorEvent(map[string]any{"state": "pop", "icon": promptIcon, "name": promptName})
	go func() {
		time.Sleep(2500 * time.Millisecond)
		HideIndicator()
	}()
}

func (s *SettingsService) SaveIndicatorPosition(x, y int) string {
	slog.Debug("[GUI] SaveIndicatorPosition", "x", x, "y", y)
	SetIndicatorSavedPosition(x, y)
	if s.cfgCopy != nil {
		s.cfgCopy.IndicatorX = x
		s.cfgCopy.IndicatorY = y
		s.validateAndSave()
	}
	return "ok"
}

// Legacy — kept for old settings HTML ResizeIndicatorForMenu calls.
var _ = url.QueryEscape // keep import for legacy
