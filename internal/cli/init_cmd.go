package cli

import (
	"fmt"
	"os"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new database",
	Long: `Create a new SQLite database for tracking job applications.
If the database file already exists, this command will refuse
to overwrite it (use --force to start fresh).

After creating the database, you'll be offered the option to
install the waypoint skill for your AI coding agent.

Examples:
  waypoint init
  waypoint init --db ~/my-jobs.db
  waypoint init --force
  waypoint init --no-skills`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if file exists
		if _, err := os.Stat(storePath); err == nil {
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				return fmt.Errorf("database %q already exists\n  Use --force to overwrite, or --db to specify a different path", storePath)
			}
			// Remove existing file and WAL sidecars
			if err := os.Remove(storePath); err != nil {
				return fmt.Errorf("remove existing database: %w", err)
			}
			os.Remove(storePath + "-wal")
			os.Remove(storePath + "-shm")
		}

		// Open (creates) the database
		s, err := db.Open(storePath)
		if err != nil {
			return fmt.Errorf("create database: %w", err)
		}
		defer s.Close()

		// Run migrations to create schema
		if err := s.RunMigrations(storePath); err != nil {
			return fmt.Errorf("database migration failed: %w", err)
		}

		fmt.Println()
		fmt.Printf("  ✓ Initialized job tracker database at %s\n", storePath)

		// Offer skill installation
		noSkills, _ := cmd.Flags().GetBool("no-skills")
		if !noSkills {
			offerSkillInstall()
		}

		fmt.Println()
		fmt.Println("  Next steps:")
		fmt.Println("    waypoint jobs add \"Company Name\" \"Position Title\"")
		fmt.Println("    waypoint jobs list")
		fmt.Println("    waypoint jobs stats")
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().Bool("force", false, "Overwrite existing database")
	initCmd.Flags().Bool("no-skills", false, "Skip the skill installation prompt")
}
