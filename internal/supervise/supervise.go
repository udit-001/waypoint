// Package supervise keeps the Waypoint server alive where no OS service
// manager can help.
//
// Windows has no per-user service manager and installing a real service needs
// admin, so supervision has to be ours. This package is that supervisor, and
// it is deliberately platform-neutral: the platform-specific facts it needs —
// how to start a server, how to ask it to stop, where to log — arrive as
// fields on Options. Production wires them up in internal/cli; tests pass
// fakes, so the whole restart/backoff/stop policy is exercised on any
// platform, not only where it ships.
//
// The supervisor never decides *whether* it should be running. Something else
// owns that: the logon entry launches it, `waypoint service stop` signals it,
// and `waypoint service status` reads the state it records. Keeping that
// decision outside is what makes a hand-run `waypoint start` invisible to
// `waypoint service stop`.
package supervise

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// Outcome is how one supervised run ended.
type Outcome int

const (
	// OutcomeAlreadyRunning: another supervisor holds the instance lock, so
	// this one did nothing. Not an error — it is the expected result of a
	// logon entry firing while the service is already up.
	OutcomeAlreadyRunning Outcome = iota
	// OutcomeStopped: a stop was requested and honoured.
	OutcomeStopped
	// OutcomeExitedCleanly: the server exited with status 0, which is not a
	// failure to restart.
	OutcomeExitedCleanly
	// OutcomeGaveUp: the server kept failing; the restart budget ran out.
	OutcomeGaveUp
)

func (o Outcome) String() string {
	switch o {
	case OutcomeAlreadyRunning:
		return "already running"
	case OutcomeStopped:
		return "stopped"
	case OutcomeExitedCleanly:
		return "server exited cleanly"
	case OutcomeGaveUp:
		return "gave up after repeated failures"
	default:
		return "unknown"
	}
}

// Budget is a sliding-window restart limit: at most Limit starts per Window.
// It is systemd's StartLimitBurst in miniature, and it exists so a server
// that cannot start — port in use, corrupt database — reports a failure
// instead of restarting forever.
type Budget struct {
	limit  int
	window time.Duration
	starts []time.Time
}

// NewBudget returns a budget allowing limit starts per window. A non-positive
// limit means "always allow", which is only useful in tests.
func NewBudget(limit int, window time.Duration) *Budget {
	return &Budget{limit: limit, window: window}
}

// Allow records a start at now if the window has room, and reports whether
// the caller may proceed.
func (b *Budget) Allow(now time.Time) bool {
	if b == nil || b.limit <= 0 {
		return true
	}
	kept := b.starts[:0]
	for _, t := range b.starts {
		if now.Sub(t) < b.window {
			kept = append(kept, t)
		}
	}
	b.starts = kept
	if len(b.starts) >= b.limit {
		return false
	}
	b.starts = append(b.starts, now)
	return true
}

// Options is everything Run needs. Every field is required unless noted: a
// supervisor that invents its own restart policy is neither testable nor
// reviewable.
type Options struct {
	// Command builds one server command. Run starts it, waits for it, and
	// kills it — the caller never touches the process, so the seam stays a
	// pure builder and a test can hand back any command at all.
	Command func() (*exec.Cmd, error)
	// Log receives the supervisor's own lines. It is the only narration a
	// background process gets — the server's own stdout/stderr are the
	// caller's business.
	Log io.Writer
	// Budget caps restarts per window. Nil means unlimited.
	Budget *Budget
	// Delay is the pause before a restart, so a fast-failing server cannot
	// spin the CPU.
	Delay time.Duration
	// Graceful is how long a stopping child gets to exit on its own before
	// it is killed.
	Graceful time.Duration
	// Stop is closed when a stop has been requested. It ends the run whether
	// a child is running or not.
	Stop <-chan struct{}
	// Claim takes the single-instance lock. It returns a nil release when
	// another supervisor already holds it, which ends the run immediately
	// with OutcomeAlreadyRunning. Nil means no lock.
	Claim func() (release func(), err error)
	// StopChild asks a running child to shut down cleanly. Nil kills it.
	StopChild func(*exec.Cmd) error
	// OnStart is called with each child's pid once it is spawned — the hook
	// that records "a server is running" for other processes to read.
	OnStart func(pid int)
	// OnExit is called once a child has been reaped, before any restart.
	OnExit func()
}

// ErrNoCommand is returned when Options.Command is missing.
var ErrNoCommand = errors.New("supervise: Options.Command is required")

// Run supervises servers until a stop arrives, the budget runs out, or the
// server exits cleanly. It does not return an error for a server that failed
// to start — that is the restart loop's business, and the outcome plus the log
// say what happened.
func Run(opts Options) (Outcome, error) {
	if opts.Command == nil {
		return 0, ErrNoCommand
	}
	stop := opts.Stop
	if stop == nil {
		stop = make(chan struct{}) // never closed: an unsupervised run
	}

	if opts.Claim != nil {
		release, err := opts.Claim()
		if err != nil {
			return 0, err
		}
		if release == nil {
			logf(opts.Log, "another supervisor is already running; exiting")
			return OutcomeAlreadyRunning, nil
		}
		defer release()
	}

	for {
		if !opts.Budget.Allow(time.Now()) {
			logf(opts.Log, "the server keeps failing to start; giving up. Fix the cause above, then run 'waypoint service restart'")
			return OutcomeGaveUp, nil
		}

		cmd, err := opts.Command()
		if err != nil {
			logf(opts.Log, "cannot build the server command: %v", err)
			if sleepOrStop(stop, opts.Delay) {
				return OutcomeStopped, nil
			}
			continue
		}
		if err := cmd.Start(); err != nil {
			logf(opts.Log, "cannot start the server: %v", err)
			if sleepOrStop(stop, opts.Delay) {
				return OutcomeStopped, nil
			}
			continue
		}

		// cmd.Wait must be called exactly once, so it runs here and every
		// path below drains this channel rather than waiting again.
		exited := make(chan error, 1)
		go func() { exited <- cmd.Wait() }()
		if opts.OnStart != nil {
			opts.OnStart(cmd.Process.Pid)
		}
		logf(opts.Log, "server started (pid %d)", cmd.Process.Pid)

		select {
		case <-stop:
			requestStop(opts, cmd)
			select {
			case <-exited:
			case <-time.After(opts.Graceful):
				logf(opts.Log, "server did not shut down in %s; ending it", opts.Graceful)
				_ = cmd.Process.Kill()
				<-exited
			}
			finish(opts)
			logf(opts.Log, "stopped")
			return OutcomeStopped, nil

		case err := <-exited:
			finish(opts)
			if err == nil {
				logf(opts.Log, "server exited cleanly; supervisor exiting")
				return OutcomeExitedCleanly, nil
			}
			logf(opts.Log, "server exited (%v); restarting in %s", err, opts.Delay)
			if sleepOrStop(stop, opts.Delay) {
				return OutcomeStopped, nil
			}
		}
	}
}

// requestStop asks the child to shut down cleanly. A nil StopChild means the
// platform cannot ask, and the grace period simply runs out.
func requestStop(opts Options, cmd *exec.Cmd) {
	if opts.StopChild == nil {
		return
	}
	if err := opts.StopChild(cmd); err != nil {
		logf(opts.Log, "ask server to stop: %v", err)
	}
}

// finish runs the exit hook exactly once per child.
func finish(opts Options) {
	if opts.OnExit != nil {
		opts.OnExit()
	}
}

// sleepOrStop waits for d, returning true if a stop arrived first.
func sleepOrStop(stop <-chan struct{}, d time.Duration) bool {
	if d <= 0 {
		return false
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-stop:
		return true
	case <-timer.C:
		return false
	}
}

func logf(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, format+"\n", args...)
}
