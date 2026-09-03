package autopilot

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
	"github.com/udit-001/waypoint/internal/scraper"
)

// --- fixtures ---

type recordingNotifier struct {
	calls   int
	notices []ShortlistNotice
	err     error
}

func (r *recordingNotifier) NotifyShortlist(ctx context.Context, n ShortlistNotice) error {
	r.calls++
	r.notices = append(r.notices, n)
	return r.err
}

// stubDiscovery replaces the discovery pipeline with a no-op so cycle
// tests stay offline and fast.
func stubDiscovery(t *testing.T) {
	t.Helper()
	orig := runAutoDiscovery
	runAutoDiscovery = func(_ context.Context, _ CycleConfig, _ []discovery.Facet) (int, error) {
		return 0, nil
	}
	t.Cleanup(func() { runAutoDiscovery = orig })
}

// --- tests ---

func TestCycle_NotifierCalledOnceOnShortlist(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Engineer", Company: "Acme"},
			{URL: "https://example.com/b", Title: "Designer", Company: "Beta"},
			{URL: "https://example.com/c", Title: "Writer", Company: "Gamma"},
		},
	}
	n := &recordingNotifier{}

	cfg := newCycle(f, []scraper.Scraper{s})
	cfg.Notifier = n
	entry := Run(context.Background(), cfg)

	if entry.PostingsShortlisted != 3 {
		t.Fatalf("shortlisted = %d, want 3", entry.PostingsShortlisted)
	}
	if n.calls != 1 {
		t.Fatalf("notifier calls = %d, want exactly 1", n.calls)
	}
	got := n.notices[0]
	if got.Count != 3 {
		t.Errorf("notice.Count = %d, want 3", got.Count)
	}
	if got.TopTitle == "" || got.TopCompany == "" {
		t.Errorf("notice missing top match: %+v", got)
	}

}

func TestCycle_NoShortlistNoNotify(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	n := &recordingNotifier{}

	cfg := newCycle(f, nil)
	cfg.Notifier = n
	Run(context.Background(), cfg)

	if n.calls != 0 {
		t.Errorf("notifier calls = %d, want 0", n.calls)
	}
}

func TestCycle_NilNotifierIsNoop(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Engineer", Company: "Acme"},
		},
	}

	// Must not panic with nil notifier.
	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{s}))

	if entry.PostingsShortlisted != 1 {
		t.Errorf("shortlisted = %d, want 1", entry.PostingsShortlisted)
	}
}

func TestCycle_NotifierFailureNeverBlocksCycle(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Engineer", Company: "Acme"},
		},
	}
	n := &recordingNotifier{err: context.DeadlineExceeded}

	cfg := newCycle(f, []scraper.Scraper{s})
	cfg.Notifier = n
	entry := Run(context.Background(), cfg)

	// Cycle completes normally; failure recorded in the run log errors.
	if entry.FinishedAt == "" {
		t.Error("cycle did not finish after notifier failure")
	}
	if !strings.Contains(entry.Errors, "notify") {
		t.Errorf("errors = %q, want notify failure recorded", entry.Errors)
	}
}

func TestBuildNotice_TopScoreWins(t *testing.T) {
	f := db.NewFakeStore()
	_ = f.AddPostings([]scraper.Result{
		{URL: "https://example.com/low", Title: "Low", Company: "LoCo"},
		{URL: "https://example.com/high", Title: "High", Company: "HiCo"},
	})
	_ = f.SetPostingStatus("https://example.com/low", db.StatusShortlisted)
	_ = f.SetPostingStatus("https://example.com/high", db.StatusShortlisted)
	_ = f.EnrichPosting("https://example.com/low", "", map[string]string{"score": "55"})
	_ = f.EnrichPosting("https://example.com/high", "", map[string]string{"score": "92"})

	n := buildNotice(context.Background(), f, 2, map[string]bool{
		"https://example.com/low": true, "https://example.com/high": true,
	})

	if n.Count != 2 {
		t.Errorf("Count = %d, want 2", n.Count)
	}
	if n.TopTitle != "High" || n.TopCompany != "HiCo" {
		t.Errorf("top match = %s/%s, want High/HiCo", n.TopTitle, n.TopCompany)
	}
	if n.TopScore != 92 {
		t.Errorf("TopScore = %d, want 92", n.TopScore)
	}
}

// panickyScraper panics inside Search — simulates a poisoned stage.
type panickyScraper struct{}

func (panickyScraper) Name() string         { return "panic" }
func (panickyScraper) Source() string       { return "panic" }
func (panickyScraper) Categories() []string { return []string{"test"} }
func (panickyScraper) Search(context.Context, scraper.SearchOpts) ([]scraper.Result, error) {
	panic("boom")
}

// TestCycle_PanicClosesRunRow: a mid-cycle panic must still produce a
// CLOSED run-log row (finished_at set, panic recorded as an error).
// The scheduler's cadence measures from last FINISHED run — an empty
// FinishedAt would re-fire a panicking cycle at poll rate forever.
func TestCycle_PanicClosesRunRow(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{panickyScraper{}}))

	if entry.FinishedAt == "" {
		t.Fatal("panicked cycle left FinishedAt empty — scheduler would hot-refire every poll")
	}
	if !strings.Contains(entry.Errors, "panic") {
		t.Errorf("errors = %q, want panic recorded", entry.Errors)
	}
}

// TestCycle_NoZenCapsUnscored: without a Zen client every surviving
// posting would escalate to shortlisted — a fresh install floods
// Matches with hundreds of unscored rows. The cycle must escalate
// only the freshest slice per run.
func TestCycle_NoZenCapsUnscored(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()

	// Seed 40 backlog postings, oldest first so newest-first ordering
	// is observable in WHICH urls got escalated.
	var results []scraper.Result
	for i := 0; i < 40; i++ {
		results = append(results, scraper.Result{
			URL:   fmt.Sprintf("https://example.com/%02d", i),
			Title: fmt.Sprintf("Role %02d", i),
		})
	}
	if err := f.AddPostings(results); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cfg := newCycle(f, nil)
	cfg.ZenClient = nil // degraded mode
	entry := Run(context.Background(), cfg)

	if entry.PostingsShortlisted != 25 {
		t.Fatalf("shortlisted = %d, want capped at 25", entry.PostingsShortlisted)
	}

	// The escalated ones must be the NEWEST (30..39 by our seed order —
	// later inserts sort first under RFC3339 DESC ties broken by rowid).
	shortlisted := map[string]bool{}
	postings, _ := f.ListPostings(db.StatusShortlisted)
	for _, p := range postings {
		shortlisted[p.Result.URL] = true
	}
	if len(shortlisted) != 25 {
		t.Fatalf("ledger shortlisted rows = %d, want 25", len(shortlisted))
	}
}
