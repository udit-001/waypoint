package scraper

import (
	"testing"
	"time"
)

func TestFilterByRecency_zeroOrNegativeReturnsAll(t *testing.T) {
	results := []Result{
		{Title: "old ad", Date: "2020-01-01"},
		{Title: "ancient ad", Date: "2018-06-15"},
	}

	for _, n := range []int{0, -1, -100} {
		got := FilterByRecency(results, n)
		if len(got) != len(results) {
			t.Errorf("FilterByRecency(_, %d): got %d results, want %d (zero/negative should be no-op)", n, len(got), len(results))
		}
	}
}

func TestFilterByRecency_dropsOldKeepsRecent(t *testing.T) {
	recent := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	old := time.Now().AddDate(0, 0, -200).Format("2006-01-02")

	results := []Result{
		{Title: "recent ad", Date: recent},
		{Title: "old ad", Date: old},
	}

	got := FilterByRecency(results, 30)
	if len(got) != 1 {
		t.Fatalf("got %d results, want 1 (only the recent ad)", len(got))
	}
	if got[0].Title != "recent ad" {
		t.Errorf("kept result: got %q, want %q", got[0].Title, "recent ad")
	}
}

func TestFilterByRecency_keepsUnparseableAndRolling(t *testing.T) {
	// A genuine date that parses but is old — must be dropped.
	old := time.Now().AddDate(0, 0, -200).Format("2006-01-02")
	results := []Result{
		{Title: "rolling", Date: "Open"},
		{Title: "empty", Date: ""},
		{Title: "garbage", Date: "not-a-date"},
		{Title: "old ad", Date: old},
	}

	got := FilterByRecency(results, 30)
	if len(got) != 3 {
		t.Fatalf("got %d results, want 3 (rolling/empty/unparseable kept; old dropped)", len(got))
	}
	for _, r := range got {
		if r.Title == "old ad" {
			t.Error("old ad should have been dropped")
		}
	}
}

