package supervise

import (
	"bytes"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// processAlive is a best-effort liveness probe used to assert that a child
// really is gone. Signal 0 is the portable existence check; on Windows it
// always errors, but these tests skip there anyway.
func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// The supervisor's whole value is the restart policy, so these tests drive it
// with real short-lived processes through the Command seam — no mocks of the
// loop itself.

func requireShell(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("tests drive the supervisor with sh")
	}
}

func sh(script string) func() (*exec.Cmd, error) {
	return func() (*exec.Cmd, error) { return exec.Command("sh", "-c", script), nil }
}

func TestBudgetAllowsLimitPerWindow(t *testing.T) {
	base := time.Now()
	b := NewBudget(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !b.Allow(base) {
			t.Fatalf("start %d should be allowed", i+1)
		}
	}
	if b.Allow(base) {
		t.Fatal("4th start inside the window must be refused")
	}
	// The oldest start ages out, so one slot frees up.
	if !b.Allow(base.Add(time.Minute)) {
		t.Fatal("start after the window must be allowed")
	}
}

func TestBudgetNilIsUnlimited(t *testing.T) {
	var b *Budget
	for i := 0; i < 100; i++ {
		if !b.Allow(time.Now()) {
			t.Fatal("nil budget must allow every start")
		}
	}
}

func TestRunExitsCleanlyWhenServerDoes(t *testing.T) {
	requireShell(t)
	var log bytes.Buffer
	out, err := Run(Options{
		Command: sh("exit 0"),
		Log:     &log,
		Budget:  NewBudget(3, time.Minute),
		Delay:   time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != OutcomeExitedCleanly {
		t.Fatalf("outcome = %v, want OutcomeExitedCleanly", out)
	}
	if !strings.Contains(log.String(), "exited cleanly") {
		t.Fatalf("log = %q, want a clean-exit line", log.String())
	}
}

func TestRunGivesUpAfterRepeatedFailures(t *testing.T) {
	requireShell(t)
	var log bytes.Buffer
	out, err := Run(Options{
		Command: sh("exit 3"),
		Log:     &log,
		Budget:  NewBudget(3, time.Minute),
		Delay:   time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != OutcomeGaveUp {
		t.Fatalf("outcome = %v, want OutcomeGaveUp", out)
	}
	if got := strings.Count(log.String(), "server started"); got != 3 {
		t.Fatalf("started %d times, want 3 (the budget):\n%s", got, log.String())
	}
	if !strings.Contains(log.String(), "giving up") {
		t.Fatalf("log must say it gave up:\n%s", log.String())
	}
}

func TestRunStopsARunningServer(t *testing.T) {
	requireShell(t)
	stop := make(chan struct{})
	var log bytes.Buffer
	started := make(chan int, 1)

	done := make(chan Outcome, 1)
	go func() {
		out, _ := Run(Options{
			Command:  sh("sleep 30"),
			Log:      &log,
			Budget:   NewBudget(3, time.Minute),
			Delay:    time.Millisecond,
			Graceful: 50 * time.Millisecond,
			Stop:     stop,
			OnStart:  func(pid int) { started <- pid },
		})
		done <- out
	}()

	pid := <-started
	close(stop)

	select {
	case out := <-done:
		if out != OutcomeStopped {
			t.Fatalf("outcome = %v, want OutcomeStopped", out)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("supervisor did not stop")
	}
	if processAlive(pid) {
		t.Fatalf("server pid %d survived the stop", pid)
	}
	if !strings.Contains(log.String(), "stopped") {
		t.Fatalf("log = %q, want a stop line", log.String())
	}
}

func TestRunStopBeforeStartEndsImmediately(t *testing.T) {
	requireShell(t)
	stop := make(chan struct{})
	close(stop)

	spawned := false
	out, err := Run(Options{
		Command: func() (*exec.Cmd, error) {
			spawned = true
			return exec.Command("sh", "-c", "exit 1"), nil
		},
		Budget: NewBudget(3, time.Minute),
		Delay:  10 * time.Millisecond,
		Stop:   stop,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The first spawn still happens (the budget is checked first), but the
	// stop must end the run rather than restarting.
	if out != OutcomeStopped {
		t.Fatalf("outcome = %v, want OutcomeStopped", out)
	}
	if !spawned {
		t.Fatal("expected one spawn before the stop was noticed")
	}
}

func TestRunRefusesWhenAnotherSupervisorHoldsTheLock(t *testing.T) {
	requireShell(t)
	out, err := Run(Options{
		Command: sh("exit 0"),
		Claim:   func() (func(), error) { return nil, nil }, // nil release = held
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != OutcomeAlreadyRunning {
		t.Fatalf("outcome = %v, want OutcomeAlreadyRunning", out)
	}
}

func TestRunReleasesTheLockOnExit(t *testing.T) {
	requireShell(t)
	released := false
	if _, err := Run(Options{
		Command: sh("exit 0"),
		Budget:  NewBudget(1, time.Minute),
		Claim:   func() (func(), error) { return func() { released = true }, nil },
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !released {
		t.Fatal("the instance lock must be released when the run ends")
	}
}

func TestRunReportsBuildFailure(t *testing.T) {
	requireShell(t)
	var log bytes.Buffer
	out, err := Run(Options{
		Command: func() (*exec.Cmd, error) { return nil, exec.ErrNotFound },
		Log:     &log,
		Budget:  NewBudget(2, time.Minute),
		Delay:   time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != OutcomeGaveUp {
		t.Fatalf("outcome = %v, want OutcomeGaveUp after the budget ran out", out)
	}
	if !strings.Contains(log.String(), "cannot build the server command") {
		t.Fatalf("log must name the build failure:\n%s", log.String())
	}
}

func TestRunReportsStartFailure(t *testing.T) {
	requireShell(t)
	var log bytes.Buffer
	out, err := Run(Options{
		Command: func() (*exec.Cmd, error) { return exec.Command("/nonexistent/waypoint-server"), nil },
		Log:     &log,
		Budget:  NewBudget(2, time.Minute),
		Delay:   time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out != OutcomeGaveUp {
		t.Fatalf("outcome = %v, want OutcomeGaveUp", out)
	}
	if !strings.Contains(log.String(), "cannot start the server") {
		t.Fatalf("log must name the start failure:\n%s", log.String())
	}
}

func TestRunRequiresACommand(t *testing.T) {
	if _, err := Run(Options{}); err != ErrNoCommand {
		t.Fatalf("err = %v, want ErrNoCommand", err)
	}
}

func TestRunCallsHooksPerChild(t *testing.T) {
	requireShell(t)
	starts, exits := 0, 0
	if _, err := Run(Options{
		Command:  sh("exit 1"),
		Budget:   NewBudget(2, time.Minute),
		Delay:    time.Millisecond,
		OnStart:  func(int) { starts++ },
		OnExit:   func() { exits++ },
		Graceful: time.Millisecond,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if starts != 2 || exits != 2 {
		t.Fatalf("starts=%d exits=%d, want both 2", starts, exits)
	}
}
