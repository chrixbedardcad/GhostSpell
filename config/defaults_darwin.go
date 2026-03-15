//go:build darwin

package config

// defaultActionHotkey is the default hotkey for the main action.
// On macOS, Ctrl maps to Cmd (⌘), so "Ctrl+G" becomes ⌘G.
const defaultActionHotkey = "Ctrl+G"

// defaultCycleHotkey is the default hotkey for cycling prompts.
const defaultCycleHotkey = "Ctrl+Shift+T"
