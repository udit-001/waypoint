package autopilot

import (
	"context"
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

	entry := Run(context.Background(), CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{s},
		ExaCap:   0,
		Recency:  14,
		Notifier: n,
	})

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

	Run(context.Background(), CycleConfig{
		Store:    f,
		Scrapers: nil, // nothing swept, backlog empty
		ExaCap:   0,
		Recency:  14,
		Notifier: n,
	})

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
	entry := Run(context.Background(), CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{s},
		ExaCap:   0,
		Recency:  14,
		Notifier: nil,
	})

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

	entry := Run(context.Background(), CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{s},
		ExaCap:   0,
		Recency:  14,
		Notifier: n,
	})

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

	n := buildNotice(context.Background(), f, 2)

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
