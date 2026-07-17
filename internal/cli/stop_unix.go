//go:build !windows

package cli

import (
	"fmt"
	"os"
	"syscall"
)

func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("no process with PID %d found", pid)
	}
	// SIGINT allows graceful shutdown — the server closes the database
	// and releases the port cleanly. The server handles both SIGINT and
	// SIGTERM identically (see server.go signal.Notify), but SIGINT is
	// the convention for "interrupt" (what Ctrl+C sends).
	// ESRCH (process already exited) and any other signal error are
	// treated as success — the process is gone either way.
	_ = proc.Signal(syscall.SIGINT)
	return nil
}

// processAlive returns true if the process is still running.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Send signal 0 (existence check — doesn't actually signal the process)
	return proc.Signal(syscall.Signal(0)) == nil
}
