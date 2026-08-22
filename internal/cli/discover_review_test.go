package cli

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
)

// setupDiscoverReviewTest points config at temp dirs, migrates the real
// database (PersistentPreRunE opens it for every command), and seeds
// one suggested candidate with a greenhouse board. Returns the seed.
func setupDiscoverReviewTest(t *testing.T) db.CompanyCandidate {
	t.Helper()
	setupBoardsTest(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	s, err := db.Open(config.DBPath(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RunMigrations(""); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	cand := db.CompanyCandidate{
		Name:   "Acme Corp",
		Domain: "acme.com",
		Facet:  "test-facet",
		Boards: []db.CandidateBoard{{Provider: "greenhouse", URL: "https://boards.greenhouse.io/acme"}},
	}
	if err := s.SaveCandidates([]db.CompanyCandidate{cand}); err != nil {
		t.Fatalf("seed candidate: %v", err)
	}
	cands, err := s.Candidates("")
	if err != nil || len(cands) == 0 || cands[0].ID == 0 {
		t.Fatalf("seed id (err=%v, cands=%+v)", err, cands)
	}
	cand.ID = cands[0].ID
	return cand
}

// stubVerifyBoard replaces the shared network seam for add; restore via
// cleanup.
func stubVerifyBoard(t *testing.T, jobs int, err error) {
	t.Helper()
	orig := discovery.ProbeBoard
	discovery.ProbeBoard = func(_ context.Context, _ discovery.BoardLink) (int, error) {
		return jobs, err
	}
	t.Cleanup(func() { discovery.ProbeBoard = orig })
}

// TestDiscoverAddPromotesToBoards: add verifies the board, writes it into
// boards.toml enabled, flips the candidate to added — the next sweep's
// raw material appears with no further steps.
func TestDiscoverAddPromotesToBoards(t *testing.T) {
	cand := setupDiscoverReviewTest(t)
	stubVerifyBoard(t, 4, nil)

	out, err := runCmd(t, "discover", "add", strconv.FormatInt(cand.ID, 10), "--json")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, want := range []string{`"status": "added"`, `"updated": true`, "greenhouse"} {
		if !strings.Contains(out, want) {
			t.Errorf("json missing %q in:\n%s", want, out)
		}
	}

	bf, cfg, err := loadBoardsStore()
	if err != nil {
		t.Fatal(err)
	}
	e := bf.Find("acme-corp")
	if e == nil {
		t.Fatalf("board acme-corp not written; boards = %+v", bf.Boards)
	}
	if e.URL != "https://boards.greenhouse.io/acme" || e.Provider != "greenhouse" || !e.Enabled {
		t.Errorf("board entry = %+v, want acme greenhouse URL enabled", e)
	}

	reloaded, _ := db.Open(config.DBPath(cfg))
	defer reloaded.Close()
	got, err := reloaded.Candidates(db.StatusCandidateAdded)
	if err != nil || len(got) != 1 || got[0].Name != "Acme Corp" {
		t.Errorf("candidate status after add = %+v (err=%v), want Acme Corp added", got, err)
	}
}

// TestDiscoverAddAlreadyWatched: a candidate whose board URL is already
// in boards.toml is a clear no-op — status stays suggested, nothing saved.
func TestDiscoverAddAlreadyWatched(t *testing.T) {
	cand := setupDiscoverReviewTest(t)

	// The user already boarded this exact URL by hand.
	bf, cfg, err := loadBoardsStore()
	if err != nil {
		t.Fatal(err)
	}
	bf.Upsert(config.BoardEntry{Name: "acme-manual", Company: "Acme", URL: cand.Boards[0].URL, Provider: "greenhouse", Enabled: true})
	if err := config.SaveBoards(cfg, bf); err != nil {
		t.Fatal(err)
	}

	_, err = runCmd(t, "discover", "add", strconv.FormatInt(cand.ID, 10))
	if err == nil || !strings.Contains(err.Error(), "already in boards.toml") {
		t.Fatalf("err = %v, want already-watched no-op message", err)
	}

	reloaded, _ := db.Open(config.DBPath(cfg))
	defer reloaded.Close()
	got, _ := reloaded.Candidates("")
	if got[0].Status != db.StatusCandidateSuggested {
		t.Errorf("status = %q, want suggested (no-op must not decide)", got[0].Status)
	}
}

// TestDiscoverDismissTombstones: dismiss flips status to dismissed and
// nothing else changes — discovery never re-suggests it.
func TestDiscoverDismissTombstones(t *testing.T) {
	cand := setupDiscoverReviewTest(t)

	out, err := runCmd(t, "discover", "dismiss", strconv.FormatInt(cand.ID, 10), "--json")
	if err != nil {
		t.Fatalf("dismiss: %v", err)
	}
	if !strings.Contains(out, `"dismissed"`) || !strings.Contains(out, `"updated": true`) {
		t.Errorf("json missing dismissed/updated in:\n%s", out)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	reloaded, _ := db.Open(config.DBPath(cfg))
	defer reloaded.Close()
	got, _ := reloaded.Candidates(db.StatusCandidateDismissed)
	if len(got) != 1 || got[0].Name != "Acme Corp" {
		t.Fatalf("dismissed candidates = %+v, want Acme Corp tombstoned", got)
	}
	// Tombstone means boards.toml untouched.
	bf, _, _ := loadBoardsStore()
	if bf.Find("acme-corp") != nil {
		t.Error("dismiss wrote to boards.toml")
	}
}

// TestDiscoverAddUnknownAndDecided: unknown ids fail with direction;
// an already-decided candidate is a no-op, not a double transition.
func TestDiscoverAddUnknownAndDecided(t *testing.T) {
	cand := setupDiscoverReviewTest(t)

	if _, err := runCmd(t, "discover", "add", "999"); err == nil || !strings.Contains(err.Error(), "no candidate") {
		t.Errorf("unknown id err = %v, want no-candidate guidance", err)
	}
	if _, err := runCmd(t, "discover", "add", "not-a-number"); err == nil || !strings.Contains(err.Error(), "must be a number") {
		t.Errorf("bad id err = %v, want numeric-id message", err)
	}

	// Decide it, then try again — both verbs must refuse.
	id := strconv.FormatInt(cand.ID, 10)
	stubVerifyBoard(t, 4, nil)
	if _, err := runCmd(t, "discover", "add", id); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if _, err := runCmd(t, "discover", "add", id); err == nil || !strings.Contains(err.Error(), "nothing to do") {
		t.Errorf("re-add err = %v, want nothing-to-do", err)
	}
	if _, err := runCmd(t, "discover", "dismiss", id); err == nil || !strings.Contains(err.Error(), "nothing to do") {
		t.Errorf("dismiss-after-add err = %v, want nothing-to-do", err)
	}
}
