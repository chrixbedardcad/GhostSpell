//go:build windows

package gui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/chrixbedardcad/GhostType/config"
	"github.com/chrixbedardcad/GhostType/llm"
	webview2 "github.com/jchv/go-webview2"
)

//go:embed frontend/index.html
var frontendFS embed.FS

// settingsGuard prevents multiple settings windows.
var (
	settingsOpen   bool
	settingsOpenMu sync.Mutex
)

// ShowWizard opens the setup wizard and blocks until the user saves or cancels.
// Returns the (potentially updated) config.
func ShowWizard(cfg *config.Config, configPath string) *config.Config {
	fmt.Println("[GUI] ShowWizard called")
	result := showWindow(cfg, configPath, "wizard")
	fmt.Printf("[GUI] ShowWizard returned: saved=%v\n", result.Saved)
	if result.Saved && result.Config != nil {
		return result.Config
	}
	return cfg
}

// ShowSettings opens the settings window in a goroutine (non-blocking).
// Only one settings window can be open at a time.
func ShowSettings(cfg *config.Config, configPath string) {
	fmt.Println("[GUI] ShowSettings called")
	settingsOpenMu.Lock()
	if settingsOpen {
		settingsOpenMu.Unlock()
		fmt.Println("[GUI] ShowSettings: window already open, skipping")
		return
	}
	settingsOpen = true
	settingsOpenMu.Unlock()
	fmt.Println("[GUI] ShowSettings: launching goroutine")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[GUI] PANIC in ShowSettings goroutine: %v\n", r)
			}
			settingsOpenMu.Lock()
			settingsOpen = false
			settingsOpenMu.Unlock()
			fmt.Println("[GUI] ShowSettings goroutine exited")
		}()
		showWindow(cfg, configPath, "settings")
	}()
}

func showWindow(cfg *config.Config, configPath string, initialView string) Result {
	fmt.Printf("[GUI] showWindow entered: view=%s\n", initialView)

	// WebView2 requires the window and message loop on the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	fmt.Println("[GUI] OS thread locked")

	// Work on a copy so cancelled edits don't corrupt the live config.
	cfgCopy := *cfg
	if cfg.LLMProviders != nil {
		cfgCopy.LLMProviders = make(map[string]config.LLMProviderDef, len(cfg.LLMProviders))
		for k, v := range cfg.LLMProviders {
			cfgCopy.LLMProviders[k] = v
		}
	}

	result := Result{Config: &cfgCopy}

	fmt.Println("[GUI] Creating WebView2 window...")
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     true,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "GhostType Setup",
			Width:  720,
			Height: 580,
			Center: true,
		},
	})
	if w == nil {
		fmt.Println("[GUI] ERROR: NewWithOptions returned nil")
		return result
	}
	defer w.Destroy()
	fmt.Println("[GUI] WebView2 window created OK")

	// --- Bind Go functions for JS bridge ---
	fmt.Println("[GUI] Binding JS functions...")

	w.Bind("getInitialView", func() string {
		fmt.Println("[GUI] JS called: getInitialView")
		return initialView
	})

	w.Bind("getConfig", func() string {
		fmt.Println("[GUI] JS called: getConfig")
		data, err := json.Marshal(&cfgCopy)
		if err != nil {
			return "{}"
		}
		return string(data)
	})

	w.Bind("getKnownModels", func(provider string) string {
		fmt.Printf("[GUI] JS called: getKnownModels(%s)\n", provider)
		models := KnownModels(provider)
		data, _ := json.Marshal(models)
		return string(data)
	})

	w.Bind("saveProvider", func(label, provider, apiKey, model, endpoint, originalLabel string) string {
		fmt.Printf("[GUI] JS called: saveProvider(%s, %s)\n", label, provider)
		if label == "" {
			return "error: label is required"
		}

		// If editing (originalLabel set) and label changed, remove old entry.
		if originalLabel != "" && originalLabel != label {
			delete(cfgCopy.LLMProviders, originalLabel)
			if cfgCopy.DefaultLLM == originalLabel {
				cfgCopy.DefaultLLM = label
			}
		}

		if cfgCopy.LLMProviders == nil {
			cfgCopy.LLMProviders = make(map[string]config.LLMProviderDef)
		}

		cfgCopy.LLMProviders[label] = config.LLMProviderDef{
			Provider:    provider,
			APIKey:      apiKey,
			Model:       model,
			APIEndpoint: endpoint,
		}

		// Set as default if first provider.
		if len(cfgCopy.LLMProviders) == 1 || cfgCopy.DefaultLLM == "" {
			cfgCopy.DefaultLLM = label
		}

		if err := config.WriteDefault(configPath, &cfgCopy); err != nil {
			return fmt.Sprintf("error: %v", err)
		}

		result.Saved = true
		result.Config = &cfgCopy
		slog.Info("Provider saved via GUI", "label", label, "provider", provider)
		return "ok"
	})

	w.Bind("deleteProvider", func(label string) string {
		fmt.Printf("[GUI] JS called: deleteProvider(%s)\n", label)
		delete(cfgCopy.LLMProviders, label)
		if cfgCopy.DefaultLLM == label {
			cfgCopy.DefaultLLM = ""
			// Pick another default if available.
			for k := range cfgCopy.LLMProviders {
				cfgCopy.DefaultLLM = k
				break
			}
		}

		if err := config.WriteDefault(configPath, &cfgCopy); err != nil {
			return fmt.Sprintf("error: %v", err)
		}

		result.Saved = true
		result.Config = &cfgCopy
		return "ok"
	})

	w.Bind("setDefault", func(label string) string {
		fmt.Printf("[GUI] JS called: setDefault(%s)\n", label)
		if _, ok := cfgCopy.LLMProviders[label]; !ok {
			return "error: provider not found"
		}
		cfgCopy.DefaultLLM = label

		if err := config.WriteDefault(configPath, &cfgCopy); err != nil {
			return fmt.Sprintf("error: %v", err)
		}

		result.Saved = true
		result.Config = &cfgCopy
		return "ok"
	})

	w.Bind("testConnection", func(provider, apiKey, model, endpoint string) string {
		fmt.Printf("[GUI] JS called: testConnection(%s)\n", provider)
		def := config.LLMProviderDef{
			Provider:    provider,
			APIKey:      apiKey,
			Model:       model,
			APIEndpoint: endpoint,
			MaxTokens:   32,
			TimeoutMs:   10000,
		}

		client, err := llm.NewClientFromDef(def)
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err = client.Send(ctx, llm.Request{
			Prompt:    "Reply with OK",
			Text:      "test",
			MaxTokens: 32,
		})
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}

		return "ok"
	})

	w.Bind("openConfigFile", func() string {
		fmt.Println("[GUI] JS called: openConfigFile")
		cmd := exec.Command("cmd", "/c", "start", "", configPath)
		if err := cmd.Start(); err != nil {
			slog.Error("Failed to open config file", "error", err)
		}
		return "ok"
	})

	w.Bind("closeWindow", func() string {
		fmt.Println("[GUI] JS called: closeWindow")
		w.Terminate()
		return "ok"
	})

	fmt.Println("[GUI] All JS functions bound")

	// Load the embedded HTML.
	html, err := frontendFS.ReadFile("frontend/index.html")
	if err != nil {
		fmt.Printf("[GUI] ERROR: Failed to read embedded HTML: %v\n", err)
		return result
	}
	fmt.Printf("[GUI] Loaded HTML (%d bytes), calling SetHtml...\n", len(html))
	w.SetHtml(string(html))
	fmt.Println("[GUI] SetHtml done, calling Run (blocks until window closes)...")

	// Run blocks until the window is closed.
	w.Run()

	fmt.Println("[GUI] Run returned, window closed")
	return result
}
