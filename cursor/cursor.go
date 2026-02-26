package cursor

// Indicator displays a small ghost icon near the mouse cursor to provide
// visual feedback during LLM processing. The icon pulses while working,
// then briefly shows a success or error state before fading out.
type Indicator interface {
	Show()    // start pulsing working indicator near cursor
	Success() // swap to success state, auto-hide after 1s
	Error()   // swap to error state, auto-hide after 2s
	Hide()    // immediately hide (for cancel)
	Close()   // destroy window, free resources
}
