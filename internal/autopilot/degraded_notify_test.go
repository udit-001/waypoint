package autopilot

import (
	"context"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
)

// TestDegradedNotify_InterruptionAloneIsSilent: a run degraded only by
// the operator's own stop (disable flip, shutdown) must not ping — the
// notice is for failures, not for deliberate cancellations.
func TestDegradedNotify_InterruptionAloneIsSilent(t *testing.T) {
	stubDiscovery(t)
	quietSleeper(t)
	f := db.NewFakeStore()
	s := &fixtureScraper{
		name:    "s1",
		results: []scraper.Result{{URL: "https://example.com/a", Title: "Job", Company: "Co"}},
	}
	n := &recordingNotifier{}

	cfg := newCycle(f, []scraper.Scraper{s})
	cfg.Notifier = n

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // interrupt before the cycle starts
	Run(ctx, cfg)

	if len(n.degraded) != 0 {
		t.Fatalf("expected no degraded notice for an interruption-only run, got %d (%v)", len(n.degraded), n.degraded)
	}
}

// TestDegradedNotify_RealFailureStillPings: a spent wall budget is a
// real stage error, not an operator action — the notice fires even
// though the run also sees the budget's own cancellation.
func TestDegradedNotify_RealFailureStillPings(t *testing.T) {
	stubDiscovery(t)
	quietSleeper(t)
	f := db.NewFakeStore()
	s := &fixtureScraper{
		name:    "s1",
		results: []scraper.Result{{URL: "https://example.com/a", Title: "Job", Company: "Co"}},
	}
	n := &recordingNotifier{}

	cfg := newCycle(f, []scraper.Scraper{s})
	cfg.Notifier = n
	cfg.Budget = &Budget{Deadline: time.Now().Add(-time.Minute)} // expired before the cycle starts

	Run(context.Background(), cfg)

	if len(n.degraded) != 1 {
		t.Fatalf("expected exactly one degraded notice for a budget-spent run, got %d", len(n.degraded))
	}
}

// TestInterruptionOnly unit-pins the prefix rule: any non-interruption
// error makes the set announce-worthy; empty and all-interruption sets
// stay silent.
func TestInterruptionOnly(t *testing.T) {
	cases := []struct {
		name string
		errs []string
		want bool
	}{
		{"empty", nil, true},
		{"interruption alone", []string{"cycle interrupted: context canceled"}, true},
		{"two interruptions", []string{"cycle interrupted: a", "cycle interrupted: b"}, true},
		{"real error alone", []string{"sweep s1: timeout"}, false},
		{"mixed", []string{"cycle interrupted: x", "sweep s1: timeout"}, false},
	}
	for _, tc := range cases {
		if got := interruptionOnly(tc.errs); got != tc.want {
			t.Errorf("%s: interruptionOnly = %v, want %v", tc.name, got, tc.want)
		}
	}
}
