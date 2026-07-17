package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/scraper"
	_ "github.com/udit-001/waypoint/internal/scraper/bitspilani"      // activate BITS Pilani scraper
	_ "github.com/udit-001/waypoint/internal/scraper/ccmb"            // activate CCMB scraper
	_ "github.com/udit-001/waypoint/internal/scraper/google"          // activate Google Jobs scraper
	_ "github.com/udit-001/waypoint/internal/scraper/icgeb"           // activate ICGEB scraper
	_ "github.com/udit-001/waypoint/internal/scraper/iisc"            // activate IISc scraper
	_ "github.com/udit-001/waypoint/internal/scraper/iisertirupati"   // activate IISER Tirupati scraper
	_ "github.com/udit-001/waypoint/internal/scraper/indeed"          // activate Indeed scraper
	_ "github.com/udit-001/waypoint/internal/scraper/indiabioscience" // activate IndiaBioscience aggregator
	_ "github.com/udit-001/waypoint/internal/scraper/instem"          // activate inStem scraper
	_ "github.com/udit-001/waypoint/internal/scraper/ipu"             // activate GGSIPU scraper
	_ "github.com/udit-001/waypoint/internal/scraper/jncasr"          // activate JNCASR scraper
	_ "github.com/udit-001/waypoint/internal/scraper/linkedin"        // activate LinkedIn scraper
	_ "github.com/udit-001/waypoint/internal/scraper/manipal"         // activate MAHE Manipal scraper
	_ "github.com/udit-001/waypoint/internal/scraper/nabi"            // activate NABI scraper
	_ "github.com/udit-001/waypoint/internal/scraper/ncbs"            // activate NCBS scraper
	_ "github.com/udit-001/waypoint/internal/scraper/niab"            // activate NIAB scraper
	_ "github.com/udit-001/waypoint/internal/scraper/nipgr"           // activate NIPGR scraper
	_ "github.com/udit-001/waypoint/internal/scraper/vit"             // activate VIT Vellore scraper
)

func defaultStagingPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "scrape-cache.json"
	}
	return filepath.Join(home, ".waypoint", "scrape-cache.json")
}

// legacyStagingHint checks for a legacy scrape-cache.json file and prints
// a migration hint if one exists without a corresponding .migrated marker.
func legacyStagingHint() {
	path := defaultStagingPath()
	migrated := path + ".migrated"

	if _, err := os.Stat(migrated); err == nil {
		return // already migrated
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return // no legacy file
	}
	var data map[string]scraper.StagedResult
	if err := json.Unmarshal(raw, &data); err != nil {
		return // corrupt — skip hint
	}
	if len(data) == 0 {
		return
	}
	fmt.Printf("  Found legacy scrape-cache.json — run 'waypoint scrape migrate' to import %d staged results.\n", len(data))
}

var scrapeCmd = &cobra.Command{
	Use:   "scrape",
	Short: "Search job portals for new postings",
	Long: `Search job portals for new postings, stage them for review, and
promote relevant ones into the tracked jobs table.

Examples:
  waypoint scrape list
  waypoint scrape run ncbs -q "research"
  waypoint scrape run ncbs --json`,
}

// --- scrape list ---

var scrapeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered job scrapers",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		legacyStagingHint()

		scrapers := scraper.All()

		type info struct {
			Name       string   `json:"name"`
			Source     string   `json:"source"`
			Categories []string `json:"categories"`
		}

		out := make([]info, 0, len(scrapers))
		for _, s := range scrapers {
			out = append(out, info{
				Name:       s.Name(),
				Source:     s.Source(),
				Categories: s.Categories(),
			})
		}

		if jsonOut {
			printJSON(out)
			return nil
		}

		if len(out) == 0 {
			fmt.Println("  No scrapers available.")
			return nil
		}

		rows := make([][]string, 0, len(out))
		for _, s := range out {
			rows = append(rows, []string{
				s.Name,
				s.Source,
				fmt.Sprintf("%v", s.Categories),
			})
		}

		fmt.Println()
		fmt.Println(formatTable([]string{"Name", "Source", "Categories"}, rows))
		fmt.Println()
		return nil
	},
}

// --- scrape run ---

var scrapeRunFlags struct {
	query    string
	location string
	limit    int
	jobage   int
	remote   string
	page     int
}

var scrapeRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Run a job scraper and print results",
	Long: `Fetch job postings from a portal, stage them to the database,
and print only new results (deduplicated against staging and the jobs table).

Examples:
  waypoint scrape run ncbs -q "research"
  waypoint scrape run ncbs --json
  waypoint scrape run ncbs -q "officer" --limit 5`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		legacyStagingHint()

		name := args[0]
		s, ok := scraper.Get(name)
		if !ok {
			return fmt.Errorf("unknown scraper %q — run 'waypoint scrape list' to see available", name)
		}

		results, err := s.Search(context.Background(), scraper.SearchOpts{
			Query:    scrapeRunFlags.query,
			Location: scrapeRunFlags.location,
			Limit:    scrapeRunFlags.limit,
			JobAge:   scrapeRunFlags.jobage,
			Remote:   scrapeRunFlags.remote,
			Page:     scrapeRunFlags.page,
		})
		if err != nil {
			return formatError("scrape failed", err)
		}

		results = scraper.Truncate(results, scrapeRunFlags.limit)

		// Dedup: filter out results already in staging or already tracked as jobs
		var newResults []scraper.Result
		for _, r := range results {
			seen, err := store.IsSeen(r.URL)
			if err != nil {
				return formatError("check staging", err)
			}
			if seen {
				continue
			}
			tracked, err := store.JobExists(r.URL)
			if err != nil {
				return formatError("check jobs", err)
			}
			if tracked {
				continue
			}
			newResults = append(newResults, r)
		}

		// Stage all new results before printing
		if len(newResults) > 0 {
			if err := store.AddStaging(newResults); err != nil {
				return formatError("stage results", err)
			}
		}

		if jsonOut {
			meta := map[string]any{"count": len(newResults)}
			printJSON(map[string]any{
				"meta":    meta,
				"results": newResults,
			})
			return nil
		}

		if len(newResults) == 0 {
			fmt.Printf("  No new positions found at %s.\n", s.Source())
			return nil
		}

		fmt.Printf("  %d new position(s) found at %s\n\n", len(newResults), s.Source())

		rows := make([][]string, 0, len(newResults))
		for _, r := range newResults {
			rows = append(rows, []string{
				r.ID,
				truncate(r.Title, 50),
				truncate(r.Company, 20),
				truncate(r.Location, 20),
				r.Date,
			})
		}

		fmt.Println(formatTable([]string{"ID", "Title", "Company", "Location", "Date"}, rows))
		fmt.Println()
		return nil
	},
}

// --- scrape staged ---

var scrapeStagedFlags struct {
	status string
}

var scrapeStagedCmd = &cobra.Command{
	Use:   "staged",
	Short: "View staged scrape results",
	Long: `List results that have been scraped and staged to the database.
Optionally filter by status.

Examples:
  waypoint scrape staged
  waypoint scrape staged --status new
  waypoint scrape staged --status dismissed --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		legacyStagingHint()

		results, err := store.ListStaging(scrapeStagedFlags.status)
		if err != nil {
			return formatError("list staging", err)
		}

		if jsonOut {
			if results == nil {
				results = []scraper.StagedResult{}
			}
			printJSON(results)
			return nil
		}

		if len(results) == 0 {
			fmt.Println("  No staged results. Run 'waypoint scrape run <name>' to search.")
			return nil
		}

		fmt.Printf("  %d staged result(s)\n\n", len(results))

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

// --- scrape dismiss ---

var scrapeDismissFlags struct {
	all bool
}

var scrapeDismissCmd = &cobra.Command{
	Use:   "dismiss [<url>...]",
	Short: "Dismiss staged results",
	Long: `Mark staged scrape results as dismissed so they don't reappear
on future scrape runs.

--all dismisses every "new" status result. Entries that are "dismissed"
or "imported" are skipped.

Examples:
  waypoint scrape dismiss "https://www.ncbs.res.in/jobportal/node/142669"
  waypoint scrape dismiss "url1" "url2" "url3"
  waypoint scrape dismiss --all`,
	RunE: func(cmd *cobra.Command, args []string) error {
		legacyStagingHint()

		if scrapeDismissFlags.all {
			results, err := store.ListStaging("new")
			if err != nil {
				return formatError("list staging", err)
			}

			dismissed := 0
			for _, r := range results {
				if err := store.SetStagingStatus(r.Result.URL, "dismissed"); err != nil {
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

		// Single URL — backward compatible path.
		if len(args) == 1 {
			url := args[0]

			_, ok, err := store.GetStaged(url)
			if err != nil {
				return formatError("check staging", err)
			}
			if !ok {
				return fmt.Errorf("no staged result with URL %q", url)
			}

			if err := store.SetStagingStatus(url, "dismissed"); err != nil {
				return formatError("dismiss", err)
			}

			if jsonOut {
				printJSON(map[string]string{"status": "dismissed", "url": url})
				return nil
			}

			fmt.Printf("  ✓ Dismissed: %s\n", url)
			return nil
		}

		// Multiple URLs — batch path. Errors for non-existent URLs
		// are reported to stderr but processing continues.
		dismissed := 0
		for _, url := range args {
			_, ok, err := store.GetStaged(url)
			if err != nil {
				return formatError("check staging", err)
			}
			if !ok {
				fmt.Fprintf(os.Stderr, "  ✗ no staged result with URL %q\n", url)
				continue
			}
			if err := store.SetStagingStatus(url, "dismissed"); err != nil {
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

// --- scrape detail ---

var scrapeDetailCmd = &cobra.Command{
	Use:   "detail <name> <id>",
	Short: "Fetch full details for a job posting",
	Long: `Fetch the full description, seniority, employment type, job function,
and industries for a job posting. Enriches the staged result if found.

Currently only LinkedIn supports detail fetching.

Examples:
  waypoint scrape detail linkedin 4439995582
  waypoint scrape detail linkedin "https://www.linkedin.com/jobs/view/4439995582"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		legacyStagingHint()

		name := args[0]
		id := args[1]

		s, ok := scraper.Get(name)
		if !ok {
			return fmt.Errorf("unknown scraper %q — run 'waypoint scrape list' to see available", name)
		}

		d, ok := s.(scraper.Detailer)
		if !ok {
			return fmt.Errorf("scraper %q does not support detail", name)
		}

		result, err := d.Detail(context.Background(), id)
		if err != nil {
			return formatError("fetch detail", err)
		}

		if err := store.EnrichStaging(result.URL, result.Description, result.Metadata); err != nil {
			return formatError("enrich staging", err)
		}

		if jsonOut {
			printJSON(result)
			return nil
		}

		fmt.Printf("  %s\n", result.Title)
		fmt.Printf("  %s · %s\n", result.Company, result.Location)
		if result.Description != "" {
			fmt.Printf("\n%s\n", result.Description)
		}
		if len(result.Metadata) > 0 {
			fmt.Println()
			for _, k := range sortedKeys(result.Metadata) {
				fmt.Printf("  %s: %s\n", k, result.Metadata[k])
			}
		}
		fmt.Printf("\n  URL: %s\n", result.URL)
		return nil
	},
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// --- scrape prune ---

var scrapePruneFlags struct {
	days int
}

var scrapePruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove old staged entries",
	Long: `Remove staged results older than N days.
Default: 30 days. Only removes entries — does not affect tracked jobs.

Examples:
  waypoint scrape prune
  waypoint scrape prune --days 7`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		legacyStagingHint()

		removed, err := store.PruneStaging(scrapePruneFlags.days)
		if err != nil {
			return formatError("prune", err)
		}

		if jsonOut {
			printJSON(map[string]int{"removed": removed, "days": scrapePruneFlags.days})
			return nil
		}

		if removed == 0 {
			fmt.Printf("  No entries older than %d days.\n", scrapePruneFlags.days)
			return nil
		}

		fmt.Printf("  ✓ Removed %d entr(y/ies) older than %d days.\n", removed, scrapePruneFlags.days)
		return nil
	},
}

// --- scrape migrate ---

var scrapeMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Import legacy scrape-cache.json into the database",
	Long: `Reads the legacy ~/.waypoint/scrape-cache.json file and imports
each entry into the scrape_staging table. The JSON file is renamed to
scrape-cache.json.migrated after import.

This is a one-time migration — safe to re-run (idempotent).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := defaultStagingPath()
		migrated := path + ".migrated"

		raw, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("  No legacy scrape-cache.json found — nothing to migrate.")
				return nil
			}
			return formatError("read legacy file", err)
		}

		var data map[string]scraper.StagedResult
		if err := json.Unmarshal(raw, &data); err != nil {
			return formatError("parse legacy file", err)
		}

		entries := make([]scraper.StagedResult, 0, len(data))
		for _, sr := range data {
			entries = append(entries, sr)
		}

		imported, err := store.MigrateStaging(entries)
		if err != nil {
			return formatError("migrate staging", err)
		}

		if err := os.Rename(path, migrated); err != nil {
			return formatError("rename legacy file", err)
		}

		if jsonOut {
			printJSON(map[string]int{"imported": imported})
			return nil
		}

		fmt.Printf("  ✓ Imported %d staged result(s).\n", imported)
		return nil
	},
}

// --- scrape promote ---

var scrapePromoteFlags struct {
	all bool
}

var scrapePromoteCmd = &cobra.Command{
	Use:   "promote [<url>]",
	Short: "Promote staged results into the tracked jobs table",
	Long: `Move staged scrape results into the tracked jobs table.

--all promotes every "new" status result. Entries that are "dismissed"
or "imported" are skipped. If a URL already exists in the jobs table,
the result is skipped but still marked "imported" so it won't reappear.

Examples:
  waypoint scrape promote "https://www.ncbs.res.in/jobportal/node/142669"
  waypoint scrape promote --all`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		legacyStagingHint()

		if scrapePromoteFlags.all {
			results, err := store.ListStaging("new")
			if err != nil {
				return formatError("list staging", err)
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

			fmt.Printf("  Promoted %d, skipped %d (already imported).\n", promoted, skipped)
			return nil
		}

		if len(args) == 0 {
			return fmt.Errorf("provide a URL or use --all")
		}

		url := args[0]
		job, err := store.Promote(url)
		if err != nil {
			return formatError("promote", err)
		}

		if jsonOut {
			printJSON(job)
			return nil
		}

		if job.ID > 0 {
			fmt.Printf("  ✓ Promoted: %s → Job #%d\n", url, job.ID)
		} else {
			fmt.Printf("  → Skipped (already tracked): %s\n", url)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(scrapeCmd)
	scrapeCmd.AddCommand(scrapeListCmd)
	scrapeCmd.AddCommand(scrapeRunCmd)
	scrapeCmd.AddCommand(scrapeStagedCmd)
	scrapeCmd.AddCommand(scrapeDismissCmd)
	scrapeCmd.AddCommand(scrapeDetailCmd)
	scrapeCmd.AddCommand(scrapePruneCmd)
	scrapeCmd.AddCommand(scrapeMigrateCmd)
	scrapeCmd.AddCommand(scrapePromoteCmd)

	scrapeRunCmd.Flags().StringVarP(&scrapeRunFlags.query, "query", "q", "", "Filter results by keyword")
	scrapeRunCmd.Flags().StringVarP(&scrapeRunFlags.location, "location", "l", "", "Location to search (e.g. 'Bengaluru, India', 'Remote')")
	scrapeRunCmd.Flags().IntVar(&scrapeRunFlags.limit, "limit", 0, "Max results (0 = all)")
	scrapeRunCmd.Flags().IntVar(&scrapeRunFlags.jobage, "jobage", 90, "Posted within N days (0 = all)")
	scrapeRunCmd.Flags().StringVar(&scrapeRunFlags.remote, "remote", "", "Workplace type: remote|hybrid|onsite (LinkedIn only)")
	scrapeRunCmd.Flags().IntVar(&scrapeRunFlags.page, "page", 1, "Page number, 1-indexed (LinkedIn/Indeed only)")

	scrapeStagedCmd.Flags().StringVar(&scrapeStagedFlags.status, "status", "", "Filter by status (new|dismissed|imported)")

	scrapePruneCmd.Flags().IntVar(&scrapePruneFlags.days, "days", 30, "Remove entries older than N days")

	scrapeDismissCmd.Flags().BoolVar(&scrapeDismissFlags.all, "all", false, "Dismiss all 'new' status results")

	scrapePromoteCmd.Flags().BoolVar(&scrapePromoteFlags.all, "all", false, "Promote all 'new' status results")
}
