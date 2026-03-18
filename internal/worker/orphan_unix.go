//go:build !windows

package worker

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// CleanupOrphanSockets removes stale Unix domain socket files left by crashed server instances.
func (m *Manager) CleanupOrphanSockets() int {
	pattern := filepath.Join(os.TempDir(), "ida-worker-*.sock")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		m.logger.Printf("[Worker] Failed to glob orphan sockets: %v", err)
		return 0
	}

	removed := 0
	for _, sock := range matches {
		if err := os.Remove(sock); err != nil {
			m.logger.Printf("[Worker] Failed to remove orphan socket %s: %v", sock, err)
		} else {
			removed++
		}
	}
	if removed > 0 {
		m.logger.Printf("[Worker] Cleaned up %d orphan socket(s)", removed)
	}
	return removed
}

// CleanupOrphanProcesses finds and kills orphaned Python worker processes via /proc.
func (m *Manager) CleanupOrphanProcesses() int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		// Not on Linux or /proc not available — skip silently
		return 0
	}

	killed := 0
	myPID := os.Getpid()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == myPID {
			continue
		}

		cmdline, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}

		// cmdline uses NUL separators; check if it contains our worker markers
		cmdStr := string(cmdline)
		if !strings.Contains(cmdStr, "ida-worker") && !strings.Contains(cmdStr, m.pythonScript) {
			continue
		}
		// Unix workers are identified by --socket argument
		if !strings.Contains(cmdStr, "--socket") {
			continue
		}

		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		m.logger.Printf("[Worker] Killing orphan worker process PID %d", pid)
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			m.logger.Printf("[Worker] Failed to SIGTERM PID %d: %v", pid, err)
		} else {
			killed++
		}
	}
	if killed > 0 {
		m.logger.Printf("[Worker] Killed %d orphan worker process(es)", killed)
	}
	return killed
}
