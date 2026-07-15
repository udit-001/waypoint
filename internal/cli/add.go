package cli

import (
	"fmt"
	"os"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/spf13/cobra"
)

var addFlags struct {
	status       string
	category     string
	salary       string
	location     string
	contact      string
	url          string
	notes        string
	notesFile    string
	date         string
	appliedDate  string
	reminderDate string
}

var addCmd = &cobra.Command{
	Use:   "add <company> <position>",
	Short: "Add a new job application",
	Long: `Add a new job application to track.

Required arguments:
  company   Company or institution name
  position  Job title or position

Use flags to add details like status, category, salary, etc.

Examples:
  waypoint jobs add "Acme Corp" "Senior Engineer"
  waypoint jobs add "Acme Corp" "Senior Engineer" --status Applied --salary "$150k"
  waypoint jobs add "Acme Corp" "Senior Engineer" --notes "Applied via referral"
  waypoint jobs add "Acme Corp" "Senior Engineer" --notes-file /tmp/notes.txt`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Read notes from file if provided (overrides --notes)
		if addFlags.notesFile != "" {
			content, err := os.ReadFile(addFlags.notesFile)
			if err != nil {
				return fmt.Errorf("reading notes file: %w", err)
			}
			addFlags.notes = string(content)
		}

		job := db.Job{
			Company:      args[0],
			Position:     args[1],
			Status:       addFlags.status,
			Salary:       addFlags.salary,
			Location:     addFlags.location,
			Contact:      addFlags.contact,
			URL:          addFlags.url,
			Notes:        addFlags.notes,
			Date:         addFlags.date,
			AppliedDate:  addFlags.appliedDate,
			ReminderDate: nil,
		}

		// Resolve category name → ID (optional — uncategorized if not specified)
		if addFlags.category != "" {
			catID, err := store.CategoryIDByName(addFlags.category)
			if err != nil {
				return formatError("failed to resolve category", err)
			}
			if catID == 0 {
				return fmt.Errorf("category %q not found — use 'waypoint categories list' to see available categories", addFlags.category)
			}
			job.CategoryID = catID
		}

		if addFlags.reminderDate != "" {
			job.ReminderDate = &addFlags.reminderDate
		}

		created, err := db.IntakeAddJob(store, job)
		if err != nil {
			return formatError("failed to add job", err)
		}

		if jsonOut {
			printJSON(created)
			return nil
		}

		fmt.Println()
		fmt.Printf("  ✓ Job added: %s — %s\n", created.Company, created.Position)
		fmt.Printf("    ID: %d\n", created.ID)
		fmt.Printf("    Status: %s\n", created.Status)
		fmt.Printf("    Category: %s\n", created.CategoryName)
		if created.Salary != "" {
			fmt.Printf("    Salary: %s\n", created.Salary)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	jobsCmd.AddCommand(addCmd)

	addCmd.Flags().StringVar(&addFlags.status, "status", "Not Applied", "Application status")
	addCmd.Flags().StringVar(&addFlags.category, "category", "", "Job category (optional, defaults to uncategorized)")
	addCmd.Flags().StringVar(&addFlags.salary, "salary", "", "Salary range")
	addCmd.Flags().StringVar(&addFlags.location, "location", "", "Job location")
	addCmd.Flags().StringVar(&addFlags.contact, "contact", "", "Contact person or email")
	addCmd.Flags().StringVar(&addFlags.url, "url", "", "Job posting URL")
	addCmd.Flags().StringVar(&addFlags.notes, "notes", "", "Notes about the job (inline)")
	addCmd.Flags().StringVar(&addFlags.notesFile, "notes-file", "", "Read notes from a file (overrides --notes)")
	addCmd.Flags().StringVar(&addFlags.date, "date", "", "Deadline date (YYYY-MM-DD)")
	addCmd.Flags().StringVar(&addFlags.appliedDate, "applied-date", "", "Date applied (YYYY-MM-DD)")
	addCmd.Flags().StringVar(&addFlags.reminderDate, "reminder-date", "", "Follow-up reminder (datetime-local)")
}
