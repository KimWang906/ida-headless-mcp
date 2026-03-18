//go:build windows

package worker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)


// writeFakeWorker writes a minimal Python worker that listens on a TCP loopback port.
func writeFakeWorker(t *testing.T) string {
	t.Helper()
	script := `import argparse, socket, time, signal, sys
parser = argparse.ArgumentParser()
parser.add_argument("--port", required=True, type=int)
parser.add_argument("--binary", required=True)
parser.add_argument("--session-id", required=True)
args = parser.parse_args()
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
sock.bind(('127.0.0.1', args.port))
sock.listen(1)
def handle_signal(signum, frame):
    sys.exit(0)
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

// processAlive reports whether the process with the given PID is still running on Windows.
func processAlive(pid int) bool {
	if pid == 0 {
		return false
	}
	out, err := exec.Command(
		"tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH",
	).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), strconv.Itoa(pid))
}

