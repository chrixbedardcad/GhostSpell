package procmgr

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// globalJob is the process-wide Job Object. Set once at startup via Init().
var (
	globalJob  *JobObject
	globalOnce sync.Once
)

// Init creates the global Job Object and cleans up stale processes from a
// previous session. Call once at app startup.
func Init(appDir string) {
	globalOnce.Do(func() {
		// Clean stale processes from previous run.
		if killed := CleanupStale(appDir); len(killed) > 0 {
			slog.Info("[procmgr] cleaned stale processes", "killed", killed)
		}

		// Create Job Object (Windows) or process group root (Unix).
		job, err := NewJobObject()
		if err != nil {
			slog.Warn("[procmgr] failed to create Job Object", "error", err)
			return
		}
		globalJob = job
		slog.Info("[procmgr] initialized")
	})
}

// Shutdown closes the global Job Object, killing all child processes.
func Shutdown() {
	if globalJob != nil {
		globalJob.Close()
		globalJob = nil
	}
}

// AgentProcess represents a running child agent process (ghostai, ghostvoice, etc.).
type AgentProcess struct {
	Name string
	Cmd  *exec.Cmd
	Port int
}

// SpawnHTTPAgent starts a child process that is expected to print "READY port=N"
// on stdout once it's ready to accept HTTP requests. The child is assigned to the
// global Job Object (Windows) or started in its own process group (Unix).
//
// The parent PID is passed via --parent-pid so the child can self-exit if orphaned.
// SpawnOptions contains optional configuration for SpawnHTTPAgent.
type SpawnOptions struct {
	Env []string // Extra environment variables ("KEY=VALUE") for the child process.
}

func SpawnHTTPAgent(name, binPath string, args []string, job *JobObject, opts ...SpawnOptions) (*AgentProcess, error) {
	// Use global job if none provided.
	if job == nil {
		job = globalJob
	}
	parentPID := os.Getpid()

	// Append --parent-pid to args.
	fullArgs := append(args, "--parent-pid", strconv.Itoa(parentPID))

	cmd := exec.Command(binPath, fullArgs...)
	cmd.Stderr = os.Stderr // Child logs to parent stderr (also writes to its own log file)

	// Apply extra environment variables (inherit parent env + extras).
	if len(opts) > 0 && len(opts[0].Env) > 0 {
		cmd.Env = append(os.Environ(), opts[0].Env...)
	}

	// Set up process group on Unix for group kill.
	setupProcessGroup(cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("procmgr: %s stdout pipe: %w", name, err)
	}

	slog.Info("[procmgr] spawning agent", "name", name, "bin", binPath, "args", fullArgs)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("procmgr: %s start: %w", name, err)
	}

	// Assign to Job Object (Windows) — no-op on Unix.
	if job != nil && cmd.Process != nil {
		assignToJob(job, cmd)
	}

	// Wait for READY line with timeout.
	scanner := bufio.NewScanner(stdoutPipe)
	port, err := waitForReady(name, scanner, 30*time.Second)
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return nil, err
	}

	slog.Info("[procmgr] agent ready", "name", name, "pid", cmd.Process.Pid, "port", port)

	return &AgentProcess{
		Name: name,
		Cmd:  cmd,
		Port: port,
	}, nil
}

// waitForReady reads lines from the child's stdout until it finds "READY port=N".
func waitForReady(name string, scanner *bufio.Scanner, timeout time.Duration) (int, error) {
	type result struct {
		port int
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		for scanner.Scan() {
			line := scanner.Text()
			slog.Debug("[procmgr] agent stdout", "name", name, "line", line)
			if strings.HasPrefix(line, "READY port=") {
				port, err := strconv.Atoi(strings.TrimPrefix(line, "READY port="))
				if err != nil {
					ch <- result{0, fmt.Errorf("procmgr: %s: invalid READY line: %q", name, line)}
					return
				}
				ch <- result{port, nil}
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- result{0, fmt.Errorf("procmgr: %s: stdout read error: %w", name, err)}
		} else {
			ch <- result{0, fmt.Errorf("procmgr: %s: process exited before READY", name)}
		}
	}()

	select {
	case r := <-ch:
		return r.port, r.err
	case <-time.After(timeout):
		return 0, fmt.Errorf("procmgr: %s: timed out waiting for READY (after %s)", name, timeout)
	}
}

// Stop gracefully shuts down the agent: POST /shutdown, then wait, then kill.
func (a *AgentProcess) Stop(timeout time.Duration) {
	if a == nil || a.Cmd == nil || a.Cmd.Process == nil {
		return
	}

	slog.Info("[procmgr] stopping agent", "name", a.Name, "pid", a.Cmd.Process.Pid)

	// Try graceful shutdown via HTTP.
	if a.Port > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		url := fmt.Sprintf("http://127.0.0.1:%d/shutdown", a.Port)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		http.DefaultClient.Do(req)
	}

	// Wait with timeout.
	done := make(chan struct{})
	go func() {
		a.Cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("[procmgr] agent stopped gracefully", "name", a.Name)
	case <-time.After(timeout):
		slog.Warn("[procmgr] agent did not stop in time, killing", "name", a.Name)
		a.Cmd.Process.Kill()
		<-done
	}
}

// Health checks if the agent's HTTP server is responsive.
func (a *AgentProcess) Health() error {
	if a == nil || a.Port == 0 {
		return fmt.Errorf("agent not running")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	url := fmt.Sprintf("http://127.0.0.1:%d/health", a.Port)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned %d", resp.StatusCode)
	}
	return nil
}

// BaseURL returns the HTTP base URL for this agent.
func (a *AgentProcess) BaseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", a.Port)
}

// PID returns the OS process ID, or 0 if not running.
func (a *AgentProcess) PID() int {
	if a != nil && a.Cmd != nil && a.Cmd.Process != nil {
		return a.Cmd.Process.Pid
	}
	return 0
}
