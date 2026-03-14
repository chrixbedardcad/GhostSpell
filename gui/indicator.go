package gui

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	indicatorWin  *application.WebviewWindow
	indicatorMu   sync.Mutex
	indicatorDone chan struct{} // closed to stop the timer goroutine
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
		Width:             148,
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
// primary screen (above the taskbar on Windows).
func ShowIndicator() {
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
			x := screen.WorkArea.X + screen.WorkArea.Width - 164
			y := screen.WorkArea.Y + screen.WorkArea.Height - 68
			win.SetPosition(x, y)
		}
	}

	win.Show()
	startIndicatorTimer()
}

// HideIndicator hides the floating ghost overlay.
func HideIndicator() {
	indicatorMu.Lock()
	win := indicatorWin
	indicatorMu.Unlock()
	if win == nil {
		return
	}

	stopIndicatorTimer()
	win.Hide()
}

// startIndicatorTimer launches a goroutine that drives the elapsed timer
// display from Go. This replaces the old JS-side setInterval approach which
// was unreliable: ExecJS calls on hidden/transitioning Wails WebViews get
// silently dropped, causing the JS interval to leak and accumulate stale
// values (200s+). With Go owning the timer, if one ExecJS update is dropped,
// the next one (1 second later) self-corrects.
func startIndicatorTimer() {
	stopIndicatorTimer() // ensure no stale goroutine

	indicatorMu.Lock()
	win := indicatorWin
	done := make(chan struct{})
	indicatorDone = done
	indicatorMu.Unlock()

	if win == nil {
		return
	}

	start := time.Now()

	go func() {
		// Immediately set "0s" — try a few times in quick succession to
		// make sure at least one ExecJS call lands even if the WebView
		// is still transitioning from hidden → visible.
		for i := 0; i < 3; i++ {
			select {
			case <-done:
				return
			default:
			}
			win.ExecJS(`document.getElementById('t').textContent='0s'`)
			time.Sleep(30 * time.Millisecond)
		}

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				s := int(time.Since(start).Seconds())
				win.ExecJS(fmt.Sprintf(`document.getElementById('t').textContent='%ds'`, s))
			}
		}
	}()
}

// stopIndicatorTimer stops the Go-side timer goroutine.
func stopIndicatorTimer() {
	indicatorMu.Lock()
	done := indicatorDone
	indicatorDone = nil
	indicatorMu.Unlock()

	if done != nil {
		select {
		case <-done: // already closed
		default:
			close(done)
		}
	}
}
