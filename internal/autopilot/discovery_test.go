package autopilot

import (
	"context"
	"errors"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
	"github.com/udit-001/waypoint/internal/scraper"
)

// seedSweeperScraper gives the cycle one scraper so scoring has work.
func seedSweeperScraper() scraper.Scraper {
	return &fixtureScraper{
		name:    "discovery-stage-scraper",
		results: []scraper.Result{{URL: "https://example.com/d1", Title: "Eng", Company: "Acme"}},
	}
}

// TestCycle_FirstRunTriggersDiscovery: a fresh store (zero candidates)
// fires the first-run trigger before hunting; state is persisted so the
// next cycle does not re-fire, and scoring still completes.
func TestCycle_FirstRunTriggersDiscovery(t *testing.T) {
	f := db.NewFakeStore()
	s := seedSweeperScraper()

	calls := 0
	origRun := runAutoDiscovery
	runAutoDiscovery = func(_ context.Context, cfg CycleConfig, _ []discovery.Facet) (int, error) {
		calls++
		// Mirror production: discovered candidates land in the ledger.
		_ = cfg.Store.SaveCandidates([]db.CompanyCandidate{
			{Name: "Discovered A", Status: db.StatusCandidateSuggested},
			{Name: "Discovered B", Status: db.StatusCandidateSuggested},
		})
		return 2, nil
	}
	t.Cleanup(func() { runAutoDiscovery = origRun })

	entry := Run(context.Background(), CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{s},
		Recency:  14,
	})
	if calls != 1 {
		t.Errorf("discovery ran %d time(s), want exactly 1 on first run", calls)
	}
	if entry.PostingsNew != 1 {
		t.Errorf("scoring did not complete alongside discovery: new=%d", entry.PostingsNew)
	}

	_, _, has, _ := f.DiscoveryLastRun()
	if !has {
		t.Error("last-run state not persisted — daemon restarts would re-fire")
	}

	// Second cycle with unchanged brief and fresh state must skip.
	calls = 0
	Run(context.Background(), CycleConfig{Store: f, Scrapers: []scraper.Scraper{s}, Recency: 14})
	if calls != 0 {
		t.Errorf("discovery re-fired without a trigger (%d runs)", calls)
	}
}

// TestCycle_DiscoveryFailureDoesNotBlock: discovery failing (cause
// logged) leaves the cycle's sweep/scoring fully intact.
func TestCycle_DiscoveryFailureDoesNotBlock(t *testing.T) {
	f := db.NewFakeStore()
	s := seedSweeperScraper()

	origRun := runAutoDiscovery
	runAutoDiscovery = func(_ context.Context, _ CycleConfig, _ []discovery.Facet) (int, error) {
		return 0, errors.New("exa search: 429 rate limited")
	}
	t.Cleanup(func() { runAutoDiscovery = origRun })

	entry := Run(context.Background(), CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{s},
		Recency:  14,
	})
	if entry.PostingsNew != 1 {
		t.Errorf("cycle blocked by discovery failure: new=%d", entry.PostingsNew)
	}
	_, _, has, _ := f.DiscoveryLastRun()
	if has {
		t.Error("failed discovery must not persist last-run state")
	}
}
