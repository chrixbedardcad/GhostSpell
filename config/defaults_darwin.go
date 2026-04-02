//go:build darwin

package config

// defaultActionHotkey is the default hotkey for the main action.
// Ctrl+G is safe on macOS — Ctrl is rarely used for system shortcuts.
const defaultActionHotkey = "Ctrl+G"

// defaultCycleHotkey is the default hotkey for cycling prompts.
const defaultCycleHotkey = "Ctrl+Shift+T"
