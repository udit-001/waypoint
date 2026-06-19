package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

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

		if err := killProcess(pid); err != nil {
			return err
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
