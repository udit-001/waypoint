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
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// ESRCH means process already exited — treat as success
	}
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
