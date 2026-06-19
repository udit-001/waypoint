package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the background web UI server",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(pidFilePath())
		if err != nil {
			return fmt.Errorf("no server PID file found — is the server running?")
		}

		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil {
			return fmt.Errorf("invalid PID file: %w", err)
		}

		proc, err := os.FindProcess(pid)
		if err != nil {
			_ = os.Remove(pidFilePath())
			return fmt.Errorf("no process with PID %d found (stale PID file removed)", pid)
		}

		if err := proc.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("failed to stop server: %w", err)
		}

		_ = os.Remove(pidFilePath())
		fmt.Println()
		fmt.Printf("  Server (PID %d) stopped\n", pid)
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
