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
	return proc.Signal(syscall.SIGTERM)
}
