package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/autopilot"
	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/exa"
	"github.com/udit-001/waypoint/internal/notify"
	"github.com/udit-001/waypoint/internal/scraper"
	"github.com/udit-001/waypoint/internal/server"
	"github.com/udit-001/waypoint/internal/zen"
)

var startFlags struct {
	port       int
	noOpen     bool
	background bool
	foreground bool
	daemon     bool
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the web UI server",
	Long: `Start a local web server with the read-only dashboard and API.

Opens the Waypoint UI in your browser. The dashboard shows your
job applications, stats, and filters — all read-only. Use the CLI
commands to add, update, or delete jobs.

Examples:
  waypoint start
  waypoint start --port 8080
  waypoint start --foreground  # Run in foreground
  waypoint start --no-open     # Don't auto-open browser`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve port: --port flag (if explicitly set) → config file → default.
		if !cmd.Flags().Changed("port") {
			if cfg, err := config.Load(); err == nil && cfg != nil && cfg.Port > 0 {
				startFlags.port = cfg.Port
			}
		}

		// Check if server is already running. This prevents silent port
		// conflicts when a user runs `waypoint start` while a server is
		// already serving. The health check is the source of truth — a
		// stale PID file without a responding server is ignored.
		if info, err := readPidFile(); err == nil && info.Port > 0 && isServerRunning(info.Port) {
			url := fmt.Sprintf("http://127.0.0.1:%d", info.Port)
			if jsonOut {
				printJSON(map[string]any{"running": true, "port": info.Port, "pid": info.PID, "url": url})
				return nil
			}
			fmt.Println()
			fmt.Printf("  Waypoint server already running (PID: %d)\n", info.PID)
			fmt.Printf("  %s\n", url)
			fmt.Printf("  Use 'waypoint stop' to stop\n")
			fmt.Println()
			return nil
		}

		background := startFlags.background && !startFlags.foreground
		if background && !startFlags.daemon {
			c, err := startDaemon(startFlags.port)
			if err != nil {
				return err
			}
			url := fmt.Sprintf("http://127.0.0.1:%d", startFlags.port)
			if jsonOut {
				printJSON(map[string]any{"running": true, "port": startFlags.port, "pid": c.Process.Pid, "url": url})
				return nil
			}
			fmt.Println()
			fmt.Printf("  Waypoint server started in background (PID: %d)\n", c.Process.Pid)
			fmt.Printf("  %s\n", url)
			fmt.Printf("  Use 'waypoint stop' to stop\n")
			fmt.Println()
			return nil
		}

		if !startFlags.daemon {
			fmt.Println()
			fmt.Printf("  Starting Waypoint server...\n")
			fmt.Println()
		}

		// Run database migrations before starting the server.
		// Goose runs here (not in Open) so only `waypoint start` triggers
		// migrations — CLI commands like `jobs add` use Open() directly.
		if err := store.RunMigrations(storePath); err != nil {
			return fmt.Errorf("database migration failed: %w", err)
		}

		// Boot recovery: any unfinished run row is a dead process's cycle.
		// Best effort; see recoverInterruptedRuns.
		recoverInterruptedRuns(store)

		// Daemon mode: capture this process's stdout/stderr into
		// $DATA_DIR/autopilot.log (WP-176) — the raw layer beneath the
		// runlog, so a cycle's stage lines and errors outlive the terminal
		// the daemon was never attached to. Best effort: a failed open
		// logs (to wherever output still goes) and the daemon runs on.
		if startFlags.daemon {
			if wcfg, err := config.Load(); err == nil {
				if f, err := captureDaemonOutput(config.AutopilotLogPath(wcfg)); err != nil {
					log.Printf("autopilot log capture failed: %v — output not captured", err)
				} else {
					defer f.Close()
				}
			} else {
				// Diagnosable silence: a daemon whose config cannot load must
				// say so wherever output still goes.
				log.Printf("autopilot log capture skipped: %v", err)
			}
		}

		// Autopilot scheduler (daemon mode only). Always started: it
		// self-gates via CycleDue (enabled flag, run-request marker,
		// cadence), so enabling mid-flight takes effect on the next poll
		// instead of requiring a restart. cycles carries the in-flight
		// cycle's cancel to the server: flipping autopilot off stops the
		// running cycle instead of waiting it out.
		var cycles cycleCancel
		if startFlags.daemon {
			go startAutopilotTicker(store, startFlags.port, &cycles)
		}

		return server.Start(server.Config{
			Port:            startFlags.port,
			DB:              store,
			NoOpen:          startFlags.noOpen,
			Silent:          startFlags.daemon,
			CancelAutopilot: cycles.fire,
			// One process-wide catalog: the picker's /api/zen/models and the
			// connector's WithSharedCatalog read the same last-known-good
			// list (WP-185).
			ZenCatalog: zen.SharedCatalog(),
			// ADR 0001: CLI writes boards.toml, web reads it. The loader
			// re-reads per request so `boards add` shows up without a restart.
			LoadBoards: func() ([]config.BoardEntry, error) {
				cfg, err := config.Load()
				if err != nil {
					return nil, err
				}
				bf, err := config.LoadBoards(cfg)
				if err != nil {
					return nil, err
				}
				return bf.Boards, nil
			},
			// WP-154: the write side of the same seam — mutate-and-save for
			// candidate promotion. Same file, same process family.
			WithBoards: func(fn func(*config.BoardsFile) error) error {
				wcfg, err := config.Load()
				if err != nil {
					return err
				}
				bf, err := config.LoadBoards(wcfg)
				if err != nil {
					return err
				}
				if err := fn(bf); err != nil {
					return err
				}
				return config.SaveBoards(wcfg, bf)
			},
		})
	},
}

// startDaemon spawns the detached background daemon on port, writes the PID
// file, and waits for it to answer /api/stats. All the failure modes are
// handled here rather than at each call site: a spawn failure, an unwritable
// PID file, or a port that never opens each leave no zombie process and no
// stale PID file behind.
//
// The one routine behind `waypoint start --background` and the restart path
// in `waypoint upgrade`, so both get the same verified-start contract.
func startDaemon(port int) (*exec.Cmd, error) {
	argv := daemonFlags(port)
	c := exec.Command(exePath(), argv...)
	c.Stdin = nil
	c.Stdout = nil
	c.Stderr = nil
	detachProcess(c)
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("failed to start background server: %w", err)
	}
	if err := writePidFile(port, c.Process.Pid); err != nil {
		_ = c.Process.Kill()
		return nil, fmt.Errorf("failed to write PID file: %w", err)
	}

	// Poll isServerRunning every 100ms for up to 2 seconds (20 attempts).
	// This catches silent failures — port in use, child crash, etc. On
	// timeout, kill the child and clean up so the user isn't left with a
	// zombie process and a stale PID file.
	if !waitForServerReady(port, 20, 100*time.Millisecond) {
		_ = c.Process.Kill()
		_ = os.Remove(config.PidPath())
		return nil, fmt.Errorf("server failed to start — port may be in use")
	}
	return c, nil
}

// waitForServerReady polls isServerRunning every interval up to maxAttempts.
// Returns true if the server responds within the deadline, false on timeout.
func waitForServerReady(port, maxAttempts int, interval time.Duration) bool {
	for i := 0; i < maxAttempts; i++ {
		if isServerRunning(port) {
			return true
		}
		time.Sleep(interval)
	}
	return false
}

// captureDaemonOutput points the daemon's stdout, stderr, and the default
// logger at an append file (WP-176): the raw layer beneath the structured
// runlog. Plain append, no rotation. Called inside the daemon process (on
// --daemon), so both spawn paths — startDaemon and the service manager —
// get the behavior from this one placement. Returns the open handle; the
// caller closes it when the daemon exits.
func captureDaemonOutput(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log capture: %w", err)
	}
	os.Stdout = f
	os.Stderr = f
	log.SetOutput(f)
	return f, nil
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().IntVar(&startFlags.port, "port", config.DefaultPort, "HTTP server port")
	startCmd.Flags().BoolVar(&startFlags.noOpen, "no-open", false, "Don't auto-open browser")
	startCmd.Flags().BoolVarP(&startFlags.foreground, "foreground", "f", false, "Run server in foreground")
	startCmd.Flags().BoolVarP(&startFlags.background, "background", "b", true, "Run server in background")
	startCmd.Flags().BoolVar(&startFlags.daemon, "daemon", false, "")
	startCmd.Flags().MarkHidden("daemon")
}

// recoverInterruptedRuns closes run rows a dead process left open, at
// boot, before the scheduler's first poll: this boot is the first witness
// that nothing is running, so the invariant CycleDue assumes ("an
// unfinished row is the cycle running now") holds again. Without it, a
// crash mid-cycle leaves the newest row unfinished forever — the web chip
// reads "running", and the scheduler falls back to "first run",
// re-running the whole sweep on every restart. Best effort: a failure
// logs and boot proceeds. Split from RunE because RunE blocks on the
// server loop; the test crosses this seam instead of starting a server.
func recoverInterruptedRuns(store db.Store) {
	n, err := store.CloseInterruptedRuns(time.Now())
	if err != nil {
		log.Printf("autopilot: boot recovery failed: %v", err)
		return
	}
	if n > 0 {
		log.Printf("autopilot: boot recovery closed %d interrupted run(s)", n)
	}
}

// cycleCancel tracks the in-flight cycle's cancel func. The ticker swaps
// it per cycle; the server's disable hook fires it. Nil-safe throughout:
// fire with nothing running is a no-op, so foreground mode (no ticker)
// can wire the same hook without care.
type cycleCancel struct {
	mu     sync.Mutex
	cancel context.CancelFunc
}

func (c *cycleCancel) set(f context.CancelFunc) {
	c.mu.Lock()
	c.cancel = f
	c.mu.Unlock()
}

func (c *cycleCancel) fire() {
	c.mu.Lock()
	f := c.cancel
	c.mu.Unlock()
	if f != nil {
		f()
	}
}

// startAutopilotTicker runs the autopilot scheduler inside the daemon.
// It polls every PollInterval and fires a cycle when CycleDue says so —
// which makes three behaviors fall out of one policy:
//
//   - boot with autopilot enabled → first cycle within seconds
//   - enable while running → PATCH drops a run-request marker, consumed
//     on the next poll (no dead air)
//   - steady state → a cycle per cadence, measured from the last
//     FINISHED run (an overrun never double-fires)
//
// Blocks until the process exits. port is the server's own port — the
// notification click target.
func startAutopilotTicker(store db.Store, port int, cycles *cycleCancel) {
	log.Printf("autopilot: scheduler started (poll every %s)", autopilot.PollInterval)

	// Shared anonymous Exa client (one budget + cache for the daemon
	// lifetime; each cycle resets the budget). No Bearer — see
	// internal/exa/client.go.
	exaClient := exa.New("", nil)
	openURL := fmt.Sprintf("http://127.0.0.1:%d/#/found", port)

	// Check immediately at boot, then poll. safeRun keeps a panic
	// anywhere in the iteration from killing scheduling forever.
	safeRun := func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("autopilot: scheduler recovered: %v", r)
			}
		}()
		if due, reason := autopilotDue(store); due {
			runAutopilotCycle(store, exaClient, openURL, reason, cycles)
		}
	}
	safeRun()

	tick := time.NewTicker(autopilot.PollInterval)
	defer tick.Stop()
	for range tick.C {
		safeRun()
	}
}

// autopilotDue applies the scheduling policy against persisted state.
// Any read failure reports not-due and logs — a broken store must not
// spin the scheduler into tight refiring. The Schedule is rebuilt each
// poll: the window and working cadence from settings, the drain
// interval fixed, and the backlog re-read from the ledger (WP-196).
func autopilotDue(store db.Store) (bool, string) {
	settings, err := store.GetSettings()
	if err != nil {
		log.Printf("autopilot: read settings: %v", err)
		return false, ""
	}
	requested, err := store.ConsumeAutopilotRunRequest()
	if err != nil {
		log.Printf("autopilot: consume run request: %v", err)
	}
	var finished string
	if last, ok, err := store.GetLastRun(); err != nil {
		log.Printf("autopilot: read last run: %v", err)
	} else if ok {
		finished = last.FinishedAt
	}
	backlog := 0
	if n, err := store.CountPostings(db.StatusNew); err != nil {
		// Poll as drained — the working cadence, never a stalled loop.
		log.Printf("autopilot: count new postings: %v", err)
	} else {
		backlog = n
	}
	schedule := autopilot.Schedule{
		Work:           autopilot.WorkWindow{Start: settings.AutopilotWorkStart, End: settings.AutopilotWorkEnd},
		WorkingCadence: time.Duration(settings.AutopilotWorkingCadence) * time.Minute,
		DrainInterval:  autopilot.DefaultDrainInterval,
		Backlog:        backlog,
	}
	return autopilot.CycleDue(settings.AutopilotEnabled == 1, schedule, finished, requested, time.Now())
}

// runAutopilotCycle executes one cycle and logs the outcome. Failures
// inside are recorded by the cycle itself; this wrapper only reports.
// The cycle runs on a context the cycles holder tracks: flipping
// autopilot off mid-flight cancels it, the stages exit at their next
// boundary, and the interrupted run closes as degraded — the same
// recovery semantics as a crash, without the crash.
func runAutopilotCycle(store db.Store, exaClient *exa.Client, openURL, reason string, cycles *cycleCancel) {
	log.Printf("autopilot: starting cycle (%s)", reason)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if cycles != nil {
		cycles.set(cancel)
		defer cycles.set(nil)
	}

	settings, _ := store.GetSettings()

	// Build zen client from stored or env API key.
	var zc *zen.Client
	if key := zen.ResolveKey(settings.ZenAPIKey); key != "" {
		zcfg := zen.DefaultConfig().WithSharedCatalog() // UA version + family routing from the curated metadata
		zcfg.APIKey = key
		zcfg.ProjectID = zen.ProjectID(storePath)        // stable per-install session (sticky routing survives restarts)
		zcfg.Model = zen.ResolveModel(settings.ZenModel) // user's pick wins, else the shipped default
		zc = zen.New(zcfg)
		zc.SetCompanySearcher(exaClient)
	}

	entry := autopilot.RunLogged(ctx, autopilot.CycleConfig{
		Store:     store,
		ZenClient: zc,
		Scrapers:  scraper.All(),
		ExaCap:    10,
		Recency:   14,
		Notifier:  notify.New(openURL),
	})

	log.Printf("autopilot: cycle complete (id=%d, verdict=%s, new=%d, shortlisted=%d, dismissed=%d, errored=%d, duration=%dms)",
		entry.ID, entry.Verdict, entry.PostingsNew, entry.PostingsShortlisted,
		entry.PostingsDismissed, entry.PostingsErrored, entry.DurationMs)
}
