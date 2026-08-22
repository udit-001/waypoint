// Package sweeper owns the per-board sweep: fetch one board, dedupe
// against the postings ledger and tracked jobs, add the fresh postings.
// The shared core behind 'waypoint boards sweep' (CLI) and the web
// Companies page's Sweep now button — so both surfaces stage, count,
// and record identically.
package sweeper

import (
	"context"
	"fmt"

	"github.com/udit-001/waypoint/internal/boards"
	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
)

// FetchBoard is the network seam: detect the provider and fetch the
// board's listings. Production delegates to boards.DetectProvider +
// Provider.Fetch; tests stub it (exported for cross-package tests —
// same pattern as discovery.ProbeBoard).
var FetchBoard = func(ctx context.Context, e config.BoardEntry, jobage, limit int) ([]scraper.Result, string, error) {
	b := boards.Board{Name: e.Name, Company: e.Company, URL: e.URL, MaxPages: e.MaxPages, Enabled: e.Enabled}
	p, hit, err := boards.DetectProvider(b)
	if err != nil {
		return nil, "", fmt.Errorf("no provider matched %s", e.URL)
	}
	results, err := p.Fetch(ctx, b, *hit, boards.FetchOpts{
		JobAgeDays: jobage,
		MaxPages:   e.MaxPages,
		Limit:      limit,
	})
	if err != nil {
		return nil, p.Name(), err
	}
	return results, p.Name(), nil
}

// Result is one board's sweep summary — the fields both surfaces report.
type Result struct {
	Fetched  int
	New      int
	Seen     int
	Provider string           // empty when detection failed before a provider ran
	Jobs     []scraper.Result // the postings added this sweep
}

// SweepOne fetches one board, dedupes each result against the postings
// ledger and tracked jobs, and batch-adds the fresh ones. Seen counts
// URLs skipped for either reason.
func SweepOne(ctx context.Context, store db.Store, e config.BoardEntry, jobage, limit int) (Result, error) {
	var res Result

	results, provider, err := FetchBoard(ctx, e, jobage, limit)
	if err != nil {
		if provider != "" {
			return res, fmt.Errorf("%s: %w", provider, err)
		}
		return res, err
	}
	res.Provider = provider
	res.Fetched = len(results)

	fresh := make([]scraper.Result, 0, len(results))
	for _, r := range results {
		seen, err := store.HasPosting(r.URL)
		if err != nil {
			return res, fmt.Errorf("check postings: %w", err)
		}
		if seen {
			res.Seen++
			continue
		}
		tracked, err := store.JobExists(r.URL)
		if err != nil {
			return res, fmt.Errorf("check jobs: %w", err)
		}
		if tracked {
			res.Seen++
			continue
		}
		fresh = append(fresh, r)
	}

	if len(fresh) > 0 {
		if err := store.AddPostings(fresh); err != nil {
			return res, fmt.Errorf("add postings: %w", err)
		}
	}
	res.New = len(fresh)
	res.Jobs = fresh
	return res, nil
}
