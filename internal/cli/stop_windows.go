//go:build windows

package cli

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// killProcess force-kills the server process and any children it spawned.
// Go's os.Process.Signal can only deliver os.Kill on Windows — SIGINT cannot
// cross a process boundary — so taskkill does the work. /T is required: the
// daemon must never leave orphans behind, and a partially killed server
// would keep holding the port.
//
// The error is returned rather than swallowed: the caller re-checks
// liveness and treats "exited between the probe and the kill" as success,
// while a survivor is reported instead of silently ignored.
func killProcess(pid int) error {
	c := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	var stderr bytes.Buffer
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return fmt.Errorf("taskkill %d: %w", pid, err)
		}
		return fmt.Errorf("taskkill %d: %w (%s)", pid, err, msg)
	}
	return nil
}

// processAlive reports whether pid is running, by reading the PID column of
// tasklist's CSV output.
//
// The obvious probe — searching the output for "INFO:" — is wrong on any
// non-English Windows, where that message is localized and a dead pid would
// read as alive. Parsing the column instead means a "no match" line simply
// fails to parse, in every locale. This matters more than it looks: the
// supervisor's lock file and every service verb decide "is it up?" here.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH").Output()
	if err != nil {
		return false
	}
	want := strconv.Itoa(pid)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Split(strings.TrimSpace(line), ",")
		if len(fields) < 2 {
			continue
		}
		if strings.Trim(fields[1], `"`) == want {
			return true
		}
	}
	return false
}
