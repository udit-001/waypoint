package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/boards"
	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/scraper"
)

var boardsCmd = &cobra.Command{
	Use:   "boards",
	Short: "Manage company ATS boards (Greenhouse, Workday, Lever, BambooHR)",
	Long: `Manage the list of company ATS boards and sweep them into staging.

A board is one company's careers site behind one vendor (Greenhouse,
Workday, Lever, BambooHR). Boards live in boards.toml inside data_dir,
so they travel with the database in backups.

The flow: find the company's careers URL (any search tool), then
  waypoint boards add <name> --url <careers-url>
which detects the provider, verifies the board is live, and saves it.
Then:
  waypoint boards sweep
fetches every enabled board and stages new postings for review.

Examples:
  waypoint boards add slack --url https://salesforce.wd12.myworkdayjobs.com/Slack/
  waypoint boards add khanacademy --url https://job-boards.greenhouse.io/khanacademy/
  waypoint boards sweep --json`,
}

func init() {
	rootCmd.AddCommand(boardsCmd)
}

// loadBoardsStore loads boards.toml via the app config (data_dir aware).
func loadBoardsStore() (*config.BoardsFile, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	bf, err := config.LoadBoards(cfg)
	if err != nil {
		return nil, nil, formatError("load boards", err)
	}
	return bf, cfg, nil
}

// toBoard converts a stored entry to the boards package shape.
func toBoard(e config.BoardEntry) boards.Board {
	return boards.Board{Name: e.Name, Company: e.Company, URL: e.URL, MaxPages: e.MaxPages, Enabled: e.Enabled}
}

// --- boards add ---

var boardsAddFlags struct {
	company string
}

var boardsAddCmd = &cobra.Command{
	Use:   "add <name> --url <url>",
	Short: "Detect, verify, and save a company ATS board",
	Long: `Detect the provider for a careers URL, verify the board is live by
fetching its first page, and save it to boards.toml (enabled).

Verification is the gate: a board is only saved when its API answers with
a parseable job list, so a typo'd or dead URL fails here instead of later.

Exit non-zero when no provider claims the URL or verification fails.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		url := cmd.Flag("url").Value.String()
		if url == "" {
			return fmt.Errorf("--url is required")
		}
		company := boardsAddFlags.company
		if company == "" {
			company = name
		}

		entry := config.BoardEntry{
			Name:    name,
			Company: company,
			URL:     url,
			Enabled: true,
			AddedAt: time.Now().UTC().Format(time.RFC3339),
		}

		b := boards.Board{Name: entry.Name, Company: entry.Company, URL: entry.URL, Enabled: true}
		p, hit, err := boards.DetectProvider(b)
		if err != nil {
			if jsonOut {
				printJSON(map[string]any{"meta": map[string]any{
					"board": name, "provider": nil, "verified": false,
					"error": fmt.Sprintf("no provider matched %s", url),
				}})
			}
			return fmt.Errorf("no provider matched %s — supported: greenhouse, workday, lever, bamboohr", url)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		results, err := p.Fetch(ctx, b, *hit, boards.FetchOpts{MaxPages: 1, Limit: 5})
		if err != nil {
			if jsonOut {
				printJSON(map[string]any{"meta": map[string]any{
					"board": name, "provider": p.Name(), "verified": false, "error": err.Error(),
				}})
			}
			return formatError("verification failed", err)
		}

		entry.Provider = p.Name()
		bf, cfg, err := loadBoardsStore()
		if err != nil {
			return err
		}
		if bf.Find(entry.Name) != nil {
			if jsonOut {
				printJSON(map[string]any{"meta": map[string]any{
					"board": entry.Name, "provider": p.Name(), "verified": true,
					"saved": false, "error": "board already exists",
				}})
			}
			return fmt.Errorf("board %q already exists — remove it first or pick another name", entry.Name)
		}
		bf.Upsert(entry)
		if err := config.SaveBoards(cfg, bf); err != nil {
			return formatError("save boards", err)
		}

		if jsonOut {
			printJSON(map[string]any{"meta": map[string]any{
				"board": entry.Name, "provider": p.Name(), "verified": true,
				"fetched": len(results),
			}})
			return nil
		}
		fmt.Printf("  Added %s (provider: %s, verified: %d jobs on first page)\n", entry.Name, p.Name(), len(results))
		return nil
	},
}

// --- boards remove ---

var boardsRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a board",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bf, cfg, err := loadBoardsStore()
		if err != nil {
			return err
		}
		if !bf.Remove(args[0]) {
			if jsonOut {
				printJSON(map[string]any{"meta": map[string]any{"board": args[0], "removed": false}})
			}
			return fmt.Errorf("no board named %q", args[0])
		}
		if err := config.SaveBoards(cfg, bf); err != nil {
			return formatError("save boards", err)
		}
		if jsonOut {
			printJSON(map[string]any{"meta": map[string]any{"board": args[0], "removed": true}})
			return nil
		}
		fmt.Printf("  Removed %s\n", args[0])
		return nil
	},
}

// --- boards list ---

var boardsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List saved boards",
	RunE: func(cmd *cobra.Command, args []string) error {
		bf, _, err := loadBoardsStore()
		if err != nil {
			return err
		}
		type row struct {
			Name     string `json:"name"`
			Company  string `json:"company"`
			Provider string `json:"provider"`
			URL      string `json:"url"`
			Enabled  bool   `json:"enabled"`
		}
		out := make([]row, 0, len(bf.Boards))
		for _, b := range bf.Boards {
			out = append(out, row{Name: b.Name, Company: b.Company, Provider: b.Provider, URL: b.URL, Enabled: b.Enabled})
		}
		if jsonOut {
			printJSON(map[string]any{"meta": map[string]any{"count": len(out)}, "boards": out})
			return nil
		}
		if len(out) == 0 {
			fmt.Println("  No boards saved. Add one with 'waypoint boards add <name> --url <url>'.")
			return nil
		}
		rows := make([][]string, 0, len(out))
		for _, b := range out {
			rows = append(rows, []string{b.Name, b.Provider, b.Company, fmt.Sprintf("%v", b.Enabled), b.URL})
		}
		fmt.Println()
		fmt.Println(formatTable([]string{"Name", "Provider", "Company", "Enabled", "URL"}, rows))
		fmt.Println()
		return nil
	},
}

// --- boards enable/disable ---

func setBoardEnabled(name string, enabled bool) error {
	bf, cfg, err := loadBoardsStore()
	if err != nil {
		return err
	}
	e := bf.Find(name)
	if e == nil {
		if jsonOut {
			printJSON(map[string]any{"meta": map[string]any{"board": name, "enabled": enabled, "found": false}})
		}
		return fmt.Errorf("no board named %q", name)
	}
	e.Enabled = enabled
	if err := config.SaveBoards(cfg, bf); err != nil {
		return formatError("save boards", err)
	}
	if jsonOut {
		printJSON(map[string]any{"meta": map[string]any{"board": name, "enabled": enabled, "found": true}})
		return nil
	}
	fmt.Printf("  %s: enabled=%v\n", name, enabled)
	return nil
}

var boardsEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Include a board in sweeps",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return setBoardEnabled(args[0], true) },
}

var boardsDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Skip a board in sweeps",
	Args:  cobra.ExactArgs(1),
	RunE:  func(cmd *cobra.Command, args []string) error { return setBoardEnabled(args[0], false) },
}

// --- boards verify ---

var boardsVerifyCmd = &cobra.Command{
	Use:   "verify [<name>]",
	Short: "Re-probe saved boards for liveness",
	Long: `Fetch the first page of one board (or every enabled board) to confirm
the board still answers. Reports per-board status; exits non-zero when any
probed board fails.

Workday boards whose URL pins no instance auto-probe known instances.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bf, _, err := loadBoardsStore()
		if err != nil {
			return err
		}
		var targets []config.BoardEntry
		if len(args) == 1 {
			e := bf.Find(args[0])
			if e == nil {
				return fmt.Errorf("no board named %q", args[0])
			}
			targets = append(targets, *e)
		} else {
			for _, b := range bf.Boards {
				if b.Enabled {
					targets = append(targets, b)
				}
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		type status struct {
			Board    string `json:"board"`
			Provider string `json:"provider"`
			Ok       bool   `json:"ok"`
			Jobs     int    `json:"jobs,omitempty"`
			Error    string `json:"error,omitempty"`
		}
		var out []status
		failed := 0
		for _, e := range targets {
			s := status{Board: e.Name, Provider: e.Provider}
			b := toBoard(e)
			p, hit, err := boards.DetectProvider(b)
			if err != nil {
				s.Error = err.Error()
			} else {
				s.Provider = p.Name()
				results, err := p.Fetch(ctx, b, *hit, boards.FetchOpts{MaxPages: 1})
				if err != nil {
					s.Error = err.Error()
				} else {
					s.Ok = true
					s.Jobs = len(results)
				}
			}
			if !s.Ok {
				failed++
			}
			out = append(out, s)
		}

		if jsonOut {
			printJSON(map[string]any{
				"meta":    map[string]any{"probed": len(out), "failed": failed},
				"results": out,
			})
		} else {
			for _, s := range out {
				if s.Ok {
					fmt.Printf("  ok       %-16s %-12s %d jobs on first page\n", s.Board, s.Provider, s.Jobs)
				} else {
					fmt.Printf("  FAILED   %-16s %-12s %s\n", s.Board, s.Provider, s.Error)
				}
			}
		}
		if failed > 0 {
			return fmt.Errorf("%d of %d board(s) failed verification", failed, len(out))
		}
		return nil
	},
}

// --- boards sweep ---

var boardsSweepFlags struct {
	jobage int
	limit  int
}

var boardsSweepCmd = &cobra.Command{
	Use:   "sweep",
	Short: "Fetch all enabled boards and stage new postings",
	Long: `Sweep every enabled board, deduplicate against staging and tracked
jobs, stage the new postings, and print a per-board summary.

The JSON meta block is the completion contract for agents:
  fetched  jobs returned by the board
  new      staged this sweep (listed in that board's jobs array)
  seen     already staged or tracked (skipped)
  failed   this board errored — verify it, fix its URL, or disable it
Sweep is done when every enabled board reports new==0 or the user has
reviewed the staged jobs; any failed board needs attention.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		bf, _, err := loadBoardsStore()
		if err != nil {
			return err
		}

		var enabled []config.BoardEntry
		for _, b := range bf.Boards {
			if b.Enabled {
				enabled = append(enabled, b)
			}
		}
		if len(enabled) == 0 {
			if jsonOut {
				printJSON(map[string]any{"meta": map[string]any{"boards": 0, "failed": 0}, "results": []any{}})
				return nil
			}
			fmt.Println("  No enabled boards. Add one with 'waypoint boards add <name> --url <url>'.")
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		type sweepResult struct {
			Board    string           `json:"board"`
			Provider string           `json:"provider"`
			Fetched  int              `json:"fetched"`
			New      int              `json:"new"`
			Seen     int              `json:"seen"`
			Failed   bool             `json:"failed"`
			Error    string           `json:"error,omitempty"`
			Jobs     []scraper.Result `json:"jobs,omitempty"` // staged this sweep — present these to the user
		}
		var out []sweepResult
		totalNew, failed := 0, 0

		for _, e := range enabled {
			sr := sweepResult{Board: e.Name, Provider: e.Provider}
			b := toBoard(e)
			p, hit, err := boards.DetectProvider(b)
			if err != nil {
				sr.Failed, sr.Error, failed = true, err.Error(), failed+1
				out = append(out, sr)
				continue
			}
			sr.Provider = p.Name()
			results, err := p.Fetch(ctx, b, *hit, boards.FetchOpts{
				JobAgeDays: boardsSweepFlags.jobage,
				MaxPages:   e.MaxPages,
				Limit:      boardsSweepFlags.limit,
			})
			if err != nil {
				sr.Failed, sr.Error, failed = true, err.Error(), failed+1
				out = append(out, sr)
				continue
			}
			sr.Fetched = len(results)

			var fresh []scraper.Result
			for _, r := range results {
				seen, err := store.IsSeen(r.URL)
				if err != nil {
					return formatError("check staging", err)
				}
				if seen {
					sr.Seen++
					continue
				}
				tracked, err := store.JobExists(r.URL)
				if err != nil {
					return formatError("check jobs", err)
				}
				if tracked {
					sr.Seen++
					continue
				}
				fresh = append(fresh, r)
			}
			if len(fresh) > 0 {
				if err := store.AddStaging(fresh); err != nil {
					return formatError("stage results", err)
				}
			}
			sr.New = len(fresh)
			totalNew += len(fresh)
			sr.Jobs = fresh
			out = append(out, sr)
		}

		if jsonOut {
			printJSON(map[string]any{
				"meta": map[string]any{
					"boards": len(enabled), "failed": failed, "new": totalNew,
				},
				"results": out,
			})
			return nil
		}
		fmt.Printf("\n  %d new posting(s) staged across %d board(s)\n\n", totalNew, len(enabled))
		rows := make([][]string, 0, len(out))
		for _, s := range out {
			status := fmt.Sprintf("fetched=%d new=%d seen=%d", s.Fetched, s.New, s.Seen)
			if s.Failed {
				status = "FAILED: " + s.Error
			}
			rows = append(rows, []string{s.Board, s.Provider, status})
		}
		fmt.Println(formatTable([]string{"Board", "Provider", "Result"}, rows))
		fmt.Println()
		return nil
	},
}

// --- boards detail ---

var boardsDetailCmd = &cobra.Command{
	Use:   "detail <board> <id>",
	Short: "Fetch the full body for one posting",
	Long: `Fetch the full description, date, and metadata for a single posting by
its board-scoped id (the id field on the listings printed by 'boards list'
or the jobs array in 'boards sweep'). The provider calls the board's
per-job detail endpoint.

The swept list carried only title, location, and the list's date. Detail
is the on-demand enrichment step for postings you're seriously
considering — for cover-letter generation or final fit judgment. The
result is merged into the staged entry when one exists (description and
metadata; the list's structured fields are kept).

Examples:
  waypoint boards detail khanacademy 123
  waypoint boards detail slack Sr-Staff-Software-Engineer--Android_JR355162
  waypoint boards detail concept2 42 --json`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, id := args[0], args[1]
		bf, _, err := loadBoardsStore()
		if err != nil {
			return err
		}
		e := bf.Find(name)
		if e == nil {
			return fmt.Errorf("no board named %q", name)
		}
		p, hit, err := boards.DetectProvider(toBoard(*e))
		if err != nil {
			return fmt.Errorf("provider for board %q: %w", name, err)
		}
		_ = hit
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		r, err := p.Detail(ctx, toBoard(*e), id)
		if err != nil {
			return formatError("detail failed", err)
		}
		// Merge description + metadata into any staged entry (no-op if absent).
		if r.URL != "" {
			if err := store.EnrichStaging(r.URL, r.Description, r.Metadata); err != nil {
				return formatError("enrich staging", err)
			}
		}
		if jsonOut {
			printJSON(map[string]any{
				"meta":   map[string]any{"board": name, "provider": p.Name(), "id": id},
				"result": r,
			})
			return nil
		}
		fmt.Printf("  %s\n", r.Title)
		if r.Company != "" || r.Location != "" {
			fmt.Printf("  %s · %s\n", r.Company, r.Location)
		}
		if r.Date != "" {
			fmt.Printf("  posted %s\n", r.Date)
		}
		if len(r.Metadata) > 0 {
			fmt.Println()
			for _, k := range sortedKeys(r.Metadata) {
				fmt.Printf("  %s: %s\n", k, r.Metadata[k])
			}
		}
		if r.Description != "" {
			fmt.Println()
			fmt.Println(r.Description)
		}
		fmt.Printf("\n  URL: %s\n", r.URL)
		return nil
	},
}

func init() {
	boardsAddCmd.Flags().StringVar(&boardsAddFlags.company, "company", "", "display company name (default: board name)")
	boardsAddCmd.Flags().String("url", "", "careers/board URL to detect and verify (required)")
	_ = boardsAddCmd.MarkFlagRequired("url")

	boardsSweepCmd.Flags().IntVar(&boardsSweepFlags.jobage, "jobage", 90, "only postings from the last N days (0 = all; same default as scrape run)")
	boardsSweepCmd.Flags().IntVar(&boardsSweepFlags.limit, "limit", 0, "cap results per board (0 = no cap)")

	boardsCmd.AddCommand(boardsAddCmd)
	boardsCmd.AddCommand(boardsRemoveCmd)
	boardsCmd.AddCommand(boardsListCmd)
	boardsCmd.AddCommand(boardsEnableCmd)
	boardsCmd.AddCommand(boardsDisableCmd)
	boardsCmd.AddCommand(boardsVerifyCmd)
	boardsCmd.AddCommand(boardsSweepCmd)
	boardsCmd.AddCommand(boardsDetailCmd)
}
