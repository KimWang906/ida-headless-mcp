//go:build windows

package worker

import (
	"os/exec"
	"strconv"
	"strings"
)

// CleanupOrphanSockets is a no-op on Windows (TCP transport creates no socket files).
func (m *Manager) CleanupOrphanSockets() int {
	return 0
}

// CleanupOrphanProcesses finds and terminates orphaned Python worker processes on Windows
// by querying the process list via wmic.
func (m *Manager) CleanupOrphanProcesses() int {
	out, err := exec.Command(
		"wmic", "process",
		"where", "name='python.exe' or name='python3.exe'",
		"get", "ProcessId,CommandLine",
		"/format:csv",
	).Output()
	if err != nil {
		return 0
	}

	killed := 0
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Only match our worker processes (identified by --port argument)
		if !strings.Contains(line, "--port") {
			continue
		}
		if !strings.Contains(line, "ida-worker") && !strings.Contains(line, m.pythonScript) {
			continue
		}
		// CSV columns: Node,CommandLine,ProcessId — PID is the last field
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		pidStr := strings.TrimSpace(parts[len(parts)-1])
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		m.logger.Printf("[Worker] Killing orphan worker process PID %d", pid)
		if err := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run(); err != nil {
			m.logger.Printf("[Worker] Failed to kill PID %d: %v", pid, err)
		} else {
			killed++
		}
	}
	if killed > 0 {
		m.logger.Printf("[Worker] Killed %d orphan worker process(es)", killed)
	}
	return killed
}
