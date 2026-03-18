package worker

import (
	"context"
	"io"
	"log"
	"testing"
	"time"

	"github.com/zboralski/ida-headless-mcp/internal/session"
)

func TestManagerWorkerHasIndependentLifecycle(t *testing.T) {
	scriptPath := writeFakeWorker(t)
	logger := log.New(io.Discard, "", 0)
	mgr := NewManager(scriptPath, logger)

	sess := &session.Session{
		ID: "test-session",
	}

	// Workers have independent lifecycle — they survive request cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // simulate request finishing immediately

	if err := mgr.Start(ctx, sess, "/bin/ls"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	t.Cleanup(func() {
		_ = mgr.Stop(sess.ID)
	})

	// Worker should still be running despite the cancelled request context
	time.Sleep(200 * time.Millisecond)
	if !processAlive(sess.WorkerPID) {
		t.Fatalf("worker process %d exited after parent context cancelled", sess.WorkerPID)
	}

	// Should be able to fetch the client
	if _, err := mgr.GetClient(sess.ID); err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}
}
