//go:build windows

package worker

import (
	"fmt"
	"net"
	"time"
)

// allocateWorkerAddr finds a free TCP loopback port and returns "127.0.0.1:PORT".
func allocateWorkerAddr(_ string) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("failed to allocate worker port: %w", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr, nil
}

// dialWorker connects to the worker over TCP.
func dialWorker(addr string) (net.Conn, error) {
	return net.Dial("tcp", addr)
}

// cleanupWorkerAddr is a no-op on Windows (TCP transport creates no socket files).
func cleanupWorkerAddr(_ string) {}

// workerArgs returns the CLI arguments for the Python worker to listen on this address.
// On Windows the address is "127.0.0.1:PORT", so we pass --port PORT.
func workerArgs(addr string) []string {
	_, port, _ := net.SplitHostPort(addr)
	return []string{"--port", port}
}

// waitForWorker polls until the TCP worker is ready to accept connections.
func waitForWorker(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", addr)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for worker at %s", addr)
}
