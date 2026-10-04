package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/autopilot"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/exa"
	"github.com/udit-001/waypoint/internal/scraper"
	"github.com/udit-001/waypoint/internal/zen"
)

var autopilotCmd = &cobra.Command{
	Use:   "autopilot",
	Short: "Manage and run the autopilot curation cycle",
	Long: `Manage the autopilot curation cycle. The autopilot runs a 6-stage
cycle on a cadence (default 6h): sweep boards → detail → prefilter →
LLM curate → store → runlog.

Use 'waypoint autopilot run' to trigger an immediate cycle. Use the
settings API to enable/disable and change the cadence.`,
}

var autopilotRunFlags struct {
	limit int
}

var autopilotRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run one immediate autopilot cycle",
	Long: `Trigger one full autopilot cycle immediately, ignoring the ticker
cadence. This is useful for testing or when you want to curate new
postings right away.

The cycle: sweep boards → detail → prefilter → LLM curate → store → runlog.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Build zen client from stored or env API key.
		settings, _ := store.GetSettings()
		var zc *zen.Client
		exaClient := exa.New("", nil)
		if key := zen.ResolveKey(settings.ZenAPIKey); key != "" {
			cfg := zen.DefaultConfig().WithSharedCatalog() // UA version + family routing from the curated metadata
			cfg.APIKey = key
			cfg.ProjectID = zen.ProjectID(storePath)        // stable per-install session
			cfg.Model = zen.ResolveModel(settings.ZenModel) // user's pick wins, else the shipped default
			zc = zen.New(cfg)
		} else {
			fmt.Println("  ⚠  No zen API key — LLM curation will be skipped (postings escalated to manual review)")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		fmt.Println("  Starting autopilot cycle...")
		started := time.Now()

		entry := autopilot.RunLogged(ctx, autopilot.CycleConfig{
			Store:     store,
			ZenClient: zc,
			ExaClient: exaClient,
			Scrapers:  scraper.All(),
			ExaCap:    10,
			Recency:   14,
			Limit:     autopilotRunFlags.limit,
		})

		elapsed := time.Since(started)

		if jsonOut {
			printJSON(map[string]any{
				"run_id":      entry.ID,
				"verdict":     entry.Verdict,
				"new":         entry.PostingsNew,
				"shortlisted": entry.PostingsShortlisted,
				"dismissed":   entry.PostingsDismissed,
				"errored":     entry.PostingsErrored,
				"perSource":   entry.PerSource,
				"stageErrors": entry.StageErrors,
				"duration_ms": entry.DurationMs,
				"elapsed_ms":  elapsed.Milliseconds(),
			})
			return nil
		}

		fmt.Printf("\n  ✓ Cycle complete (run #%d)\n\n", entry.ID)
		fmt.Printf("    Verdict:          %s\n", entry.Verdict)
		fmt.Printf("    New postings:     %d\n", entry.PostingsNew)
		fmt.Printf("    Shortlisted:      %d\n", entry.PostingsShortlisted)
		fmt.Printf("    Dismissed:        %d\n", entry.PostingsDismissed)
		fmt.Printf("    Errors:           %d\n", entry.PostingsErrored)
		fmt.Printf("    Duration:         %s\n", elapsed.Round(time.Second))
		fmt.Println()
		return nil
	},
}

var autopilotRunsFlags struct {
	limit int
}

var autopilotRunsCmd = &cobra.Command{
	Use:   "runs",
	Short: "List recent runs: verdict, per-source outcomes, stage errors",
	Long: `List recent autopilot runs, newest first.

Every run ends with a verdict that answers "why did nothing show up?":
quiet (sources ran fine, nothing new fit the brief), nothing-survived
(postings found, none survived curation), waiting-on-you (shortlists await
review), degraded (a stage errored). Per-source counts are the evidence
stored under the verdict; a skipped source is a per-source fact, not a
verdict.

Use --json for the agent-grade record.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		runs, err := store.ListRunLogs(autopilotRunsFlags.limit)
		if err != nil {
			return formatError("failed to read autopilot runs", err)
		}
		if jsonOut {
			printJSON(map[string]any{"runs": runs})
			return nil
		}
		printRuns(runs)
		return nil
	},
}

// printRuns renders the runlog for humans: the verdict is the headline,
// the per-source rows are the evidence beneath it. Same records the JSON
// read — one log, two renderings.
func printRuns(runs []db.RunLog) {
	if len(runs) == 0 {
		fmt.Println("  No autopilot runs yet.")
		return
	}
	fmt.Printf("  %d run(s), newest first\n\n", len(runs))
	for _, r := range runs {
		verdict := string(r.Verdict)
		if verdict == "" {
			verdict = "running"
		}
		fmt.Printf("  %s  %s\n", r.StartedAt, verdict)
		fmt.Printf("    new %d  shortlisted %d  dismissed %d  errored %d  %s\n",
			r.PostingsNew, r.PostingsShortlisted, r.PostingsDismissed, r.PostingsErrored,
			formatRunDuration(r.DurationMs))
		for _, s := range r.PerSource {
			if s.Skipped {
				fmt.Printf("    %-18s skipped  %s\n", sourceLabel(s.Source), s.Reason)
				continue
			}
			fmt.Printf("    %-18s scraped %d  new %d  shortlisted %d  dismissed %d  errored %d\n",
				sourceLabel(s.Source), s.Scraped, s.New, s.Shortlisted, s.Dismissed, s.Errored)
		}
		for _, e := range r.StageErrors {
			fmt.Printf("    ! %s\n", e)
		}
		fmt.Println()
	}
}

// sourceLabel names a per-source row. An empty source is a ledger row from
// before source recording (migration 00018).
func sourceLabel(source string) string {
	if source == "" {
		return "(unattributed)"
	}
	return source
}

func formatRunDuration(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.Duration(ms * int64(time.Millisecond)).Round(time.Second).String()
}

var autopilotStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show autopilot status and last run",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		settings, _ := store.GetSettings()
		lastRun, hasRun, _ := store.GetLastRun()

		if jsonOut {
			result := map[string]any{
				"enabled":          settings.AutopilotEnabled == 1,
				"cadence":          settings.AutopilotCadence,
				"disabledScrapers": db.ParseDisabledScrapers(settings.AutopilotDisabledScrapers),
				"lastRun":          nil,
			}
			if hasRun {
				result["lastRun"] = lastRun
			}
			printJSON(result)
			return nil
		}

		if settings.AutopilotEnabled == 1 {
			fmt.Printf("  Autopilot:  enabled (cadence: %dh)\n", settings.AutopilotCadence)
		} else {
			fmt.Println("  Autopilot:  disabled")
		}
		if settings.ZenAPIKey != "" {
			fmt.Println("  Zen API:    configured")
		} else {
			fmt.Println("  Zen API:    not configured (LLM curation skipped)")
		}
		if hasRun {
			verdict := string(lastRun.Verdict)
			if verdict == "" {
				verdict = "running"
			}
			fmt.Printf("  Last run:   %s — %s (new=%d, shortlisted=%d, dismissed=%d, errored=%d)\n",
				lastRun.StartedAt, verdict, lastRun.PostingsNew,
				lastRun.PostingsShortlisted, lastRun.PostingsDismissed,
				lastRun.PostingsErrored)
		} else {
			fmt.Println("  Last run:   none")
		}
		fmt.Println()
		return nil
	},
}

func init() {
	autopilotRunCmd.Flags().IntVar(&autopilotRunFlags.limit, "limit", 0, "max new postings to curate per cycle (0 = all)")
	autopilotRunsCmd.Flags().IntVar(&autopilotRunsFlags.limit, "limit", db.DefaultRunLogLimit, "max runs to list")
	autopilotCmd.AddCommand(autopilotRunCmd)
	autopilotCmd.AddCommand(autopilotRunsCmd)
	autopilotCmd.AddCommand(autopilotStatusCmd)
	rootCmd.AddCommand(autopilotCmd)
}
