//go:build !windows

package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

// A stale or reused PID file must never let `waypoint stop` signal an
// arbitrary process: on unix os.FindProcess succeeds for any pid, so stop
// verifies the pid actually belongs to a waypoint process first. These tests
// live at the probe seam (probeIdentity + decideStop) because that is where
// the policy is.

func TestProbeIdentityForeignProcess(t *testing.T) {
	// A live process that is not waypoint: with a lifetime bound so a test
	// failure cannot leak a stray process.
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("spawn sleep: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	info := &pidInfo{Port: 0, PID: cmd.Process.Pid}
	outcome := decideStop(info, probeIdentity(info))
	if outcome != stopForeignPID {
		t.Fatalf("outcome = %v, want stopForeignPID", outcome)
	}
	// The foreign process must have survived: the probe never signals.
	if err := syscall.Kill(cmd.Process.Pid, syscall.Signal(0)); err != nil {
		t.Fatalf("sleep must survive the probe: %v", err)
	}
}

func TestProbeIdentityTestBinaryIsForeign(t *testing.T) {
	// No recorded exe path: the test binary is not waypoint-named, so it must
	// be refused. This is the legacy/renamed-binary fallback.
	info := &pidInfo{Port: 0, PID: os.Getpid()}
	outcome := decideStop(info, probeIdentity(info))
	if outcome != stopForeignPID {
		t.Fatalf("outcome = %v, want stopForeignPID (test binary is not waypoint)", outcome)
	}
}

func TestProbeIdentityRecordedExeIsOurs(t *testing.T) {
	// A PID file that records this binary's own path must be treated as ours
	// regardless of its name — the identity is the path, not the basename.
	info := &pidInfo{Port: 0, PID: os.Getpid(), Exe: exePath()}
	outcome := decideStop(info, probeIdentity(info))
	if outcome != stopStopped {
		t.Fatalf("outcome = %v, want stopStopped (recorded exe matches)", outcome)
	}
}

func TestProbeIdentityWrongExeIsForeign(t *testing.T) {
	info := &pidInfo{Port: 0, PID: os.Getpid(), Exe: filepath.Join(t.TempDir(), "waypoint")}
	if outcome := decideStop(info, probeIdentity(info)); outcome != stopForeignPID {
		t.Fatalf("outcome = %v, want stopForeignPID (recorded exe does not match)", outcome)
	}
}

func TestProbeIdentityDeadPID(t *testing.T) {
	info := &pidInfo{Port: 0, PID: 999999999}
	if outcome := decideStop(info, probeIdentity(info)); outcome != stopStalePID {
		t.Fatalf("outcome = %v, want stopStalePID", outcome)
	}
}

func TestProcessNameProbes(t *testing.T) {
	if processName(os.Getpid()) == "" {
		t.Fatal("processName must resolve the test process")
	}
	// Test binaries are "cli.test"-shaped, never bare "waypoint".
	if processIsWaypoint(os.Getpid(), "") {
		t.Fatal("test binary must not count as waypoint by name")
	}
	if processName(999999999) != "" {
		t.Fatal("impossible pid must not resolve")
	}
	if processIsWaypoint(999999999, exePath()) {
		t.Fatal("impossible pid must not count as waypoint")
	}
}

func TestProcessExeMatchesSelf(t *testing.T) {
	// The recorded path and the kernel's view of a running process must agree
	// — this is the end-to-end evidence the whole identity rule rests on.
	if !samePath(processExe(os.Getpid()), exePath()) {
		t.Fatalf("processExe(self) = %q, exePath() = %q", processExe(os.Getpid()), exePath())
	}
}

func TestSamePathToleratesDeletedSuffix(t *testing.T) {
	// /proc/<pid>/exe reports "<path> (deleted)" after the binary is replaced
	// under a running process (the state right after `waypoint upgrade`).
	dir := t.TempDir()
	bin := filepath.Join(dir, "waypoint")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !samePath(bin+" (deleted)", bin) {
		t.Fatal("samePath must ignore the kernel's (deleted) suffix")
	}
	if samePath(bin, filepath.Join(dir, "other")) {
		t.Fatal("samePath must reject different paths")
	}
}
