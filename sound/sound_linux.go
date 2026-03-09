//go:build linux

package sound

import (
	"os"
	"os/exec"
	"sync"
)

var (
	workingCmd   *exec.Cmd
	workingTmp   string
	workingCmdMu sync.Mutex
)

// findPlayer returns the first available audio player.
func findPlayer() string {
	for _, p := range []string{"paplay", "aplay"} {
		if path, err := exec.LookPath(p); err == nil {
			return path
		}
	}
	return ""
}

// playWAV plays a WAV sound (fire-and-forget). Blocks until done.
func playWAV(data []byte) {
	player := findPlayer()
	if player == "" {
		return
	}

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

	cmd := exec.Command(player, tmpPath)
	if err := cmd.Start(); err != nil {
		os.Remove(tmpPath)
		return
	}
	cmd.Wait()
	os.Remove(tmpPath)
}

// playWAVLoop is like playWAV but tracks the process so it can be killed.
func playWAVLoop(data []byte) {
	player := findPlayer()
	if player == "" {
		return
	}

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

	cmd := exec.Command(player, tmpPath)
	if err := cmd.Start(); err != nil {
		os.Remove(tmpPath)
		return
	}

	workingCmdMu.Lock()
	workingCmd = cmd
	workingTmp = tmpPath
	workingCmdMu.Unlock()

	cmd.Wait()

	workingCmdMu.Lock()
	if workingCmd == cmd {
		workingCmd = nil
	}
	workingCmdMu.Unlock()

	os.Remove(tmpPath)
}

// stopPlayback kills the working loop's sound process.
func stopPlayback() {
	workingCmdMu.Lock()
	cmd := workingCmd
	tmp := workingTmp
	workingCmd = nil
	workingTmp = ""
	workingCmdMu.Unlock()

	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
	if tmp != "" {
		os.Remove(tmp)
	}
}
