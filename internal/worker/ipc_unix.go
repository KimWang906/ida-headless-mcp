//go:build !windows

package worker

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// allocateWorkerAddr returns a Unix domain socket path for the given session ID.
func allocateWorkerAddr(id string) (string, error) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("ida-worker-%s.sock", id))
	return path, nil
}

// dialWorker connects to the worker over a Unix domain socket.
func dialWorker(addr string) (net.Conn, error) {
	return net.Dial("unix", addr)
}

// cleanupWorkerAddr removes a stale Unix socket file.
func cleanupWorkerAddr(addr string) {
	if addr != "" {
		os.Remove(addr)
	}
}

// workerArgs returns the CLI arguments for the Python worker to listen on this address.
func workerArgs(addr string) []string {
	return []string{"--socket", addr}
}

// waitForWorker polls until the Unix domain socket is ready to accept connections.
func waitForWorker(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(addr); err == nil {
			conn, err := net.Dial("unix", addr)
			if err == nil {
				conn.Close()
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for worker socket %s", addr)
}
