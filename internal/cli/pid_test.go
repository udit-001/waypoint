package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/udit-001/waypoint/internal/config"
)

func withTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cleanup := config.SetConfigDirForTesting(dir)
	t.Cleanup(cleanup)
	return dir
}

func TestWritePidFile(t *testing.T) {
	withTempConfigDir(t)

	if err := writePidFile(8080, 12345); err != nil {
		t.Fatalf("writePidFile() error = %v", err)
	}

	data, err := os.ReadFile(config.PidPath())
	if err != nil {
		t.Fatalf("read PID file: %v", err)
	}

	var info pidInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("unmarshal PID file: %v", err)
	}
	if info.Port != 8080 {
		t.Errorf("Port = %d, want 8080", info.Port)
	}
	if info.PID != 12345 {
		t.Errorf("PID = %d, want 12345", info.PID)
	}
}

func TestReadPidFileJSON(t *testing.T) {
	withTempConfigDir(t)

	// Write a JSON PID file
	data, _ := json.Marshal(pidInfo{Port: 9090, PID: 54321})
	if err := os.WriteFile(config.PidPath(), data, 0644); err != nil {
		t.Fatalf("write PID file: %v", err)
	}

	info, err := readPidFile()
	if err != nil {
		t.Fatalf("readPidFile() error = %v", err)
	}
	if info.Port != 9090 {
		t.Errorf("Port = %d, want 9090", info.Port)
	}
	if info.PID != 54321 {
		t.Errorf("PID = %d, want 54321", info.PID)
	}
}

func TestReadPidFileLegacy(t *testing.T) {
	withTempConfigDir(t)

	// Write a legacy raw-PID file
	if err := os.WriteFile(config.PidPath(), []byte("99999"), 0644); err != nil {
		t.Fatalf("write PID file: %v", err)
	}

	info, err := readPidFile()
	if err != nil {
		t.Fatalf("readPidFile() error = %v", err)
	}
	if info.PID != 99999 {
		t.Errorf("PID = %d, want 99999", info.PID)
	}
	if info.Port != 0 {
		t.Errorf("Port = %d, want 0 (legacy)", info.Port)
	}
}

func TestReadPidFileMissing(t *testing.T) {
	withTempConfigDir(t)

	_, err := readPidFile()
	if err == nil {
		t.Fatal("readPidFile() expected error for missing file, got nil")
	}
}

func TestIsServerRunningTrue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/stats" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	port := portFromAddr(srv.Listener.Addr().String())
	if !isServerRunning(port) {
		t.Error("isServerRunning() = false, want true")
	}
}

func TestIsServerRunningFalse(t *testing.T) {
	// Use a port that's almost certainly not serving anything
	if isServerRunning(1) {
		t.Error("isServerRunning(1) = true, want false")
	}
}

func TestIsServerRunningNonWaypointServer(t *testing.T) {
	// A server that doesn't have /api/stats should return false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	port := portFromAddr(srv.Listener.Addr().String())
	if isServerRunning(port) {
		t.Error("isServerRunning() = true for non-waypoint server, want false")
	}
}

// portFromAddr extracts the port from a "host:port" address.
func portFromAddr(addr string) int {
	// addr is like "127.0.0.1:12345" or "[::1]:12345"
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			n := 0
			for j := i + 1; j < len(addr); j++ {
				n = n*10 + int(addr[j]-'0')
			}
			return n
		}
	}
	return 0
}
