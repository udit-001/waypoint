package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new database",
	Long: `Create a new SQLite database for tracking job applications.
If the database file already exists, this command will refuse
to overwrite it (use --force to start fresh).

A config file is created in your OS config directory
(~/.config/waypoint/config.toml on Linux) storing the data
directory and port, so you don't need to pass --db and --port
flags every time.

After creating the database, you'll be offered the option to
install the waypoint skill for your AI coding agent.

Examples:
  waypoint init
  waypoint init --db ~/my-jobs.db
  waypoint init --force
  waypoint init --no-skills`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Step 1: Ensure config file exists (idempotent).
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if cfg == nil {
			cfg = &config.Config{
				DataDir: config.DefaultDataDir(),
				Port:    config.DefaultPort,
			}
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("create config: %w", err)
			}
			fmt.Printf("  ✓ Created config: %s\n", config.ConfigPath())
		} else {
			fmt.Printf("  • Config exists: %s\n", config.ConfigPath())
		}

		// Step 2: Ensure database exists (idempotent unless --force).
		force, _ := cmd.Flags().GetBool("force")
		if _, err := os.Stat(storePath); err == nil {
			if !force {
				fmt.Printf("  • Database already initialized: %s\n", storePath)
			} else {
				// --force only recreates the database, never the config.
				if err := os.Remove(storePath); err != nil {
					return fmt.Errorf("remove existing database: %w", err)
				}
				os.Remove(storePath + "-wal")
				os.Remove(storePath + "-shm")
				s, err := db.Open(storePath)
				if err != nil {
					return fmt.Errorf("create database: %w", err)
				}
				if err := s.RunMigrations(storePath); err != nil {
					return fmt.Errorf("database migration failed: %w", err)
				}
				s.Close()
				fmt.Printf("  ✓ Recreated database: %s\n", storePath)
			}
		} else {
			// Database doesn't exist — create it.
			s, err := db.Open(storePath)
			if err != nil {
				return fmt.Errorf("create database: %w", err)
			}
			if err := s.RunMigrations(storePath); err != nil {
				return fmt.Errorf("database migration failed: %w", err)
			}
			s.Close()
			fmt.Printf("  ✓ Created database: %s\n", storePath)
		}

		// Step 3: Offer skill installation.
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
