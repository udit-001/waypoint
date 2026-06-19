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
	Short: "Update a job application",
	Long: `Update fields of a job application by its ID.
At least one field flag must be provided.

Examples:
  job-tracker update 42 --status "Offer"
  job-tracker update 42 --notes "Phone screen went well"
  job-tracker update 42 --company "New Corp" --position "CTO"
  job-tracker update 42 --salary "$200k" --location "Remote"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid job ID: %s", args[0])
		}

		// Build updates map from non-empty flags
		updates := make(map[string]any)
		if cmd.Flags().Changed("company") {
			updates["company"] = updateFlags.company
		}
		if cmd.Flags().Changed("position") {
			updates["position"] = updateFlags.position
		}
		if cmd.Flags().Changed("status") {
			updates["status"] = updateFlags.status
		}
		if cmd.Flags().Changed("category") {
			updates["category"] = updateFlags.category
		}
		if cmd.Flags().Changed("salary") {
			updates["salary"] = updateFlags.salary
		}
		if cmd.Flags().Changed("location") {
			updates["location"] = updateFlags.location
		}
		if cmd.Flags().Changed("contact") {
			updates["contact"] = updateFlags.contact
		}
		if cmd.Flags().Changed("url") {
			updates["url"] = updateFlags.url
		}
		if cmd.Flags().Changed("notes") {
			updates["notes"] = updateFlags.notes
		}
		if cmd.Flags().Changed("date") {
			updates["date"] = updateFlags.date
		}
		if cmd.Flags().Changed("applied-date") {
			updates["applied_date"] = updateFlags.appliedDate
		}
		if cmd.Flags().Changed("reminder-date") {
			updates["reminder_date"] = updateFlags.reminderDate
		}

		if len(updates) == 0 {
			return fmt.Errorf("at least one field flag required (e.g. --status, --notes)")
		}

		job, err := store.UpdateJob(id, updates)
		if err != nil {
			return formatError("failed to update job", err)
		}

		if jsonOut {
			printJSON(job)
			return nil
		}

		fmt.Println()
		fmt.Printf("  ✓ Job updated: %s — %s\n", job.Company, job.Position)
		fmt.Printf("    ID: %d\n", job.ID)
		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().StringVar(&updateFlags.company, "company", "", "Company name")
	updateCmd.Flags().StringVar(&updateFlags.position, "position", "", "Job title")
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
