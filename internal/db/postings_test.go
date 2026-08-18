package db

import (
	"path/filepath"
	"testing"

	"github.com/udit-001/waypoint/internal/scraper"
)

func TestPostings_empty(t *testing.T) {
	f := NewFakeStore()

	seen, err := f.HasPosting("https://example.com")
	if err != nil {
		t.Fatalf("HasPosting error: %v", err)
	}
	if seen {
		t.Error("HasPosting should be false on empty ledger")
	}

	results, err := f.ListPostings("")
	if err != nil {
		t.Fatalf("ListPostings error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("List should return 0 on empty, got %d", len(results))
	}
}

func TestPostings_addAndHasPosting(t *testing.T) {
	f := NewFakeStore()

	results := []scraper.Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
		{ID: "2", Title: "Job B", URL: "https://example.com/2"},
	}

	if err := f.AddPostings(results); err != nil {
		t.Fatalf("AddPostings failed: %v", err)
	}

	seen, _ := f.HasPosting("https://example.com/1")
	if !seen {
		t.Error("HasPosting should be true after Add")
	}
	seen, _ = f.HasPosting("https://example.com/2")
	if !seen {
		t.Error("HasPosting should be true after Add")
	}
	seen, _ = f.HasPosting("https://example.com/3")
	if seen {
		t.Error("HasPosting should be false for un-added URL")
	}
}

func TestPostings_addIdempotent(t *testing.T) {
	f := NewFakeStore()

	results := []scraper.Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
	}
	f.AddPostings(results)

	// Re-add the same URL — should not duplicate
	if err := f.AddPostings(results); err != nil {
		t.Fatalf("second AddPostings failed: %v", err)
	}

	list, _ := f.ListPostings("")
	if len(list) != 1 {
		t.Errorf("expected 1 entry after re-adding, got %d", len(list))
	}
}

func TestPostings_listByStatus(t *testing.T) {
	f := NewFakeStore()

	f.AddPostings([]scraper.Result{
		{ID: "1", Title: "New", URL: "https://example.com/1"},
		{ID: "2", Title: "Also New", URL: "https://example.com/2"},
	})

	f.SetPostingStatus("https://example.com/1", "dismissed")

	newResults, _ := f.ListPostings("new")
	if len(newResults) != 1 {
		t.Fatalf("expected 1 new, got %d", len(newResults))
	}
	if newResults[0].Result.URL != "https://example.com/2" {
		t.Errorf("expected URL 2, got %s", newResults[0].Result.URL)
	}

	dismissed, _ := f.ListPostings("dismissed")
	if len(dismissed) != 1 {
		t.Errorf("expected 1 dismissed, got %d", len(dismissed))
	}

	all, _ := f.ListPostings("")
	if len(all) != 2 {
		t.Errorf("expected 2 total, got %d", len(all))
	}
}

func TestPostings_setStatus(t *testing.T) {
	f := NewFakeStore()

	f.AddPostings([]scraper.Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
	})

	if err := f.SetPostingStatus("https://example.com/1", "dismissed"); err != nil {
		t.Fatalf("SetPostingStatus failed: %v", err)
	}

	sr, ok, _ := f.GetPosting("https://example.com/1")
	if !ok {
		t.Fatal("GetPosting should find the entry")
	}
	if sr.Status != "dismissed" {
		t.Errorf("expected status dismissed, got %s", sr.Status)
	}

	// Idempotent — setting same status again
	if err := f.SetPostingStatus("https://example.com/1", "dismissed"); err != nil {
		t.Fatalf("second SetPostingStatus failed: %v", err)
	}

	// Unknown URL — should error
	if err := f.SetPostingStatus("https://example.com/unknown", "dismissed"); err == nil {
		t.Error("expected error for unknown URL")
	}

	// "promoted" status also works
	if err := f.SetPostingStatus("https://example.com/1", StatusPromoted); err != nil {
		t.Fatalf("SetPostingStatus to promoted failed: %v", err)
	}
	sr, _, _ = f.GetPosting("https://example.com/1")
	if sr.Status != StatusPromoted {
		t.Errorf("expected status promoted, got %s", sr.Status)
	}
}

func TestPostings_getPosting(t *testing.T) {
	f := NewFakeStore()

	f.AddPostings([]scraper.Result{
		{ID: "1", Title: "Job A", Company: "Corp", URL: "https://example.com/1"},
	})

	sr, ok, err := f.GetPosting("https://example.com/1")
	if err != nil {
		t.Fatalf("GetPosting error: %v", err)
	}
	if !ok {
		t.Fatal("GetPosting should find the entry")
	}
	if sr.Result.Title != "Job A" {
		t.Errorf("expected title 'Job A', got %s", sr.Result.Title)
	}
	if sr.Result.Company != "Corp" {
		t.Errorf("expected company 'Corp', got %s", sr.Result.Company)
	}
	if sr.Status != "new" {
		t.Errorf("expected status 'new', got %s", sr.Status)
	}

	// Unknown URL — returns false, no error
	_, ok, err = f.GetPosting("https://example.com/unknown")
	if err != nil {
		t.Fatalf("GetPosting unknown URL error: %v", err)
	}
	if ok {
		t.Error("GetPosting should return false for unknown URL")
	}
}

func TestPostings_prune(t *testing.T) {
	f := NewFakeStore()

	f.AddPostings([]scraper.Result{
		{ID: "1", Title: "Old", URL: "https://example.com/old"},
		{ID: "2", Title: "Recent", URL: "https://example.com/recent"},
	})

	// Force old date on first entry
	sr := f.Postings["https://example.com/old"]
	sr.FirstSeen = "2020-01-01"
	f.Postings["https://example.com/old"] = sr

	removed, err := f.PrunePostings(30)
	if err != nil {
		t.Fatalf("PrunePostings failed: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	seen, _ := f.HasPosting("https://example.com/old")
	if seen {
		t.Error("old entry should be pruned")
	}
	seen, _ = f.HasPosting("https://example.com/recent")
	if !seen {
		t.Error("recent entry should remain")
	}
}

func TestPostings_enrich(t *testing.T) {
	f := NewFakeStore()

	f.AddPostings([]scraper.Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
	})

	// Enrich with description and metadata
	if err := f.EnrichPosting("https://example.com/1", "A great role", map[string]string{
		"salary": "100k",
	}); err != nil {
		t.Fatalf("EnrichPosting failed: %v", err)
	}

	sr, _, _ := f.GetPosting("https://example.com/1")
	if sr.Result.Description != "A great role" {
		t.Errorf("expected description 'A great role', got %s", sr.Result.Description)
	}
	if sr.Result.Metadata["salary"] != "100k" {
		t.Errorf("expected metadata salary=100k, got %v", sr.Result.Metadata)
	}

	// Enrich again — metadata should merge, not replace
	if err := f.EnrichPosting("https://example.com/1", "Updated desc", map[string]string{
		"remote": "yes",
	}); err != nil {
		t.Fatalf("second EnrichPosting failed: %v", err)
	}

	sr, _, _ = f.GetPosting("https://example.com/1")
	if sr.Result.Description != "Updated desc" {
		t.Errorf("expected description 'Updated desc', got %s", sr.Result.Description)
	}
	if sr.Result.Metadata["salary"] != "100k" {
		t.Error("original metadata should persist after merge")
	}
	if sr.Result.Metadata["remote"] != "yes" {
		t.Error("new metadata should be merged in")
	}

	// Enrich unknown URL — no-op, no error
	if err := f.EnrichPosting("https://example.com/unknown", "desc", nil); err != nil {
		t.Fatalf("EnrichPosting on unknown URL should not error, got: %v", err)
	}
}

func TestPostings_addPreservesResultFields(t *testing.T) {
	f := NewFakeStore()

	results := []scraper.Result{
		{
			ID:          "42",
			Title:       "Senior Engineer",
			Company:     "Acme",
			Location:    "Remote",
			Date:        "2026-08-01",
			URL:         "https://example.com/job/42",
			Description: "Full-stack role",
			Metadata:    map[string]string{"level": "senior", "team": "platform"},
		},
	}

	if err := f.AddPostings(results); err != nil {
		t.Fatalf("AddPostings failed: %v", err)
	}

	sr, ok, _ := f.GetPosting("https://example.com/job/42")
	if !ok {
		t.Fatal("GetPosting should find the entry")
	}
	if sr.Result.Title != "Senior Engineer" {
		t.Errorf("expected title 'Senior Engineer', got %s", sr.Result.Title)
	}
	if sr.Result.Company != "Acme" {
		t.Errorf("expected company 'Acme', got %s", sr.Result.Company)
	}
	if sr.Result.Location != "Remote" {
		t.Errorf("expected location 'Remote', got %s", sr.Result.Location)
	}
	if sr.Result.Date != "2026-08-01" {
		t.Errorf("expected date '2026-08-01', got %s", sr.Result.Date)
	}
	if sr.Result.Description != "Full-stack role" {
		t.Errorf("expected description, got %s", sr.Result.Description)
	}
	if sr.Result.Metadata["level"] != "senior" {
		t.Errorf("expected metadata level=senior, got %v", sr.Result.Metadata)
	}
	if sr.Result.Metadata["team"] != "platform" {
		t.Errorf("expected metadata team=platform, got %v", sr.Result.Metadata)
	}
}

// --- Dismiss batch tests ---

func TestDismiss_multipleURLs(t *testing.T) {
	f := NewFakeStore()

	f.AddPostings([]scraper.Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
		{ID: "2", Title: "Job B", URL: "https://example.com/2"},
		{ID: "3", Title: "Job C", URL: "https://example.com/3"},
	})

	// Dismiss multiple URLs, simulating the batch path: a non-existent
	// URL errors but doesn't prevent dismissing the others.
	urls := []string{
		"https://example.com/1",
		"https://example.com/missing",
		"https://example.com/3",
	}
	dismissed := 0
	for _, url := range urls {
		if err := f.SetPostingStatus(url, "dismissed"); err != nil {
			continue
		}
		dismissed++
	}

	if dismissed != 2 {
		t.Errorf("expected 2 dismissed, got %d", dismissed)
	}

	// Dismissed URLs should be "dismissed"
	sr, _, _ := f.GetPosting("https://example.com/1")
	if sr.Status != "dismissed" {
		t.Errorf("URL 1: expected status 'dismissed', got %q", sr.Status)
	}
	sr, _, _ = f.GetPosting("https://example.com/3")
	if sr.Status != "dismissed" {
		t.Errorf("URL 3: expected status 'dismissed', got %q", sr.Status)
	}

	// Untouched URL should still be "new"
	sr, _, _ = f.GetPosting("https://example.com/2")
	if sr.Status != "new" {
		t.Errorf("URL 2: expected status 'new', got %q", sr.Status)
	}
}

func TestDismiss_allBatch(t *testing.T) {
	f := NewFakeStore()

	// Three "new" results
	f.AddPostings([]scraper.Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
		{ID: "2", Title: "Job B", URL: "https://example.com/2"},
		{ID: "3", Title: "Job C", URL: "https://example.com/3"},
	})

	// One dismissed — should be skipped by --all
	f.AddPostings([]scraper.Result{
		{ID: "4", Title: "Job D", URL: "https://example.com/4"},
	})
	f.SetPostingStatus("https://example.com/4", "dismissed")

	// One already promoted — should be skipped by --all
	f.AddPostings([]scraper.Result{
		{ID: "5", Title: "Job E", URL: "https://example.com/5"},
	})
	f.SetPostingStatus("https://example.com/5", StatusPromoted)

	// Dismiss all "new" results
	newResults, _ := f.ListPostings("new")
	dismissed := 0
	for _, r := range newResults {
		if err := f.SetPostingStatus(r.Result.URL, "dismissed"); err != nil {
			t.Fatalf("SetPostingStatus %s failed: %v", r.Result.URL, err)
		}
		dismissed++
	}

	if dismissed != 3 {
		t.Errorf("expected 3 dismissed, got %d", dismissed)
	}

	// All "new" entries should now be "dismissed"
	newAfter, _ := f.ListPostings("new")
	if len(newAfter) != 0 {
		t.Errorf("expected 0 new after --all, got %d", len(newAfter))
	}

	// Dismissed and promoted entries should be unchanged
	sr, _, _ := f.GetPosting("https://example.com/4")
	if sr.Status != "dismissed" {
		t.Errorf("dismissed entry should stay dismissed, got %q", sr.Status)
	}
	sr, _, _ = f.GetPosting("https://example.com/5")
	if sr.Status != StatusPromoted {
		t.Errorf("promoted entry should stay promoted, got %q", sr.Status)
	}
}

// --- SQLite round-trip ---

// TestPostings_sqliteRoundTrip exercises the postings ledger against the
// concrete SQLite store: every status in the vocabulary round-trips,
// promote creates a jobs row and marks the posting promoted, and promote
// is idempotent on URL.
func TestPostings_sqliteRoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	if err := s.AddPostings([]scraper.Result{
		{ID: "1", Title: "Job A", Company: "Acme", URL: "https://example.com/1"},
		{ID: "2", Title: "Job B", Company: "Bolt", URL: "https://example.com/2"},
		{ID: "3", Title: "Job C", Company: "Cove", URL: "https://example.com/3"},
	}); err != nil {
		t.Fatalf("AddPostings: %v", err)
	}

	has, err := s.HasPosting("https://example.com/1")
	if err != nil || !has {
		t.Fatalf("HasPosting after AddPostings = %v, %v", has, err)
	}

	// Every status in the vocabulary round-trips through the ledger.
	for url, status := range map[string]string{
		"https://example.com/1": StatusNew,
		"https://example.com/2": StatusShortlisted,
		"https://example.com/3": StatusDismissed,
	} {
		if url == "https://example.com/1" {
			continue // stays new
		}
		if err := s.SetPostingStatus(url, status); err != nil {
			t.Fatalf("SetPostingStatus(%s): %v", url, err)
		}
	}
	for url, want := range map[string]string{
		"https://example.com/1": StatusNew,
		"https://example.com/2": StatusShortlisted,
		"https://example.com/3": StatusDismissed,
	} {
		p, ok, err := s.GetPosting(url)
		if err != nil || !ok {
			t.Fatalf("GetPosting(%s): ok=%v err=%v", url, ok, err)
		}
		if p.Status != want {
			t.Errorf("%s: status = %q, want %q", url, p.Status, want)
		}
	}

	shortlisted, err := s.ListPostings(StatusShortlisted)
	if err != nil || len(shortlisted) != 1 {
		t.Errorf("ListPostings(shortlisted) = %d entries, %v; want 1", len(shortlisted), err)
	}

	// Promote creates a jobs row and marks the posting promoted.
	job, err := s.Promote("https://example.com/2")
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	if job.ID == 0 || job.Position != "Job B" || job.Status != "Not Applied" {
		t.Errorf("promoted job = %+v", job)
	}
	p, _, _ := s.GetPosting("https://example.com/2")
	if p.Status != StatusPromoted {
		t.Errorf("posting after promote = %q, want %q", p.Status, StatusPromoted)
	}

	// Promote is idempotent on URL — no second jobs row.
	job2, err := s.Promote("https://example.com/2")
	if err != nil {
		t.Fatalf("second Promote: %v", err)
	}
	if job2.ID != 0 {
		t.Errorf("second promote should skip, got job %d", job2.ID)
	}
	count, _ := s.JobCount()
	if count != 1 {
		t.Errorf("JobCount = %d, want 1", count)
	}
}

// TestPostings_fakeShortlisted mirrors the SQLite round-trip for the
// shortlisted status on the FakeStore (CLI/server tests use the fake).
func TestPostings_fakeShortlisted(t *testing.T) {
	f := NewFakeStore()

	f.AddPostings([]scraper.Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
	})

	if err := f.SetPostingStatus("https://example.com/1", StatusShortlisted); err != nil {
		t.Fatalf("SetPostingStatus(shortlisted): %v", err)
	}

	shortlisted, _ := f.ListPostings(StatusShortlisted)
	if len(shortlisted) != 1 {
		t.Fatalf("ListPostings(shortlisted) = %d, want 1", len(shortlisted))
	}
	if shortlisted[0].Status != StatusShortlisted {
		t.Errorf("status = %q, want %q", shortlisted[0].Status, StatusShortlisted)
	}
}
