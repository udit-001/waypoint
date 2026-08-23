package db

import (
	"encoding/json"
	"testing"

	"github.com/udit-001/waypoint/internal/scraper"
)

// TestAutopilotRunRequest_roundTrip: a requested run is consumable
// exactly once — the first Consume sees it, the second doesn't.
// This read-and-clear contract is what makes the daemon's scheduler
// poll idempotent: two polls can't double-fire one request.
func TestAutopilotRunRequest_roundTrip(t *testing.T) {
	s := sqliteStore(t)

	got, err := s.ConsumeAutopilotRunRequest()
	if err != nil {
		t.Fatalf("Consume (absent): %v", err)
	}
	if got {
		t.Fatal("fresh store reported a run request; want none")
	}

	if err := s.RequestAutopilotRun(); err != nil {
		t.Fatalf("RequestAutopilotRun: %v", err)
	}

	got, err = s.ConsumeAutopilotRunRequest()
	if err != nil {
		t.Fatalf("Consume (present): %v", err)
	}
	if !got {
		t.Fatal("requested run not seen by first consume")
	}

	got, err = s.ConsumeAutopilotRunRequest()
	if err != nil {
		t.Fatalf("Consume (after clear): %v", err)
	}
	if got {
		t.Fatal("run request survived consumption — scheduler would re-fire")
	}
}

// TestUpsertSettingsAutopilot_freshDB: a virgin database accepts an
// autopilot enable before any settings read has run — the lazy column
// ensure must live on the write path too (regression: fresh installs
// got "no such column: autopilot_enabled" from the Settings toggle).
func TestUpsertSettingsAutopilot_freshDB(t *testing.T) {
	s := sqliteStore(t)

	if err := s.UpsertSettings(map[string]any{"autopilot_enabled": 1}); err != nil {
		t.Fatalf("UpsertSettings on fresh db: %v", err)
	}
	st, err := s.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if st.AutopilotEnabled != 1 {
		t.Errorf("AutopilotEnabled = %d, want 1", st.AutopilotEnabled)
	}
}

// TestZenModel_roundTrip: the chosen curation model persists through
// GetSettings/UpsertSettings and defaults to empty (= shipped default).
func TestZenModel_roundTrip(t *testing.T) {
	s := sqliteStore(t)

	st, err := s.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if st.ZenModel != "" {
		t.Errorf("fresh ZenModel = %q, want empty", st.ZenModel)
	}

	if err := s.UpsertSettings(map[string]any{"zen_model": "mimo-v2.5-free"}); err != nil {
		t.Fatalf("UpsertSettings: %v", err)
	}
	st, err = s.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings after upsert: %v", err)
	}
	if st.ZenModel != "mimo-v2.5-free" {
		t.Errorf("ZenModel = %q, want mimo-v2.5-free", st.ZenModel)
	}
}

// TestPromote_snapshotsReview: promoting carries the curation metadata
// onto the job as review_json — the detail panel must survive ledger
// pruning.
func TestPromote_snapshotsReview(t *testing.T) {
	s := sqliteStore(t)
	if err := s.AddPostings([]scraper.Result{{
		URL: "https://example.com/a", Title: "Engineer", Company: "Acme",
		Metadata: map[string]string{
			"overview": "Data platform team; Go stack.",
			"note":     "strong brief fit",
			"score":    "84",
			"reasons":  `[{"kind":"match","field":"role","text":"backend"}]`,
		},
	}}); err != nil {
		t.Fatalf("seed posting: %v", err)
	}
	_ = s.SetPostingStatus("https://example.com/a", StatusShortlisted)

	job, err := s.Promote("https://example.com/a")
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}
	var rev struct {
		Overview string `json:"overview"`
		Score    string `json:"score"`
	}
	if err := json.Unmarshal([]byte(job.ReviewJSON), &rev); err != nil {
		t.Fatalf("review_json invalid: %v", err)
	}
	if rev.Overview != "Data platform team; Go stack." || rev.Score != "84" {
		t.Errorf("snapshot = %+v", rev)
	}
}
