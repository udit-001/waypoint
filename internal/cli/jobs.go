package cli

import "github.com/spf13/cobra"

// jobsCmd is the parent command for job-related subcommands.
var jobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "Manage job applications",
	Long: `Add, list, get, update, and delete job applications.

Examples:
  waypoint jobs add "Google" "SWE" --status Applied
  waypoint jobs list --status Applied
  waypoint jobs get 42
  waypoint jobs update 42 --status Offer
  waypoint jobs delete 42 --force`,
}

func init() {
	rootCmd.AddCommand(jobsCmd)
}
