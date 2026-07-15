package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

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
	"github.com/spf13/cobra"
)

func defaultStagingPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "scrape-cache.json"
	}
	return filepath.Join(home, ".waypoint", "scrape-cache.json")
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
	Long: `Fetch job postings from a portal, stage them to a seen-cache file,
and print only new results (deduplicated against staging and the jobs table).

Examples:
  waypoint scrape run ncbs -q "research"
  waypoint scrape run ncbs --json
  waypoint scrape run ncbs -q "officer" --limit 5`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Open staging file
		staging, err := scraper.OpenStaging(defaultStagingPath())
		if err != nil {
			return formatError("open staging", err)
		}

		// Dedup: filter out results already in staging or already tracked as jobs
		var newResults []scraper.Result
		for _, r := range results {
			if staging.IsSeen(r.URL) {
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
			if err := staging.Add(newResults); err != nil {
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
	Long: `List results that have been scraped and staged to the seen-cache file.
Optionally filter by status.

Examples:
  waypoint scrape staged
  waypoint scrape staged --status new
  waypoint scrape staged --status dismissed --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		staging, err := scraper.OpenStaging(defaultStagingPath())
		if err != nil {
			return formatError("open staging", err)
		}

		results := staging.List(scrapeStagedFlags.status)

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
				r.Result.Company,
				r.Status,
				r.FirstSeen,
			})
		}

		fmt.Println(formatTable([]string{"Title", "Source", "Status", "First Seen"}, rows))
		fmt.Println()
		return nil
	},
}

// --- scrape dismiss ---

var scrapeDismissCmd = &cobra.Command{
	Use:   "dismiss <url>",
	Short: "Dismiss a staged result",
	Long: `Mark a staged scrape result as dismissed so it doesn't reappear
on future scrape runs.

Examples:
  waypoint scrape dismiss "https://www.ncbs.res.in/jobportal/node/142669"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		staging, err := scraper.OpenStaging(defaultStagingPath())
		if err != nil {
			return formatError("open staging", err)
		}

		if !staging.IsSeen(url) {
			return fmt.Errorf("no staged result with URL %q", url)
		}

		if err := staging.Dismiss(url); err != nil {
			return formatError("dismiss", err)
		}

		if jsonOut {
			printJSON(map[string]string{"status": "dismissed", "url": url})
			return nil
		}

		fmt.Printf("  ✓ Dismissed: %s\n", url)
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

		staging, err := scraper.OpenStaging(defaultStagingPath())
		if err != nil {
			return formatError("open staging", err)
		}
		if err := staging.Enrich(result.ID, result.Description, result.Metadata); err != nil {
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
		staging, err := scraper.OpenStaging(defaultStagingPath())
		if err != nil {
			return formatError("open staging", err)
		}

		removed, err := staging.Prune(scrapePruneFlags.days)
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

func init() {
	rootCmd.AddCommand(scrapeCmd)
	scrapeCmd.AddCommand(scrapeListCmd)
	scrapeCmd.AddCommand(scrapeRunCmd)
	scrapeCmd.AddCommand(scrapeStagedCmd)
	scrapeCmd.AddCommand(scrapeDismissCmd)
	scrapeCmd.AddCommand(scrapeDetailCmd)
	scrapeCmd.AddCommand(scrapePruneCmd)

	scrapeRunCmd.Flags().StringVarP(&scrapeRunFlags.query, "query", "q", "", "Filter results by keyword")
	scrapeRunCmd.Flags().StringVarP(&scrapeRunFlags.location, "location", "l", "", "Location to search (e.g. 'Bengaluru, India', 'Remote')")
	scrapeRunCmd.Flags().IntVar(&scrapeRunFlags.limit, "limit", 0, "Max results (0 = all)")
	scrapeRunCmd.Flags().IntVar(&scrapeRunFlags.jobage, "jobage", 0, "Posted within N days (0 = all). LinkedIn/Indeed only.")
	scrapeRunCmd.Flags().StringVar(&scrapeRunFlags.remote, "remote", "", "Workplace type: remote|hybrid|onsite (LinkedIn only)")
	scrapeRunCmd.Flags().IntVar(&scrapeRunFlags.page, "page", 1, "Page number, 1-indexed (LinkedIn/Indeed only)")

	scrapeStagedCmd.Flags().StringVar(&scrapeStagedFlags.status, "status", "", "Filter by status (new|dismissed)")

	scrapePruneCmd.Flags().IntVar(&scrapePruneFlags.days, "days", 30, "Remove entries older than N days")
}
