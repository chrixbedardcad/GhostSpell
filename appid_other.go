//go:build !windows

package main

// setAppID is a no-op on non-Windows platforms.
func setAppID() {}
