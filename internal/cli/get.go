package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/db"
)

var getFlags struct {
	history bool
}

var getCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show job details",
	Long: `Show full details of a job application by its ID.

Examples:
  waypoint jobs get 42
  waypoint jobs get 42 --history    # Include activity history
  waypoint jobs get 42 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid job ID: %s", args[0])
		}

		job, err := store.GetJob(id)
		if err != nil {
			return fmt.Errorf("job %d not found", id)
		}

		if jsonOut {
			if getFlags.history {
				history, err := store.GetJobHistory(id)
				if err != nil {
					return fmt.Errorf("get history: %w", err)
				}
				if history == nil {
					history = []db.HistoryEntry{}
				}
				printJSON(struct {
					Job     any               `json:"job"`
					History []db.HistoryEntry `json:"history"`
				}{Job: job, History: history})
			} else {
				printJSON(job)
			}
			return nil
		}

		fmt.Println()
		fmt.Printf("  ID:          %d\n", job.ID)
		fmt.Printf("  Company:     %s\n", job.Company)
		fmt.Printf("  Position:    %s\n", job.Position)
		fmt.Printf("  Status:      %s\n", job.Status)
		fmt.Printf("  Category:    %s\n", job.CategoryName)
		if job.Salary != "" {
			fmt.Printf("  Salary:      %s\n", job.Salary)
		}
		if job.Location != "" {
			fmt.Printf("  Location:    %s\n", job.Location)
		}
		if job.Contact != "" {
			fmt.Printf("  Contact:     %s\n", job.Contact)
		}
		if job.Date != "" {
			fmt.Printf("  Deadline:    %s\n", job.Date)
		}
		if job.AppliedDate != "" {
			fmt.Printf("  Applied:     %s\n", job.AppliedDate)
		}
		if job.URL != "" {
			fmt.Printf("  URL:         %s\n", job.URL)
		}
		if job.ReminderDate != nil && *job.ReminderDate != "" {
			fmt.Printf("  Reminder:    %s\n", *job.ReminderDate)
		}
		fmt.Printf("  Created:     %s\n", formatDateTime(job.CreatedAt))
		fmt.Printf("  Updated:     %s\n", formatDateTime(job.UpdatedAt))

		if job.Notes != "" {
			fmt.Println()
			fmt.Println("  Notes:")
			fmt.Printf("    %s\n", job.Notes)
		}

		if getFlags.history {
			history, err := store.GetJobHistory(id)
			if err != nil {
				return fmt.Errorf("get history: %w", err)
			}

			fmt.Println()
			if len(history) == 0 {
				fmt.Println("  History: (none)")
			} else {
				fmt.Println("  History:")
				for _, h := range history {
					action := h.Action
					switch action {
					case "Status":
						action = fmt.Sprintf("Status: %s → %s", h.From, h.To)
					}
					fmt.Printf("    %s  %s\n", formatDateTime(h.Timestamp), action)
				}
			}
		}

		fmt.Println()
		return nil
	},
}

func init() {
	jobsCmd.AddCommand(getCmd)
	getCmd.Flags().BoolVar(&getFlags.history, "history", false, "Show activity history")
}
