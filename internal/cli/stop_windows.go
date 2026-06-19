//go:build windows

package cli

import (
	"fmt"
	"os/exec"
	"strconv"
)

func killProcess(pid int) error {
	c := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
	out, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop server: %w\n%s", err, string(out))
	}
	return nil
}
