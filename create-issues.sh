#!/bin/bash
# Script to create GitHub issues 8-20 for the GhostType project.
# Prerequisites: gh CLI authenticated (run 'gh auth login' first)
# Usage: ./create-issues.sh

set -e

REPO="chrixbedardcad/GhostType"

echo "Creating labels if they don't exist..."
gh label create "priority-critical" --color "B60205" --description "Critical priority" --repo "$REPO" 2>/dev/null || true
gh label create "priority-high" --color "D93F0B" --description "High priority" --repo "$REPO" 2>/dev/null || true
gh label create "priority-medium" --color "FBCA04" --description "Medium priority" --repo "$REPO" 2>/dev/null || true
gh label create "priority-low" --color "0E8A16" --description "Low priority" --repo "$REPO" 2>/dev/null || true
gh label create "milestone-v0.1" --color "1D76DB" --description "Milestone v0.1" --repo "$REPO" 2>/dev/null || true
gh label create "milestone-v0.2" --color "5319E7" --description "Milestone v0.2" --repo "$REPO" 2>/dev/null || true
gh label create "milestone-v0.3" --color "006B75" --description "Milestone v0.3" --repo "$REPO" 2>/dev/null || true
gh label create "milestone-v0.4" --color "BFD4F2" --description "Milestone v0.4" --repo "$REPO" 2>/dev/null || true
gh label create "milestone-v0.5" --color "C2E0C6" --description "Milestone v0.5" --repo "$REPO" 2>/dev/null || true
echo "Labels created."
echo ""

# ============================================================
# ISSUE 8: Core - Wire correction workflow F6
# ============================================================
echo "Creating Issue 8: Core - Wire correction workflow F6..."
gh issue create --repo "$REPO" \
  --title "Core - Wire correction workflow F6" \
  --label "priority-critical,milestone-v0.1" \
  --body "$(cat <<'EOF'
## Description

Wire all modules together into the main correction workflow triggered by F6.

**Depends on:** Issues #2, #3, #4, #5, #6, #7

## Requirements

### On startup
- Load config
- Initialize LLM client
- Register hotkeys
- Start listening

### When F6 is pressed
1. Save current clipboard content
2. Simulate Ctrl+A (select all in active app)
3. Wait 50ms
4. Simulate Ctrl+C (copy selected text)
5. Wait 100ms
6. Read clipboard to get input text
7. If text is empty, log warning and abort (do not modify anything)
8. Send text to LLM API with correction prompt from config
9. If API returns error, log error, restore clipboard, abort (do not modify text)
10. If API returns valid result, write result to clipboard
11. Simulate Ctrl+A (select all again)
12. Wait 50ms
13. Simulate Ctrl+V (paste corrected text)
14. Wait 50ms
15. Restore original clipboard content

### When Escape is pressed
- Cancel any in-progress API call

### Critical Safety Rule
**NEVER replace text if API call fails. Original text must remain untouched.**

### Console Output
- Print status messages to console for MVP (notifications come later)

## Testing

- [ ] Test in Notepad with English text containing typos
- [ ] Test in Notepad with French text containing typos
- [ ] Test in Firestorm chat input
- [ ] Test with invalid API key (text should not be modified)
- [ ] Test with no network (text should not be modified)
- [ ] Test Ctrl+Z after replacement (should restore original)
EOF
)"
echo "Issue 8 created."
echo ""

# ============================================================
# ISSUE 9: System Tray - Icon and menu
# ============================================================
echo "Creating Issue 9: System Tray - Icon and menu..."
gh issue create --repo "$REPO" \
  --title "System Tray - Icon and menu" \
  --label "priority-medium,milestone-v0.1" \
  --body "$(cat <<'EOF'
## Description

Add system tray icon so GhostType runs as a background app with a tray menu.

**Depends on:** Issue #8

## Requirements

- Use `github.com/getlantern/systray` for system tray
- Show icon in system tray on startup
- Tray menu items:
  - **Status** (shows provider and model)
  - **Open Config** (opens `config.json` in default editor)
  - **Open Log** (opens log file)
  - Separator
  - **Quit**
- Status indicator:
  - Green icon when idle
  - Yellow when processing API call
  - Red on error
- App should start minimized with no visible window, only the tray icon
- Quit menu item unregisters hotkeys and exits cleanly
- Create simple icon assets in `assets/` folder
EOF
)"
echo "Issue 9 created."
echo ""

# ============================================================
# ISSUE 10: Notifications - Toast notifications for status
# ============================================================
echo "Creating Issue 10: Notifications - Toast notifications for status..."
gh issue create --repo "$REPO" \
  --title "Notifications - Toast notifications for status" \
  --label "priority-medium,milestone-v0.1" \
  --body "$(cat <<'EOF'
## Description

Add Windows toast notifications to show processing status.

**Depends on:** Issue #8

## Requirements

- Use `github.com/go-toast/toast` for Windows notifications
- Show notification when F6 is pressed: "Correcting..." with GhostType icon
- Show notification when correction completes: "Text corrected" (auto-dismiss 2 seconds)
- Show notification on error: "Correction failed: [reason]" (auto-dismiss 5 seconds)
- Show notification when config is invalid on startup
- Show notification if hotkey registration fails
- Notifications should be brief and non-intrusive
- Configurable via `show_notifications` in `config.json` (can be disabled)
EOF
)"
echo "Issue 10 created."
echo ""

# ============================================================
# ISSUE 11: Logging - Structured logging to file
# ============================================================
echo "Creating Issue 11: Logging - Structured logging to file..."
gh issue create --repo "$REPO" \
  --title "Logging - Structured logging to file" \
  --label "priority-medium,milestone-v0.1" \
  --body "$(cat <<'EOF'
## Description

Add structured logging throughout the application.

**Depends on:** Issue #2

## Requirements

- Use `log/slog` from Go stdlib
- Log to file specified in `config.json` (default `ghosttype.log`)
- Log levels: debug, info, warn, error (configurable in `config.json`)
- Log format: `[TIMESTAMP] [LEVEL] [MODULE] message`
- Log on startup: config loaded, provider, model, registered hotkeys
- Log on each correction: input text length, provider called, response time, success/failure
- Log all errors with details
- In debug mode: log full API requests and responses with **API key REDACTED**
- **Never log the raw API key**
EOF
)"
echo "Issue 11 created."
echo ""

# ============================================================
# ISSUE 12: Translation mode F7
# ============================================================
echo "Creating Issue 12: Translation mode F7..."
gh issue create --repo "$REPO" \
  --title "Translation mode F7" \
  --label "priority-high,milestone-v0.2" \
  --body "$(cat <<'EOF'
## Description

Add translation mode triggered by F7 hotkey.

**Depends on:** Issue #8

## Requirements

- Register F7 as global hotkey
- When F7 is pressed, same clipboard workflow as F6 but use translation prompt
- Translation prompt uses `default_translate_target` from config
- Auto-detect source language via the LLM prompt
- Register Ctrl+F7 as toggle for translation target language
- Ctrl+F7 cycles through the `languages` array in config
- Show brief notification with target language name when toggling: "To French", "To English"
- Add `mode/translate.go` for translation logic
- Update `main.go` to wire F7 and Ctrl+F7

## Testing

- [ ] Type English text, press F7, should translate to French (or configured target)
- [ ] Type French text, press F7, should translate to English
- [ ] Press Ctrl+F7 to toggle, verify notification shows correct language
- [ ] Press F7 after toggle, verify translation uses new target
EOF
)"
echo "Issue 12 created."
echo ""

# ============================================================
# ISSUE 13: Rewrite mode F8
# ============================================================
echo "Creating Issue 13: Rewrite mode F8..."
gh issue create --repo "$REPO" \
  --title "Rewrite mode F8" \
  --label "priority-high,milestone-v0.3" \
  --body "$(cat <<'EOF'
## Description

Add rewrite mode triggered by F8 hotkey with configurable templates.

**Depends on:** Issue #8

## Requirements

- Register F8 as global hotkey
- When F8 is pressed, same clipboard workflow as F6 but use the active rewrite template prompt
- Register Ctrl+F8 as toggle for cycling rewrite templates
- Ctrl+F8 cycles through `rewrite_templates` array in config
- Show brief notification with template name when toggling: "Funny", "Formal", "Sarcastic"
- Default active template is the first one in the array
- Add `mode/rewrite.go` for rewrite logic
- Update `main.go` to wire F8 and Ctrl+F8

## Testing

- [ ] Type text, press F8, should rewrite using first template
- [ ] Press Ctrl+F8 to cycle, verify notification shows template name
- [ ] Press F8 after cycling, verify it uses the new template
- [ ] Test with various templates: funny, formal, sarcastic, poetic
EOF
)"
echo "Issue 13 created."
echo ""

# ============================================================
# ISSUE 14: Cursor notification - Floating label near cursor
# ============================================================
echo "Creating Issue 14: Cursor notification - Floating label near cursor..."
gh issue create --repo "$REPO" \
  --title "Cursor notification - Floating label near cursor" \
  --label "priority-medium,milestone-v0.2" \
  --body "$(cat <<'EOF'
## Description

Replace toast notifications for toggle actions with a small floating label near the mouse cursor.

**Depends on:** Issues #12, #13

## Requirements

- When Ctrl+F7 or Ctrl+F8 is pressed, show a tiny semi-transparent label near the cursor
- Label displays the selected language name or template name
- Use Windows `GetCursorPos` to get cursor position
- Render as a small borderless always-on-top transparent window
- Label appears for 2 seconds then fades out
- Minimal size, clean font, dark background with white text, rounded corners
- Create `cursor/cursor.go` interface and `cursor/cursor_windows.go` implementation
- This is only for toggle notifications, not for correction/translation/rewrite status
EOF
)"
echo "Issue 14 created."
echo ""

# ============================================================
# ISSUE 15: Ollama local LLM support
# ============================================================
echo "Creating Issue 15: Ollama local LLM support..."
gh issue create --repo "$REPO" \
  --title "Ollama local LLM support" \
  --label "priority-medium,milestone-v0.2" \
  --body "$(cat <<'EOF'
## Description

Add Ollama as a local LLM provider for offline and free usage.

**Depends on:** Issue #3

## Requirements

- Implement Ollama provider in `llm/ollama.go`
- Send POST request to `http://localhost:11434/api/generate` (or configured endpoint)
- Format request body with model and prompt
- Parse streaming or non-streaming response
- Handle errors: Ollama not running, model not found, timeout
- No API key required for Ollama
- Write unit tests in `llm/ollama_test.go`
- Document recommended models for correction tasks (mistral, llama3, etc.)
EOF
)"
echo "Issue 15 created."
echo ""

# ============================================================
# ISSUE 16: Linux support - X11 hotkeys and key simulation
# ============================================================
echo "Creating Issue 16: Linux support - X11 hotkeys and key simulation..."
gh issue create --repo "$REPO" \
  --title "Linux support - X11 hotkeys and key simulation" \
  --label "priority-medium,milestone-v0.2" \
  --body "$(cat <<'EOF'
## Description

Add Linux support for global hotkeys and key simulation.

## Requirements

- Implement `hotkey/hotkey_linux.go` using X11 `XGrabKey` for global hotkey registration
- Implement `keyboard/keyboard_linux.go` using XTest extension for key simulation
- Implement `cursor/cursor_linux.go` for cursor position (X11)
- Test on Ubuntu with X11
- Wayland support is out of scope, defer to v0.4
- Use build tags to separate platform-specific code
EOF
)"
echo "Issue 16 created."
echo ""

# ============================================================
# ISSUE 17: macOS support - Global hotkeys and key simulation
# ============================================================
echo "Creating Issue 17: macOS support - Global hotkeys and key simulation..."
gh issue create --repo "$REPO" \
  --title "macOS support - Global hotkeys and key simulation" \
  --label "priority-medium,milestone-v0.3" \
  --body "$(cat <<'EOF'
## Description

Add macOS support for global hotkeys and key simulation.

## Requirements

- Implement `hotkey/hotkey_darwin.go` using Carbon `RegisterEventHotKey` or `NSEvent addGlobalMonitorForEvents`
- Implement `keyboard/keyboard_darwin.go` using `CGEventCreateKeyboardEvent`
- Implement `cursor/cursor_darwin.go` for cursor position
- Handle macOS accessibility permissions (app needs accessibility access to simulate keys)
- Use Cmd instead of Ctrl for key simulation (Cmd+A, Cmd+C, Cmd+V)
- Use build tags to separate platform-specific code
- Test on macOS Ventura or later
EOF
)"
echo "Issue 17 created."
echo ""

# ============================================================
# ISSUE 18: Gemini and xAI provider support
# ============================================================
echo "Creating Issue 18: Gemini and xAI provider support..."
gh issue create --repo "$REPO" \
  --title "Gemini and xAI provider support" \
  --label "priority-low,milestone-v0.4" \
  --body "$(cat <<'EOF'
## Description

Add Google Gemini and xAI Grok as LLM providers.

**Depends on:** Issue #3

## Requirements

- Implement Gemini provider in `llm/gemini.go` with Google Generative AI API
- Implement xAI provider in `llm/xai.go` with xAI chat completions API
- Write unit tests for both
- Update factory function in `llm/client.go`
- Test with real API keys for both providers
EOF
)"
echo "Issue 18 created."
echo ""

# ============================================================
# ISSUE 19: GUI config panel
# ============================================================
echo "Creating Issue 19: GUI config panel..."
gh issue create --repo "$REPO" \
  --title "GUI config panel" \
  --label "priority-low,milestone-v0.4" \
  --body "$(cat <<'EOF'
## Description

Add a simple GUI window for editing config without touching JSON.

## Requirements

- Accessible from system tray menu: "Settings"
- Fields for: provider dropdown, API key input, model input, hotkey configuration
- Language list editor
- Rewrite template editor (add, remove, reorder)
- Save button writes to `config.json`
- Test button validates API key by sending a test request
- Use Fyne for cross-platform GUI
- Config hot-reload after save without restarting the app
EOF
)"
echo "Issue 19 created."
echo ""

# ============================================================
# ISSUE 20: Optional overlay with diff view
# ============================================================
echo "Creating Issue 20: Optional overlay with diff view..."
gh issue create --repo "$REPO" \
  --title "Optional overlay with diff view" \
  --label "priority-low,milestone-v0.5" \
  --body "$(cat <<'EOF'
## Description

Add an optional overlay mode that shows corrections before replacing.

## Requirements

- When `overlay.enabled` is true in config, change the workflow: instead of immediately replacing text, show a transparent overlay near the cursor displaying the corrected text with changes highlighted
- Green highlight for added/changed words, strikethrough for removed words
- User presses F6 again to accept or Escape to dismiss
- This is an ALTERNATIVE workflow to the one-key-one-action default
- Default is overlay disabled (one-key-one-action)
- Use Fyne or a borderless window for overlay rendering
EOF
)"
echo "Issue 20 created."
echo ""

echo "============================================"
echo "All 13 issues (8-20) created successfully!"
echo "============================================"
