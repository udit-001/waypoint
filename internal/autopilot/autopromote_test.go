package autopilot

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
	"github.com/udit-001/waypoint/internal/scraper"
)

// stubDiscover replaces the Discover seam for one test, capturing the
// facets the stage handed it and returning fixed candidates.
func stubDiscover(t *testing.T, cands []discovery.Candidate) *[]discovery.Facet {
	t.Helper()
	seen := &[]discovery.Facet{}
	orig := discoverCandidates
	discoverCandidates = func(_ context.Context, facets []discovery.Facet, _ map[string]bool, _ discovery.Options) ([]discovery.Candidate, error) {
		*seen = facets
		return cands, nil
	}
	t.Cleanup(func() { discoverCandidates = orig })
	return seen
}

// tempBoards points the config dir at a temp dir whose data_dir is a
// second temp dir, so boards.toml reads/writes land in the sandbox.
func tempBoards(t *testing.T) string {
	t.Helper()
	cfgDir := t.TempDir()
	dataDir := t.TempDir()
	config.SetConfigDirForTesting(cfgDir)
	t.Cleanup(func() { config.SetConfigDirForTesting("") })
	if err := config.Save(&config.Config{DataDir: dataDir}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	return dataDir
}

// TestDiscoveryStage_AutoPromotes: verified candidates land in
// boards.toml immediately, flagged discovered with their domain, and
// their ledger rows flip to added — the human promote step is gone
// (WP-181). The runlog carries the count.
func TestDiscoveryStage_AutoPromotes(t *testing.T) {
	f := db.NewFakeStore()
	dataDir := tempBoards(t)
	stubDiscover(t, []discovery.Candidate{{
		Name: "Vercel", Domain: "vercel.com",
		Boards: []discovery.BoardLink{{Provider: "greenhouse", URL: "https://job-boards.greenhouse.io/vercel/"}},
	}})

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{seedSweeperScraper()}))

	bf, err := config.LoadBoards(&config.Config{DataDir: dataDir})
	if err != nil {
		t.Fatalf("LoadBoards: %v", err)
	}
	e := bf.Find("vercel")
	if e == nil {
		t.Fatal("discovered board never landed in boards.toml")
	}
	if !e.Discovered || e.Domain != "vercel.com" {
		t.Errorf("entry = %+v, want discovered flag + domain", e)
	}
	if !e.Enabled {
		t.Error("discovered board must be functional immediately (enabled)")
	}

	cands, _ := f.Candidates("")
	var landed bool
	for _, c := range cands {
		if c.Name == "Vercel" {
			landed = true
			if c.Status != db.StatusCandidateAdded {
				t.Errorf("candidate status = %q, want added", c.Status)
			}
		}
	}
	if !landed {
		t.Error("discovered candidate missing from the ledger")
	}

	if entry.DiscoveredBoards != 1 {
		t.Errorf("runlog DiscoveredBoards = %d, want 1", entry.DiscoveredBoards)
	}
}

// TestDiscoveryStage_BlockedDomainsSkipped: a blocked domain never
// reaches the prober — re-discovery skips it entirely (WP-181).
func TestDiscoveryStage_BlockedDomainsSkipped(t *testing.T) {
	f := db.NewFakeStore()
	tempBoards(t)
	if err := f.BlockDomain("moodys.com"); err != nil {
		t.Fatalf("BlockDomain: %v", err)
	}
	seen := stubDiscover(t, nil) // nothing survives: the blocked company is gone

	Run(context.Background(), newCycle(f, []scraper.Scraper{seedSweeperScraper()}))

	for _, fac := range *seen {
		for _, c := range fac.Companies {
			if c.Domain == "moodys.com" {
				t.Errorf("blocked domain %s reached the prober (facet %s)", c.Domain, fac.Name)
			}
		}
	}
}

// TestDiscoveryStage_LandingFailureIsStageError: when boards.toml can't
// be written, the discovery stage errors — a degraded run, not a silent
// loss of discovered companies.
func TestDiscoveryStage_LandingFailureIsStageError(t *testing.T) {
	f := db.NewFakeStore()
	tempBoards(t)
	// Point the data dir at a file so SaveBoards cannot create the dir.
	cfgDir := t.TempDir()
	blockedPath := filepath.Join(cfgDir, "not-a-dir")
	if err := os.WriteFile(blockedPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Re-point data_dir at the file.
	if err := config.Save(&config.Config{DataDir: blockedPath}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	stubDiscover(t, []discovery.Candidate{{
		Name: "Vercel", Domain: "vercel.com",
		Boards: []discovery.BoardLink{{Provider: "greenhouse", URL: "https://job-boards.greenhouse.io/vercel/"}},
	}})

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{seedSweeperScraper()}))

	if entry.Verdict != db.VerdictDegraded {
		t.Errorf("verdict = %q, want degraded", entry.Verdict)
	}
	found := false
	for _, e := range entry.StageErrors {
		if strings.Contains(e, "discovery") {
			found = true
		}
	}
	if !found {
		t.Errorf("stage errors %v name no discovery failure", entry.StageErrors)
	}
}

// TestDiscoveryStage_ManuallyDismissedNeverRelands: a company the user
// dismissed from the review queue before auto-promote existed must not
// silently re-enter boards.toml on re-discovery.
func TestDiscoveryStage_ManuallyDismissedNeverRelands(t *testing.T) {
	f := db.NewFakeStore()
	dataDir := tempBoards(t)
	// Pre-seed the ledger: the company was reviewed and dismissed.
	if err := f.SaveCandidates([]db.CompanyCandidate{
		{Name: "Vercel", Domain: "vercel.com", Status: db.StatusCandidateDismissed,
			Boards: []db.CandidateBoard{{Provider: "greenhouse", URL: "https://job-boards.greenhouse.io/vercel/"}}},
	}); err != nil {
		t.Fatal(err)
	}
	stubDiscover(t, []discovery.Candidate{{
		Name: "Vercel", Domain: "vercel.com",
		Boards: []discovery.BoardLink{{Provider: "greenhouse", URL: "https://job-boards.greenhouse.io/vercel/"}},
	}})

	Run(context.Background(), newCycle(f, []scraper.Scraper{seedSweeperScraper()}))

	bf, err := config.LoadBoards(&config.Config{DataDir: dataDir})
	if err != nil {
		t.Fatalf("LoadBoards: %v", err)
	}
	if bf.Find("vercel") != nil {
		t.Error("dismissed company was auto-promoted — dismissals must hold")
	}
}
