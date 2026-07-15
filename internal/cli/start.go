package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/udit-001/waypoint/internal/server"
	"github.com/spf13/cobra"
)

func pidFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "server.pid"
	}
	return filepath.Join(home, ".waypoint", "server.pid")
}

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
			_ = os.WriteFile(pidFilePath(), []byte(fmt.Sprintf("%d", c.Process.Pid)), 0644)
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

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().IntVar(&startFlags.port, "port", 8080, "HTTP server port")
	startCmd.Flags().BoolVar(&startFlags.noOpen, "no-open", false, "Don't auto-open browser")
	startCmd.Flags().BoolVarP(&startFlags.foreground, "foreground", "f", false, "Run server in foreground")
	startCmd.Flags().BoolVarP(&startFlags.background, "background", "b", true, "Run server in background")
	startCmd.Flags().BoolVar(&startFlags.daemon, "daemon", false, "")
	startCmd.Flags().MarkHidden("daemon")
}
