//go:build ghostvoice && windows

package main

import _ "embed"

// embeddedGhostVoice contains the ghostvoice.exe binary, embedded at build time.
// The voicebin/ directory is populated by CI or _build.bat before `go build`.
//
//go:embed voicebin/ghostvoice.exe
var embeddedGhostVoice []byte
