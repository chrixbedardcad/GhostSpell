//go:build windows

package main

import (
	"log"
	"os"
	"syscall"
	"time"

	"github.com/chrixbedardcad/GhostType/clipboard"
	"github.com/chrixbedardcad/GhostType/hotkey"
	"github.com/chrixbedardcad/GhostType/keyboard"
)

var (
	kernel32Win  = syscall.NewLazyDLL("kernel32.dll")
	procBeep     = kernel32Win.NewProc("Beep")
)

// winBeep plays a short tone using the Windows Beep API.
// freq is in Hz, duration in milliseconds.
func winBeep(freq, durationMs uint32) {
	procBeep.Call(uintptr(freq), uintptr(durationMs))
}

func runLive() {
	RunWindowsLive()
}

// RunWindowsLive registers F7 as a global hotkey and runs the clipboard
// workflow with a simple test message (no LLM).
func RunWindowsLive() {
	log.Println("[INIT] Creating clipboard, keyboard, hotkey managers")
	cb := clipboard.NewWindowsClipboard()
	kb := keyboard.NewWindowsSimulator()
	hk := hotkey.NewWindowsManager()

	log.Println("[INIT] Registering F7 hotkey...")
	err := hk.Register("correct", "F7", func() {
		log.Println("[F7] ---- Correction triggered! ----")
		winBeep(800, 100) // Short beep to confirm F7 press

		// Step 1: Save original clipboard
		log.Println("[STEP 1] Saving original clipboard...")
		if err := cb.Save(); err != nil {
			log.Printf("[ERROR] Failed to save clipboard: %v", err)
			return
		}
		log.Println("[STEP 1] Clipboard saved OK")

		// Step 2: Select all text in active window
		log.Println("[STEP 2] Sending Ctrl+A (SelectAll)...")
		if err := kb.SelectAll(); err != nil {
			log.Printf("[ERROR] SelectAll failed: %v", err)
			cb.Restore()
			return
		}
		log.Println("[STEP 2] SelectAll sent, sleeping 50ms")
		time.Sleep(50 * time.Millisecond)

		// Step 3: Copy selected text
		log.Println("[STEP 3] Sending Ctrl+C (Copy)...")
		if err := kb.Copy(); err != nil {
			log.Printf("[ERROR] Copy failed: %v", err)
			cb.Restore()
			return
		}
		log.Println("[STEP 3] Copy sent, sleeping 100ms")
		time.Sleep(100 * time.Millisecond)

		// Step 4: Read clipboard to get input text
		log.Println("[STEP 4] Reading clipboard...")
		text, err := cb.Read()
		if err != nil {
			log.Printf("[ERROR] Failed to read clipboard: %v", err)
			cb.Restore()
			return
		}

		if text == "" {
			log.Println("[WARN] Nothing to correct (empty text)")
			cb.Restore()
			return
		}

		log.Printf("[STEP 4] Clipboard text (%d chars): %q", len(text), text)

		// Step 5: Apply simple correction (no LLM)
		corrected := correctText(text)
		log.Printf("[STEP 5] Corrected text: %q", corrected)

		// Step 6: Write result to clipboard
		log.Println("[STEP 6] Writing corrected text to clipboard...")
		if err := cb.Write(corrected); err != nil {
			log.Printf("[ERROR] Failed to write clipboard: %v", err)
			cb.Restore()
			return
		}
		log.Println("[STEP 6] Clipboard written OK")

		// Step 7: Select all and paste
		log.Println("[STEP 7] Sending Ctrl+A (SelectAll)...")
		if err := kb.SelectAll(); err != nil {
			log.Printf("[ERROR] SelectAll (paste prep) failed: %v", err)
			cb.Restore()
			return
		}
		time.Sleep(50 * time.Millisecond)

		log.Println("[STEP 7] Sending Ctrl+V (Paste)...")
		if err := kb.Paste(); err != nil {
			log.Printf("[ERROR] Paste failed: %v", err)
			cb.Restore()
			return
		}
		log.Println("[STEP 7] Paste sent, sleeping 50ms")
		time.Sleep(50 * time.Millisecond)

		// Step 8: Restore original clipboard
		log.Println("[STEP 8] Restoring original clipboard...")
		cb.Restore()
		log.Println("[STEP 8] Clipboard restored")

		winBeep(1200, 150) // Higher beep to confirm correction complete
		log.Println("[OK] ---- Correction complete! ----")
	})
	if err != nil {
		log.Printf("[FATAL] Failed to register F7: %v", err)
		return
	}
	log.Println("[INIT] F7 hotkey registered OK")

	log.Println("[INIT] Registering Escape hotkey...")
	err = hk.Register("quit", "Escape", func() {
		log.Println("Escape pressed — exiting cleanly.")
		hk.Unregister("correct")
		hk.Unregister("quit")
		os.Exit(0)
	})
	if err != nil {
		log.Printf("[FATAL] Failed to register Escape: %v", err)
		return
	}
	log.Println("[INIT] Escape hotkey registered OK")

	log.Println("F7 registered! Press F7 in any text field to test.")
	log.Println("Text will be uppercased with [CORRECTED] prefix.")
	log.Println("Press Escape to exit.")

	hk.Listen()
}
