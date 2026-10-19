package discovery

import (
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/config"
)

// TestLandVerified_flagsDiscovered: a landed candidate's board carries
// the discovered flag and the company domain, so keep/block can act
// without consulting the ledger. One entry name = one board in
// boards.toml, so of a candidate's verified boards the first lands and
// the rest skip (the company name links to the first verified board).
func TestLandVerified_flagsDiscovered(t *testing.T) {
	bf := &config.BoardsFile{}
	cands := []Candidate{
		{Name: "Vercel", Domain: "vercel.com", Boards: []BoardLink{
			{Provider: "greenhouse", URL: "https://job-boards.greenhouse.io/vercel/"},
			{Provider: "lever", URL: "https://jobs.lever.co/vercel/"},
		}},
	}

	landed, skipped, err := LandVerified(cands, bf)
	if err != nil {
		t.Fatalf("LandVerified: %v", err)
	}
	if len(landed) != 1 || landed[0] != "Vercel" {
		t.Fatalf("landed = %v, want [Vercel] (first verified board wins)", landed)
	}
	if skipped != 1 {
		t.Errorf("skipped = %d, want 1 (the lever board)", skipped)
	}
	e := bf.Find(BoardEntryName("Vercel"))
	if e == nil {
		t.Fatalf("no board entry named %q", BoardEntryName("Vercel"))
	}
	if e.URL != "https://job-boards.greenhouse.io/vercel/" || e.Provider != "greenhouse" {
		t.Errorf("entry = %s %s, want the first verified board", e.URL, e.Provider)
	}
	if !e.Discovered {
		t.Errorf("entry %s not flagged discovered", e.URL)
	}
	if e.Domain != "vercel.com" {
		t.Errorf("entry %s domain = %q, want vercel.com", e.URL, e.Domain)
	}
	if e.Company != "Vercel" {
		t.Errorf("entry %s company = %q", e.URL, e.Company)
	}
}

// TestLandVerified_skipsWatched: a board URL already in boards.toml is
// not re-added and does not error.
func TestLandVerified_skipsWatched(t *testing.T) {
	bf := &config.BoardsFile{}
	bf.Upsert(config.BoardEntry{
		Name: "vercel", Company: "Vercel", Enabled: true,
		URL: "https://job-boards.greenhouse.io/vercel/", Provider: "greenhouse",
	})

	landed, skipped, err := LandVerified([]Candidate{{
		Name: "Vercel", Domain: "vercel.com",
		Boards: []BoardLink{{Provider: "greenhouse", URL: "https://job-boards.greenhouse.io/vercel/"}},
	}}, bf)
	if err != nil {
		t.Fatalf("LandVerified: %v", err)
	}
	if len(landed) != 0 || skipped != 0 {
		t.Errorf("landed = %v skipped = %d, want none (already watched)", landed, skipped)
	}
}

// TestLandVerified_nameConflict: a same-name board pointing elsewhere is
// the Promote contract's conflict — write nothing.
func TestLandVerified_nameConflict(t *testing.T) {
	bf := &config.BoardsFile{}
	bf.Upsert(config.BoardEntry{
		Name: "vercel", Company: "Vercel", Enabled: true, URL: "https://other.example/",
	})

	_, _, err := LandVerified([]Candidate{{
		Name: "Vercel", Domain: "vercel.com",
		Boards: []BoardLink{{Provider: "greenhouse", URL: "https://job-boards.greenhouse.io/vercel/"}},
	}}, bf)
	if err == nil {
		t.Fatal("want conflict error, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %v, want a conflict message", err)
	}
}

// TestLandVerified_empty: no candidates, no writes, no error.
func TestLandVerified_empty(t *testing.T) {
	bf := &config.BoardsFile{}
	landed, skipped, err := LandVerified(nil, bf)
	if err != nil || len(landed) != 0 || skipped != 0 {
		t.Errorf("LandVerified(nil) = %v, %d, %v; want none, 0, nil", landed, skipped, err)
	}
}
