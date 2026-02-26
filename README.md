<p align="center">
  <img src="GhostType_logo.png" alt="GhostType Logo" width="300">
</p>

# GhostType

**AI-powered multilingual auto-correction, translation, and creative rewriting for virtual world chat.**

GhostType is a lightweight background service that hooks into your chat application (primarily Firestorm Second Life viewer) and provides real-time spelling correction, language translation, and creative text rewriting — powered by your choice of LLM provider.

Type in French, hit **Ctrl+G**, get it corrected. Switch to English, hit **Ctrl+G**, corrected too. Want to translate or rewrite instead? Change the active mode in the system tray (or `config.json`) — **Ctrl+G** always does whatever mode is active. Undo with **Ctrl+Z** or cancel with **Escape**. That simple.

---

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Hotkeys](#hotkeys)
- [Configuration](#configuration)
- [Supported Providers](#supported-providers)
- [Custom Rewrite Templates](#custom-rewrite-templates)
- [Building from Source](#building-from-source)
- [How It Works](#how-it-works)
- [Roadmap](#roadmap)
- [Troubleshooting](#troubleshooting)
- [License](#license)

---

## Features

- **Correct** — Auto-detects French or English and fixes spelling, grammar, and syntax errors
- **Translate** — Instantly translates between French and English (or any configured language pair)
- **Rewrite** — Rewrites your text using customizable prompt templates (funny, formal, sarcastic, flirty, poetic, and more)
- **Multi-Provider** — Works with Anthropic Claude, OpenAI GPT, Google Gemini, xAI Grok, or local Ollama models
- **Hotkey Driven** — One hotkey to learn: Ctrl+G performs the active mode (correct, translate, or rewrite). Escape to cancel. Optional dedicated hotkeys for power users
- **Configurable** — JSON config file for API keys, providers, hotkeys, prompts, overlay settings, and custom rewrite templates
- **Lightweight** — Single binary, runs in the background, under 50 MB memory, near-zero CPU at idle
- **Cross-Platform** — Windows first, Linux and macOS coming in future releases

---

## Quick Start

### 1. Download

Download the latest release for your platform from the [Releases](https://github.com/chrixbedardcad/GhostType/releases) page.

### 2. Configure

On first run, GhostType creates a default `config.json` in the same directory. Open it and add your API key:

```json
{
  "llm_provider": "anthropic",
  "api_key": "YOUR_API_KEY_HERE",
  "model": "claude-sonnet-4-5-20250929"
}
```

### 3. Run

```bash
ghosttype.exe
```

GhostType starts minimized in your system tray. Open Firestorm, type something in chat, and press **F6**.

---

## Hotkeys

### Default

| Hotkey | Action |
|--------|--------|
| **Ctrl+G** | Perform active mode (correct, translate, or rewrite) |
| **Escape** | Cancel in-progress operation |
| **Ctrl+Z** | Undo replacement (native) |

### Optional (add in `config.json`)

Power users can add dedicated hotkeys for specific modes:

| Config Key | Example | Action |
|------------|---------|--------|
| `hotkeys.translate` | `"Ctrl+J"` | Translate directly |
| `hotkeys.toggle_language` | `"Ctrl+F8"` | Cycle translation target language |
| `hotkeys.rewrite` | `"F9"` | Rewrite directly |
| `hotkeys.cycle_template` | `"Ctrl+F9"` | Cycle rewrite template |

All hotkeys are configurable in `config.json`. Set `active_mode` to `"correct"`, `"translate"`, or `"rewrite"` to choose what **Ctrl+G** does.

---

## Configuration

GhostType is configured entirely through `config.json`. Here is a full example:

```json
{
  "llm_provider": "anthropic",
  "api_key": "sk-ant-xxxxx",
  "model": "claude-sonnet-4-5-20250929",
  "api_endpoint": "",
  "languages": ["en", "fr"],
  "language_names": {
    "en": "English",
    "fr": "French"
  },
  "default_translate_target": "en",
  "active_mode": "correct",
  "hotkeys": {
    "correct": "Ctrl+G",
    "cancel": "Escape",
    "translate": "",
    "toggle_language": "",
    "rewrite": "",
    "cycle_template": ""
  },
  "prompts": {
    "correct": "Detect the language. Fix spelling and grammar. Return ONLY corrected text.",
    "translate": "Translate to {target_language}. Return ONLY the translation.",
    "rewrite_templates": [
      { "name": "funny", "prompt": "Rewrite as a funny, witty reply. Return ONLY the text." },
      { "name": "formal", "prompt": "Rewrite in a formal tone. Return ONLY the text." },
      { "name": "sarcastic", "prompt": "Rewrite with heavy sarcasm. Return ONLY the text." },
      { "name": "flirty", "prompt": "Rewrite in a playful, flirty tone. Return ONLY the text." },
      { "name": "poetic", "prompt": "Rewrite as a romantic poet. Return ONLY the text." }
    ]
  },
  "overlay": {
    "enabled": true,
    "opacity": 0.85,
    "auto_dismiss_seconds": 10,
    "highlight_changes": true,
    "font_size": 14
  },
  "max_tokens": 256,
  "timeout_ms": 5000,
  "preserve_clipboard": true,
  "log_level": "info",
  "log_file": "ghosttype.log"
}
```

---

## Supported Providers

| Provider | Config Value | Notes |
|----------|-------------|-------|
| Anthropic Claude | `anthropic` | Recommended. Excellent multilingual support. |
| OpenAI GPT | `openai` | GPT-4o or GPT-4 Turbo recommended. |
| Google Gemini | `gemini` | Good for multilingual tasks. |
| xAI Grok | `xai` | Fast inference. |
| Ollama (local) | `ollama` | Free, private, no API key needed. Requires Ollama running locally. |

Set `api_endpoint` to override the default endpoint for any provider — useful for proxies or custom deployments.

---

## Custom Rewrite Templates

You can add your own rewrite styles by editing the `rewrite_templates` array in `config.json`:

```json
{
  "name": "pirate",
  "prompt": "Rewrite this as a pirate would say it. Return ONLY the rewritten text."
}
```

Cycle through templates in real-time with **Ctrl+F8**. A brief floating label appears near your cursor showing the newly selected template name (e.g., "Funny", "Professional"). Similarly, toggle the translation target language with **Ctrl+F7** — a label appears showing "To French", "To English", etc.

---

## Building from Source

### Requirements

- Go 1.22 or later
- Windows 10/11 (MVP target)

### Build

```bash
git clone https://github.com/chrixbedardcad/GhostType.git
cd GhostType
go mod download
go build -o ghosttype.exe
```

### Run Tests

```bash
go test ./...
```

---

## How It Works

1. GhostType runs in the background and watches for hotkey presses.
2. It works globally — hotkeys fire regardless of which window is focused.
3. Choose your active mode: **correct**, **translate**, or **rewrite** (via `active_mode` in config, or system tray in a future update).
4. When you press **Ctrl+G**, GhostType detects any selected text. If you have a selection, only that text is processed. If nothing is selected, it selects all text in the active input.
5. The text is sent to your configured LLM provider with the appropriate prompt for the active mode.
6. The result replaces the original text. Press **Escape** to cancel, or **Ctrl+Z** to undo.
7. Your original clipboard content is preserved and restored.

---

## Roadmap

| Version | Focus | Highlights |
|---------|-------|------------|
| **v0.1** | MVP (current) | Windows desktop app. Correction mode. Anthropic and OpenAI support. |
| **v0.2** | Translation & Overlay | Translation mode. Ctrl+F7 toggle language with cursor notification. Transparent overlay. Ollama support. Linux. |
| **v0.3** | Rewrite Mode | Creative rewrite templates. Ctrl+F8 toggle template with cursor notification. Config hot-reload. macOS. |
| **v0.4** | More Providers | Gemini and xAI support. GUI config panel. Additional languages. |
| **v0.5** | Power Features | Real-time Grammarly-style correction. Usage stats. Custom plugins. |

---

## Troubleshooting

| Problem | Solution |
|---------|----------|
| GhostType doesn't respond to hotkeys | Verify GhostType is running. Check `ghosttype.log` for registration errors. If hotkeys conflict with other apps, change them in `config.json`. |
| API errors | Check your API key in `config.json`. Check `ghosttype.log` for details. Verify your provider account has credits. |
| Slow corrections | Response time depends on provider and network. Try a faster model or switch to a local Ollama instance. |
| Hotkey conflicts | If F6/F7/F8 conflict with other apps, change the hotkeys in `config.json`. |

---

## License

MIT

## Author

Chris

## Acknowledgments

Inspired by the UX patterns of [Grammarly](https://www.grammarly.com/), [LanguageTool](https://languagetool.org/), [Espanso](https://espanso.org/), [Raycast AI](https://www.raycast.com/), and macOS inline autocorrect. Built for the Second Life community.
