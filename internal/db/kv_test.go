package db

import (
	"path/filepath"
	"testing"

	"github.com/udit-001/waypoint/internal/scraper"
)

// TestMigration00014_freshDB creates the kv table from scratch on a fresh
// database (all migrations run in order).
func TestMigration00014_freshDB(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	if n, err := tableCount(s, "kv"); err != nil || n != 1 {
		t.Errorf("kv table missing (count=%d, err=%v)", n, err)
	}
}

// TestSetBoardSweepState_roundTrip: a written sweep state reads back under
// the board's name; overwriting replaces it rather than accumulating rows.
func TestSetBoardSweepState_roundTrip(t *testing.T) {
	s := sqliteStore(t)

	ok := BoardSweepState{At: "2026-08-22T10:00:00Z", OK: true}
	if err := s.SetBoardSweepState("slack", ok); err != nil {
		t.Fatalf("SetBoardSweepState: %v", err)
	}

	got, err := s.GetBoardSweepStates()
	if err != nil {
		t.Fatalf("GetBoardSweepStates: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d states, want 1", len(got))
	}
	state, okRead := got["slack"]
	if !okRead {
		t.Fatal("state for board \"slack\" missing")
	}
	if state.At != ok.At || !state.OK || state.Error != "" {
		t.Errorf("state = %+v, want %+v", state, ok)
	}

	// Overwrite: a failed sweep replaces the earlier success.
	failed := BoardSweepState{At: "2026-08-22T16:00:00Z", OK: false, Error: "rate-limited"}
	if err := s.SetBoardSweepState("slack", failed); err != nil {
		t.Fatalf("SetBoardSweepState overwrite: %v", err)
	}
	got, err = s.GetBoardSweepStates()
	if err != nil {
		t.Fatalf("GetBoardSweepStates after overwrite: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("overwrite accumulated rows: got %d states, want 1", len(got))
	}
	state = got["slack"]
	if state.OK || state.Error != failed.Error || state.At != failed.At {
		t.Errorf("state = %+v, want %+v", state, failed)
	}
}

// TestGetBoardSweepStates_empty: an untouched store yields an empty (non-nil)
// map, not an error — boards that never swept simply have no entry.
func TestGetBoardSweepStates_empty(t *testing.T) {
	s := sqliteStore(t)

	got, err := s.GetBoardSweepStates()
	if err != nil {
		t.Fatalf("GetBoardSweepStates: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d states, want 0", len(got))
	}
}

// TestNewPostingCounts: ledger postings awaiting review ("new") count per
// company, matched case-insensitively and keyed lowercase; reviewed postings
// leave the count.
func TestNewPostingCounts(t *testing.T) {
	s := sqliteStore(t)

	mustAddPostings(t, s,
		scraper.Result{URL: "https://x.com/1", Title: "Eng", Company: "Acme"},
		scraper.Result{URL: "https://x.com/2", Title: "Eng", Company: "ACME"},
		scraper.Result{URL: "https://x.com/3", Title: "Design", Company: "Beta"},
	)
	// One more Acme posting, already dismissed — must not count.
	mustAddPostings(t, s, scraper.Result{URL: "https://x.com/4", Title: "Ops", Company: "Acme"})
	if err := s.SetPostingStatus("https://x.com/4", StatusDismissed); err != nil {
		t.Fatalf("SetPostingStatus: %v", err)
	}

	counts, err := s.NewPostingCounts()
	if err != nil {
		t.Fatalf("NewPostingCounts: %v", err)
	}
	if counts["acme"] != 2 {
		t.Errorf("counts[\"acme\"] = %d, want 2", counts["acme"])
	}
	if counts["beta"] != 1 {
		t.Errorf("counts[\"beta\"] = %d, want 1", counts["beta"])
	}
	if len(counts) != 2 {
		t.Errorf("counts has %d companies, want 2 (dismissed row excluded)", len(counts))
	}
}

func mustAddPostings(t *testing.T, s *SQLiteStore, results ...scraper.Result) {
	t.Helper()
	if err := s.AddPostings(results); err != nil {
		t.Fatalf("AddPostings: %v", err)
	}
}

// TestSourceSelection_roundTrip: a selection written under its key reads
// back verbatim; a miss reports found=false rather than an error. The
// selection is the cached source-selection decision (WP-179) — ids in
// priority order, one reason per id.
func TestSourceSelection_roundTrip(t *testing.T) {
	s := sqliteStore(t)

	sel := SourceSelection{
		IDs:     []string{"indiabioscience", "iisc"},
		Reasons: map[string]string{"indiabioscience": "aggregator serves any industry", "iisc": "matches the academic preference"},
		Note:    "specialist portals cover the academic preference; aggregators ride along",
	}
	if err := s.SaveSourceSelection("k1", sel); err != nil {
		t.Fatalf("SaveSourceSelection: %v", err)
	}

	got, found, err := s.SourceSelection("k1")
	if err != nil || !found {
		t.Fatalf("SourceSelection: found=%v err=%v", found, err)
	}
	if len(got.IDs) != 2 || got.IDs[0] != "indiabioscience" || got.IDs[1] != "iisc" {
		t.Errorf("IDs = %v, want [indiabioscience iisc] in priority order", got.IDs)
	}
	if got.Reasons["iisc"] != sel.Reasons["iisc"] || got.Note != sel.Note {
		t.Errorf("round-trip mismatch: %+v", got)
	}

	if _, found, _ := s.SourceSelection("k2"); found {
		t.Error("uncached key reported found")
	}
}

// TestDiscoveryCompanies_roundTrip: the agentic-discovery cache (WP-180)
// stores the submitted company list under a brief hash and reads it back
// verbatim; a miss reports found=false so the caller runs the agent loop.
func TestDiscoveryCompanies_roundTrip(t *testing.T) {
	s := sqliteStore(t)

	want := []DiscoveredCompany{
		{Name: "MongoDB", Domain: "mongodb.com", Why: "developer data platform"},
		{Name: "Citi", Domain: "citi.com", Why: "banking cluster"},
	}
	if err := s.SaveDiscoveryCompanies("hash1", want); err != nil {
		t.Fatalf("SaveDiscoveryCompanies: %v", err)
	}

	got, found, err := s.DiscoveryCompanies("hash1")
	if err != nil || !found {
		t.Fatalf("DiscoveryCompanies: found=%v err=%v", found, err)
	}
	if len(got) != 2 || got[0].Name != "MongoDB" || got[1].Why != "banking cluster" {
		t.Errorf("round-trip mismatch: %+v", got)
	}

	if _, found, _ := s.DiscoveryCompanies("hash2"); found {
		t.Error("uncached hash reported found")
	}
}
