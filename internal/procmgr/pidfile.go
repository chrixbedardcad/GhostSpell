package procmgr

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
)

const pidFileName = "ghost.pid"

// PIDFile tracks PIDs of all GhostSpell processes.
type PIDFile struct {
	Ghost      int `json:"ghost"`
	GhostAI    int `json:"ghostai,omitempty"`
	GhostVoice int `json:"ghostvoice,omitempty"`
}

// WritePIDFile writes the PID file to the given directory.
func WritePIDFile(dir string, pf PIDFile) error {
	data, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, pidFileName)
	return os.WriteFile(path, data, 0644)
}

// ReadPIDFile reads the PID file from the given directory.
// Returns zero-valued PIDFile if the file doesn't exist.
func ReadPIDFile(dir string) (PIDFile, error) {
	path := filepath.Join(dir, pidFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PIDFile{}, nil
		}
		return PIDFile{}, err
	}
	var pf PIDFile
	err = json.Unmarshal(data, &pf)
	return pf, err
}

// CleanupStale reads the PID file, kills any processes that are still running
// from a previous session, and removes the PID file. Returns PIDs that were killed.
func CleanupStale(dir string) []int {
	pf, err := ReadPIDFile(dir)
	if err != nil {
		return nil
	}

	var killed []int
	myPID := os.Getpid()

	for _, entry := range []struct {
		name string
		pid  int
	}{
		{"ghostai", pf.GhostAI},
		{"ghostvoice", pf.GhostVoice},
	} {
		if entry.pid <= 0 || entry.pid == myPID {
			continue
		}
		if IsAlive(entry.pid) {
			slog.Warn("[procmgr] killing stale process", "name", entry.name, "pid", entry.pid)
			if proc, err := os.FindProcess(entry.pid); err == nil {
				proc.Kill()
				killed = append(killed, entry.pid)
			}
		}
	}

	// Remove stale PID file.
	os.Remove(filepath.Join(dir, pidFileName))
	return killed
}

// RemovePIDFile deletes the PID file.
func RemovePIDFile(dir string) {
	os.Remove(filepath.Join(dir, pidFileName))
}
