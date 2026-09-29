package cli

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/supervise"
)

// The supervisor's policy. These are the numbers a user would otherwise have
// to discover by watching a flapping background process.
const (
	// supervisorRestartLimit/window is the crash-loop guard: five starts per
	// minute, then give up and say so in the log.
	supervisorRestartLimit  = 5
	supervisorRestartWindow = time.Minute
	// supervisorRestartDelay is the pause before a restart.
	supervisorRestartDelay = 2 * time.Second
	// supervisorGracefulStop is how long a stopping server gets before it is
	// ended.
	supervisorGracefulStop = 10 * time.Second
	// supervisorStopPoll is how often the supervisor looks for the stop
	// marker. It is the latency of `service stop`, not a throughput knob.
	supervisorStopPoll = 250 * time.Millisecond
	// supervisorStartTimeout / supervisorStopTimeout bound the waits in
	// `service start` and `service stop`.
	supervisorStartTimeout = 15 * time.Second
	supervisorStopTimeout  = 20 * time.Second
)

// claimSupervisor takes the supervisor's single-instance lock. It returns a
// nil release when another supervisor already holds it — the expected result
// of a logon entry firing while the service is already up, not an error — and
// a cleanup func when this process won.
//
// The lock is a file created with O_EXCL, so two supervisors racing cannot
// both win. A lock left behind by a supervisor that died without cleaning up
// names a pid that is no longer ours, and is taken over.
func claimSupervisor() (func(), error) {
	path := config.SupervisorPidPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		won, err := createLock(path, pidInfo{PID: os.Getpid(), Exe: exePath()})
		if err != nil {
			return nil, err
		}
		if won {
			return func() { _ = os.Remove(path) }, nil
		}
		if h, err := readPidInfo(path); err == nil && pidIsOurs(h.PID, h.Exe) {
			return nil, nil
		}
		// Debris from a supervisor that died, or a pid now owned by somebody
		// else: either way it is not a supervisor, so it does not hold a lock.
		_ = os.Remove(path)
	}
	return nil, errors.New("could not claim the supervisor lock")
}

// createLock creates path exclusively and writes info into it, reporting
// false when the file already exists.
func createLock(path string, info pidInfo) (bool, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, err
	}
	data, err := json.Marshal(info)
	if err == nil {
		_, err = f.Write(data)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return true, err
}

// runningSupervisor returns the supervisor named in the lock file when one is
// actually alive and is ours. This is the single answer to "is the built-in
// supervisor up?" that every service verb reads.
func runningSupervisor() *pidInfo {
	info, err := readPidInfo(config.SupervisorPidPath())
	if err != nil || info.PID <= 0 {
		return nil
	}
	if !pidIsOurs(info.PID, info.Exe) {
		return nil
	}
	return info
}

// requestSupervisorStop asks a running supervisor to shut down by writing the
// stop marker. A file rather than a signal, so the same code works on every
// platform and a test can drive a stop without processes.
func requestSupervisorStop() error {
	if err := os.MkdirAll(config.ConfigDir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(config.StopRequestPath(), []byte("stop\n"), 0o644)
}

// stopRequested closes the returned channel when the stop marker appears or a
// termination signal arrives. Both routes exist because the marker is how
// `service stop` reaches a detached supervisor, while the signal is how
// Ctrl+C reaches one running in a terminal.
func stopRequested(markerPath string) <-chan struct{} {
	stop := make(chan struct{})
	var once sync.Once
	fire := func() { once.Do(func() { close(stop) }) }

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		fire()
	}()

	go func() {
		for {
			if _, err := os.Stat(markerPath); err == nil {
				fire()
				return
			}
			time.Sleep(supervisorStopPoll)
		}
	}()
	return stop
}

// waitForSupervisorReady waits until a supervisor is alive and its server is
// answering — the contract `service start` promises its caller.
func waitForSupervisorReady(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if runningSupervisor() != nil && isServerRunning(port) {
			return true
		}
		time.Sleep(supervisorStopPoll)
	}
	return false
}

// waitForSupervisorGone waits until no supervisor is alive.
func waitForSupervisorGone(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if runningSupervisor() == nil {
			return true
		}
		time.Sleep(supervisorStopPoll)
	}
	return false
}

// startSupervisorDetached launches the supervisor in its own session and
// returns without waiting for it. This is the logon entry's entire job: a
// process Explorer starts owns a console for as long as it lives, so it must
// hand off and exit at once.
func startSupervisorDetached(exe string) error {
	if runningSupervisor() != nil {
		return nil
	}
	argv := supervisorArgv(exe)
	c := exec.Command(argv[0], argv[1:]...)
	c.Stdin = nil
	c.Stdout = nil
	c.Stderr = nil
	detachProcess(c)
	return c.Start()
}

// stopSupervisorNow asks a running supervisor to stop and waits for it to go.
func stopSupervisorNow() error {
	if err := requestSupervisorStop(); err != nil {
		return err
	}
	if !waitForSupervisorGone(supervisorStopTimeout) {
		return errors.New("the supervisor did not stop; see " + config.ServiceLogPath())
	}
	if err := os.Remove(config.StopRequestPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// pauseSupervisor stops a running supervisor so the binary it runs can be
// replaced, reporting whether one was stopped and should be started again.
// The supervisor owns its server, so a caller that pauses it must not also
// stop the server by pid file — the supervisor does that on the way out.
func pauseSupervisor() (paused bool, err error) {
	if runningSupervisor() == nil {
		return false, nil
	}
	return true, stopSupervisorNow()
}

// runSupervise is the supervisor itself: it keeps `waypoint start --daemon`
// alive, records the server's pid so `waypoint stop` and `service status` can
// see it, and narrates both processes into the service log.
func runSupervise() (supervise.Outcome, error) {
	if err := os.MkdirAll(config.ConfigDir(), 0o755); err != nil {
		return 0, formatError("create config directory", err)
	}
	logFile, err := os.OpenFile(config.ServiceLogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, formatError("open service log", err)
	}
	defer logFile.Close()

	// A marker left by an earlier stop must not end this run.
	_ = os.Remove(config.StopRequestPath())

	exe := exePath()
	port := configuredPort()
	outcome, err := supervise.Run(supervise.Options{
		Command: func() (*exec.Cmd, error) {
			c := exec.Command(exe, daemonFlags(0)...)
			c.Stdin = nil
			c.Stdout = logFile
			c.Stderr = logFile
			return c, nil
		},
		Log:       logFile,
		Budget:    supervise.NewBudget(supervisorRestartLimit, supervisorRestartWindow),
		Delay:     supervisorRestartDelay,
		Graceful:  supervisorGracefulStop,
		Stop:      stopRequested(config.StopRequestPath()),
		Claim:     claimSupervisor,
		StopChild: func(c *exec.Cmd) error { return killProcess(c.Process.Pid) },
		OnStart:   func(pid int) { _ = writePidFile(port, pid) },
		OnExit:    func() { _ = os.Remove(config.PidPath()) },
	})
	_ = os.Remove(config.StopRequestPath())
	return outcome, err
}

// --- commands ---

var serviceRunCmd = &cobra.Command{
	Use:    "run",
	Short:  "Start the supervisor and exit (the logon entry point)",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := startSupervisorDetached(exePath()); err != nil {
			return formatError("start supervisor", err)
		}
		return nil
	},
}

var serviceSuperviseCmd = &cobra.Command{
	Use:    "supervise",
	Short:  "Keep the server running until a stop is requested",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		outcome, err := runSupervise()
		if err != nil {
			return formatError("supervise", err)
		}
		if jsonOut {
			printJSON(map[string]string{"status": outcome.String()})
		}
		return nil
	},
}
