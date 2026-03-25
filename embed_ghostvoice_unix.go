//go:build ghostvoice && !windows

package main

import _ "embed"

// embeddedGhostVoice contains the ghostvoice binary, embedded at build time.
// The voicebin/ directory is populated by CI or build scripts before `go build`.
//
//go:embed voicebin/ghostvoice
var embeddedGhostVoice []byte
