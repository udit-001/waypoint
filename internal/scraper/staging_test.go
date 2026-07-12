package scraper

import (
	"path/filepath"
	"testing"
)

func TestStaging_empty(t *testing.T) {
	s, err := OpenStaging(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("OpenStaging on missing file: %v", err)
	}
	if s.IsSeen("https://example.com") {
		t.Error("IsSeen should be false on empty staging")
	}
	if results := s.List(""); len(results) != 0 {
		t.Errorf("List should return 0 on empty, got %d", len(results))
	}
}

func TestStaging_addAndIsSeen(t *testing.T) {
	s, _ := OpenStaging(filepath.Join(t.TempDir(), "staging.json"))

	results := []Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
		{ID: "2", Title: "Job B", URL: "https://example.com/2"},
	}

	if err := s.Add(results); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if !s.IsSeen("https://example.com/1") {
		t.Error("IsSeen should be true after Add")
	}
	if !s.IsSeen("https://example.com/2") {
		t.Error("IsSeen should be true after Add")
	}
	if s.IsSeen("https://example.com/3") {
		t.Error("IsSeen should be false for un-added URL")
	}
}

func TestStaging_addIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "staging.json")
	s1, _ := OpenStaging(path)

	results := []Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
	}
	s1.Add(results)

	// Reopen and add the same URL again
	s2, _ := OpenStaging(path)
	if err := s2.Add(results); err != nil {
		t.Fatalf("second Add failed: %v", err)
	}

	list := s2.List("")
	if len(list) != 1 {
		t.Errorf("expected 1 entry after re-adding, got %d", len(list))
	}
}

func TestStaging_listByStatus(t *testing.T) {
	s, _ := OpenStaging(filepath.Join(t.TempDir(), "staging.json"))

	s.Add([]Result{
		{ID: "1", Title: "New", URL: "https://example.com/1"},
		{ID: "2", Title: "Also New", URL: "https://example.com/2"},
	})

	s.Dismiss("https://example.com/1")

	newResults := s.List("new")
	if len(newResults) != 1 {
		t.Errorf("expected 1 new, got %d", len(newResults))
	}
	if newResults[0].Result.URL != "https://example.com/2" {
		t.Errorf("expected URL 2, got %s", newResults[0].Result.URL)
	}

	dismissed := s.List("dismissed")
	if len(dismissed) != 1 {
		t.Errorf("expected 1 dismissed, got %d", len(dismissed))
	}

	all := s.List("")
	if len(all) != 2 {
		t.Errorf("expected 2 total, got %d", len(all))
	}
}

func TestStaging_dismiss(t *testing.T) {
	s, _ := OpenStaging(filepath.Join(t.TempDir(), "staging.json"))

	s.Add([]Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
	})

	if err := s.Dismiss("https://example.com/1"); err != nil {
		t.Fatalf("Dismiss failed: %v", err)
	}

	entry := s.data["https://example.com/1"]
	if entry.Status != "dismissed" {
		t.Errorf("expected status dismissed, got %s", entry.Status)
	}

	// Dismiss again — should be idempotent
	if err := s.Dismiss("https://example.com/1"); err != nil {
		t.Fatalf("second Dismiss failed: %v", err)
	}

	// Dismiss unknown URL — should error
	if err := s.Dismiss("https://example.com/unknown"); err == nil {
		t.Error("expected error for unknown URL")
	}
}

func TestStaging_persistsAcrossOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "staging.json")

	s1, _ := OpenStaging(path)
	s1.Add([]Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
	})
	s1.Dismiss("https://example.com/1")

	// Reopen — data should persist
	s2, _ := OpenStaging(path)
	if !s2.IsSeen("https://example.com/1") {
		t.Error("IsSeen should be true after reopen")
	}
	list := s2.List("dismissed")
	if len(list) != 1 {
		t.Errorf("expected 1 dismissed after reopen, got %d", len(list))
	}
}

func TestStaging_prune(t *testing.T) {
	s, _ := OpenStaging(filepath.Join(t.TempDir(), "staging.json"))

	// Add a result and manually set its first_seen to an old date
	s.Add([]Result{
		{ID: "1", Title: "Old", URL: "https://example.com/old"},
		{ID: "2", Title: "Recent", URL: "https://example.com/recent"},
	})

	// Force old date on first entry
	entry := s.data["https://example.com/old"]
	entry.FirstSeen = "2020-01-01"
	s.data["https://example.com/old"] = entry

	removed, err := s.Prune(30)
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if s.IsSeen("https://example.com/old") {
		t.Error("old entry should be pruned")
	}
	if !s.IsSeen("https://example.com/recent") {
		t.Error("recent entry should remain")
	}
}
