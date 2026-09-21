package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
)

func seedPostings(f *db.FakeStore) {
	f.AddPostings([]scraper.Result{
		{URL: "https://example.com/a", Title: "Engineer", Company: "Acme", Location: "Remote", Date: "2026-08-01"},
		{URL: "https://example.com/b", Title: "Designer", Company: "Beta", Location: "NYC", Date: "2026-08-05"},
	})
}

func TestPostingsListEmpty(t *testing.T) {
	store = db.NewFakeStore()
	jsonOut = false

	out := captureStdout(t, func() {
		if err := postingsListCmd.RunE(postingsListCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	if !strings.Contains(out, "No postings") {
		t.Errorf("expected 'No postings' in output, got: %s", out)
	}
}

func TestPostingsListJSON(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	store = f
	jsonOut = true

	out := captureStdout(t, func() {
		if err := postingsListCmd.RunE(postingsListCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	var results []db.Posting
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 postings, got %d", len(results))
	}
}

func TestPostingsListFilterStatus(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	// Dismiss one posting.
	_ = f.SetPostingStatus("https://example.com/a", db.StatusDismissed)
	store = f
	jsonOut = true

	postingsListFlags.status = "dismissed"
	defer func() { postingsListFlags.status = "" }()

	out := captureStdout(t, func() {
		if err := postingsListCmd.RunE(postingsListCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	var results []db.Posting
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 dismissed posting, got %d", len(results))
	}
	if results[0].Result.URL != "https://example.com/a" {
		t.Errorf("expected URL a, got %s", results[0].Result.URL)
	}
}

func TestPostingsGetFound(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	store = f
	jsonOut = false

	postingsGetCmd.SetArgs([]string{"https://example.com/a"})
	defer postingsGetCmd.SetArgs(nil)

	out := captureStdout(t, func() {
		if err := postingsGetCmd.RunE(postingsGetCmd, []string{"https://example.com/a"}); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	if !strings.Contains(out, "Engineer") {
		t.Errorf("expected 'Engineer' in output, got: %s", out)
	}
	if !strings.Contains(out, "Acme") {
		t.Errorf("expected 'Acme' in output, got: %s", out)
	}
}

func TestPostingsGetNotFound(t *testing.T) {
	store = db.NewFakeStore()

	err := postingsGetCmd.RunE(postingsGetCmd, []string{"https://example.com/missing"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no posting") {
		t.Errorf("expected 'no posting' in error, got: %v", err)
	}
}

func TestPostingsGetJSON(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	store = f
	jsonOut = true

	out := captureStdout(t, func() {
		if err := postingsGetCmd.RunE(postingsGetCmd, []string{"https://example.com/a"}); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	var p db.Posting
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if p.Result.Title != "Engineer" {
		t.Errorf("title = %q, want Engineer", p.Result.Title)
	}
}

func TestPostingsPromoteSingle(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	store = f
	jsonOut = false

	out := captureStdout(t, func() {
		if err := postingsPromoteCmd.RunE(postingsPromoteCmd, []string{"https://example.com/a"}); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	if !strings.Contains(out, "Promoted") {
		t.Errorf("expected 'Promoted' in output, got: %s", out)
	}

	// Posting should now be "promoted".
	p, _, _ := f.GetPosting("https://example.com/a")
	if p.Status != db.StatusPromoted {
		t.Errorf("status = %q, want promoted", p.Status)
	}

	// A job should exist.
	jobs, _ := f.GetJobs()
	found := false
	for _, j := range jobs {
		if j.URL == "https://example.com/a" {
			found = true
			if j.Position != "Engineer" {
				t.Errorf("job position = %q, want Engineer", j.Position)
			}
		}
	}
	if !found {
		t.Error("expected a job to be created from promoted posting")
	}
}

func TestPostingsPromoteAll(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	store = f

	postingsPromoteFlags.all = true
	defer func() { postingsPromoteFlags.all = false }()

	out := captureStdout(t, func() {
		if err := postingsPromoteCmd.RunE(postingsPromoteCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	if !strings.Contains(out, "Promoted 2") {
		t.Errorf("expected 'Promoted 2' in output, got: %s", out)
	}
}

func TestPostingsPromoteNotFound(t *testing.T) {
	store = db.NewFakeStore()

	err := postingsPromoteCmd.RunE(postingsPromoteCmd, []string{"https://example.com/missing"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPostingsPromoteIdempotent(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	store = f

	// Promote once.
	job1, err := f.Promote("https://example.com/a")
	if err != nil || job1.ID == 0 {
		t.Fatalf("first promote failed: %v", err)
	}

	// Promote again — should skip job creation but still mark promoted.
	out := captureStdout(t, func() {
		if err := postingsPromoteCmd.RunE(postingsPromoteCmd, []string{"https://example.com/a"}); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	if !strings.Contains(out, "Skipped") {
		t.Errorf("expected 'Skipped' on second promote, got: %s", out)
	}
}

func TestPostingsDismissSingle(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	store = f
	jsonOut = false

	out := captureStdout(t, func() {
		if err := postingsDismissCmd.RunE(postingsDismissCmd, []string{"https://example.com/a"}); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	if !strings.Contains(out, "Dismissed") {
		t.Errorf("expected 'Dismissed' in output, got: %s", out)
	}

	p, _, _ := f.GetPosting("https://example.com/a")
	if p.Status != db.StatusDismissed {
		t.Errorf("status = %q, want dismissed", p.Status)
	}
}

func TestPostingsDismissAll(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	store = f

	postingsDismissFlags.all = true
	defer func() { postingsDismissFlags.all = false }()

	out := captureStdout(t, func() {
		if err := postingsDismissCmd.RunE(postingsDismissCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	if !strings.Contains(out, "Dismissed 2") {
		t.Errorf("expected 'Dismissed 2' in output, got: %s", out)
	}
}

func TestPostingsDismissNotFound(t *testing.T) {
	store = db.NewFakeStore()
	jsonOut = false

	// Single missing URL: prints warning to stderr, returns nil.
	out := captureStdout(t, func() {
		if err := postingsDismissCmd.RunE(postingsDismissCmd, []string{"https://example.com/missing"}); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	if !strings.Contains(out, "Dismissed 0") {
		t.Errorf("expected 'Dismissed 0' in output, got: %s", out)
	}
}

func TestPostingsPrune(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	// Manually set an old first_seen.
	p := f.Postings["https://example.com/a"]
	p.FirstSeen = "2020-01-01"
	f.Postings["https://example.com/a"] = p
	store = f
	jsonOut = false

	postingsPruneFlags.days = 30
	defer func() { postingsPruneFlags.days = 30 }()

	out := captureStdout(t, func() {
		if err := postingsPruneCmd.RunE(postingsPruneCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	if !strings.Contains(out, "Removed 1") {
		t.Errorf("expected 'Removed 1' in output, got: %s", out)
	}

	// Only the recent posting should remain.
	_, ok, _ := f.GetPosting("https://example.com/a")
	if ok {
		t.Error("expected posting a to be pruned")
	}
	_, ok, _ = f.GetPosting("https://example.com/b")
	if !ok {
		t.Error("expected posting b to remain")
	}
}

func TestPostingsPruneJSON(t *testing.T) {
	f := db.NewFakeStore()
	seedPostings(f)
	store = f
	jsonOut = true

	postingsPruneFlags.days = 30
	defer func() { postingsPruneFlags.days = 30 }()

	out := captureStdout(t, func() {
		if err := postingsPruneCmd.RunE(postingsPruneCmd, nil); err != nil {
			t.Fatalf("RunE error: %v", err)
		}
	})

	var result map[string]int
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, out)
	}
	if _, ok := result["removed"]; !ok {
		t.Error("expected 'removed' key in JSON output")
	}
}

func TestPostingsCommandsRegistered(t *testing.T) {
	cmds := make(map[string]bool)
	for _, cmd := range postingsCmd.Commands() {
		cmds[cmd.Name()] = true
	}
	for _, want := range []string{"list", "get", "promote", "dismiss", "prune"} {
		if !cmds[want] {
			t.Errorf("postings subcommand %q not registered", want)
		}
	}
}

func TestScrapeStagedDeprecated(t *testing.T) {
	// Verify scrape staged is marked deprecated.
	if scrapeStagedCmd.Deprecated == "" {
		t.Error("expected scrapeStagedCmd to be deprecated")
	}
}

func TestScrapePromoteDeprecated(t *testing.T) {
	if scrapePromoteCmd.Deprecated == "" {
		t.Error("expected scrapePromoteCmd to be deprecated")
	}
}

func TestScrapeDismissDeprecated(t *testing.T) {
	if scrapeDismissCmd.Deprecated == "" {
		t.Error("expected scrapeDismissCmd to be deprecated")
	}
}
