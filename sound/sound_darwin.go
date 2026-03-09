//go:build darwin

package sound

import (
	"os"
	"os/exec"
	"sync"
)

var (
	activeCmd    *exec.Cmd
	activeTmp    string
	activeCmdMu  sync.Mutex
)

func playWAV(data []byte) {
	f, err := os.CreateTemp("", "ghosttype-*.wav")
	if err != nil {
		return
	}
	tmpPath := f.Name()
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return
	}
	f.Close()

	cmd := exec.Command("afplay", tmpPath)
	if err := cmd.Start(); err != nil {
		os.Remove(tmpPath)
		return
	}

	activeCmdMu.Lock()
	activeCmd = cmd
	activeTmp = tmpPath
	activeCmdMu.Unlock()

	cmd.Wait()

	activeCmdMu.Lock()
	if activeCmd == cmd {
		activeCmd = nil
	}
	activeCmdMu.Unlock()

	os.Remove(tmpPath)
}

// stopPlayback kills the currently playing sound process.
func stopPlayback() {
	activeCmdMu.Lock()
	cmd := activeCmd
	tmp := activeTmp
	activeCmd = nil
	activeTmp = ""
	activeCmdMu.Unlock()

	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
	if tmp != "" {
		os.Remove(tmp)
	}
}
