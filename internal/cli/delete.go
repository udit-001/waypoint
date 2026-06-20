package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var deleteFlags struct {
	force bool
}

var deleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a job application",
	Long: `Delete a job application by its ID. Prompts for confirmation
unless --force is used.

Examples:
  waypoint delete 42
  waypoint delete 42 --force`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid job ID: %s", args[0])
		}

		// Fetch job for confirmation message
		job, err := store.GetJob(id)
		if err != nil {
			return fmt.Errorf("job %d not found", id)
		}

		if !deleteFlags.force {
			fmt.Printf("  Delete job %d (%s — %s)? [y/N]: ", id, job.Company, job.Position)
			var confirm string
			fmt.Scanln(&confirm)
			if confirm != "y" && confirm != "Y" && confirm != "yes" {
				fmt.Println("  Cancelled.")
				return nil
			}
		}

		if err := store.DeleteJob(id); err != nil {
			return formatError("failed to delete job", err)
		}

		if jsonOut {
			printJSON(map[string]any{
				"deleted":  true,
				"id":       id,
				"company":  job.Company,
				"position": job.Position,
			})
			return nil
		}

		fmt.Println()
		fmt.Printf("  ✓ Deleted job %d: %s — %s\n", id, job.Company, job.Position)
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().BoolVar(&deleteFlags.force, "force", false, "Skip confirmation prompt")
}
