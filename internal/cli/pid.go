package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/udit-001/waypoint/internal/config"
)

// pidInfo is the JSON structure of the PID file. It carries the three facts
// a caller needs to act on the file safely: which port to health-check, which
// process to signal, and which executable that process should be. The exe
// path is what makes `waypoint stop` safe against a reused PID even when the
// binary was renamed — see stop_identity_unix.go.
type pidInfo struct {
	Port int    `json:"port"`
	PID  int    `json:"pid"`
	Exe  string `json:"exe,omitempty"`
}

// readPidFile reads and parses the PID file. Handles both the current
// JSON format ({"port":...,"pid":...}) and the legacy raw-PID format
// (a bare integer). Legacy files return pidInfo with Port=0, so
// callers can skip port-based health checks.
func readPidFile() (*pidInfo, error) {
	return readPidInfo(config.PidPath())
}

// readPidInfo parses a PID file at an arbitrary path — the server's or the
// supervisor's. Missing files and unparseable content are both errors: the
// caller decides whether that means "nothing running" or "corrupt state".
func readPidInfo(path string) (*pidInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var info pidInfo
	if err := json.Unmarshal(data, &info); err == nil {
		return &info, nil
	}
	// Legacy format: raw PID text
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, err
	}
	return &pidInfo{PID: pid}, nil
}

// writePidFile writes the PID file in JSON format, recording the executable
// that owns the server so a later `waypoint stop` can verify identity by path
// rather than by guessing from a process name.
func writePidFile(port, pid int) error {
	return writePidInfo(config.PidPath(), pidInfo{Port: port, PID: pid, Exe: exePath()})
}

// writePidInfo writes a PID file at an arbitrary path (server or supervisor).
func writePidInfo(path string, info pidInfo) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// exePath resolves the current executable with symlinks resolved, matching
// what /proc/<pid>/exe reports for this process. Empty when it cannot be
// resolved — callers then fall back to name matching.
func exePath() string {
	p, err := os.Executable()
	if err != nil {
		return ""
	}
	if rp, err := filepath.EvalSymlinks(p); err == nil {
		return rp
	}
	return p
}

// isServerRunning returns true if a waypoint server responds on the given
// port. The health check hits GET /api/stats — a waypoint-specific endpoint
// that only a waypoint server would respond to, making the check precise
// enough to distinguish our server from any other HTTP server on the port.
func isServerRunning(port int) bool {
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/stats", port))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
