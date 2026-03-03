package tray

import (
	"fmt"
	"log/slog"
	"runtime"

	"github.com/chrixbedardcad/GhostType/internal/version"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// ModelLabel describes a configured provider for the tray Models menu section.
type ModelLabel struct {
	Label     string // e.g. "claude"
	Provider  string // e.g. "Anthropic"
	Model     string // e.g. "claude-sonnet-4-6"
	IsDefault bool
}

// Config holds tray configuration and callbacks.
type Config struct {
	// IconPNG is the raw PNG bytes for the tray icon.
	IconPNG []byte

	// Callbacks — called on the tray thread.
	OnModeChange   func(modeName string) // "correct", "translate", "rewrite"
	OnTargetSelect func(idx int)
	OnTemplSelect  func(idx int)
	OnSoundToggle  func(enabled bool)
	OnCancel       func()
	OnSettings     func()
	OnModelSelect  func(label string)
	OnExit         func()

	// State readers — called to build the menu.
	GetActiveMode   func() string // returns "correct", "translate", or "rewrite"
	GetTargetIdx    func() int
	GetTemplateIdx  func() int
	GetSoundEnabled func() bool
	GetIsProcessing func() bool
	GetModelLabels  func() []ModelLabel

	// Static data for building menu items.
	TargetLabels  []string // translate target display labels
	TemplateNames []string // rewrite template display names
}

// trayState holds the runtime state of the system tray.
type trayState struct {
	cfg     Config
	app     *application.App
	systray *application.SystemTray
}

// Start launches the system tray icon in a background goroutine.
// Returns a stop function that removes the icon and shuts down.
func Start(cfg Config) (stop func()) {
	ts := &trayState{cfg: cfg}

	ts.app = application.New(application.Options{
		Name: "GhostType",
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})

	ts.systray = ts.app.SystemTray.New()

	if len(cfg.IconPNG) > 0 {
		ts.systray.SetIcon(cfg.IconPNG)
	}
	ts.systray.SetTooltip(fmt.Sprintf("GhostType v%s", version.Version))

	// Build and set the initial menu.
	ts.refreshMenu()

	// On macOS, left-click opens the menu. On Windows/Linux, right-click does.
	// Rebuild the menu before showing to reflect current state.
	ts.systray.OnClick(func() {
		ts.refreshMenu()
	})
	ts.systray.OnRightClick(func() {
		ts.refreshMenu()
	})

	go func() {
		runtime.LockOSThread()
		if err := ts.app.Run(); err != nil {
			slog.Error("Tray Wails app error", "error", err)
		}
	}()

	return func() {
		ts.app.Quit()
	}
}

// refreshMenu rebuilds the tray context menu from current state.
func (ts *trayState) refreshMenu() {
	menu := application.NewMenu()

	// Version header (disabled).
	menu.Add(fmt.Sprintf("GhostType v%s", version.Version)).SetEnabled(false)
	menu.AddSeparator()

	// Mode selection (radio group).
	activeMode := "correct"
	if ts.cfg.GetActiveMode != nil {
		activeMode = ts.cfg.GetActiveMode()
	}

	correctItem := menu.AddRadio("Correct", activeMode == "correct")
	translateItem := menu.AddRadio("Translate", activeMode == "translate")
	rewriteItem := menu.AddRadio("Rewrite", activeMode == "rewrite")

	correctItem.OnClick(func(ctx *application.Context) {
		ts.uncheckModes(correctItem, translateItem, rewriteItem)
		correctItem.SetChecked(true)
		if ts.cfg.OnModeChange != nil {
			ts.cfg.OnModeChange("correct")
		}
	})
	translateItem.OnClick(func(ctx *application.Context) {
		ts.uncheckModes(correctItem, translateItem, rewriteItem)
		translateItem.SetChecked(true)
		if ts.cfg.OnModeChange != nil {
			ts.cfg.OnModeChange("translate")
		}
	})
	rewriteItem.OnClick(func(ctx *application.Context) {
		ts.uncheckModes(correctItem, translateItem, rewriteItem)
		rewriteItem.SetChecked(true)
		if ts.cfg.OnModeChange != nil {
			ts.cfg.OnModeChange("rewrite")
		}
	})

	// Models section.
	menu.AddSeparator()
	menu.Add("Models:").SetEnabled(false)

	if ts.cfg.GetModelLabels != nil {
		models := ts.cfg.GetModelLabels()
		if len(models) > 0 {
			for _, ml := range models {
				displayName := ml.Label
				if displayName == "" {
					displayName = ml.Model
				}
				item := menu.AddRadio("  "+displayName, ml.IsDefault)
				label := ml.Label // capture for closure
				item.OnClick(func(ctx *application.Context) {
					if ts.cfg.OnModelSelect != nil {
						ts.cfg.OnModelSelect(label)
					}
					ts.refreshMenu()
				})
			}
		} else {
			menu.Add("  Add a model in Settings...").SetEnabled(false)
		}
	}

	settingsItem := menu.Add("  Settings...")
	settingsItem.OnClick(func(ctx *application.Context) {
		if ts.cfg.OnSettings != nil {
			ts.cfg.OnSettings()
		}
	})

	// Language targets.
	if len(ts.cfg.TargetLabels) > 0 {
		menu.AddSeparator()
		menu.Add("Language:").SetEnabled(false)

		targetIdx := 0
		if ts.cfg.GetTargetIdx != nil {
			targetIdx = ts.cfg.GetTargetIdx()
		}

		for i, name := range ts.cfg.TargetLabels {
			item := menu.AddRadio("  "+name, i == targetIdx)
			idx := i // capture for closure
			item.OnClick(func(ctx *application.Context) {
				if ts.cfg.OnTargetSelect != nil {
					ts.cfg.OnTargetSelect(idx)
				}
				ts.refreshMenu()
			})
		}
	}

	// Rewrite templates.
	if len(ts.cfg.TemplateNames) > 0 {
		menu.AddSeparator()
		menu.Add("Template:").SetEnabled(false)

		templIdx := 0
		if ts.cfg.GetTemplateIdx != nil {
			templIdx = ts.cfg.GetTemplateIdx()
		}

		for i, name := range ts.cfg.TemplateNames {
			item := menu.AddRadio("  "+name, i == templIdx)
			idx := i // capture for closure
			item.OnClick(func(ctx *application.Context) {
				if ts.cfg.OnTemplSelect != nil {
					ts.cfg.OnTemplSelect(idx)
				}
				ts.refreshMenu()
			})
		}
	}

	// Sound toggle.
	menu.AddSeparator()
	soundEnabled := false
	if ts.cfg.GetSoundEnabled != nil {
		soundEnabled = ts.cfg.GetSoundEnabled()
	}
	soundItem := menu.AddCheckbox("Sound", soundEnabled)
	soundItem.OnClick(func(ctx *application.Context) {
		if ts.cfg.OnSoundToggle != nil {
			ts.cfg.OnSoundToggle(ctx.IsChecked())
		}
	})

	// Cancel LLM.
	isProcessing := false
	if ts.cfg.GetIsProcessing != nil {
		isProcessing = ts.cfg.GetIsProcessing()
	}
	cancelItem := menu.Add("Cancel LLM")
	cancelItem.SetEnabled(isProcessing)
	cancelItem.OnClick(func(ctx *application.Context) {
		if ts.cfg.OnCancel != nil {
			ts.cfg.OnCancel()
		}
	})

	// Exit.
	menu.AddSeparator()
	exitItem := menu.Add("Exit")
	exitItem.OnClick(func(ctx *application.Context) {
		if ts.cfg.OnExit != nil {
			ts.cfg.OnExit()
		}
	})

	ts.systray.SetMenu(menu)
}

// uncheckModes unchecks all mode radio items.
func (ts *trayState) uncheckModes(items ...*application.MenuItem) {
	for _, item := range items {
		item.SetChecked(false)
	}
}
