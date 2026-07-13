package cli

import (
	"fmt"
	"strings"

	"github.com/SwatiBio/waypoint/internal/db"
	"github.com/spf13/cobra"
)

var listFlags struct {
	status   string
	category string
	search   string
	limit    int
	all      bool
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List job applications",
	Long: `List job applications with optional filtering.

Examples:
  waypoint jobs list                    # All jobs
  waypoint jobs list --status Applied   # Only applied jobs
  waypoint jobs list --category Tech    # Tech category
  waypoint jobs list --search "google"  # Search company/position/notes
  waypoint jobs list --json             # Machine-readable output`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		jobs, err := db.ListJobs(store, db.ListOpts{
			Search:   listFlags.search,
			Status:   listFlags.status,
			Category: listFlags.category,
		})
		if err != nil {
			return formatError("failed to list jobs", err)
		}

		// Apply limit
		if listFlags.limit > 0 && len(jobs) > listFlags.limit {
			jobs = jobs[:listFlags.limit]
		}

		if jsonOut {
			if jobs == nil {
				jobs = []db.Job{}
			}
			printJSON(jobs)
			return nil
		}

		fmt.Println()
		if len(jobs) == 0 {
			fmt.Println("  No jobs found.")
			fmt.Println()
			return nil
		}

		fmt.Printf("  %d job(s) found\n\n", len(jobs))

		// Only show table if we have just a few jobs, or simplify
		if len(jobs) > 50 && !listFlags.all {
			fmt.Println("  (Too many to display. Use --status/--category/--search to filter, or --all)")
			fmt.Println()
			if jsonOut {
				printJSON(jobs)
			}
			return nil
		}

		rows := make([][]string, 0, len(jobs))
		for _, j := range jobs {
			rows = append(rows, []string{
				fmt.Sprintf("%d", j.ID),
				truncate(j.Company, 28),
				truncate(j.Position, 30),
				j.Status,
				j.CategoryName,
				formatDateShort(j.UpdatedAt),
			})
		}

		fmt.Println(formatTable(
			[]string{"ID", "Company", "Position", "Status", "Category", "Updated"},
			rows,
		))
		fmt.Println()
		return nil
	},
}

func init() {
	jobsCmd.AddCommand(listCmd)

	listCmd.Flags().StringVar(&listFlags.status, "status", "", "Filter by status")
	listCmd.Flags().StringVar(&listFlags.category, "category", "", "Filter by category")
	listCmd.Flags().StringVar(&listFlags.search, "search", "", "Search in company, position, notes")
	listCmd.Flags().IntVar(&listFlags.limit, "limit", 0, "Max results")
	listCmd.Flags().BoolVar(&listFlags.all, "all", false, "Show all results regardless of count")
}

// formatDateShort formats an RFC3339 timestamp as YYYY-MM-DD.
func formatDateShort(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}

// formatDateTime formats an RFC3339 timestamp as YYYY-MM-DD HH:MM.
func formatDateTime(ts string) string {
	if len(ts) >= 16 {
		return strings.Replace(ts[:16], "T", " ", 1)
	}
	return ts
}
