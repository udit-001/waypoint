package autopilot

import (
	"context"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
	"github.com/udit-001/waypoint/internal/scraper"
)

// WP-183: the Sources tab's company-discovery toggle is a hard off —
// when discovery_enabled is 0, no trigger may fire and no trigger state
// is written (so re-enabling starts from "first run", not a stale clock).
func TestCycle_DiscoveryDisabledSkipsStage(t *testing.T) {
	f := db.NewFakeStore()
	if err := f.UpsertSettings(map[string]any{"discovery_enabled": 0}); err != nil {
		t.Fatalf("UpsertSettings: %v", err)
	}
	s := seedSweeperScraper()

	calls := 0
	origRun := runAutoDiscovery
	runAutoDiscovery = func(_ context.Context, cfg CycleConfig, _ []discovery.Facet) (discoveryRun, error) {
		calls++
		return discoveryRun{}, nil
	}
	t.Cleanup(func() { runAutoDiscovery = origRun })

	Run(context.Background(), newCycle(f, []scraper.Scraper{s}))

	if calls != 0 {
		t.Errorf("discovery ran %d time(s) with the toggle off, want 0", calls)
	}
	// No trigger state written: re-enabling must fire the first-run
	// trigger again, not inherit a stale last-run timestamp.
	if _, _, has, _ := f.DiscoveryLastRun(); has {
		t.Error("trigger state written while discovery disabled")
	}
}
