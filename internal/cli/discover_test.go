package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
)

// stubDiscover swaps the discovery pipeline seam for a canned result.
func stubDiscover(t *testing.T, result []discovery.Candidate) {
	t.Helper()
	old := runDiscovery
	runDiscovery = func(ctx context.Context, facets []discovery.Facet, watched map[string]bool, opts discovery.Options) ([]discovery.Candidate, error) {
		return result, nil
	}
	t.Cleanup(func() { runDiscovery = old })
}

// runDiscoverRun invokes the run command directly (RunE, like the
// postings tests) so the fake store survives PersistentPreRunE.
func runDiscoverRun(t *testing.T) string {
	t.Helper()
	out := captureStdout(t, func() {
		if err := discoverRunCmd.RunE(discoverRunCmd, nil); err != nil {
			t.Fatalf("discover run: %v", err)
		}
	})
	return out
}

// TestDiscoverRun_persistsAndPrints: candidates land in the store and
// the table output carries name, boards, and status.
func TestDiscoverRun_persistsAndPrints(t *testing.T) {
	setupBoardsTest(t)
	stubDiscover(t, []discovery.Candidate{{
		Name: "Arcesium", Domain: "arcesium.com", Facet: "fininfra",
		Boards: []discovery.BoardLink{{Provider: "greenhouse", URL: "https://job-boards.greenhouse.io/arcesium/"}},
	}})

	out := runDiscoverRun(t)
	if !strings.Contains(out, "Arcesium") || !strings.Contains(out, "greenhouse") {
		t.Errorf("output missing candidate info:\n%s", out)
	}
	if !strings.Contains(out, "suggested") {
		t.Errorf("output missing status column:\n%s", out)
	}

	cands, err := store.Candidates("")
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 || cands[0].Name != "Arcesium" || cands[0].Status != db.StatusCandidateSuggested {
		t.Fatalf("store = %+v, want one suggested Arcesium", cands)
	}
}

// TestDiscoverRun_watchedBoardsPassed: every configured board URL flows
// into Discover's watched set.
func TestDiscoverRun_watchedBoardsPassed(t *testing.T) {
	setupBoardsTest(t)
	bf, cfg, err := loadBoardsStore()
	if err != nil {
		t.Fatal(err)
	}
	bf.Upsert(config.BoardEntry{Name: "citi", Company: "Citi", URL: "https://citi.wd103.myworkdayjobs.com/CitiCareers/", Enabled: true})
	if err := config.SaveBoards(cfg, bf); err != nil {
		t.Fatal(err)
	}

	var gotWatched map[string]bool
	old := runDiscovery
	runDiscovery = func(ctx context.Context, facets []discovery.Facet, watched map[string]bool, opts discovery.Options) ([]discovery.Candidate, error) {
		gotWatched = watched
		return nil, nil
	}
	t.Cleanup(func() { runDiscovery = old })

	runDiscoverRun(t)
	if !gotWatched["https://citi.wd103.myworkdayjobs.com/citicareers"] {
		t.Errorf("watched set = %v, want the configured board URL (normalized)", gotWatched)
	}
}

// TestDiscoverRun_json: --json output is valid JSON carrying meta and
// the candidate rows.
func TestDiscoverRun_json(t *testing.T) {
	setupBoardsTest(t)
	jsonOut = true
	t.Cleanup(func() { jsonOut = false })
	stubDiscover(t, []discovery.Candidate{{
		Name: "FactSet", Domain: "factset.com", Facet: "market-data",
		Boards: []discovery.BoardLink{{Provider: "workday", URL: "https://factset.wd1.myworkdayjobs.com/FactSetCareers/"}},
	}})

	out := runDiscoverRun(t)
	var payload struct {
		Meta       map[string]any           `json:"meta"`
		Candidates []map[string]interface{} `json:"candidates"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if payload.Meta == nil || payload.Meta["found"].(float64) != 1 {
		t.Errorf("meta = %+v, want found=1", payload.Meta)
	}
	if len(payload.Candidates) != 1 || payload.Candidates[0]["name"] != "FactSet" {
		t.Errorf("candidates = %+v, want FactSet", payload.Candidates)
	}
	if got := payload.Candidates[0]["status"]; got != db.StatusCandidateSuggested {
		t.Errorf("status = %v, want suggested", got)
	}
}

// TestDiscoverRun_emptyResult prints a friendly line and no table.
func TestDiscoverRun_emptyResult(t *testing.T) {
	setupBoardsTest(t)
	stubDiscover(t, nil)

	out := runDiscoverRun(t)
	if !strings.Contains(out, "No new companies") {
		t.Errorf("output = %q, want an empty-result message", out)
	}
}

// TestDiscoverRun_rerunShowsDismissedStatus: a company dismissed in a
// prior run re-discovered today reports its persisted dismissed status,
// not a fresh suggested one.
func TestDiscoverRun_rerunShowsDismissedStatus(t *testing.T) {
	setupBoardsTest(t)
	fake := store.(*db.FakeStore)
	if err := fake.SaveCandidates([]db.CompanyCandidate{{
		Name: "Paytm", Domain: "paytm.com", Facet: "payments-fintech",
	}}); err != nil {
		t.Fatal(err)
	}
	seeded, err := fake.Candidates("")
	if err != nil || len(seeded) != 1 {
		t.Fatalf("seed: %v (%+v)", err, seeded)
	}
	if err := fake.SetCandidateStatus(seeded[0].ID, db.StatusCandidateDismissed); err != nil {
		t.Fatal(err)
	}
	stubDiscover(t, []discovery.Candidate{{
		Name: "Paytm", Domain: "paytm.com", Facet: "payments-fintech",
		Boards: []discovery.BoardLink{{Provider: "lever", URL: "https://jobs.lever.co/paytm/"}},
	}})

	out := runDiscoverRun(t)
	if !strings.Contains(out, "dismissed") {
		t.Errorf("output should surface persisted dismissed status:\n%s", out)
	}
}
