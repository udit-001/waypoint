package cli

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/db"
)

var postingsCmd = &cobra.Command{
	Use:   "postings",
	Short: "Review and manage the job posting queue",
	Long: `Manage the posting review queue — scraped job postings waiting
for your decision. Promote the ones you want to track, dismiss
the rest.

The posting/promote/dismiss triad is the vocabulary used both
on the command line and by the autopilot agent.

Examples:
  waypoint postings list
  waypoint postings list --status new
  waypoint postings get "https://boards.greenhouse.io/acme/jobs/123"
  waypoint postings promote "https://boards.greenhouse.io/acme/jobs/123"
  waypoint postings dismiss "https://boards.greenhouse.io/acme/jobs/123"
  waypoint postings prune`,
}

// --- postings list ---

var postingsListFlags struct {
	status string
}

var postingsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List postings in the review queue",
	Long: `List postings that have been scraped into the postings ledger.
Optionally filter by status.

Examples:
  waypoint postings list
  waypoint postings list --status new
  waypoint postings list --status shortlisted --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := store.ListPostings(postingsListFlags.status)
		if err != nil {
			return formatError("list postings", err)
		}

		if jsonOut {
			if results == nil {
				results = []db.Posting{}
			}
			printJSON(results)
			return nil
		}

		if len(results) == 0 {
			fmt.Println("  No postings. Run 'waypoint scrape run <name>' to search.")
			return nil
		}

		fmt.Printf("  %d posting(s)\n\n", len(results))

		rows := make([][]string, 0, len(results))
		for _, r := range results {
			rows = append(rows, []string{
				truncate(r.Result.Title, 45),
				truncate(r.Result.Company, 20),
				truncate(r.Result.URL, 50),
				r.Status,
				r.FirstSeen,
			})
		}

		fmt.Println(formatTable([]string{"Title", "Company", "URL", "Status", "First Seen"}, rows))
		fmt.Println()
		return nil
	},
}

// --- postings get ---

var postingsGetCmd = &cobra.Command{
	Use:   "get <url>",
	Short: "Show details for a single posting",
	Long: `Display the full details of a posting in the review queue.

Examples:
  waypoint postings get "https://boards.greenhouse.io/acme/jobs/123"
  waypoint postings get "https://boards.greenhouse.io/acme/jobs/123" --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rawURL := args[0]
		p, ok, err := store.GetPosting(rawURL)
		if err != nil {
			return formatError("get posting", err)
		}
		if !ok {
			return fmt.Errorf("no posting with URL %q", rawURL)
		}

		if jsonOut {
			printJSON(p)
			return nil
		}

		fmt.Println()
		fmt.Printf("  Title:    %s\n", p.Result.Title)
		fmt.Printf("  Company:  %s\n", p.Result.Company)
		if p.Result.Location != "" {
			fmt.Printf("  Location: %s\n", p.Result.Location)
		}
		if p.Result.Date != "" {
			fmt.Printf("  Date:     %s\n", p.Result.Date)
		}
		fmt.Printf("  Status:   %s\n", p.Status)
		fmt.Printf("  Seen:     %s\n", p.FirstSeen)
		fmt.Printf("  URL:      %s\n", p.Result.URL)
		if p.Result.Description != "" {
			desc := p.Result.Description
			if len(desc) > 200 {
				desc = desc[:197] + "..."
			}
			fmt.Printf("\n  %s\n", desc)
		}
		if len(p.Result.Metadata) > 0 {
			fmt.Println()
			for _, k := range sortedKeys(p.Result.Metadata) {
				fmt.Printf("  %s: %s\n", k, p.Result.Metadata[k])
			}
		}
		fmt.Println()
		return nil
	},
}

// --- postings promote ---

var postingsPromoteFlags struct {
	all bool
}

var postingsPromoteCmd = &cobra.Command{
	Use:   "promote [<url>]",
	Short: "Promote a posting into your applications",
	Long: `Move a posting into the tracked jobs table as "Not Applied".

If the URL already exists in the jobs table, the posting is
skipped but still marked "promoted" so it won't reappear.

--all promotes every "new" status posting.

Examples:
  waypoint postings promote "https://boards.greenhouse.io/acme/jobs/123"
  waypoint postings promote --all`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if postingsPromoteFlags.all {
			results, err := store.ListPostings(db.StatusNew)
			if err != nil {
				return formatError("list postings", err)
			}

			promoted, skipped := 0, 0
			for _, r := range results {
				job, err := store.Promote(r.Result.URL)
				if err != nil {
					return formatError("promote "+r.Result.URL, err)
				}
				if job.ID > 0 {
					promoted++
				} else {
					skipped++
				}
			}

			if jsonOut {
				printJSON(map[string]int{
					"promoted": promoted,
					"skipped":  skipped,
				})
				return nil
			}

			fmt.Printf("  Promoted %d, skipped %d (already tracked).\n", promoted, skipped)
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("provide a URL or use --all")
		}

		rawURL := args[0]
		job, err := store.Promote(rawURL)
		if err != nil {
			return formatError("promote", err)
		}

		if jsonOut {
			printJSON(job)
			return nil
		}

		if job.ID > 0 {
			fmt.Printf("  ✓ Promoted: %s → Job #%d\n", rawURL, job.ID)
		} else {
			fmt.Printf("  → Skipped (already tracked): %s\n", rawURL)
		}
		return nil
	},
}

// --- postings dismiss ---

var postingsDismissFlags struct {
	all bool
}

var postingsDismissCmd = &cobra.Command{
	Use:   "dismiss [<url>...]",
	Short: "Dismiss postings from the queue",
	Long: `Mark postings as dismissed so they don't reappear on future
scrape runs. Dismissals are durable — the autopilot loop
re-encounters URLs every cycle.

--all dismisses every "new" status posting.

Examples:
  waypoint postings dismiss "https://boards.greenhouse.io/acme/jobs/123"
  waypoint postings dismiss "url1" "url2" "url3"
  waypoint postings dismiss --all`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if postingsDismissFlags.all {
			results, err := store.ListPostings(db.StatusNew)
			if err != nil {
				return formatError("list postings", err)
			}

			dismissed := 0
			for _, r := range results {
				if err := store.SetPostingStatus(r.Result.URL, db.StatusDismissed); err != nil {
					return formatError("dismiss "+r.Result.URL, err)
				}
				dismissed++
			}

			if jsonOut {
				printJSON(map[string]int{"dismissed": dismissed})
				return nil
			}

			fmt.Printf("  Dismissed %d results.\n", dismissed)
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("provide at least one URL or use --all")
		}

		// Multiple URLs — batch path.
		dismissed := 0
		for _, rawURL := range args {
			_, ok, err := store.GetPosting(rawURL)
			if err != nil {
				return formatError("check postings", err)
			}
			if !ok {
				fmt.Fprintf(os.Stderr, "  ✗ no posting with URL %q\n", rawURL)
				continue
			}
			if err := store.SetPostingStatus(rawURL, db.StatusDismissed); err != nil {
				return formatError("dismiss", err)
			}
			dismissed++
		}

		if jsonOut {
			printJSON(map[string]int{"dismissed": dismissed})
			return nil
		}

		fmt.Printf("  Dismissed %d results.\n", dismissed)
		return nil
	},
}

// --- postings prune ---

var postingsPruneFlags struct {
	days int
}

var postingsPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove old postings from the queue",
	Long: `Remove postings older than N days.
Default: 30 days. Only removes entries — does not affect tracked jobs.

Examples:
  waypoint postings prune
  waypoint postings prune --days 7`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		removed, err := store.PrunePostings(postingsPruneFlags.days)
		if err != nil {
			return formatError("prune", err)
		}

		if jsonOut {
			printJSON(map[string]int{"removed": removed, "days": postingsPruneFlags.days})
			return nil
		}

		if removed == 0 {
			fmt.Printf("  No entries older than %d days.\n", postingsPruneFlags.days)
			return nil
		}

		fmt.Printf("  ✓ Removed %d entr(y/ies) older than %d days.\n", removed, postingsPruneFlags.days)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(postingsCmd)
	postingsCmd.AddCommand(postingsListCmd)
	postingsCmd.AddCommand(postingsGetCmd)
	postingsCmd.AddCommand(postingsPromoteCmd)
	postingsCmd.AddCommand(postingsDismissCmd)
	postingsCmd.AddCommand(postingsPruneCmd)

	postingsListCmd.Flags().StringVar(&postingsListFlags.status, "status", "", "Filter by status (new|shortlisted|dismissed|promoted)")
	postingsPromoteCmd.Flags().BoolVar(&postingsPromoteFlags.all, "all", false, "Promote all 'new' status postings")
	postingsDismissCmd.Flags().BoolVar(&postingsDismissFlags.all, "all", false, "Dismiss all 'new' status postings")
	postingsPruneCmd.Flags().IntVar(&postingsPruneFlags.days, "days", 30, "Remove entries older than N days")
}

// urlEncodePathSegment encodes a URL for safe use as a path value in
// http.NewRequest patterns. The {url} pattern receives the raw URL as
// a path segment, so special characters must be escaped.
func urlEncodePathSegment(raw string) string {
	return url.PathEscape(raw)
}

// --- scrape aliases (backward compat) ---

// These alias the old scrape-group staging commands to the new postings
// vocabulary. They delegate directly to the postings commands.

var scrapeStagedAlias = &cobra.Command{
	Use:        "staged",
	Deprecated: "use 'waypoint postings list' instead",
	Short:      "View postings in the review queue (deprecated: use 'waypoint postings list')",
	Args:       cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		postingsListFlags.status = scrapeStagedFlags.status
		return postingsListCmd.RunE(cmd, args)
	},
}

var scrapePromoteAlias = &cobra.Command{
	Use:        "promote [<url>]",
	Deprecated: "use 'waypoint postings promote' instead",
	Short:      "Promote postings (deprecated: use 'waypoint postings promote')",
	Args:       cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		postingsPromoteFlags.all = scrapePromoteFlags.all
		return postingsPromoteCmd.RunE(cmd, args)
	},
}

var scrapeDismissAlias = &cobra.Command{
	Use:        "dismiss [<url>...]",
	Deprecated: "use 'waypoint postings dismiss' instead",
	Short:      "Dismiss postings (deprecated: use 'waypoint postings dismiss')",
	RunE: func(cmd *cobra.Command, args []string) error {
		postingsDismissFlags.all = scrapeDismissFlags.all
		return postingsDismissCmd.RunE(cmd, args)
	},
}

// registerScrapeAliases wires the deprecated scrape staging commands
// to the new postings commands. Called from scrape.go init().
func registerScrapeAliases() {
	// Replace scrape staged with alias.
	for i, c := range scrapeCmd.Commands() {
		if c.Name() == "staged" {
			scrapeCmd.Commands()[i] = scrapeStagedAlias
			break
		}
	}
	// Replace scrape promote with alias.
	for i, c := range scrapeCmd.Commands() {
		if c.Name() == "promote" {
			scrapeCmd.Commands()[i] = scrapePromoteAlias
			break
		}
	}
	// Replace scrape dismiss with alias.
	for i, c := range scrapeCmd.Commands() {
		if c.Name() == "dismiss" {
			scrapeCmd.Commands()[i] = scrapeDismissAlias
			break
		}
	}
	_ = strings.TrimSpace // keep import used
}
