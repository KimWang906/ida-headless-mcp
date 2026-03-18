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
	"sync"
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

// findPython returns the first Python executable found on PATH.
func findPython() string {
	for _, name := range []string{"python3", "python", "py"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return "python3" // fallback: let the OS surface the error
}

// Start spawns a Python worker for the session.
// The IPC transport (Unix socket or TCP) is chosen automatically per OS.
func (m *Manager) Start(ctx context.Context, sess *session.Session, binaryPath string) error {
	// Allocate IPC address: Unix socket path on Unix/macOS, TCP loopback on Windows
	addr, err := allocateWorkerAddr(sess.ID)
	if err != nil {
		return fmt.Errorf("failed to allocate worker address: %w", err)
	}
	sess.WorkerAddr = addr

	// Remove any leftover socket file from a previous run (no-op on Windows)
	cleanupWorkerAddr(addr)

	// Workers have an independent lifecycle — they outlive the HTTP request that spawned them
	workerCtx, cancel := context.WithCancel(context.Background())

	cmdArgs := append([]string{m.pythonScript}, workerArgs(addr)...)
	cmdArgs = append(cmdArgs, "--binary", binaryPath, "--session-id", sess.ID)
	cmd := exec.CommandContext(workerCtx, findPython(), cmdArgs...)

	// In tests, discard output to prevent "Test I/O incomplete" errors
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
	m.logger.Printf("[Worker] Started PID %d for session %s (addr: %s)", sess.WorkerPID, sess.ID, addr)

	// Wait for the worker to be ready to accept connections
	if err := waitForWorker(addr, 10*time.Second); err != nil {
		cancel()
		if killErr := cmd.Process.Kill(); killErr != nil {
			m.logger.Printf("[Worker] Failed to kill PID %d: %v", cmd.Process.Pid, killErr)
		}
		if waitErr := cmd.Wait(); waitErr != nil && !errors.Is(waitErr, os.ErrProcessDone) {
			m.logger.Printf("[Worker] Failed to wait for PID %d: %v", cmd.Process.Pid, waitErr)
		}
		return fmt.Errorf("worker not ready: %w", err)
	}

	// Create Connect RPC clients routed through the IPC transport
	httpClient := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return dialWorker(addr)
			},
		},
	}

	baseURL := "http://worker"
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

// Stop terminates the worker for a session
func (m *Manager) Stop(sessionID string) error {
	m.mu.RLock()
	worker, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("no worker for session %s", sessionID)
	}

	m.logger.Printf("[Worker] Stopping session %s PID %d", sessionID, worker.cmd.Process.Pid)

	// Close session gracefully before killing the process
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if worker.SessionCtrl != nil {
		(*worker.SessionCtrl).CloseSession(ctx, connect.NewRequest(&pb.CloseSessionRequest{Save: true}))
	}

	worker.cancel()
	var killErr error
	if worker.cmd.Process != nil {
		killErr = worker.cmd.Process.Kill()
		if killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			m.logger.Printf("[Worker] Failed to kill PID %d: %v", worker.cmd.Process.Pid, killErr)
		}
	}

	// Wait for the process to exit to prevent zombies
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

// GetClient returns the Connect RPC clients for a session
func (m *Manager) GetClient(sessionID string) (*WorkerClient, error) {
	m.mu.RLock()
	worker, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no worker for session %s", sessionID)
	}
	return worker, nil
}
