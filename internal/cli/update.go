package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/udit-001/waypoint/internal/db"
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
	notesFile    string
	date         string
	appliedDate  string
	reminderDate string
}

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a job application's fields",
	Long: `Update one or more fields of a job application by its ID.

Only the flags you provide are changed — all other fields remain untouched.
Pass an empty value to clear a field (e.g. --date "" to remove the deadline).

Examples:
  waypoint jobs update 42 --status Offer --notes "Got the offer!"
  waypoint jobs update 42 --notes-file /tmp/notes.txt
  waypoint jobs update 42 --company "Google LLC" --position "Senior Engineer"
  waypoint jobs update 42 --salary "$180k" --location "Mountain View, CA"
  waypoint jobs update 42 --date "" --notes ""`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid job ID: %s", args[0])
		}

		// Read notes from file if provided (overrides --notes)
		notesChanged := cmd.Flags().Changed("notes")
		if updateFlags.notesFile != "" {
			content, err := os.ReadFile(updateFlags.notesFile)
			if err != nil {
				return fmt.Errorf("reading notes file: %w", err)
			}
			updateFlags.notes = string(content)
			notesChanged = true
		}

		// Build updates map from explicitly-changed flags
		updates := make(map[string]any)
		flags := []struct {
			flagName string
			column   string
			val      *string
		}{
			{"company", "company", &updateFlags.company},
			{"position", "position", &updateFlags.position},
			{"status", "status", &updateFlags.status},
			{"salary", "salary", &updateFlags.salary},
			{"location", "location", &updateFlags.location},
			{"contact", "contact", &updateFlags.contact},
			{"url", "url", &updateFlags.url},
			{"date", "date", &updateFlags.date},
			{"applied-date", "applied_date", &updateFlags.appliedDate},
			{"reminder-date", "reminder_date", &updateFlags.reminderDate},
		}

		for _, f := range flags {
			if cmd.Flags().Changed(f.flagName) {
				updates[f.column] = *f.val
			}
		}

		// Notes handled separately due to --notes-file override
		if notesChanged {
			updates["notes"] = updateFlags.notes
		}

		// Resolve category name → ID (empty string means uncategorized)
		if cmd.Flags().Changed("category") {
			if updateFlags.category == "" {
				updates["category_id"] = int64(0)
			} else {
				catID, err := store.CategoryIDByName(updateFlags.category)
				if err != nil {
					return formatError("failed to resolve category", err)
				}
				if catID == 0 {
					return fmt.Errorf("category %q not found — use 'waypoint categories list' to see available categories", updateFlags.category)
				}
				updates["category_id"] = catID
			}
		}

		if len(updates) == 0 {
			return fmt.Errorf("no fields to update — use --flags to specify changes")
		}

		updated, err := db.IntakeUpdateJob(store, id, updates)
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
			case "category_id":
				fmt.Printf("    Category: %s\n", updated.CategoryName)
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
	jobsCmd.AddCommand(updateCmd)
	updateCmd.Flags().StringVar(&updateFlags.company, "company", "", "Company or institution name")
	updateCmd.Flags().StringVar(&updateFlags.position, "position", "", "Job title or position")
	updateCmd.Flags().StringVar(&updateFlags.status, "status", "", "Application status")
	updateCmd.Flags().StringVar(&updateFlags.category, "category", "", "Job category (pass \"\" to uncategorize)")
	updateCmd.Flags().StringVar(&updateFlags.salary, "salary", "", "Salary range")
	updateCmd.Flags().StringVar(&updateFlags.location, "location", "", "Job location")
	updateCmd.Flags().StringVar(&updateFlags.contact, "contact", "", "Contact person or email")
	updateCmd.Flags().StringVar(&updateFlags.url, "url", "", "Job posting URL")
	updateCmd.Flags().StringVar(&updateFlags.notes, "notes", "", "Notes about the job (pass \"\" to clear)")
	updateCmd.Flags().StringVar(&updateFlags.notesFile, "notes-file", "", "Read notes from a file (overrides --notes)")
	updateCmd.Flags().StringVar(&updateFlags.date, "date", "", "Deadline date YYYY-MM-DD (pass \"\" to clear)")
	updateCmd.Flags().StringVar(&updateFlags.appliedDate, "applied-date", "", "Date applied YYYY-MM-DD (pass \"\" to clear)")
	updateCmd.Flags().StringVar(&updateFlags.reminderDate, "reminder-date", "", "Follow-up reminder (pass \"\" to clear)")
}
