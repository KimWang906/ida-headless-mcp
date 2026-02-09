package worker

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/zboralski/ida-headless-mcp/internal/session"
	pb "github.com/zboralski/ida-headless-mcp/ida/worker/v1"
	"github.com/zboralski/ida-headless-mcp/ida/worker/v1/workerconnect"
)

// Manager handles Python worker processes
type Manager struct {
	pythonScript string
	sessions     map[string]*WorkerClient
	logger       *log.Logger
	mu           sync.RWMutex
}

// WorkerClient wraps Connect clients for a session
type WorkerClient struct {
	SessionCtrl *workerconnect.SessionControlClient
	Analysis    *workerconnect.AnalysisToolsClient
	Health      *workerconnect.HealthcheckClient
	cmd         *exec.Cmd
	cancel      context.CancelFunc
	ctx         context.Context
	session     *session.Session
	binaryPath  string
}

// Controller captures the worker operations required by the server.
type Controller interface {
	Start(ctx context.Context, sess *session.Session, binaryPath string) error
	Stop(sessionID string) error
	GetClient(sessionID string) (*WorkerClient, error)
	CleanupOrphanSockets() int
	CleanupOrphanProcesses() int
}

// NewManager creates worker manager
func NewManager(pythonScript string, logger *log.Logger) *Manager {
	return &Manager{
		pythonScript: pythonScript,
		sessions:     make(map[string]*WorkerClient),
		logger:       logger,
	}
}

// Start spawns Python worker for session
func (m *Manager) Start(ctx context.Context, sess *session.Session, binaryPath string) error {
	// Create Unix domain socket
	if err := os.RemoveAll(sess.SocketPath); err != nil {
		return fmt.Errorf("failed to remove old socket: %w", err)
	}

	// Start Python worker with independent lifecycle from HTTP request
	// Workers outlive the request that spawned them
	workerCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(workerCtx, "python3", m.pythonScript,
		"--socket", sess.SocketPath,
		"--binary", binaryPath,
		"--session-id", sess.ID)

	// In tests, discard output to prevent "Test I/O incomplete" errors
	// In production, inherit parent process output
	if flag.Lookup("test.v") != nil {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start worker: %w", err)
	}

	sess.WorkerPID = cmd.Process.Pid
	m.logger.Printf("[Worker] Started PID %d for session %s", sess.WorkerPID, sess.ID)

	// Wait for socket to be ready
	if err := m.waitForSocket(sess.SocketPath, 10*time.Second); err != nil {
		cancel()
		// Kill and wait to avoid zombie process
		if killErr := cmd.Process.Kill(); killErr != nil {
			m.logger.Printf("[Worker] Failed to kill PID %d: %v", cmd.Process.Pid, killErr)
		}
		// Wait for process to exit and be reaped
		if waitErr := cmd.Wait(); waitErr != nil && !errors.Is(waitErr, os.ErrProcessDone) {
			m.logger.Printf("[Worker] Failed to wait for PID %d: %v", cmd.Process.Pid, waitErr)
		}
		return fmt.Errorf("worker socket not ready: %w", err)
	}

	// Create Connect clients over Unix socket
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", sess.SocketPath)
			},
		},
	}

	baseURL := "http://unix"
	sessionClient := workerconnect.NewSessionControlClient(httpClient, baseURL)
	analysisClient := workerconnect.NewAnalysisToolsClient(httpClient, baseURL)
	healthClient := workerconnect.NewHealthcheckClient(httpClient, baseURL)

	worker := &WorkerClient{
		SessionCtrl: &sessionClient,
		Analysis:    &analysisClient,
		Health:      &healthClient,
		cmd:         cmd,
		cancel:      cancel,
		ctx:         workerCtx,
		session:     sess,
		binaryPath:  binaryPath,
	}

	m.mu.Lock()
	m.sessions[sess.ID] = worker
	m.mu.Unlock()

	go m.monitorWorker(sess.ID, worker)

	return nil
}

func (m *Manager) monitorWorker(sessionID string, worker *WorkerClient) {
	err := worker.cmd.Wait()
	if err != nil && worker.ctx.Err() == nil {
		m.logger.Printf("[Worker] Process %d exited with error for session %s: %v", worker.session.WorkerPID, sessionID, err)
	} else {
		m.logger.Printf("[Worker] Process %d exited for session %s", worker.session.WorkerPID, sessionID)
	}

	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
}

// Stop terminates worker for session
func (m *Manager) Stop(sessionID string) error {
	m.mu.RLock()
	worker, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no worker for session %s", sessionID)
	}

	m.logger.Printf("[Worker] Stopping session %s PID %d", sessionID, worker.cmd.Process.Pid)

	// Close session gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if worker.SessionCtrl != nil {
		(*worker.SessionCtrl).CloseSession(ctx, connect.NewRequest(&pb.CloseSessionRequest{Save: true}))
	}

	// Cancel context and kill process
	worker.cancel()
	var killErr error
	if worker.cmd.Process != nil {
		killErr = worker.cmd.Process.Kill()
		if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			m.logger.Printf("[Worker] Failed to kill PID %d: %v", worker.cmd.Process.Pid, killErr)
		}
	}

	// Wait for process to exit and be reaped - prevent zombie
	// The monitorWorker goroutine will also call Wait(), but that's safe
	// (subsequent Wait() calls return the cached result)
	if waitErr := worker.cmd.Wait(); waitErr != nil && !errors.Is(waitErr, os.ErrProcessDone) {
		m.logger.Printf("[Worker] Process %d wait error: %v", worker.cmd.Process.Pid, waitErr)
	}

	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()

	if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
		return fmt.Errorf("failed to kill worker: %w", killErr)
	}
	return nil
}

// GetClient returns Connect clients for session
func (m *Manager) GetClient(sessionID string) (*WorkerClient, error) {
	m.mu.RLock()
	worker, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no worker for session %s", sessionID)
	}
	return worker, nil
}

// CleanupOrphanSockets removes stale /tmp/ida-worker-*.sock files
// that may have been left behind by previous server crashes.
// It should be called before RestoreSessions so that fresh sockets
// are created for each restored session.
func (m *Manager) CleanupOrphanSockets() int {
	matches, err := filepath.Glob("/tmp/ida-worker-*.sock")
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

// CleanupOrphanProcesses finds and kills orphaned Python worker processes
// that may still be running from a previous server instance.
func (m *Manager) CleanupOrphanProcesses() int {
	// Find processes whose command line matches our worker pattern
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
		if err != nil {
			continue
		}
		if pid == myPID {
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
		if !strings.Contains(cmdStr, "--socket") {
			continue
		}

		proc, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		m.logger.Printf("[Worker] Killing orphan worker process PID %d", pid)
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			// Process may have already exited
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

// waitForSocket polls until socket exists
func (m *Manager) waitForSocket(socketPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			// Try to connect
			conn, err := net.Dial("unix", socketPath)
			if err == nil {
				conn.Close()
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for socket %s", socketPath)
}
