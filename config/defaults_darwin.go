//go:build darwin

package config

// defaultCorrectHotkey is the default hotkey for the "correct" action.
// On macOS, Ctrl maps to Cmd (⌘), so "Ctrl+G" becomes ⌘G.
const defaultCorrectHotkey = "Ctrl+G"
