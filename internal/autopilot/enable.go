package autopilot

// The enable contract (WP-158): one gate + one write + one run request.
// The web settings handler and the CLI are both adapters over this — the
// policy lives here so "enabling autopilot" means the same thing on every
// surface.

import (
	"context"
	"fmt"

	"github.com/udit-001/waypoint/internal/db"
)

// CanEnable is the gate: autopilot turns on only when the curation brief
// is complete — without a settled search intent the cycle curates noise.
func CanEnable(ctx context.Context, store db.Store) error {
	brief, err := store.GetBrief()
	if err != nil {
		return fmt.Errorf("read curation brief: %w", err)
	}
	if !brief.Complete {
		return fmt.Errorf("complete your curation brief before enabling autopilot (open: %s)",
			joinOpen(brief.Open))
	}
	return nil
}

// Enable persists the toggle and asks the daemon for an immediate cycle —
// the scheduler consumes that marker within one poll interval, so matches
// arrive in seconds, not after a full cadence. cadence 0 leaves the stored
// cadence alone (default 6h).
func Enable(ctx context.Context, store db.Store, cadence int) error {
	if err := CanEnable(ctx, store); err != nil {
		return err
	}
	updates := map[string]any{"autopilot_enabled": 1}
	if cadence > 0 {
		updates["autopilot_cadence"] = cadence
	}
	if err := store.UpsertSettings(updates); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	// Best effort: a dropped run request costs one cadence of delay, not a
	// broken toggle.
	_ = store.RequestAutopilotRun()
	return nil
}

// Disable persists the toggle off. The cadence and every other setting are
// untouched — re-enabling restores the same search.
func Disable(store db.Store) error {
	if err := store.UpsertSettings(map[string]any{"autopilot_enabled": 0}); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

// joinOpen renders the brief's open items for the gate's error message.
func joinOpen(open []string) string {
	out := ""
	for i, o := range open {
		if i > 0 {
			out += ", "
		}
		out += o
	}
	return out
}
