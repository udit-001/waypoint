package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/udit-001/waypoint/internal/autopilot"
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
			cfg := zen.DefaultConfig()
			cfg.APIKey = key
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
				"new":         entry.PostingsNew,
				"shortlisted": entry.PostingsShortlisted,
				"dismissed":   entry.PostingsDismissed,
				"errored":     entry.PostingsErrored,
				"duration_ms": entry.DurationMs,
				"elapsed_ms":  elapsed.Milliseconds(),
			})
			return nil
		}

		fmt.Printf("\n  ✓ Cycle complete (run #%d)\n\n", entry.ID)
		fmt.Printf("    New postings:     %d\n", entry.PostingsNew)
		fmt.Printf("    Shortlisted:      %d\n", entry.PostingsShortlisted)
		fmt.Printf("    Dismissed:        %d\n", entry.PostingsDismissed)
		fmt.Printf("    Errors:           %d\n", entry.PostingsErrored)
		fmt.Printf("    Duration:         %s\n", elapsed.Round(time.Second))
		fmt.Println()
		return nil
	},
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
				"enabled": settings.AutopilotEnabled == 1,
				"cadence": settings.AutopilotCadence,
				"lastRun": nil,
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
			fmt.Printf("  Last run:   %s (new=%d, shortlisted=%d, dismissed=%d, errors=%d)\n",
				lastRun.StartedAt, lastRun.PostingsNew,
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
	autopilotCmd.AddCommand(autopilotRunCmd)
	autopilotCmd.AddCommand(autopilotStatusCmd)
	rootCmd.AddCommand(autopilotCmd)
}
