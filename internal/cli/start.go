package cli

import (
	"fmt"

	"jobtracker/internal/server"
	"github.com/spf13/cobra"
)

var startFlags struct {
	port   int
	noOpen bool
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the web UI server",
	Long: `Start a local web server with the read-only dashboard and API.

Opens the Job Tracker UI in your browser. The dashboard shows your
job applications, stats, and filters — all read-only. Use the CLI
commands to add, update, or delete jobs.

Examples:
  job-tracker start
  job-tracker start --port 8080
  job-tracker start --no-open    # Don't auto-open browser`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		fmt.Printf("  Starting Job Tracker server...\n")
		fmt.Println()

		return server.Start(server.Config{
			Port:   startFlags.port,
			DB:     store,
			NoOpen: startFlags.noOpen,
		})
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().IntVar(&startFlags.port, "port", 8080, "HTTP server port")
	startCmd.Flags().BoolVar(&startFlags.noOpen, "no-open", false, "Don't auto-open browser")
}
