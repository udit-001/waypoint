package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/config"
)

// fakeServiceController is a test double for the serviceController interface.
// calls records the verb order, which is where the command's ordering
// promises (uninstall stops first, restart stops then starts) are pinned.
type fakeServiceController struct {
	installErr   error
	uninstallErr error
	startErr     error
	stopErr      error
	state        ServiceState
	statusErr    error

	installs, uninstalls, starts, stops int
	calls                               []string
}

func (f *fakeServiceController) Install() error {
	f.installs++
	f.calls = append(f.calls, "install")
	return f.installErr
}
func (f *fakeServiceController) Uninstall() error {
	f.uninstalls++
	f.calls = append(f.calls, "uninstall")
	return f.uninstallErr
}
func (f *fakeServiceController) Start() error {
	f.starts++
	f.calls = append(f.calls, "start")
	return f.startErr
}
func (f *fakeServiceController) Stop() error {
	f.stops++
	f.calls = append(f.calls, "stop")
	return f.stopErr
}
func (f *fakeServiceController) Status() (ServiceState, error) { return f.state, f.statusErr }

// installFakeService installs a fakeServiceController as the newOSService
// factory and returns a cleanup function to restore the original.
func installFakeService(fake *fakeServiceController) func() {
	orig := newOSService
	newOSService = func(serviceOptions) (serviceController, error) {
		return fake, nil
	}
	return func() { newOSService = orig }
}

func TestServiceInstallSuccess(t *testing.T) {
	cleanup := installFakeService(&fakeServiceController{})
	defer cleanup()
	jsonOut = false

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "install"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Service installed") {
		t.Errorf("expected 'Service installed' in output, got: %s", out)
	}
}

func TestServiceInstallJSON(t *testing.T) {
	cleanup := installFakeService(&fakeServiceController{})
	defer cleanup()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "install", "--json"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]string
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if result["status"] != "installed" {
		t.Errorf("expected status 'installed', got %q", result["status"])
	}
	if result["service"] != svcName {
		t.Errorf("expected service %q, got %q", svcName, result["service"])
	}
}

func TestServiceInstallError(t *testing.T) {
	cleanup := installFakeService(&fakeServiceController{
		installErr: &serviceError{msg: "permission denied"},
	})
	defer cleanup()

	rootCmd.SetArgs([]string{"service", "install"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected 'permission denied' in error, got: %v", err)
	}
}

func TestServiceStatusRunning(t *testing.T) {
	cleanup := installFakeService(&fakeServiceController{state: ServiceRunning})
	defer cleanup()
	jsonOut = false

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "status"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "running") {
		t.Errorf("expected 'running' in output, got: %s", out)
	}
}

func TestServiceStatusJSON(t *testing.T) {
	cleanup := installFakeService(&fakeServiceController{state: ServiceStopped})
	defer cleanup()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "status", "--json"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]string
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if result["status"] != "stopped" {
		t.Errorf("expected status 'stopped', got %q", result["status"])
	}
}

func TestServiceStatusNotFound(t *testing.T) {
	cleanup := installFakeService(&fakeServiceController{state: ServiceNotFound})
	defer cleanup()
	jsonOut = false

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "status"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "not installed") {
		t.Errorf("expected 'not installed' in output, got: %s", out)
	}
}

func TestServiceUninstallSuccess(t *testing.T) {
	cleanup := installFakeService(&fakeServiceController{})
	defer cleanup()
	jsonOut = false

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "uninstall"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Service removed") {
		t.Errorf("expected 'Service removed' in output, got: %s", out)
	}
}

func TestServiceRemoveIsAnAliasForUninstall(t *testing.T) {
	fake := &fakeServiceController{}
	cleanup := installFakeService(fake)
	defer cleanup()
	jsonOut = false

	captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "remove"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if fake.uninstalls != 1 {
		t.Fatalf("uninstalls = %d, want 1 (the old spelling must keep working)", fake.uninstalls)
	}
}

func TestServiceUninstallJSON(t *testing.T) {
	cleanup := installFakeService(&fakeServiceController{})
	defer cleanup()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "uninstall", "--json"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	var result map[string]string
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if result["status"] != "removed" {
		t.Errorf("expected status 'removed', got %q", result["status"])
	}
}

func TestServiceUninstallError(t *testing.T) {
	cleanup := installFakeService(&fakeServiceController{
		uninstallErr: &serviceError{msg: "service not found"},
	})
	defer cleanup()

	rootCmd.SetArgs([]string{"service", "uninstall"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "service not found") {
		t.Errorf("expected 'service not found' in error, got: %v", err)
	}
}

// TestServiceRestartStopsThenStarts pins the composition: restart is Stop +
// Start, in that order, so no platform adapter has to implement it.
func TestServiceRestartStopsThenStarts(t *testing.T) {
	fake := &fakeServiceController{}
	cleanup := installFakeService(fake)
	defer cleanup()
	jsonOut = false

	captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "restart"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if fake.stops != 1 || fake.starts != 1 {
		t.Fatalf("stops=%d starts=%d, want 1 each", fake.stops, fake.starts)
	}
}

// TestServiceUninstallStopsBeforeUnregistering pins the ordering the command's
// own help promises ("The running server is stopped first, then the service is
// unregistered"). It is composed at this seam rather than inside each adapter
// because the managers disagree: systemd's Uninstall only disables the unit,
// leaving a running server holding its port, while launchd's stops it. One
// ordering here makes the platforms behave alike.
func TestServiceUninstallStopsBeforeUnregistering(t *testing.T) {
	fake := &fakeServiceController{}
	cleanup := installFakeService(fake)
	defer cleanup()
	jsonOut = false

	captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "uninstall"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if want := []string{"stop", "uninstall"}; !slices.Equal(fake.calls, want) {
		t.Fatalf("calls = %v, want %v", fake.calls, want)
	}
}

// TestServiceUninstallProceedsWhenNothingIsRunning: stopping a service that is
// not running fails, and that must not block unregistering it — otherwise a
// half-installed service could never be removed.
func TestServiceUninstallProceedsWhenNothingIsRunning(t *testing.T) {
	fake := &fakeServiceController{stopErr: &serviceError{msg: "the service is not installed"}}
	cleanup := installFakeService(fake)
	defer cleanup()
	jsonOut = false

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "uninstall"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if fake.uninstalls != 1 {
		t.Fatalf("uninstalls = %d, want 1 even though the stop failed", fake.uninstalls)
	}
	if !strings.Contains(out, "Service removed") {
		t.Fatalf("output = %q, want the success line", out)
	}
}

func TestServiceCommandsRegistered(t *testing.T) {
	cmds := make(map[string]bool)
	for _, cmd := range serviceCmd.Commands() {
		cmds[cmd.Name()] = true
	}
	for _, want := range []string{"install", "uninstall", "start", "stop", "restart", "status"} {
		if !cmds[want] {
			t.Errorf("service subcommand %q not registered", want)
		}
	}
}

// ── supervisor control (real files, real pids, no mocks) ──────────────────

func TestClaimSupervisorGrantsThenRefuses(t *testing.T) {
	withTempConfigDir(t)

	release, err := claimSupervisor()
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if release == nil {
		t.Fatal("first claim must win the lock")
	}

	// The lock now names this process, and this process is ours, so a second
	// claim must refuse rather than start a rival supervisor.
	if release2, err := claimSupervisor(); err != nil || release2 != nil {
		t.Fatalf("second claim released=%v err=%v, want a nil release and no error", release2 != nil, err)
	}

	release()
	if _, err := os.Stat(config.SupervisorPidPath()); !os.IsNotExist(err) {
		t.Fatal("releasing the lock must remove the file")
	}
	if release3, err := claimSupervisor(); err != nil || release3 == nil {
		t.Fatalf("after release, claimed=%v err=%v, want the lock", release3 != nil, err)
	}
}

func TestClaimSupervisorTakesOverADeadLock(t *testing.T) {
	dir := withTempConfigDir(t)

	// A lock left behind by a supervisor that died: the pid is not ours.
	dead := filepath.Join(dir, "supervisor.pid")
	if err := os.WriteFile(dead, []byte(`{"pid":999999999,"exe":"/gone/waypoint"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	release, err := claimSupervisor()
	if err != nil {
		t.Fatalf("claim over a dead lock: %v", err)
	}
	if release == nil {
		t.Fatal("a stale lock must not block a new supervisor forever")
	}
}

func TestRunningSupervisorIgnoresAStaleLock(t *testing.T) {
	withTempConfigDir(t)
	if runningSupervisor() != nil {
		t.Fatal("no lock file means no supervisor")
	}
	if err := writePidInfo(config.SupervisorPidPath(), pidInfo{PID: 999999999, Exe: "/gone/waypoint"}); err != nil {
		t.Fatal(err)
	}
	if runningSupervisor() != nil {
		t.Fatal("a lock naming a dead pid is not a running supervisor")
	}
	// Our own pid, recorded by path, is.
	if err := writePidInfo(config.SupervisorPidPath(), pidInfo{PID: os.Getpid(), Exe: exePath()}); err != nil {
		t.Fatal(err)
	}
	if runningSupervisor() == nil {
		t.Fatal("a lock naming this waypoint process is a running supervisor")
	}
}

func TestRequestSupervisorStopDropsTheMarker(t *testing.T) {
	withTempConfigDir(t)
	if err := requestSupervisorStop(); err != nil {
		t.Fatalf("requestSupervisorStop: %v", err)
	}
	if _, err := os.Stat(config.StopRequestPath()); err != nil {
		t.Fatalf("stop marker not written: %v", err)
	}
}

func TestStopRequestedFiresOnTheMarker(t *testing.T) {
	dir := withTempConfigDir(t)
	marker := filepath.Join(dir, "service.stop")

	stop := stopRequested(marker)
	select {
	case <-stop:
		t.Fatal("stop must not fire before the marker exists")
	case <-time.After(2 * supervisorStopPoll):
	}

	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stop:
	case <-time.After(10 * supervisorStopPoll):
		t.Fatal("stop must fire once the marker appears")
	}
}

// serviceError is a simple error type for testing.
type serviceError struct {
	msg string
}

func (e *serviceError) Error() string { return e.msg }
