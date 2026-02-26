# GhostType Icon Assets — Requirements Document

**Issue:** [#64](https://github.com/chrixbedardcad/GhostType/issues/64)
**Related:** [#50 — Cursor indicator implementation](https://github.com/chrixbedardcad/GhostType/issues/50)
**Date:** 2026-02-26

---

## Overview

GhostType needs 12 icon assets to power the cursor indicator animation system and system tray states. All icons must follow a consistent ghost art style matching the existing GhostType logo.

All assets live in the `assets/` directory and are embedded into the binary via Go's `//go:embed` directive.

---

## General Art Direction

- **Style:** Consistent with the existing GhostType ghost logo — friendly, minimal, recognizable
- **Background:** Transparent on all assets
- **Outlines:** Clean, simple shapes that remain legible at small sizes
- **Color palette:** Mostly monochrome ghost body; use accent colors sparingly:
  - Green for success states
  - Red for error states
  - Orange for timeout states
  - Gray for cancel/neutral states
- **Total embedded size budget:** Under 1 MB for all 12 assets combined

---

## Asset Specifications

### 1. Processing State Icons

Displayed near the mouse cursor while an API call is in progress. These icons pulse (opacity cycles 100% -> 40% -> 100% over 1 second) until the operation completes.

| # | Filename | Size | Format | Description | Usage |
|---|----------|------|--------|-------------|-------|
| 1 | `gh_correction_icon.png` | 48x48 | PNG | Ghost with a pencil or checkmark hint | Ctrl+G — correction mode |
| 2 | `gh_translation_icon.png` | 48x48 | PNG | Ghost with a globe or language symbol | Ctrl+J — translation mode |
| 3 | `gh_creative_icon.png` | 48x48 | PNG | Ghost with a sparkle or magic wand | Ctrl+Y — rewrite mode |

**Behavior:**
- Positioned 20px right and 20px below the mouse cursor
- Follows cursor movement (position updates every 100ms)
- Opacity pulses continuously until API call completes
- Text label rendered below the icon:
  - Correction: "Correcting..."
  - Translation: "Translating to {language}..."
  - Rewrite: "Rewriting ({template})..."

---

### 2. Result State Icons

Displayed briefly after an API call completes, replacing the processing icon. No pulsing — shown at 100% opacity, then fade out.

| # | Filename | Size | Format | Description | Duration | Fade |
|---|----------|------|--------|-------------|----------|------|
| 4 | `gh_success_icon.png` | 48x48 | PNG | Ghost with green checkmark, happy expression | 1 second | 300ms |
| 5 | `gh_error_icon.png` | 48x48 | PNG | Ghost with red X, worried expression | 2 seconds | 300ms |
| 6 | `gh_timeout_icon.png` | 48x48 | PNG | Ghost with clock or hourglass | 2 seconds | 300ms |
| 7 | `gh_cancel_icon.png` | 48x48 | PNG | Ghost with stop hand or dash | 0.5 seconds | 200ms |

**Text labels:**
- Success: "Done" (green)
- Error: error message (red)
- Timeout: "Timed out" (orange)
- Cancel: "Cancelled" (gray)

---

### 3. Toggle State Icons

Displayed near the cursor when the user cycles modes or settings. Static display (no pulsing), auto-dismiss after 2 seconds with 300ms fade out.

| # | Filename | Size | Format | Description | Usage |
|---|----------|------|--------|-------------|-------|
| 8 | `gh_toggle_language_icon.png` | 48x48 | PNG | Ghost with speech bubbles showing language codes (e.g. FR, EN) | Ctrl+Shift+J — toggle translation target |
| 9 | `gh_toggle_template_icon.png` | 48x48 | PNG | Ghost with paintbrush or palette | Ctrl+Shift+R — cycle rewrite template |

**Text labels:**
- Language toggle: "To {language}" (e.g. "To French")
- Template cycle: "{template name}" (e.g. "Professional")

---

### 4. System Tray Icons

Used in the Windows system tray to reflect app state. ICO format with multiple embedded resolutions for crisp rendering at any DPI.

| # | Filename | Resolutions | Format | Description | Usage |
|---|----------|-------------|--------|-------------|-------|
| 10 | `gh_tray_idle_icon.ico` | 16x16, 32x32, 48x48, 256x256 | ICO | Ghost head, normal state | Tray — idle |
| 11 | `gh_tray_busy_icon.ico` | 16x16, 32x32, 48x48, 256x256 | ICO | Ghost head with subtle glow or pulse hint | Tray — processing API call |
| 12 | `gh_tray_error_icon.ico` | 16x16, 32x32, 48x48, 256x256 | ICO | Ghost head with red tint | Tray — error state (reverts to idle after 5s) |

---

## Icon Usage Map

### Action Hotkey Flow (Ctrl+G, Ctrl+J, Ctrl+Y)

```
User presses hotkey
    |
    v
Show processing icon (correction / translation / creative)
Pulse opacity: 100% -> 40% -> 100% (1s cycle)
Follow cursor position (update every 100ms)
    |
    +---> Success: swap to gh_success_icon.png, hold 1s, fade 300ms
    +---> Error:   swap to gh_error_icon.png, hold 2s, fade 300ms
    +---> Timeout: swap to gh_timeout_icon.png, hold 2s, fade 300ms
    +---> Cancel:  swap to gh_cancel_icon.png, hold 0.5s, fade 200ms
```

### Toggle Hotkey Flow (Ctrl+Shift+J, Ctrl+Shift+R)

```
User presses toggle hotkey
    |
    v
Show toggle icon (language / template) at 100% opacity
Display text label with current setting
No pulsing — static
    |
    v
Hold 2 seconds, fade out 300ms
```

### System Tray State Machine

```
Idle (gh_tray_idle_icon.ico)
    |
    +---> API call starts --> Busy (gh_tray_busy_icon.ico)
    |                              |
    |                              +---> Success --> Idle
    |                              +---> Error ----> Error (gh_tray_error_icon.ico)
    |                                                    |
    |                                                    +---> 5 seconds --> Idle
    |
```

---

## Technical Requirements

### Cursor Indicator Window (Windows)

- Borderless, always-on-top, click-through overlay
- Win32 flags: `WS_EX_TOPMOST | WS_EX_TRANSPARENT | WS_EX_LAYERED | WS_EX_NOACTIVATE`
- Must not steal focus from the target application
- Clicking through the indicator must reach the application behind it
- Animation runs in a separate goroutine — must not block the main thread or API call

### Text Label Rendering

- Small font, rendered as part of the same borderless window
- Semi-transparent dark background with white text, rounded corners
- Positioned below the icon

### Performance Constraints

- Window creation: under 50ms
- Animation CPU usage: under 1%
- All embedded assets combined: under 1 MB

---

## Acceptance Checklist

- [ ] All 12 files exist in `assets/` with the exact filenames listed above
- [ ] All 9 PNG files are exactly 48x48 pixels with transparent background
- [ ] All 3 ICO files contain 16x16, 32x32, 48x48, and 256x256 layers
- [ ] All icons are visually consistent with the GhostType brand
- [ ] Icons are clearly distinguishable at both 48x48 and 16x16 sizes
- [ ] Total embedded size of all assets is under 1 MB
- [ ] Each icon's purpose is immediately recognizable without text labels
