package autopilot

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
)

// --- fixtures ---

// fixtureScraper is a fake scraper that returns predetermined results.
type fixtureScraper struct {
	name    string
	results []scraper.Result
}

func (f *fixtureScraper) Name() string         { return f.name }
func (f *fixtureScraper) Source() string       { return f.name }
func (f *fixtureScraper) Categories() []string { return []string{"test"} }
func (f *fixtureScraper) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	return f.results, nil
}

// --- tests ---

func TestCycle_SweepAddPostings(t *testing.T) {
	f := db.NewFakeStore()
	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Engineer", Company: "Acme"},
			{URL: "https://example.com/b", Title: "Designer", Company: "Beta"},
		},
	}

	entry := Run(context.Background(), CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{s},
		ExaCap:   0, // no Exa
		Recency:  14,
	})

	if entry.PostingsNew != 2 {
		t.Errorf("postingsNew = %d, want 2", entry.PostingsNew)
	}

	// Verify postings were added to the ledger.
	postings, _ := f.ListPostings("")
	if len(postings) != 2 {
		t.Fatalf("expected 2 postings in ledger, got %d", len(postings))
	}
}

func TestCycle_DedupAgainstExisting(t *testing.T) {
	f := db.NewFakeStore()
	// Pre-seed one posting.
	_ = f.AddPostings([]scraper.Result{
		{URL: "https://example.com/a", Title: "Existing"},
	})

	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Existing"}, // dup
			{URL: "https://example.com/b", Title: "New"},
		},
	}

	entry := Run(context.Background(), CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{s},
		ExaCap:   0,
		Recency:  14,
	})

	if entry.PostingsNew != 1 {
		t.Errorf("postingsNew = %d, want 1 (dedup)", entry.PostingsNew)
	}
}

func TestCycle_DedupAgainstJobs(t *testing.T) {
	f := db.NewFakeStore()
	// Pre-seed a job with the same URL.
	f.Jobs[1] = db.Job{URL: "https://example.com/a", Position: "Tracked"}

	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Tracked"},
			{URL: "https://example.com/b", Title: "New"},
		},
	}

	entry := Run(context.Background(), CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{s},
		ExaCap:   0,
		Recency:  14,
	})

	if entry.PostingsNew != 1 {
		t.Errorf("postingsNew = %d, want 1 (dedup against jobs)", entry.PostingsNew)
	}
}

func TestCycle_PrefilterAvoidCompany(t *testing.T) {
	f := db.NewFakeStore()
	// Set up avoid list.
	_ = f.UpsertProfile(map[string]any{
		"avoid_companies": `["evil corp"]`,
	})

	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Engineer", Company: "Evil Corp"},
			{URL: "https://example.com/b", Title: "Designer", Company: "Good Co"},
		},
	}

	entry := Run(context.Background(), CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{s},
		ExaCap:   0,
		Recency:  14,
	})

	if entry.PostingsNew != 2 {
		t.Errorf("postingsNew = %d, want 2", entry.PostingsNew)
	}

	// Evil Corp should be dismissed, Good Co should be shortlisted (no zen).
	p1, _, _ := f.GetPosting("https://example.com/a")
	if p1.Status != db.StatusDismissed {
		t.Errorf("Evil Corp status = %q, want dismissed", p1.Status)
	}

	p2, _, _ := f.GetPosting("https://example.com/b")
	if p2.Status != db.StatusShortlisted {
		t.Errorf("Good Co status = %q, want shortlisted", p2.Status)
	}

	if entry.PostingsDismissed != 1 {
		t.Errorf("postingsDismissed = %d, want 1", entry.PostingsDismissed)
	}
	if entry.PostingsShortlisted != 1 {
		t.Errorf("postingsShortlisted = %d, want 1", entry.PostingsShortlisted)
	}
}

func TestCycle_PrefilterTargetCompany(t *testing.T) {
	f := db.NewFakeStore()
	_ = f.UpsertProfile(map[string]any{
		"companies": `["google"]`,
	})

	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "SWE", Company: "Google"},
		},
	}

	entry := Run(context.Background(), CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{s},
		ExaCap:   0,
		Recency:  14,
	})

	p, _, _ := f.GetPosting("https://example.com/a")
	if p.Status != db.StatusShortlisted {
		t.Errorf("Google status = %q, want shortlisted", p.Status)
	}
	if entry.PostingsShortlisted != 1 {
		t.Errorf("postingsShortlisted = %d, want 1", entry.PostingsShortlisted)
	}
}

func TestCycle_NoZenEscalatesToShortlist(t *testing.T) {
	f := db.NewFakeStore()
	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Engineer", Company: "Unknown Co"},
		},
	}

	// No zen client — should escalate to shortlist for manual review.
	entry := Run(context.Background(), CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{s},
		ExaCap:   0,
		Recency:  14,
	})

	p, _, _ := f.GetPosting("https://example.com/a")
	if p.Status != db.StatusShortlisted {
		t.Errorf("status = %q, want shortlisted (no zen = escalate)", p.Status)
	}
	if entry.PostingsShortlisted != 1 {
		t.Errorf("postingsShortlisted = %d, want 1", entry.PostingsShortlisted)
	}
}

func TestCycle_RunLogRecorded(t *testing.T) {
	f := db.NewFakeStore()
	s := &fixtureScraper{
		name:    "test-scraper",
		results: []scraper.Result{},
	}

	entry := Run(context.Background(), CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{s},
		ExaCap:   0,
		Recency:  14,
	})

	// Store the run log.
	id, err := f.AddRunLog(entry)
	if err != nil {
		t.Fatalf("AddRunLog: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}

	// Verify last run.
	last, ok, err := f.GetLastRun()
	if err != nil {
		t.Fatalf("GetLastRun: %v", err)
	}
	if !ok {
		t.Fatal("expected last run")
	}
	if last.StartedAt == "" {
		t.Error("expected started_at")
	}
	if last.FinishedAt == "" {
		t.Error("expected finished_at")
	}
	if last.DurationMs <= 0 {
		t.Errorf("durationMs = %d, want > 0", last.DurationMs)
	}

	// Verify JSON serialization.
	b, err := json.Marshal(last)
	if err != nil {
		t.Fatalf("marshal run log: %v", err)
	}
	var parsed db.RunLog
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal run log: %v", err)
	}
}

func TestCycle_ContextCancellation(t *testing.T) {
	f := db.NewFakeStore()
	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Engineer", Company: "Acme"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	entry := Run(ctx, CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{s},
		ExaCap:   0,
		Recency:  14,
	})

	// Should still complete (sweep may add postings before checking context).
	if entry.StartedAt == "" {
		t.Error("expected started_at even with cancelled context")
	}
}

func TestCycle_MultipleScrapers(t *testing.T) {
	f := db.NewFakeStore()
	s1 := &fixtureScraper{
		name:    "scraper-1",
		results: []scraper.Result{{URL: "https://a.com/1", Title: "Job 1"}},
	}
	s2 := &fixtureScraper{
		name:    "scraper-2",
		results: []scraper.Result{{URL: "https://b.com/2", Title: "Job 2"}},
	}

	entry := Run(context.Background(), CycleConfig{
		Store:    f,
		Scrapers: []scraper.Scraper{s1, s2},
		ExaCap:   0,
		Recency:  14,
	})

	if entry.PostingsNew != 2 {
		t.Errorf("postingsNew = %d, want 2 (from 2 scrapers)", entry.PostingsNew)
	}
}

func TestAddError(t *testing.T) {
	j := addError("", "first error")
	if j != `["first error"]` {
		t.Errorf("addError empty = %q", j)
	}

	j = addError(j, "second error")
	var errs []string
	if err := json.Unmarshal([]byte(j), &errs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(errs) != 2 {
		t.Errorf("len = %d, want 2", len(errs))
	}
}

func TestBuildPrefilterProfile(t *testing.T) {
	f := db.NewFakeStore()
	_ = f.UpsertProfile(map[string]any{
		"avoid_companies": `["evil corp"]`,
		"companies":       `["google", "stripe"]`,
		"salary_floor":    `[{"region":"IN","amount":1000000}]`,
	})

	profile := buildPrefilterProfile(f)
	if len(profile.AvoidCompanies) != 1 {
		t.Errorf("avoid = %v, want 1 entry", profile.AvoidCompanies)
	}
	if len(profile.Companies) != 2 {
		t.Errorf("companies = %v, want 2 entries", profile.Companies)
	}
	if len(profile.SalaryFloor) != 1 {
		t.Errorf("salaryFloor = %v, want 1 entry", profile.SalaryFloor)
	}
	if profile.SalaryFloor[0].Currency != "INR" {
		t.Errorf("currency = %q, want INR", profile.SalaryFloor[0].Currency)
	}
}

func init() {
	// Override sleep for tests (no backoff delays).
	sleep = func(d time.Duration) {}
}
