package gui

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/chrixbedardcad/GhostSpell/internal/sysinfo"
	"github.com/chrixbedardcad/GhostSpell/internal/version"
)

// redactSecrets removes API keys and tokens from log text.
func redactSecrets(text string) string {
	// Redact common API key patterns: sk-..., key-..., bearer tokens, etc.
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9_-]{10,})`),
		regexp.MustCompile(`(?i)(key-[a-zA-Z0-9_-]{10,})`),
		regexp.MustCompile(`(?i)(api[_-]?key\s*[:=]\s*)"?([^"\s,}{]+)`),
		regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9_.-]{20,})`),
		regexp.MustCompile(`(?i)(refresh[_-]?token\s*[:=]\s*)"?([^"\s,}{]+)`),
		regexp.MustCompile(`(?i)(eyJ[a-zA-Z0-9_-]{20,}\.[a-zA-Z0-9_-]{20,})`), // JWT
	}
	result := text
	for _, re := range patterns {
		result = re.ReplaceAllString(result, "[REDACTED]")
	}
	return result
}

// SubmitBugReport collects diagnostics and opens a GitHub issue with pre-filled content.
// The description parameter is the user's bug description which appears first in the issue.
func (s *SettingsService) SubmitBugReport(description string) string {
	guiLog("[GUI] JS called: SubmitBugReport")

	// Collect system info.
	sys := sysinfo.Collect()

	// Build provider list (names only, no keys).
	// Use cfgCopy if Settings is open; otherwise fall back to the live config
	// (the tray menu's "Report a Bug..." calls this without opening Settings).
	var providers []string
	var defaultModel string
	cfg := s.cfgCopy
	if cfg == nil && s.liveCfg != nil {
		if s.liveMu != nil {
			s.liveMu.Lock()
			defer s.liveMu.Unlock()
		}
		cfg = s.liveCfg
	}
	if cfg != nil {
		for name := range cfg.Providers {
			providers = append(providers, name)
		}
		defaultModel = cfg.DefaultModel
	}

	// Collect log tail.
	logTail := ""
	if s.DebugTailFn != nil {
		if tail, err := s.DebugTailFn(); err == nil {
			logTail = redactSecrets(tail)
		}
	}

	// Generate a fingerprint from system info + description for duplicate detection.
	fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(
		version.Version+sys.OS+sys.OSVersion+sys.Arch+description,
	)))[:12]

	// Build issue body — user description first, then diagnostics.
	var body strings.Builder

	if description != "" {
		body.WriteString("## Description\n\n")
		body.WriteString(description)
		body.WriteString("\n\n")
	}

	body.WriteString("## System Information\n\n")
	body.WriteString("| | |\n|---|---|\n")
	fmt.Fprintf(&body, "| **Version** | %s |\n", version.Version)
	fmt.Fprintf(&body, "| **OS** | %s %s (%s) |\n", sys.OS, sys.OSVersion, sys.Arch)
	fmt.Fprintf(&body, "| **Locale** | %s |\n", sys.Locale)
	fmt.Fprintf(&body, "| **Keyboard** | %s |\n", sys.KeyboardLayout)
	fmt.Fprintf(&body, "| **Providers** | %s |\n", strings.Join(providers, ", "))
	fmt.Fprintf(&body, "| **Default Model** | %s |\n", defaultModel)
	fmt.Fprintf(&body, "| **Fingerprint** | `%s` |\n", fingerprint)

	// Save the full log to a temp file for drag-and-drop attachment.
	logFilePath := ""
	if logTail != "" {
		tmpDir := os.TempDir()
		logFilePath = filepath.Join(tmpDir, fmt.Sprintf("ghostspell-bugreport-%s.log", fingerprint))
		if err := os.WriteFile(logFilePath, []byte(logTail), 0644); err != nil {
			slog.Error("[bugreport] failed to write log file", "error", err)
			logFilePath = ""
		} else {
			slog.Info("[bugreport] log saved for attachment", "path", logFilePath)
		}
	}

	if logFilePath != "" {
		body.WriteString("\n## Log File\n\n")
		body.WriteString("**Please drag and drop the log file into this issue:**\n\n")
		fmt.Fprintf(&body, "`%s`\n\n", logFilePath)
		body.WriteString("_(The file has been saved automatically. Drag it from the path above into this text area.)_\n")
	}

	// Build the GitHub new issue URL.
	title := fmt.Sprintf("Bug Report — v%s %s/%s", version.Version, sys.OS, sys.Arch)
	issueURL := fmt.Sprintf(
		"https://github.com/chrixbedardcad/GhostSpell/issues/new?title=%s&body=%s&labels=bug",
		url.QueryEscape(title),
		url.QueryEscape(body.String()),
	)

	// GitHub URLs have a practical limit of ~8192 chars. If we exceed it,
	// truncate the log portion and retry.
	if len(issueURL) > 8000 {
		// Rebuild with shorter log.
		body.Reset()
		if description != "" {
			body.WriteString("## Description\n\n")
			body.WriteString(description)
			body.WriteString("\n\n")
		}
		body.WriteString("## System Information\n\n")
		body.WriteString("| | |\n|---|---|\n")
		fmt.Fprintf(&body, "| **Version** | %s |\n", version.Version)
		fmt.Fprintf(&body, "| **OS** | %s %s (%s) |\n", sys.OS, sys.OSVersion, sys.Arch)
		fmt.Fprintf(&body, "| **Locale** | %s |\n", sys.Locale)
		fmt.Fprintf(&body, "| **Keyboard** | %s |\n", sys.KeyboardLayout)
		fmt.Fprintf(&body, "| **Providers** | %s |\n", strings.Join(providers, ", "))
		fmt.Fprintf(&body, "| **Default Model** | %s |\n", defaultModel)
		fmt.Fprintf(&body, "| **Fingerprint** | `%s` |\n", fingerprint)
		body.WriteString("\n_Log was too large for URL — please paste from clipboard or attach ghostspell.log_\n")

		issueURL = fmt.Sprintf(
			"https://github.com/chrixbedardcad/GhostSpell/issues/new?title=%s&body=%s&labels=bug",
			url.QueryEscape(title),
			url.QueryEscape(body.String()),
		)
	}

	// Open browser.
	if err := openBrowser(issueURL); err != nil {
		slog.Error("[bugreport] failed to open browser", "error", err)
		return "error: failed to open browser — " + err.Error()
	}

	// Open the log file in file explorer so the user can drag it into the issue.
	if logFilePath != "" {
		OpenFile(logFilePath)
	}

	slog.Info("[bugreport] bug report submitted", "fingerprint", fingerprint, "log_file", logFilePath)
	return "ok"
}

// openBrowser opens a URL in the default browser.
func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	case "darwin":
		cmd = exec.Command("open", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}
