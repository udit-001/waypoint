package discovery

import (
	"testing"
	"time"
)

var base = time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

func TestShouldRunDiscovery(t *testing.T) {
	t.Run("fresh install triggers first-run", func(t *testing.T) {
		got := ShouldRunDiscovery(TriggerInput{Now: base, Candidates: 0})
		if got != TriggerFirstRun {
			t.Errorf("got %q, want first-run", got)
		}
	})

	t.Run("brief edit re-triggers even within interval", func(t *testing.T) {
		got := ShouldRunDiscovery(TriggerInput{
			Now: base.Add(time.Hour), Candidates: 5,
			BriefHash: "new", HasLastRun: true, LastBriefHash: "old",
			LastRunAt: base,
		})
		if got != TriggerBriefChange {
			t.Errorf("got %q, want brief-changed", got)
		}
	})

	t.Run("unchanged brief within interval does not run", func(t *testing.T) {
		got := ShouldRunDiscovery(TriggerInput{
			Now: base.Add(24 * time.Hour), IntervalDays: 30, Candidates: 5,
			BriefHash: "same", HasLastRun: true, LastBriefHash: "same",
			LastRunAt: base,
		})
		if got != "" {
			t.Errorf("got %q, want no trigger", got)
		}
	})

	t.Run("elapsed interval re-triggers", func(t *testing.T) {
		got := ShouldRunDiscovery(TriggerInput{
			Now: base.Add(31 * 24 * time.Hour), IntervalDays: 30, Candidates: 5,
			BriefHash: "same", HasLastRun: true, LastBriefHash: "same",
			LastRunAt: base,
		})
		if got != TriggerIntervalDue {
			t.Errorf("got %q, want interval-due", got)
		}
	})

	t.Run("interval disabled means only brief-change fires", func(t *testing.T) {
		got := ShouldRunDiscovery(TriggerInput{
			Now: base.Add(400 * 24 * time.Hour), IntervalDays: 0, Candidates: 5,
			BriefHash: "same", HasLastRun: true, LastBriefHash: "same",
			LastRunAt: base,
		})
		if got != "" {
			t.Errorf("got %q, want no trigger with interval<=0", got)
		}
	})

	t.Run("candidates but no recorded run does not fire", func(t *testing.T) {
		got := ShouldRunDiscovery(TriggerInput{Now: base, Candidates: 3, BriefHash: "h"})
		if got != "" {
			t.Errorf("got %q, want no trigger without last-run state", got)
		}
	})
}
