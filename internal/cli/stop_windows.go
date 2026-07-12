//go:build windows

package cli

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
)

func killProcess(pid int) error {
	c := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
	_ = c.Run()
	// taskkill exits with 128 when process is not found — already stopped
	return nil
}

// processAlive returns true if the process is still running.
func processAlive(pid int) bool {
	_, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	c := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH")
	out, _ := c.Output()
	return len(out) > 0 && !bytes.Contains(out, []byte("INFO:"))
}
