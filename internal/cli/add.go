package cli

import (
	"fmt"

	"github.com/SwatiBio/job-tracker/internal/db"
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
  job-tracker add "Acme Corp" "Senior Engineer"
  job-tracker add "Acme Corp" "Senior Engineer" --status Applied --salary "$150k"
  job-tracker add "Acme Corp" "Senior Engineer" --notes "Applied via referral"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		job := db.Job{
			Company:      args[0],
			Position:     args[1],
			Status:       addFlags.status,
			Category:     addFlags.category,
			Salary:       addFlags.salary,
			Location:     addFlags.location,
			Contact:      addFlags.contact,
			URL:          addFlags.url,
			Notes:        addFlags.notes,
			Date:         addFlags.date,
			AppliedDate:  addFlags.appliedDate,
			ReminderDate: nil,
		}

		if addFlags.reminderDate != "" {
			job.ReminderDate = &addFlags.reminderDate
		}

		created, err := store.AddJob(job)
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
		fmt.Printf("    Category: %s\n", created.Category)
		if created.Salary != "" {
			fmt.Printf("    Salary: %s\n", created.Salary)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)

	addCmd.Flags().StringVar(&addFlags.status, "status", "Not Applied", "Application status")
	addCmd.Flags().StringVar(&addFlags.category, "category", "General", "Job category")
	addCmd.Flags().StringVar(&addFlags.salary, "salary", "", "Salary range")
	addCmd.Flags().StringVar(&addFlags.location, "location", "", "Job location")
	addCmd.Flags().StringVar(&addFlags.contact, "contact", "", "Contact person or email")
	addCmd.Flags().StringVar(&addFlags.url, "url", "", "Job posting URL")
	addCmd.Flags().StringVar(&addFlags.notes, "notes", "", "Notes about the job")
	addCmd.Flags().StringVar(&addFlags.date, "date", "", "Deadline date (YYYY-MM-DD)")
	addCmd.Flags().StringVar(&addFlags.appliedDate, "applied-date", "", "Date applied (YYYY-MM-DD)")
	addCmd.Flags().StringVar(&addFlags.reminderDate, "reminder-date", "", "Follow-up reminder (datetime-local)")
}
