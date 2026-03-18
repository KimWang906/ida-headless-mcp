//go:build windows

package main

import (
	"os"
	"os/signal"
)

// notifyShutdown registers OS signals that trigger graceful shutdown.
// On Windows only SIGINT (Ctrl-C) is reliably deliverable.
func notifyShutdown(ch chan os.Signal) {
	signal.Notify(ch, os.Interrupt)
}
