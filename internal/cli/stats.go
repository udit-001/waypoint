package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show application statistics",
	Long: `Show summary statistics of your job applications.

Examples:
  waypoint stats
  waypoint stats --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		jobs, err := store.GetJobs()
		if err != nil {
			return formatError("failed to get stats", err)
		}

		total := len(jobs)

		// Count by status
		statusCounts := make(map[string]int)
		categoryCounts := make(map[string]int)
		for _, j := range jobs {
			statusCounts[j.Status]++
			if j.Category != "" {
				categoryCounts[j.Category]++
			}
		}

		statusOrder := []string{"Not Applied", "Applied", "Offer", "Rejected", "Withdrawn"}

		if jsonOut {
			stats := map[string]any{
				"total":          total,
				"byStatus":       statusCounts,
				"byCategory":     categoryCounts,
			}
			printJSON(stats)
			return nil
		}

		fmt.Println()
		fmt.Printf("  Total applications:  %d\n\n", total)

		fmt.Println("  By Status:")
		for _, s := range statusOrder {
			count := statusCounts[s]
			bar := ""
			if total > 0 {
				pct := float64(count) / float64(total) * 100
				barLen := int(pct / 5)
				for i := 0; i < barLen && i < 20; i++ {
					bar += "█"
				}
				if bar == "" && count > 0 {
					bar = "▏"
				}
			}
			fmt.Printf("    %-15s %3d  %s\n", s+":", count, bar)
		}

		if len(categoryCounts) > 0 {
			fmt.Println()
			fmt.Println("  By Category:")
			for cat, count := range categoryCounts {
				fmt.Printf("    %-15s %3d\n", cat+":", count)
			}
		}

		fmt.Println()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
