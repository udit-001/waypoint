package cli

import (
	"slices"
	"testing"
)

// The daemon command line is one fact with three callers: the service manager
// (systemd/launchd run it), the supervisor (it is the supervisor's child), and
// `start --background` (it is what the detached child runs). If they drift, a
// flag added in one place silently does not apply in the other two.
func TestDaemonFlagsAreOneSpelling(t *testing.T) {
	if got, want := daemonFlags(0), []string{"start", "--no-open", "--daemon"}; !slices.Equal(got, want) {
		t.Fatalf("daemonFlags(0) = %v, want %v", got, want)
	}
	if got, want := daemonFlags(8443), []string{"start", "--port", "8443", "--no-open", "--daemon"}; !slices.Equal(got, want) {
		t.Fatalf("daemonFlags(8443) = %v, want %v", got, want)
	}
}

// A pinned port would go stale the moment the config changes, so the entries
// that outlive a run — the service unit and the login entry — must not carry
// one; only `start --background`, which was asked for a specific port, does.
func TestServiceManagerRunsTheUnpinnedDaemon(t *testing.T) {
	if !slices.Equal(svcRunCmd, daemonFlags(0)) {
		t.Fatalf("svcRunCmd = %v, want the unpinned daemon flags %v", svcRunCmd, daemonFlags(0))
	}
	if slices.Contains(svcRunCmd, "--port") {
		t.Fatalf("svcRunCmd = %v, must not pin a port", svcRunCmd)
	}
}
