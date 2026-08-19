package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// fakeServiceController is a test double for the serviceController interface.
type fakeServiceController struct {
	installErr   error
	statusResult string
	statusErr    error
	removeErr    error
}

func (f *fakeServiceController) Install() error          { return f.installErr }
func (f *fakeServiceController) Status() (string, error) { return f.statusResult, f.statusErr }
func (f *fakeServiceController) Remove() error           { return f.removeErr }

// installFakeService installs a fakeServiceController as the newOSService
// factory and returns a cleanup function to restore the original.
func installFakeService(fake *fakeServiceController) func() {
	orig := newOSService
	newOSService = func(name, displayName, description string, args []string) (serviceController, error) {
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
	cleanup := installFakeService(&fakeServiceController{
		statusResult: "running",
	})
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
	cleanup := installFakeService(&fakeServiceController{
		statusResult: "stopped",
	})
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
	cleanup := installFakeService(&fakeServiceController{
		statusResult: "not found",
	})
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

func TestServiceRemoveSuccess(t *testing.T) {
	cleanup := installFakeService(&fakeServiceController{})
	defer cleanup()
	jsonOut = false

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "remove"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, "Service removed") {
		t.Errorf("expected 'Service removed' in output, got: %s", out)
	}
}

func TestServiceRemoveJSON(t *testing.T) {
	cleanup := installFakeService(&fakeServiceController{})
	defer cleanup()

	out := captureStdout(t, func() {
		rootCmd.SetArgs([]string{"service", "remove", "--json"})
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

func TestServiceRemoveError(t *testing.T) {
	cleanup := installFakeService(&fakeServiceController{
		removeErr: &serviceError{msg: "service not found"},
	})
	defer cleanup()

	rootCmd.SetArgs([]string{"service", "remove"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "service not found") {
		t.Errorf("expected 'service not found' in error, got: %v", err)
	}
}

func TestServiceCommandsRegistered(t *testing.T) {
	cleanup := installFakeService(&fakeServiceController{})
	defer cleanup()

	// Verify all three subcommands are registered
	cmds := make(map[string]bool)
	for _, cmd := range serviceCmd.Commands() {
		cmds[cmd.Name()] = true
	}
	for _, want := range []string{"install", "status", "remove"} {
		if !cmds[want] {
			t.Errorf("service subcommand %q not registered", want)
		}
	}
}

// serviceError is a simple error type for testing.
type serviceError struct {
	msg string
}

func (e *serviceError) Error() string { return e.msg }
