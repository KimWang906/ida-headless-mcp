//go:build !windows

package worker

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// writeFakeWorker writes a minimal Python worker that listens on a Unix domain socket.
func writeFakeWorker(t *testing.T) string {
	t.Helper()
	script := `#!/usr/bin/env python3
import argparse, os, socket, time, signal, sys
parser = argparse.ArgumentParser()
parser.add_argument("--socket", required=True)
parser.add_argument("--binary", required=True)
parser.add_argument("--session-id", required=True)
args = parser.parse_args()
if os.path.exists(args.socket):
    os.remove(args.socket)
sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
sock.bind(args.socket)
sock.listen(1)
def handle_signal(signum, frame):
    sys.exit(0)
signal.signal(signal.SIGTERM, handle_signal)
signal.signal(signal.SIGINT, handle_signal)
while True:
    try:
        conn, _ = sock.accept()
        conn.close()
    except Exception:
        time.sleep(0.1)
`
	path := filepath.Join(t.TempDir(), "fake_worker.py")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake worker: %v", err)
	}
	return path
}

// processAlive reports whether the process with the given PID is still running.
func processAlive(pid int) bool {
	if pid == 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}

