//go:build windows

package cli

import (
	"os/exec"
	"strconv"
)

func killProcess(pid int) error {
	c := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
	_ = c.Run()
	// taskkill exits with 128 when process is not found — already stopped
	return nil
}
