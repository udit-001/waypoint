package cli

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/config"
)

// identityProbe is the evidence gathered about the process named in the PID
// file. It is the seam between the shared stop decision and the per-OS probe:
// unix resolves the executable and can verify it; Windows cannot (no
// portable liveness probe, and Go cannot deliver a console interrupt), so
// Windows falls back to port health.
type identityProbe struct {
	Alive       bool // the pid exists
	Ours        bool // the pid's executable is waypoint (meaningful only when Known)
	Known       bool // the executable could be probed at all
	PortHealthy bool // a waypoint server answers on the PID file's port
}

// stopOutcome is what stopServerByPidfile found.
type stopOutcome int

const (
	stopNoServer     stopOutcome = iota // no PID file
	stopStalePID                        // the process is gone
	stopForeignPID                      // live, but not verified as waypoint — never signaled
	stopUnverifiable                    // no port recorded and no probe — never signaled
	stopStopped                         // verified ours, signal delivered
)

// decideStop is the whole stop policy in one pure function. The rule that
// matters: a resolved executable is authoritative, so a reused PID is never
// signaled even when something waypoint-like happens to answer on the port.
// Where no probe exists (Windows), port health is the only identity evidence
// available and it needs a pinned port to mean anything.
func decideStop(info *pidInfo, p identityProbe) stopOutcome {
	if info == nil || info.PID <= 0 {
		return stopNoServer
	}
	if p.Known {
		switch {
		case !p.Alive:
			return stopStalePID
		case !p.Ours:
			return stopForeignPID
		default:
			return stopStopped
		}
	}
	if info.Port <= 0 {
		return stopUnverifiable
	}
	switch {
	case p.PortHealthy:
		return stopStopped
	case p.Alive:
		return stopForeignPID
	default:
		return stopStalePID
	}
}

// stopMessage is the one human/JSON message for an outcome.
func stopMessage(outcome stopOutcome, info *pidInfo) string {
	pid := 0
	if info != nil {
		pid = info.PID
	}
	switch outcome {
	case stopStopped:
		return fmt.Sprintf("Server (PID %d) stopped", pid)
	case stopStalePID:
		return "No running Waypoint server found (stale PID file cleaned up)"
	case stopForeignPID:
		return fmt.Sprintf("PID %d is alive but is not a Waypoint process — not stopped. Stale PID file cleaned up.", pid)
	case stopUnverifiable:
		return fmt.Sprintf("PID %d could not be verified as Waypoint (no port recorded) — not stopped. Stale PID file cleaned up.", pid)
	default:
		return "No running Waypoint server found"
	}
}

// stopServerByPidfile stops the server named in the PID file and removes the
// file. This is the one routine behind `waypoint stop` and the restart path
// in `waypoint upgrade`, so both refuse to signal an unverified process.
//
// Every "nothing to stop" case (missing, stale, foreign, unverifiable) is an
// outcome rather than an error — callers can tell "stopped" from "there was
// nothing to stop". The one error is a verified stop that outlived the kill,
// and then the PID file is deliberately kept: it still names a live server,
// so removing it would hide the failure.
func stopServerByPidfile() (stopOutcome, *pidInfo, error) {
	info, err := readPidFile()
	if err != nil {
		return stopNoServer, nil, nil
	}
	outcome := decideStop(info, probeIdentity(info))
	if outcome == stopStopped {
		if kerr := killProcess(info.PID); kerr != nil && processAlive(info.PID) {
			return stopStopped, info, formatError("stop server", kerr)
		}
		waitForProcessExit(info.PID, 5*time.Second)
	}
	_ = os.Remove(config.PidPath())
	return outcome, info, nil
}

// waitForProcessExit polls until the process is gone or the deadline passes.
// Best-effort: the signal has already been delivered, so a timeout is not an
// error — the OS reaps eventually and the PID file is already gone.
func waitForProcessExit(pid int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the background web UI server",
	Long: `Stop the local Waypoint web server if one is running.

Reads the server PID file, verifies the process is actually Waypoint,
sends a graceful shutdown signal, and cleans up the PID file. A stale,
reused, or unverifiable PID is never signaled — the file is removed and
nothing else happens.

Examples:
  waypoint stop
  waypoint stop --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// A service manager that has Waypoint registered and running owns the
		// server: stopping it behind the manager's back invites a restart
		// (systemd Restart=always), so point at the command that owns both.
		if serviceActive() {
			msg := "Waypoint is managed by the background service — use 'waypoint service stop'"
			if h := runningSupervisor(); h != nil {
				msg = fmt.Sprintf("Waypoint is running under the service supervisor (PID %d) — use 'waypoint service stop'", h.PID)
			}
			if jsonOut {
				printJSON(map[string]any{"running": true, "message": msg})
			}
			return errors.New(msg)
		}

		// A supervisor with no registered, running service is just what is
		// keeping a hand-run server alive: stopping it is what the user means.
		if h := runningSupervisor(); h != nil {
			if err := stopSupervisorNow(); err != nil {
				return formatError("stop supervisor", err)
			}
			msg := fmt.Sprintf("Waypoint supervisor (PID %d) stopped", h.PID)
			if jsonOut {
				printJSON(map[string]any{"running": false, "message": msg})
				return nil
			}
			fmt.Println()
			fmt.Printf("  %s\n", msg)
			fmt.Println()
			return nil
		}
		outcome, info, err := stopServerByPidfile()
		if err != nil {
			return err
		}
		msg := stopMessage(outcome, info)
		if jsonOut {
			printJSON(map[string]any{"running": false, "message": msg})
			return nil
		}
		fmt.Println()
		fmt.Printf("  %s\n", msg)
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
