package autopilot

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
	"github.com/udit-001/waypoint/internal/zen"
)

// seedBacklog pre-seeds the ledger with n "new" postings whose dates
// step forward, so date-order assertions can name the newest by URL.
func seedBacklog(t *testing.T, f *db.FakeStore, n int) {
	t.Helper()
	results := make([]scraper.Result, 0, n)
	for i := 0; i < n; i++ {
		results = append(results, scraper.Result{
			URL:     fmt.Sprintf("https://example.com/p%02d", i),
			Title:   fmt.Sprintf("Job %02d", i),
			Company: fmt.Sprintf("Co %02d", i),
			Date:    fmt.Sprintf("2026-01-%02d", i+1),
		})
	}
	if err := f.AddPostings(results); err != nil {
		t.Fatalf("AddPostings: %v", err)
	}
}

// statuses maps ledger URL -> status for the seeded backlog URLs.
func statuses(f *db.FakeStore) map[string]string {
	out := map[string]string{}
	postings, _ := f.ListPostings("")
	for _, p := range postings {
		out[p.Result.URL] = p.Status
	}
	return out
}

// countByStatus counts ledger postings per status.
func countByStatus(f *db.FakeStore, status string) int {
	n := 0
	for _, s := range statuses(f) {
		if s == status {
			n++
		}
	}
	return n
}

// TestBudget_CapKeepsNewest: a merged backlog larger than MaxScored
// escalates exactly MaxScored postings — the newest ones — and closes
// the run without stage errors.
func TestBudget_CapKeepsNewest(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	seedBacklog(t, f, 5)

	cfg := newCycle(f, nil) // no scrapers: the backlog is the whole list
	cfg.Budget = &Budget{MaxScored: 3}

	entry := Run(context.Background(), cfg)

	if entry.PostingsShortlisted != 3 {
		t.Errorf("postingsShortlisted = %d, want 3 (scored cap)", entry.PostingsShortlisted)
	}
	if got := countByStatus(f, db.StatusNew); got != 2 {
		t.Errorf("postings left new = %d, want 2", got)
	}
	// The cap must keep the newest: p02..p04 have the latest dates.
	st := statuses(f)
	for _, u := range []string{"https://example.com/p02", "https://example.com/p03", "https://example.com/p04"} {
		if st[u] != db.StatusShortlisted {
			t.Errorf("%s status = %q, want shortlisted (newest kept)", u, st[u])
		}
	}
	for _, u := range []string{"https://example.com/p00", "https://example.com/p01"} {
		if st[u] != db.StatusNew {
			t.Errorf("%s status = %q, want new (oldest queued)", u, st[u])
		}
	}
	if len(entry.StageErrors) != 0 {
		t.Errorf("stage errors = %v, want none", entry.StageErrors)
	}
	if entry.FinishedAt == "" {
		t.Error("expected run to close with finished_at")
	}
}

// TestBudget_CapOff: MaxScored 0 means no cap — every posting escalates.
func TestBudget_CapOff(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	seedBacklog(t, f, 5)

	cfg := newCycle(f, nil)
	cfg.Budget = &Budget{Deadline: time.Now().Add(time.Hour)} // wall on, cap off

	entry := Run(context.Background(), cfg)

	if entry.PostingsShortlisted != 5 {
		t.Errorf("postingsShortlisted = %d, want 5 (no cap)", entry.PostingsShortlisted)
	}
	if len(entry.StageErrors) != 0 {
		t.Errorf("stage errors = %v, want none", entry.StageErrors)
	}
}

// TestBudget_ExpirySkipsStages: a spent wall budget skips sweep, detail,
// and curate with stage errors naming the budget, while selection and
// notify still run and the run row closes (degraded, not open).
func TestBudget_ExpirySkipsStages(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	seedBacklog(t, f, 3)
	n := &recordingNotifier{}

	cfg := newCycle(f, []scraper.Scraper{&fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/swept", Title: "Swept", Company: "Swept Co"},
		},
	}})
	cfg.Notifier = n
	cfg.Budget = &Budget{Deadline: time.Now().Add(-time.Minute)}

	entry := Run(context.Background(), cfg)

	// Sweep skipped: nothing new entered the ledger; curate skipped: the
	// backlog stays "new" for the next cycle.
	if entry.PostingsNew != 0 {
		t.Errorf("postingsNew = %d, want 0 (sweep skipped)", entry.PostingsNew)
	}
	if _, ok, _ := f.GetPosting("https://example.com/swept"); ok {
		t.Error("swept posting entered the ledger despite a spent budget")
	}
	if got := countByStatus(f, db.StatusNew); got != 3 {
		t.Errorf("backlog left new = %d, want 3 (curate skipped)", got)
	}

	// One stage error per gated stage, each naming the budget.
	if len(entry.StageErrors) != 3 {
		t.Fatalf("stage errors = %v, want 3 (sweep, detail, curate)", entry.StageErrors)
	}
	for i, stage := range []string{"sweep", "detail", "curate"} {
		msg := entry.StageErrors[i]
		if !containsAll(msg, "wall budget", stage) {
			t.Errorf("stage error %d = %q, want it to name the wall budget and %q", i, msg, stage)
		}
	}

	// Select and notify still ran; the run row closed honestly.
	if entry.Verdict != db.VerdictDegraded {
		t.Errorf("verdict = %q, want degraded (stage errors recorded)", entry.Verdict)
	}
	if entry.FinishedAt == "" {
		t.Error("expected run to close with finished_at")
	}
}

// TestBudget_FarFutureBehavesLikeNoBudget: a wall limit far out changes
// nothing — every stage runs and every posting escalates.
func TestBudget_FarFutureBehavesLikeNoBudget(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	seedBacklog(t, f, 5)

	cfg := newCycle(f, nil)
	cfg.Budget = &Budget{Deadline: time.Now().Add(24 * time.Hour)}

	entry := Run(context.Background(), cfg)

	if entry.PostingsShortlisted != 5 {
		t.Errorf("postingsShortlisted = %d, want 5 (all stages ran)", entry.PostingsShortlisted)
	}
	if len(entry.StageErrors) != 0 {
		t.Errorf("stage errors = %v, want none", entry.StageErrors)
	}
}

// TestBudget_NilNormalizesToDefaults: Run with a nil Budget applies
// DefaultScoredCap and a DefaultWall that has not expired — the extra
// backlog above the cap stays "new" and no stage errors appear. A zen
// stub answers every curate so the scored cap (not the degraded path's
// unscored cap) is what truncates.
func TestBudget_NilNormalizesToDefaults(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	seedBacklog(t, f, DefaultScoredCap+5)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeSSE(w)
	}))
	t.Cleanup(srv.Close)

	cfg := newCycle(f, nil)
	cfg.ZenClient = zen.New(zen.Config{BaseURL: srv.URL, APIKey: "k", Model: "m"})

	entry := Run(context.Background(), cfg)

	if entry.PostingsShortlisted != DefaultScoredCap {
		t.Errorf("postingsShortlisted = %d, want %d (default cap)", entry.PostingsShortlisted, DefaultScoredCap)
	}
	if got := countByStatus(f, db.StatusNew); got != 5 {
		t.Errorf("postings left new = %d, want 5 (above the default cap)", got)
	}
	if len(entry.StageErrors) != 0 {
		t.Errorf("stage errors = %v, want none (default wall not spent)", entry.StageErrors)
	}
}

// containsAll reports whether s contains every substring.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
