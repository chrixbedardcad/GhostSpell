//go:build !windows

package main

import "fmt"

// runLive is called from main() when not in test mode.
// On non-Windows platforms, it shows instructions for how to run the POC.
func runLive() {
	fmt.Println("This POC requires Windows for live hotkey support.")
	fmt.Println("Use '-test' flag to run the simulated demo:")
	fmt.Println("  go run ./cmd/poc -test")
	fmt.Println()
	fmt.Println("On Windows, build and run:")
	fmt.Println("  go build -o ghosttype-poc.exe ./cmd/poc")
	fmt.Println("  .\\ghosttype-poc.exe")
}
