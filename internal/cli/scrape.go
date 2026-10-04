package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/db"
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
	var data map[string]db.Posting
	if err := json.Unmarshal(raw, &data); err != nil {
		return // corrupt — skip hint
	}
	if len(data) == 0 {
		return
	}
	fmt.Printf("  Found legacy scrape-cache.json — run 'waypoint scrape migrate' to import %d postings.\n", len(data))
}

var scrapeCmd = &cobra.Command{
	Use:   "scrape",
	Short: "Scrape job portals for new postings",
	Long: `Scrape job portals for new postings, add them to the postings
ledger for review, and
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
	today    string
}

var scrapeRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Run a job scraper and print results",
	Long: `Fetch job postings from a portal, add them to the postings ledger,
and print only new results (deduplicated against the postings ledger
and the jobs table).

Examples:
  waypoint scrape run ncbs -q "research"
  waypoint scrape run ncbs --json
  waypoint scrape run ncbs -q "officer" --limit 5
  waypoint scrape run linkedin --today 2026-08-12 --jobage 30`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		legacyStagingHint()

		name := args[0]
		s, ok := scraper.Get(name)
		if !ok {
			return fmt.Errorf("unknown scraper %q — run 'waypoint scrape list' to see available", name)
		}

		// Resolve the reference date for recency filtering. --today lets an
		// agent anchor "now" (e.g. the CLI running under PowerShell where the
		// agent has no clock context); otherwise fall back to the machine clock.
		today, err := parseToday(scrapeRunFlags.today)
		if err != nil {
			return formatError("invalid --today", err)
		}

		results, err := s.Search(context.Background(), scraper.SearchOpts{
			Query:    scrapeRunFlags.query,
			Location: scrapeRunFlags.location,
			Limit:    scrapeRunFlags.limit,
			JobAge:   scrapeRunFlags.jobage,
			Remote:   scrapeRunFlags.remote,
			Page:     scrapeRunFlags.page,
			Today:    today,
		})
		if err != nil {
			return formatError("scrape failed", err)
		}

		results = scraper.Truncate(results, scrapeRunFlags.limit)

		// Dedup: filter out results already in the postings ledger or tracked as jobs
		var newResults []scraper.Result
		for _, r := range results {
			seen, err := store.HasPosting(r.URL)
			if err != nil {
				return formatError("check postings", err)
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
			if err := store.AddPostings(newResults); err != nil {
				return formatError("add postings", err)
			}
		}

		if jsonOut {
			meta := map[string]any{
				"count": len(newResults),
				"today": effectiveDate(today).Format("2006-01-02"),
			}
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

// --- scrape disable / enable (WP-175: autopilot scraper opt-out) ---

// runScraperOptOut toggles one scraper's autopilot opt-out. The name is
// validated against the scraper registry; the write crosses the same
// db.Store seam as every other settings change. Idempotent: disabling an
// already-disabled scraper (or enabling an enabled one) writes nothing.
func runScraperOptOut(name string, disable bool) error {
	if _, ok := scraper.Get(name); !ok {
		return fmt.Errorf("unknown scraper %q — run 'waypoint scrape list' to see available", name)
	}

	settings, err := store.GetSettings()
	if err != nil {
		return formatError("read settings", err)
	}
	names := db.ParseDisabledScrapers(settings.AutopilotDisabledScrapers)

	idx := -1
	for i, n := range names {
		if n == name {
			idx = i
			break
		}
	}
	changed := false
	switch {
	case disable && idx < 0:
		names = append(names, name)
		changed = true
	case !disable && idx >= 0:
		names = append(names[:idx], names[idx+1:]...)
		changed = true
	}

	if changed {
		if err := store.UpsertSettings(map[string]any{
			"autopilot_disabled_scrapers": db.DisabledScrapersJSON(names),
		}); err != nil {
			return formatError("save settings", err)
		}
	}

	// Re-read: report the canonical stored list, not the local guess.
	settings, err = store.GetSettings()
	if err != nil {
		return formatError("read settings", err)
	}
	disabled := db.ParseDisabledScrapers(settings.AutopilotDisabledScrapers)

	if jsonOut {
		printJSON(map[string]any{
			"scraper":          name,
			"disabled":         disable,
			"disabledScrapers": disabled,
		})
		return nil
	}

	if disable {
		fmt.Printf("  %s will be skipped by autopilot (%d opted out).\n", name, len(disabled))
	} else {
		fmt.Printf("  %s is available to autopilot again (%d opted out).\n", name, len(disabled))
	}
	return nil
}

var scrapeDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Opt a scraper out of autopilot source selection",
	Long: `Mark a scraper as opted out of the autopilot's source selection. The
choice persists in settings; the source-selection contract receives the
list as a constraint it cannot override.

Examples:
  waypoint scrape disable linkedin
  waypoint scrape disable indeed --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runScraperOptOut(args[0], true)
	},
}

var scrapeEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Make a scraper available to autopilot again",
	Long: `Remove a scraper from the autopilot opt-out list so source selection
can name it again when the brief makes it relevant.

Examples:
  waypoint scrape enable linkedin
  waypoint scrape enable indeed --json`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runScraperOptOut(args[0], false)
	},
}

// --- scrape staged ---

var scrapeStagedFlags struct {
	status string
}

var scrapeStagedCmd = &cobra.Command{
	Use:        "staged",
	Deprecated: "use 'waypoint postings list' instead",
	Short:      "View postings in the review queue (deprecated: use 'waypoint postings list')",
	Long: `List postings that have been scraped into the postings ledger. Optionally filter by status.

This command is deprecated. Use 'waypoint postings list' instead.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		legacyStagingHint()
		postingsListFlags.status = scrapeStagedFlags.status
		return postingsListCmd.RunE(cmd, args)
	},
}

// --- scrape dismiss ---

var scrapeDismissFlags struct {
	all bool
}

var scrapeDismissCmd = &cobra.Command{
	Use:        "dismiss [<url>...]",
	Deprecated: "use 'waypoint postings dismiss' instead",
	Short:      "Dismiss postings (deprecated: use 'waypoint postings dismiss')",
	Long: `Mark postings as dismissed so they don't reappear on future scrape runs.

This command is deprecated. Use 'waypoint postings dismiss' instead.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		legacyStagingHint()
		postingsDismissFlags.all = scrapeDismissFlags.all
		return postingsDismissCmd.RunE(cmd, args)
	},
}

// --- scrape detail ---

var scrapeDetailCmd = &cobra.Command{
	Use:   "detail <name> <id>",
	Short: "Fetch full details for a job posting",
	Long: `Fetch the full description, seniority, employment type, job function,
and industries for a job posting. Enriches the posting if found.

Currently only LinkedIn supports detail fetching. For board postings
(Greenhouse, Workday, Lever, BambooHR) use 'waypoint boards detail
<board> <id>' instead — that path enriches the posting.

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

		if err := store.EnrichPosting(result.URL, result.Description, result.Metadata); err != nil {
			return formatError("enrich posting", err)
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

// parseToday parses a --today date (YYYY-MM-DD). An empty value returns the
// zero time, which callers treat as "no explicit anchor" (use the machine
// clock).
func parseToday(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("want YYYY-MM-DD, got %q", s)
	}
	return t, nil
}

// effectiveDate returns the reference date used for recency filtering: the
// explicit --today anchor when set, otherwise the machine clock.
func effectiveDate(today time.Time) time.Time {
	if today.IsZero() {
		return time.Now()
	}
	return today
}

// --- scrape prune ---

var scrapePruneFlags struct {
	days int
}

var scrapePruneCmd = &cobra.Command{
	Use:        "prune",
	Deprecated: "use 'waypoint postings prune' instead",
	Short:      "Remove old postings (deprecated: use 'waypoint postings prune')",
	Long: `Remove postings older than N days.

This command is deprecated. Use 'waypoint postings prune' instead.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		legacyStagingHint()
		postingsPruneFlags.days = scrapePruneFlags.days
		return postingsPruneCmd.RunE(cmd, args)
	},
}

// --- scrape migrate ---

var scrapeMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Import legacy scrape-cache.json into the database",
	Long: `Reads the legacy ~/.waypoint/scrape-cache.json file and imports
each entry into the postings table. The JSON file is renamed to
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

		var data map[string]db.Posting
		if err := json.Unmarshal(raw, &data); err != nil {
			return formatError("parse legacy file", err)
		}

		entries := make([]db.Posting, 0, len(data))
		for _, sr := range data {
			entries = append(entries, sr)
		}

		imported, err := store.MigratePostings(entries)
		if err != nil {
			return formatError("migrate postings", err)
		}

		if err := os.Rename(path, migrated); err != nil {
			return formatError("rename legacy file", err)
		}

		if jsonOut {
			printJSON(map[string]int{"imported": imported})
			return nil
		}

		fmt.Printf("  ✓ Imported %d posting(s).\n", imported)
		return nil
	},
}

// --- scrape promote ---

var scrapePromoteFlags struct {
	all bool
}

var scrapePromoteCmd = &cobra.Command{
	Use:        "promote [<url>]",
	Deprecated: "use 'waypoint postings promote' instead",
	Short:      "Promote postings (deprecated: use 'waypoint postings promote')",
	Long: `Move postings into the tracked jobs table.

This command is deprecated. Use 'waypoint postings promote' instead.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		legacyStagingHint()
		postingsPromoteFlags.all = scrapePromoteFlags.all
		return postingsPromoteCmd.RunE(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(scrapeCmd)
	scrapeCmd.AddCommand(scrapeListCmd)
	scrapeCmd.AddCommand(scrapeRunCmd)
	scrapeCmd.AddCommand(scrapeDetailCmd)
	scrapeCmd.AddCommand(scrapeDisableCmd)
	scrapeCmd.AddCommand(scrapeEnableCmd)
	scrapeCmd.AddCommand(scrapeMigrateCmd)

	// Deprecated staging commands → replaced by postings group.
	// Keep the old commands as deprecated aliases for backward compat.
	scrapeCmd.AddCommand(scrapeStagedCmd)
	scrapeCmd.AddCommand(scrapePromoteCmd)
	scrapeCmd.AddCommand(scrapeDismissCmd)
	scrapeCmd.AddCommand(scrapePruneCmd)

	scrapeRunCmd.Flags().StringVarP(&scrapeRunFlags.query, "query", "q", "", "Filter results by keyword")
	scrapeRunCmd.Flags().StringVarP(&scrapeRunFlags.location, "location", "l", "", "Location to search (e.g. 'Bengaluru, India', 'Remote')")
	scrapeRunCmd.Flags().IntVar(&scrapeRunFlags.limit, "limit", 0, "Max results (0 = all)")
	scrapeRunCmd.Flags().IntVar(&scrapeRunFlags.jobage, "jobage", 90, "Posted within N days (0 = all)")
	scrapeRunCmd.Flags().StringVar(&scrapeRunFlags.remote, "remote", "", "Workplace type: remote|hybrid|onsite (LinkedIn only)")
	scrapeRunCmd.Flags().IntVar(&scrapeRunFlags.page, "page", 1, "Page number, 1-indexed (LinkedIn/Indeed only)")
	scrapeRunCmd.Flags().StringVar(&scrapeRunFlags.today, "today", "", "Reference date YYYY-MM-DD for recency filtering (default: machine clock)")

	scrapeStagedCmd.Flags().StringVar(&scrapeStagedFlags.status, "status", "", "Filter by status (new|shortlisted|dismissed|promoted)")

	scrapePruneCmd.Flags().IntVar(&scrapePruneFlags.days, "days", 30, "Remove entries older than N days")

	scrapeDismissCmd.Flags().BoolVar(&scrapeDismissFlags.all, "all", false, "Dismiss all 'new' status results")

	scrapePromoteCmd.Flags().BoolVar(&scrapePromoteFlags.all, "all", false, "Promote all 'new' status results")
}
