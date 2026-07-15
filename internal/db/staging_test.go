package db

import (
	"testing"

	"github.com/udit-001/waypoint/internal/scraper"
)

func TestStaging_empty(t *testing.T) {
	f := NewFakeStore()

	seen, err := f.IsSeen("https://example.com")
	if err != nil {
		t.Fatalf("IsSeen error: %v", err)
	}
	if seen {
		t.Error("IsSeen should be false on empty staging")
	}

	results, err := f.ListStaging("")
	if err != nil {
		t.Fatalf("ListStaging error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("List should return 0 on empty, got %d", len(results))
	}
}

func TestStaging_addAndIsSeen(t *testing.T) {
	f := NewFakeStore()

	results := []scraper.Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
		{ID: "2", Title: "Job B", URL: "https://example.com/2"},
	}

	if err := f.AddStaging(results); err != nil {
		t.Fatalf("AddStaging failed: %v", err)
	}

	seen, _ := f.IsSeen("https://example.com/1")
	if !seen {
		t.Error("IsSeen should be true after Add")
	}
	seen, _ = f.IsSeen("https://example.com/2")
	if !seen {
		t.Error("IsSeen should be true after Add")
	}
	seen, _ = f.IsSeen("https://example.com/3")
	if seen {
		t.Error("IsSeen should be false for un-added URL")
	}
}

func TestStaging_addIdempotent(t *testing.T) {
	f := NewFakeStore()

	results := []scraper.Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
	}
	f.AddStaging(results)

	// Re-add the same URL — should not duplicate
	if err := f.AddStaging(results); err != nil {
		t.Fatalf("second AddStaging failed: %v", err)
	}

	list, _ := f.ListStaging("")
	if len(list) != 1 {
		t.Errorf("expected 1 entry after re-adding, got %d", len(list))
	}
}

func TestStaging_listByStatus(t *testing.T) {
	f := NewFakeStore()

	f.AddStaging([]scraper.Result{
		{ID: "1", Title: "New", URL: "https://example.com/1"},
		{ID: "2", Title: "Also New", URL: "https://example.com/2"},
	})

	f.SetStagingStatus("https://example.com/1", "dismissed")

	newResults, _ := f.ListStaging("new")
	if len(newResults) != 1 {
		t.Fatalf("expected 1 new, got %d", len(newResults))
	}
	if newResults[0].Result.URL != "https://example.com/2" {
		t.Errorf("expected URL 2, got %s", newResults[0].Result.URL)
	}

	dismissed, _ := f.ListStaging("dismissed")
	if len(dismissed) != 1 {
		t.Errorf("expected 1 dismissed, got %d", len(dismissed))
	}

	all, _ := f.ListStaging("")
	if len(all) != 2 {
		t.Errorf("expected 2 total, got %d", len(all))
	}
}

func TestStaging_setStatus(t *testing.T) {
	f := NewFakeStore()

	f.AddStaging([]scraper.Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
	})

	if err := f.SetStagingStatus("https://example.com/1", "dismissed"); err != nil {
		t.Fatalf("SetStagingStatus failed: %v", err)
	}

	sr, ok, _ := f.GetStaged("https://example.com/1")
	if !ok {
		t.Fatal("GetStaged should find the entry")
	}
	if sr.Status != "dismissed" {
		t.Errorf("expected status dismissed, got %s", sr.Status)
	}

	// Idempotent — setting same status again
	if err := f.SetStagingStatus("https://example.com/1", "dismissed"); err != nil {
		t.Fatalf("second SetStagingStatus failed: %v", err)
	}

	// Unknown URL — should error
	if err := f.SetStagingStatus("https://example.com/unknown", "dismissed"); err == nil {
		t.Error("expected error for unknown URL")
	}

	// "imported" status also works
	if err := f.SetStagingStatus("https://example.com/1", "imported"); err != nil {
		t.Fatalf("SetStagingStatus to imported failed: %v", err)
	}
	sr, _, _ = f.GetStaged("https://example.com/1")
	if sr.Status != "imported" {
		t.Errorf("expected status imported, got %s", sr.Status)
	}
}

func TestStaging_getStaged(t *testing.T) {
	f := NewFakeStore()

	f.AddStaging([]scraper.Result{
		{ID: "1", Title: "Job A", Company: "Corp", URL: "https://example.com/1"},
	})

	sr, ok, err := f.GetStaged("https://example.com/1")
	if err != nil {
		t.Fatalf("GetStaged error: %v", err)
	}
	if !ok {
		t.Fatal("GetStaged should find the entry")
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
	_, ok, err = f.GetStaged("https://example.com/unknown")
	if err != nil {
		t.Fatalf("GetStaged unknown URL error: %v", err)
	}
	if ok {
		t.Error("GetStaged should return false for unknown URL")
	}
}

func TestStaging_prune(t *testing.T) {
	f := NewFakeStore()

	f.AddStaging([]scraper.Result{
		{ID: "1", Title: "Old", URL: "https://example.com/old"},
		{ID: "2", Title: "Recent", URL: "https://example.com/recent"},
	})

	// Force old date on first entry
	sr := f.Staging["https://example.com/old"]
	sr.FirstSeen = "2020-01-01"
	f.Staging["https://example.com/old"] = sr

	removed, err := f.PruneStaging(30)
	if err != nil {
		t.Fatalf("PruneStaging failed: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	seen, _ := f.IsSeen("https://example.com/old")
	if seen {
		t.Error("old entry should be pruned")
	}
	seen, _ = f.IsSeen("https://example.com/recent")
	if !seen {
		t.Error("recent entry should remain")
	}
}

func TestStaging_enrich(t *testing.T) {
	f := NewFakeStore()

	f.AddStaging([]scraper.Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
	})

	// Enrich with description and metadata
	if err := f.EnrichStaging("https://example.com/1", "A great role", map[string]string{
		"salary": "100k",
	}); err != nil {
		t.Fatalf("EnrichStaging failed: %v", err)
	}

	sr, _, _ := f.GetStaged("https://example.com/1")
	if sr.Result.Description != "A great role" {
		t.Errorf("expected description 'A great role', got %s", sr.Result.Description)
	}
	if sr.Result.Metadata["salary"] != "100k" {
		t.Errorf("expected metadata salary=100k, got %v", sr.Result.Metadata)
	}

	// Enrich again — metadata should merge, not replace
	if err := f.EnrichStaging("https://example.com/1", "Updated desc", map[string]string{
		"remote": "yes",
	}); err != nil {
		t.Fatalf("second EnrichStaging failed: %v", err)
	}

	sr, _, _ = f.GetStaged("https://example.com/1")
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
	if err := f.EnrichStaging("https://example.com/unknown", "desc", nil); err != nil {
		t.Fatalf("EnrichStaging on unknown URL should not error, got: %v", err)
	}
}

func TestStaging_addPreservesResultFields(t *testing.T) {
	f := NewFakeStore()

	results := []scraper.Result{
		{
			ID:       "42",
			Title:    "Senior Engineer",
			Company:  "Acme",
			Location: "Remote",
			Date:     "2026-08-01",
			URL:      "https://example.com/job/42",
			Description: "Full-stack role",
			Metadata: map[string]string{"level": "senior", "team": "platform"},
		},
	}

	if err := f.AddStaging(results); err != nil {
		t.Fatalf("AddStaging failed: %v", err)
	}

	sr, ok, _ := f.GetStaged("https://example.com/job/42")
	if !ok {
		t.Fatal("GetStaged should find the entry")
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
