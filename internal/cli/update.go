package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var updateFlags struct {
	company      string
	position     string
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

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a job application's fields",
	Long: `Update one or more fields of a job application by its ID.

Only the flags you provide are changed — all other fields remain untouched.

Examples:
  waypoint update 42 --status Offer --notes "Got the offer!"
  waypoint update 42 --company "Google LLC" --position "Senior Engineer"
  waypoint update 42 --salary "$180k" --location "Mountain View, CA"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid job ID: %s", args[0])
		}

		// Build updates map from non-empty flags
		updates := make(map[string]any)
		flags := map[string]*string{
			"company":       &updateFlags.company,
			"position":      &updateFlags.position,
			"status":        &updateFlags.status,
			"category":      &updateFlags.category,
			"salary":        &updateFlags.salary,
			"location":      &updateFlags.location,
			"contact":       &updateFlags.contact,
			"url":           &updateFlags.url,
			"notes":         &updateFlags.notes,
			"date":          &updateFlags.date,
			"applied_date":  &updateFlags.appliedDate,
			"reminder_date": &updateFlags.reminderDate,
		}

		for key, val := range flags {
			if *val != "" {
				updates[key] = *val
			}
		}

		if len(updates) == 0 {
			return fmt.Errorf("no fields to update — use --flags to specify changes")
		}

		updated, err := store.UpdateJob(id, updates)
		if err != nil {
			return formatError("failed to update job", err)
		}

		if jsonOut {
			printJSON(updated)
			return nil
		}

		fmt.Println()
		fmt.Printf("  ✓ Updated job %d: %s — %s\n", updated.ID, updated.Company, updated.Position)
		for key := range updates {
			switch key {
			case "company":
				fmt.Printf("    Company:  %s\n", updated.Company)
			case "position":
				fmt.Printf("    Position: %s\n", updated.Position)
			case "status":
				fmt.Printf("    Status:   %s\n", updated.Status)
			case "category":
				fmt.Printf("    Category: %s\n", updated.Category)
			case "salary":
				fmt.Printf("    Salary:   %s\n", updated.Salary)
			case "location":
				fmt.Printf("    Location: %s\n", updated.Location)
			case "contact":
				fmt.Printf("    Contact:  %s\n", updated.Contact)
			case "url":
				fmt.Printf("    URL:      %s\n", updated.URL)
			case "notes":
				fmt.Printf("    Notes:    %s\n", updated.Notes)
			}
		}
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().StringVar(&updateFlags.company, "company", "", "Company or institution name")
	updateCmd.Flags().StringVar(&updateFlags.position, "position", "", "Job title or position")
	updateCmd.Flags().StringVar(&updateFlags.status, "status", "", "Application status")
	updateCmd.Flags().StringVar(&updateFlags.category, "category", "", "Job category")
	updateCmd.Flags().StringVar(&updateFlags.salary, "salary", "", "Salary range")
	updateCmd.Flags().StringVar(&updateFlags.location, "location", "", "Job location")
	updateCmd.Flags().StringVar(&updateFlags.contact, "contact", "", "Contact person or email")
	updateCmd.Flags().StringVar(&updateFlags.url, "url", "", "Job posting URL")
	updateCmd.Flags().StringVar(&updateFlags.notes, "notes", "", "Notes about the job")
	updateCmd.Flags().StringVar(&updateFlags.date, "date", "", "Deadline date (YYYY-MM-DD)")
	updateCmd.Flags().StringVar(&updateFlags.appliedDate, "applied-date", "", "Date applied (YYYY-MM-DD)")
	updateCmd.Flags().StringVar(&updateFlags.reminderDate, "reminder-date", "", "Follow-up reminder (datetime-local)")
}
