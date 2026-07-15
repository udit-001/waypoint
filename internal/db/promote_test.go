package db

import (
	"testing"

	"github.com/udit-001/waypoint/internal/scraper"
)

// --- MigrateStaging tests ---

func TestMigrateStaging_basic(t *testing.T) {
	f := NewFakeStore()

	entries := []scraper.StagedResult{
		{
			FirstSeen: "2026-06-01",
			Status:    "new",
			Result: scraper.Result{
				ID:      "1",
				Title:   "Job A",
				Company: "Corp A",
				URL:     "https://example.com/1",
			},
		},
		{
			FirstSeen: "2026-06-02",
			Status:    "dismissed",
			Result: scraper.Result{
				ID:      "2",
				Title:   "Job B",
				Company: "Corp B",
				URL:     "https://example.com/2",
			},
		},
	}

	imported, err := f.MigrateStaging(entries)
	if err != nil {
		t.Fatalf("MigrateStaging failed: %v", err)
	}
	if imported != 2 {
		t.Errorf("expected 2 imported, got %d", imported)
	}

	// Verify entry 1 — status "new" preserved
	sr, ok, _ := f.GetStaged("https://example.com/1")
	if !ok {
		t.Fatal("entry 1 not found")
	}
	if sr.Status != "new" {
		t.Errorf("expected status 'new', got %q", sr.Status)
	}
	if sr.FirstSeen != "2026-06-01" {
		t.Errorf("expected first_seen '2026-06-01', got %q", sr.FirstSeen)
	}
	if sr.Result.Title != "Job A" {
		t.Errorf("expected title 'Job A', got %q", sr.Result.Title)
	}

	// Verify entry 2 — status "dismissed" preserved
	sr, ok, _ = f.GetStaged("https://example.com/2")
	if !ok {
		t.Fatal("entry 2 not found")
	}
	if sr.Status != "dismissed" {
		t.Errorf("expected status 'dismissed', got %q", sr.Status)
	}
}

func TestMigrateStaging_idempotent(t *testing.T) {
	f := NewFakeStore()

	entries := []scraper.StagedResult{
		{
			FirstSeen: "2026-06-01",
			Status:    "new",
			Result:    scraper.Result{ID: "1", Title: "Job A", URL: "https://example.com/1"},
		},
	}

	imported, _ := f.MigrateStaging(entries)
	if imported != 1 {
		t.Fatalf("first migration: expected 1 imported, got %d", imported)
	}

	// Re-migrate — should skip existing URLs
	imported, err := f.MigrateStaging(entries)
	if err != nil {
		t.Fatalf("second MigrateStaging failed: %v", err)
	}
	if imported != 0 {
		t.Errorf("second migration: expected 0 imported, got %d", imported)
	}
}

func TestMigrateStaging_empty(t *testing.T) {
	f := NewFakeStore()

	imported, err := f.MigrateStaging(nil)
	if err != nil {
		t.Fatalf("MigrateStaging empty failed: %v", err)
	}
	if imported != 0 {
		t.Errorf("expected 0 imported, got %d", imported)
	}
}

func TestMigrateStaging_preservesMetadata(t *testing.T) {
	f := NewFakeStore()

	entries := []scraper.StagedResult{
		{
			FirstSeen: "2026-06-01",
			Status:    "new",
			Result: scraper.Result{
				ID:          "1",
				Title:       "Job A",
				URL:         "https://example.com/1",
				Description: "A great role",
				Metadata:    map[string]string{"level": "senior", "team": "platform"},
			},
		},
	}

	if _, err := f.MigrateStaging(entries); err != nil {
		t.Fatalf("MigrateStaging failed: %v", err)
	}

	sr, _, _ := f.GetStaged("https://example.com/1")
	if sr.Result.Description != "A great role" {
		t.Errorf("expected description, got %q", sr.Result.Description)
	}
	if sr.Result.Metadata["level"] != "senior" {
		t.Errorf("expected metadata level=senior, got %v", sr.Result.Metadata)
	}
	if sr.Result.Metadata["team"] != "platform" {
		t.Errorf("expected metadata team=platform, got %v", sr.Result.Metadata)
	}
}

func TestMigrateStaging_defaultsEmptyFields(t *testing.T) {
	f := NewFakeStore()

	entries := []scraper.StagedResult{
		{
			// FirstSeen and Status left empty
			Result: scraper.Result{ID: "1", Title: "Job A", URL: "https://example.com/1"},
		},
	}

	if _, err := f.MigrateStaging(entries); err != nil {
		t.Fatalf("MigrateStaging failed: %v", err)
	}

	sr, _, _ := f.GetStaged("https://example.com/1")
	if sr.Status != "new" {
		t.Errorf("expected default status 'new', got %q", sr.Status)
	}
	if sr.FirstSeen == "" {
		t.Error("expected non-empty first_seen")
	}
}

// --- Promote tests ---

func TestPromote_single(t *testing.T) {
	f := NewFakeStore()

	f.AddStaging([]scraper.Result{
		{
			ID:       "1",
			Title:    "Senior Engineer",
			Company:  "Acme",
			Location: "Remote",
			Date:     "2026-08-01",
			URL:      "https://example.com/job/1",
		},
	})

	job, err := f.Promote("https://example.com/job/1")
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	if job.ID == 0 {
		t.Fatal("expected job ID > 0")
	}
	if job.Position != "Senior Engineer" {
		t.Errorf("expected position 'Senior Engineer', got %q", job.Position)
	}
	if job.Company != "Acme" {
		t.Errorf("expected company 'Acme', got %q", job.Company)
	}
	if job.URL != "https://example.com/job/1" {
		t.Errorf("expected URL, got %q", job.URL)
	}
	if job.Location != "Remote" {
		t.Errorf("expected location 'Remote', got %q", job.Location)
	}
	if job.Status != "Not Applied" {
		t.Errorf("expected status 'Not Applied', got %q", job.Status)
	}

	// Staging entry should be marked "imported"
	sr, _, _ := f.GetStaged("https://example.com/job/1")
	if sr.Status != "imported" {
		t.Errorf("expected staging status 'imported', got %q", sr.Status)
	}

	// History should be recorded
	history, _ := f.GetJobHistory(job.ID)
	if len(history) == 0 {
		t.Fatal("expected history entries")
	}
	if history[0].Action != "Created" {
		t.Errorf("expected first history action 'Created', got %q", history[0].Action)
	}
}

func TestPromote_dateNormalization(t *testing.T) {
	f := NewFakeStore()

	f.AddStaging([]scraper.Result{
		{
			ID:    "1",
			Title: "Job A",
			URL:   "https://example.com/1",
			Date:  "15 July, 2026",
		},
	})

	job, err := f.Promote("https://example.com/1")
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}
	if job.Date != "2026-07-15" {
		t.Errorf("expected normalized date '2026-07-15', got %q", job.Date)
	}
}

func TestPromote_rollingDeadline(t *testing.T) {
	f := NewFakeStore()

	f.AddStaging([]scraper.Result{
		{
			ID:    "1",
			Title: "Job A",
			URL:   "https://example.com/1",
			Date:  "Open until filled",
		},
	})

	job, err := f.Promote("https://example.com/1")
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}
	if job.Date != "" {
		t.Errorf("expected empty date for rolling deadline, got %q", job.Date)
	}
}

func TestPromote_idempotent(t *testing.T) {
	f := NewFakeStore()

	f.AddStaging([]scraper.Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
	})

	// First promote — creates the job
	job1, err := f.Promote("https://example.com/1")
	if err != nil {
		t.Fatalf("first Promote failed: %v", err)
	}
	if job1.ID == 0 {
		t.Fatal("first promote should create a job")
	}

	// Second promote — URL already in jobs, should skip creation
	job2, err := f.Promote("https://example.com/1")
	if err != nil {
		t.Fatalf("second Promote failed: %v", err)
	}
	if job2.ID != 0 {
		t.Errorf("second promote should skip (ID=0), got %d", job2.ID)
	}

	// Only one job in the table
	count, _ := f.JobCount()
	if count != 1 {
		t.Errorf("expected 1 job, got %d", count)
	}

	// Staging still marked imported
	sr, _, _ := f.GetStaged("https://example.com/1")
	if sr.Status != "imported" {
		t.Errorf("expected staging status 'imported', got %q", sr.Status)
	}
}

func TestPromote_unknownURL(t *testing.T) {
	f := NewFakeStore()

	_, err := f.Promote("https://example.com/unknown")
	if err == nil {
		t.Fatal("expected error for unknown URL")
	}
}

func TestPromote_preservesResultFields(t *testing.T) {
	f := NewFakeStore()

	f.AddStaging([]scraper.Result{
		{
			ID:          "42",
			Title:       "Senior Engineer",
			Company:     "Acme",
			Location:    "Remote",
			Date:        "2026-08-01",
			URL:         "https://example.com/job/42",
			Description: "Full-stack role",
			Metadata:    map[string]string{"level": "senior"},
		},
	})

	job, err := f.Promote("https://example.com/job/42")
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}

	// Job fields mapped from Result
	if job.Position != "Senior Engineer" {
		t.Errorf("Position = %q, want 'Senior Engineer'", job.Position)
	}
	if job.Company != "Acme" {
		t.Errorf("Company = %q, want 'Acme'", job.Company)
	}
	if job.Location != "Remote" {
		t.Errorf("Location = %q, want 'Remote'", job.Location)
	}
	if job.URL != "https://example.com/job/42" {
		t.Errorf("URL = %q, want 'https://example.com/job/42'", job.URL)
	}
	if job.Date != "2026-08-01" {
		t.Errorf("Date = %q, want '2026-08-01'", job.Date)
	}
}

func TestPromote_allBatch(t *testing.T) {
	f := NewFakeStore()

	// Three "new" results
	f.AddStaging([]scraper.Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
		{ID: "2", Title: "Job B", URL: "https://example.com/2"},
		{ID: "3", Title: "Job C", URL: "https://example.com/3"},
	})

	// One dismissed — should be skipped by --all
	f.AddStaging([]scraper.Result{
		{ID: "4", Title: "Job D", URL: "https://example.com/4"},
	})
	f.SetStagingStatus("https://example.com/4", "dismissed")

	// One already imported — should be skipped by --all
	f.AddStaging([]scraper.Result{
		{ID: "5", Title: "Job E", URL: "https://example.com/5"},
	})
	f.SetStagingStatus("https://example.com/5", "imported")

	// Promote all "new" results
	newResults, _ := f.ListStaging("new")
	promoted, skipped := 0, 0
	for _, r := range newResults {
		job, err := f.Promote(r.Result.URL)
		if err != nil {
			t.Fatalf("Promote %s failed: %v", r.Result.URL, err)
		}
		if job.ID > 0 {
			promoted++
		} else {
			skipped++
		}
	}

	if promoted != 3 {
		t.Errorf("expected 3 promoted, got %d", promoted)
	}
	if skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", skipped)
	}

	// All "new" entries should now be "imported"
	newAfter, _ := f.ListStaging("new")
	if len(newAfter) != 0 {
		t.Errorf("expected 0 new after --all, got %d", len(newAfter))
	}

	// Dismissed and imported entries should be unchanged
	sr, _, _ := f.GetStaged("https://example.com/4")
	if sr.Status != "dismissed" {
		t.Errorf("dismissed entry should stay dismissed, got %q", sr.Status)
	}
	sr, _, _ = f.GetStaged("https://example.com/5")
	if sr.Status != "imported" {
		t.Errorf("imported entry should stay imported, got %q", sr.Status)
	}

	// 3 jobs in the table
	count, _ := f.JobCount()
	if count != 3 {
		t.Errorf("expected 3 jobs, got %d", count)
	}
}

func TestPromote_allWithExistingJob(t *testing.T) {
	f := NewFakeStore()

	// Stage a result
	f.AddStaging([]scraper.Result{
		{ID: "1", Title: "Job A", URL: "https://example.com/1"},
	})

	// Manually add the same URL to the jobs table
	f.InsertJob(Job{
		Company: "Manual", Position: "Manual", URL: "https://example.com/1",
		Status: "Not Applied", CreatedAt: "2026-01-01", UpdatedAt: "2026-01-01",
	})

	// Promote — should skip (URL already in jobs) but mark staging as imported
	job, err := f.Promote("https://example.com/1")
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}
	if job.ID != 0 {
		t.Errorf("expected ID=0 (skipped), got %d", job.ID)
	}

	// Staging should be imported
	sr, _, _ := f.GetStaged("https://example.com/1")
	if sr.Status != "imported" {
		t.Errorf("expected staging status 'imported', got %q", sr.Status)
	}

	// Still only 1 job
	count, _ := f.JobCount()
	if count != 1 {
		t.Errorf("expected 1 job, got %d", count)
	}
}
