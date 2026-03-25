//go:build !ghostvoice

package main

// embeddedGhostVoice is nil when built without the ghostvoice tag.
// Voice will still work if the user has ghostvoice next to the executable or on PATH.
var embeddedGhostVoice []byte
