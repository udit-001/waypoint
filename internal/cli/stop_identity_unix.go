//go:build !windows

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// processExe returns the resolved executable path of pid, or "" when it
// cannot be determined. Linux reads /proc/<pid>/exe; macOS has no /proc, but
// `ps -o comm=` prints the full path there.
//
// The kernel appends " (deleted)" to /proc/<pid>/exe when the binary was
// replaced under a running process — the normal state right after
// `waypoint upgrade` and after `go install` rewrites the binary in place.
func processExe(pid int) string {
	if exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid)); err == nil {
		return strings.TrimSuffix(exe, " (deleted)")
	}
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
	if line == "" || !strings.ContainsRune(line, os.PathSeparator) {
		return ""
	}
	return line
}

// processName best-effort resolves the executable name behind a pid: the base
// of the resolved path, then /proc/<pid>/comm. Empty means the name could not
// be determined.
func processName(pid int) string {
	if exe := processExe(pid); exe != "" {
		return filepath.Base(exe)
	}
	if comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)); err == nil {
		return strings.TrimSpace(string(comm))
	}
	return ""
}

// samePath compares two executable paths after resolving symlinks, so a
// binary reached through a shim (Homebrew, /usr/local/bin) still matches the
// real path recorded at start. The kernel's " (deleted)" suffix is ignored
// here as well as in processExe, so either source can supply the path.
func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	norm := func(p string) string {
		p = strings.TrimSuffix(p, " (deleted)")
		if rp, err := filepath.EvalSymlinks(p); err == nil {
			p = rp
		}
		return filepath.Clean(p)
	}
	return norm(a) == norm(b)
}

// processIsWaypoint reports whether the pid belongs to the executable the PID
// file recorded, or — for a PID file written before the exe field existed —
// to a waypoint-shaped binary name. An undeterminable name counts as NOT
// waypoint: stop refuses to signal what it cannot verify. Accepted cost — on a
// host where /proc is masked, `waypoint stop` refuses loudly instead of
// killing blind, and the message names the port so the server can be found.
func processIsWaypoint(pid int, recordedExe string) bool {
	if samePath(processExe(pid), recordedExe) {
		return true
	}
	name := processName(pid)
	return name == "waypoint" ||
		name == "waypoint.exe" ||
		strings.HasPrefix(name, "waypoint.") // waypoint.test, dev builds
}

// probeIdentity is the unix adapter. The executable probe is authoritative,
// so it decides alone (see decideStop); port health is gathered for the
// legacy/unverifiable path only and never overrides a resolved identity.
func probeIdentity(info *pidInfo) identityProbe {
	if info == nil || info.PID <= 0 {
		return identityProbe{}
	}
	return identityProbe{
		Alive:       processAlive(info.PID),
		Ours:        processIsWaypoint(info.PID, info.Exe),
		Known:       true,
		PortHealthy: info.Port > 0 && isServerRunning(info.Port),
	}
}

// pidIsOurs reports whether a pid recorded in a Waypoint lock file still
// belongs to this installation. It is the unix answer to "is that supervisor
// mine?", and the executable probe answers it outright.
func pidIsOurs(pid int, recordedExe string) bool {
	return processIsWaypoint(pid, recordedExe)
}
