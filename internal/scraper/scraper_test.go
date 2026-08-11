package scraper

import (
	"testing"
	"time"
)

// refDate is a fixed anchor so recency tests are deterministic,
// independent of the machine clock.
var refDate = time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)

func TestFilterByRecency_zeroOrNegativeReturnsAll(t *testing.T) {
	results := []Result{
		{Title: "old ad", Date: "2020-01-01"},
		{Title: "ancient ad", Date: "2018-06-15"},
	}

	for _, n := range []int{0, -1, -100} {
		got := FilterByRecency(results, n, refDate)
		if len(got) != len(results) {
			t.Errorf("FilterByRecency(_, %d): got %d results, want %d (zero/negative should be no-op)", n, len(got), len(results))
		}
	}
}

func TestFilterByRecency_dropsOldKeepsRecent(t *testing.T) {
	recent := refDate.AddDate(0, 0, -2).Format("2006-01-02")
	old := refDate.AddDate(0, 0, -200).Format("2006-01-02")

	results := []Result{
		{Title: "recent ad", Date: recent},
		{Title: "old ad", Date: old},
	}

	got := FilterByRecency(results, 30, refDate)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1 (only the recent ad)", len(got))
	}
	if got[0].Title != "recent ad" {
		t.Errorf("kept result: got %q, want %q", got[0].Title, "recent ad")
	}
}

func TestFilterByRecency_keepsUnparseableAndRolling(t *testing.T) {
	// A genuine date that parses but is old — must be dropped.
	old := refDate.AddDate(0, 0, -200).Format("2006-01-02")
	results := []Result{
		{Title: "rolling", Date: "Open"},
		{Title: "empty", Date: ""},
		{Title: "garbage", Date: "not-a-date"},
		{Title: "old ad", Date: old},
	}

	got := FilterByRecency(results, 30, refDate)
	if len(got) != 3 {
		t.Fatalf("got %d results, want 3 (rolling/empty/unparseable kept; old dropped)", len(got))
	}
	for _, r := range got {
		if r.Title == "old ad" {
			t.Error("old ad should have been dropped")
		}
	}
}

func TestFilterByRecency_keepsBoundaryDay(t *testing.T) {
	// A result dated exactly today-N (UTC) must be kept when jobAgeDays=N:
	// the recency window is inclusive of its boundary day. The cutoff must
	// be date-aligned (midnight) so the time-of-day doesn't exclude it.
	boundary := refDate.UTC().Truncate(24*time.Hour).AddDate(0, 0, -30).Format("2006-01-02")
	results := []Result{{Title: "boundary ad", Date: boundary}}
	got := FilterByRecency(results, 30, refDate)
	if len(got) != 1 {
		t.Errorf("expected boundary day (today-30) kept with JobAge=30, got %d", len(got))
	}
}

func TestFilterByRecency_zeroTodayUsesExplicitCutoff(t *testing.T) {
	// A zero 'today' falls back to the machine clock, so a result exactly
	// at 'now-30 days' is kept. This documents the fallback path used when
	// no --today anchor is supplied.
	recent := time.Now().AddDate(0, 0, -29).Format("2006-01-02")
	results := []Result{{Title: "recent ad", Date: recent}}
	got := FilterByRecency(results, 30, time.Time{})
	if len(got) != 1 {
		t.Errorf("expected recent (now-29d) kept with JobAge=30 and zero today, got %d", len(got))
	}
}
