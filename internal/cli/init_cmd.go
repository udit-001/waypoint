package cli

import (
	"fmt"
	"os"

	"github.com/SwatiBio/job-tracker/internal/db"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new database",
	Long: `Create a new SQLite database for tracking job applications.
If the database file already exists, this command will refuse
to overwrite it (use --force to start fresh).

Examples:
  job-tracker init
  job-tracker init --db ~/my-jobs.db
  job-tracker init --force`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if file exists
		if _, err := os.Stat(storePath); err == nil {
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				return fmt.Errorf("database %q already exists\n  Use --force to overwrite, or --db to specify a different path", storePath)
			}
			// Remove existing file
			if err := os.Remove(storePath); err != nil {
				return fmt.Errorf("remove existing database: %w", err)
			}
		}

		// Open (creates) the database
		s, err := db.Open(storePath)
		if err != nil {
			return fmt.Errorf("create database: %w", err)
		}
		defer s.Close()

		// Set store so other commands can use it
		store = s

		fmt.Println()
		fmt.Printf("  ✓ Initialized job tracker database at %s\n", storePath)
		fmt.Println()
		fmt.Println("  Next steps:")
		fmt.Println("    job-tracker add \"Company Name\" \"Position Title\"")
		fmt.Println("    job-tracker list")
		fmt.Println("    job-tracker stats")
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().Bool("force", false, "Overwrite existing database")
}
