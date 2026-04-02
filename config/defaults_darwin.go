//go:build darwin

package config

// defaultActionHotkey is the default hotkey for the main action.
// Cmd+Shift+G avoids conflicts with Cmd+G (Find Next) on macOS.
const defaultActionHotkey = "Cmd+Shift+G"

// defaultCycleHotkey is the default hotkey for cycling prompts.
const defaultCycleHotkey = "Ctrl+Shift+T"
