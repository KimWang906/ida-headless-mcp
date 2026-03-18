//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// notifyShutdown registers OS signals that trigger graceful shutdown.
// On Unix/macOS both SIGINT (Ctrl-C) and SIGTERM (kill / systemd stop) are handled.
func notifyShutdown(ch chan os.Signal) {
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
}
