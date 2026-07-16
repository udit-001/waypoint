package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/udit-001/waypoint/internal/config"
)

// pidInfo is the JSON structure of the PID file. Carrying the port
// alongside the PID lets callers health-check the server without
// guessing the port — essential for "already running" detection and
// for upgrade restarts on the correct port.
type pidInfo struct {
	Port int `json:"port"`
	PID  int `json:"pid"`
}

// readPidFile reads and parses the PID file. Handles both the current
// JSON format ({"port":...,"pid":...}) and the legacy raw-PID format
// (a bare integer). Legacy files return pidInfo with Port=0, so
// callers can skip port-based health checks.
func readPidFile() (*pidInfo, error) {
	data, err := os.ReadFile(config.PidPath())
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

// writePidFile writes the PID file in JSON format.
func writePidFile(port, pid int) error {
	data, err := json.Marshal(pidInfo{Port: port, PID: pid})
	if err != nil {
		return err
	}
	return os.WriteFile(config.PidPath(), data, 0644)
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
