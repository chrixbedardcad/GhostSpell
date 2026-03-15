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
	indicatorMu  sync.Mutex
)

// CreateIndicator creates a small, hidden, frameless overlay window showing an
// animated ghost. Call this before app.Run(). Use ShowIndicator / HideIndicator
// to toggle visibility when processing starts/stops.
func CreateIndicator(app *application.App) {
	indicatorMu.Lock()
	defer indicatorMu.Unlock()

	// On Windows, BackgroundTypeTransparent + Frameless causes WS_EX_LAYERED
	// which is incompatible with WebView2 (window renders invisible).
	// Use BackgroundTypeTranslucent instead — it triggers WS_EX_NOREDIRECTIONBITMAP
	// which works with WebView2's DirectComposition renderer.
	// Similarly, IgnoreMouseEvents adds WS_EX_LAYERED on Windows, so skip it.
	bgType := application.BackgroundTypeTransparent
	ignoreMouse := true
	if runtime.GOOS == "windows" {
		bgType = application.BackgroundTypeTranslucent
		ignoreMouse = false
	}

	indicatorWin = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:              "ghostspell-indicator",
		Title:             "",
		Width:             260,
		Height:            52,
		Frameless:         true,
		AlwaysOnTop:       true,
		BackgroundType:    bgType,
		BackgroundColour:  application.RGBA{Red: 0, Green: 0, Blue: 0, Alpha: 0},
		DisableResize:     true,
		Hidden:            true,
		IgnoreMouseEvents: ignoreMouse,
		URL:               "/indicator.html",
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                   true,
			DisableFramelessWindowDecorations: true,
		},
		Mac: application.MacWindow{
			TitleBar:    application.MacTitleBar{Hide: true},
			Backdrop:    application.MacBackdropTransparent,
			WindowLevel: application.MacWindowLevelFloating,
		},
	})
	slog.Info("[gui] Indicator overlay window created (hidden)")
}

// ShowIndicator displays the floating ghost in the bottom-right corner of the
// primary screen (above the taskbar on Windows). The prompt icon and name are
// displayed alongside the ghost and elapsed timer.
func ShowIndicator(promptIcon, promptName string) {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	// Position bottom-right of the primary screen's work area.
	app := application.Get()
	if app != nil {
		screen := app.Screen.GetPrimary()
		if screen != nil {
			x := screen.WorkArea.X + screen.WorkArea.Width - 276
			y := screen.WorkArea.Y + screen.WorkArea.Height - 68
			win.SetPosition(x, y)
		}
	}

	slog.Debug("[indicator] ShowIndicator called", "prompt", promptName, "icon", promptIcon)

	// Pass prompt data via URL query parameters. This is 100% reliable on
	// all platforms — no ExecJS needed. The page is tiny and loads instantly.
	// SetURL on a hidden window works because it's a navigation request,
	// not JS injection. The JS parses the URL on load to display the prompt.
	u := "/indicator.html?i=" + url.QueryEscape(promptIcon) + "&n=" + url.QueryEscape(promptName)
	win.SetURL(u)
	time.Sleep(100 * time.Millisecond) // let the page load
	win.Show()
}

// HideIndicator hides the floating ghost overlay.
func HideIndicator() {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	slog.Debug("[indicator] HideIndicator called")
	win.Hide()
}

// PopIndicator briefly shows the indicator pill with the prompt icon and name
// (no timer), then auto-hides after a short delay. Used for visual feedback
// when cycling prompts via hotkey.
func PopIndicator(promptIcon, promptName string) {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	// Position bottom-right of the primary screen's work area.
	app := application.Get()
	if app != nil {
		screen := app.Screen.GetPrimary()
		if screen != nil {
			x := screen.WorkArea.X + screen.WorkArea.Width - 276
			y := screen.WorkArea.Y + screen.WorkArea.Height - 68
			win.SetPosition(x, y)
		}
	}

	slog.Debug("[indicator] PopIndicator called", "prompt", promptName, "icon", promptIcon)

	// Use the pop variant — shows prompt info without a timer.
	u := "/indicator.html?i=" + url.QueryEscape(promptIcon) + "&n=" + url.QueryEscape(promptName) + "&pop=1"
	win.SetURL(u)
	time.Sleep(100 * time.Millisecond)
	win.Show()

	// Auto-hide after 1.5 seconds.
	go func() {
		time.Sleep(1500 * time.Millisecond)
		indicatorMu.Lock()
		w := indicatorWin
		indicatorMu.Unlock()
		if w != nil {
			w.Hide()
		}
	}()
}
