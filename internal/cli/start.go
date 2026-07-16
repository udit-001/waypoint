package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/server"
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
commands to add, upgrade, or delete jobs.

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
			daemonArgs := []string{
				os.Args[0], "start",
				"--port", strconv.Itoa(startFlags.port),
				"--no-open",
				"--daemon",
			}
			c := exec.Command(daemonArgs[0], daemonArgs[1:]...)
			c.Stdin = nil
			c.Stdout = nil
			c.Stderr = nil
			detachProcess(c)
			if err := c.Start(); err != nil {
				return fmt.Errorf("failed to start background server: %w", err)
			}
			if err := writePidFile(startFlags.port, c.Process.Pid); err != nil {
				return fmt.Errorf("failed to write PID file: %w", err)
			}

			// Poll isServerRunning every 100ms for up to 2 seconds (20 attempts).
			// This catches silent failures — port in use, child crash, etc.
			// On timeout, kill the child and clean up so the user isn't left
			// with a zombie process and a stale PID file.
			if !waitForServerReady(startFlags.port, 20, 100*time.Millisecond) {
				c.Process.Kill()
				_ = os.Remove(config.PidPath())
				return fmt.Errorf("Server failed to start — port may be in use")
			}

			fmt.Println()
			fmt.Printf("  Waypoint server started in background (PID: %d)\n", c.Process.Pid)
			fmt.Printf("  http://127.0.0.1:%d\n", startFlags.port)
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

		return server.Start(server.Config{
			Port:   startFlags.port,
			DB:     store,
			NoOpen: true,
			Silent: startFlags.daemon,
		})
	},
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

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().IntVar(&startFlags.port, "port", config.DefaultPort, "HTTP server port")
	startCmd.Flags().BoolVar(&startFlags.noOpen, "no-open", false, "Don't auto-open browser")
	startCmd.Flags().BoolVarP(&startFlags.foreground, "foreground", "f", false, "Run server in foreground")
	startCmd.Flags().BoolVarP(&startFlags.background, "background", "b", true, "Run server in background")
	startCmd.Flags().BoolVar(&startFlags.daemon, "daemon", false, "")
	startCmd.Flags().MarkHidden("daemon")
}
