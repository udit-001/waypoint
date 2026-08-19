package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/autopilot"
	"github.com/udit-001/waypoint/internal/detail"
)

var detailCmd = &cobra.Command{
	Use:   "detail <url>",
	Short: "Fetch full details for a posting via the four-tier chain",
	Long: `Fetch the full job-posting body for a URL using the four-tier detail
chain. Tiers are tried in order — board provider, scraper detailer,
Exa fetch+parse, raw — and the first success wins.

The result is merged into the posting row in the database (description,
salary, deadline, requirements, location, provenance). The raw markdown
is always retained as a degradation path for curation.

Examples:
  waypoint detail https://boards.greenhouse.io/spacex/jobs/7690206
  waypoint detail https://jobs.lever.co/kitware/1d458d7b
  waypoint detail https://jobs.ashbyhq.com/applied/b2238f05 --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rawURL := args[0]

		// Look up the posting in the database.
		posting, ok, err := store.GetPosting(rawURL)
		if err != nil {
			return formatError("lookup posting", err)
		}

		// Build the chain dependencies.
		chain := buildDetailChain()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		var boardURL, boardID string
		var existingDesc string
		var existingMeta map[string]string

		if ok {
			boardURL = autopilot.FindBoardURL(rawURL)
			boardID = detail.NormalizeBoardID(rawURL)
			existingDesc = posting.Result.Description
			existingMeta = posting.Result.Metadata
		} else {
			boardURL = autopilot.FindBoardURL(rawURL)
			boardID = detail.NormalizeBoardID(rawURL)
		}

		detailResult, err := chain.FetchDetail(ctx, rawURL, boardURL, boardID, existingDesc, existingMeta)
		if err != nil {
			return formatError("fetch detail", err)
		}

		// Merge into the posting if it exists.
		if ok {
			newDesc, newMeta := detail.MergeDetailResultInto(
				posting.Result.Description,
				posting.Result.Metadata,
				detailResult,
			)
			if err := store.EnrichPosting(rawURL, newDesc, newMeta); err != nil {
				return formatError("enrich posting", err)
			}
		}

		if jsonOut {
			printJSON(map[string]any{
				"url":    rawURL,
				"source": detailResult.Source,
				"fields": detailResult,
			})
			return nil
		}

		fmt.Println()
		fmt.Printf("  Source: %s\n", detailResult.Source)
		if detailResult.Title != "" {
			fmt.Printf("  Title:  %s\n", detailResult.Title)
		}
		if detailResult.Company != "" {
			fmt.Printf("  Co:     %s\n", detailResult.Company)
		}
		if detailResult.Location != "" {
			fmt.Printf("  Loc:    %s\n", detailResult.Location)
		}
		if detailResult.Salary != "" {
			fmt.Printf("  Salary: %s\n", detailResult.Salary)
		}
		if detailResult.Deadline != "" {
			fmt.Printf("  Due:    %s\n", detailResult.Deadline)
		}
		if detailResult.Requirements != "" {
			reqs := detailResult.Requirements
			if len(reqs) > 120 {
				reqs = reqs[:117] + "..."
			}
			fmt.Printf("  Reqs:   %s\n", reqs)
		}
		if detailResult.Description != "" {
			desc := detailResult.Description
			if len(desc) > 200 {
				desc = desc[:197] + "..."
			}
			fmt.Printf("\n  %s\n", desc)
		}
		fmt.Println()
		return nil
	},
}

// buildDetailChain delegates to autopilot's construction site so the
// tier order is identical for the CLI and the autopilot cycle.
func buildDetailChain() *detail.Chain {
	return autopilot.BuildDetailChain()
}

func init() {
	rootCmd.AddCommand(detailCmd)
}
