package autopilot

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
	"github.com/udit-001/waypoint/internal/exa"
	"github.com/udit-001/waypoint/internal/scraper"
	"github.com/udit-001/waypoint/internal/zen"
)

// seedSweeperScraper gives the cycle one scraper so scoring has work.
func seedSweeperScraper() scraper.Scraper {
	return &fixtureScraper{
		name:    "discovery-stage-scraper",
		results: []scraper.Result{{URL: "https://example.com/d1", Title: "Eng", Company: "Acme"}},
	}
}

// TestCycle_FirstRunTriggersDiscovery: a fresh store (zero candidates)
// fires the first-run trigger before hunting; state is persisted so the
// next cycle does not re-fire, and scoring still completes.
func TestCycle_FirstRunTriggersDiscovery(t *testing.T) {
	f := db.NewFakeStore()
	s := seedSweeperScraper()

	calls := 0
	origRun := runAutoDiscovery
	runAutoDiscovery = func(_ context.Context, cfg CycleConfig, _ []discovery.Facet) (discoveryRun, error) {
		calls++
		// Mirror production: discovered candidates land in the ledger.
		_ = cfg.Store.SaveCandidates([]db.CompanyCandidate{
			{Name: "Discovered A", Status: db.StatusCandidateSuggested},
			{Name: "Discovered B", Status: db.StatusCandidateSuggested},
		})
		return discoveryRun{Discovered: 2}, nil
	}
	t.Cleanup(func() { runAutoDiscovery = origRun })

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{s}))
	if calls != 1 {
		t.Errorf("discovery ran %d time(s), want exactly 1 on first run", calls)
	}
	if entry.PostingsNew != 1 {
		t.Errorf("scoring did not complete alongside discovery: new=%d", entry.PostingsNew)
	}

	_, _, has, _ := f.DiscoveryLastRun()
	if !has {
		t.Error("last-run state not persisted — daemon restarts would re-fire")
	}

	// Second cycle with unchanged brief and fresh state must skip.
	calls = 0
	Run(context.Background(), newCycle(f, []scraper.Scraper{s}))
	if calls != 0 {
		t.Errorf("discovery re-fired without a trigger (%d runs)", calls)
	}
}

// TestCycle_DiscoveryFailureDoesNotBlock: discovery failing (cause
// logged) leaves the cycle's sweep/scoring fully intact.
func TestCycle_DiscoveryFailureDoesNotBlock(t *testing.T) {
	f := db.NewFakeStore()
	s := seedSweeperScraper()

	origRun := runAutoDiscovery
	runAutoDiscovery = func(_ context.Context, _ CycleConfig, _ []discovery.Facet) (discoveryRun, error) {
		return discoveryRun{}, errors.New("exa search: 429 rate limited")
	}
	t.Cleanup(func() { runAutoDiscovery = origRun })

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{s}))
	if entry.PostingsNew != 1 {
		t.Errorf("cycle blocked by discovery failure: new=%d", entry.PostingsNew)
	}
	_, _, has, _ := f.DiscoveryLastRun()
	if has {
		t.Error("failed discovery must not persist last-run state")
	}
}

// --- discoveryUniverse (WP-180): the agent loop behind the seam ---

// withKeysClients gives a cycle both keys and non-nil clients so the
// agent path is selected; the clients are never dialed (the loop is
// stubbed).
func withKeysClients(t *testing.T, f *db.FakeStore) CycleConfig {
	t.Helper()
	f.Settings.ZenAPIKey = "zen-key"
	f.Settings.ExaAPIKey = "exa-key"
	cfg := newCycle(f, nil)
	// A fatal-401 upstream: any stage that dials zen fails fast (no retry
	// backoff) — the loops themselves are stubbed in these tests.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	t.Cleanup(dead.Close)
	cfg.ZenClient = zen.New(zen.Config{BaseURL: dead.URL})
	cfg.ExaClient = exa.New("", nil)
	return cfg
}

func agentStub(t *testing.T, fn func() (*discovery.AgentUniverseResult, error)) *int {
	t.Helper()
	orig := runAgentUniverse
	calls := 0
	runAgentUniverse = func(_ context.Context, _ string, _ discovery.AgentLooper, _ discovery.Searcher) (*discovery.AgentUniverseResult, error) {
		calls++
		return fn()
	}
	t.Cleanup(func() { runAgentUniverse = orig })
	return &calls
}

func agentUniverseOK() *discovery.AgentUniverseResult {
	return &discovery.AgentUniverseResult{
		Facets: []discovery.Facet{{
			Name:      discovery.AgentFacetName,
			Companies: []discovery.Company{{Name: "Vercel", Domain: "vercel.com"}},
		}},
		Companies: []discovery.AgentCompany{{Name: "Vercel", Domain: "vercel.com", Why: "deploy platform"}},
	}
}

// TestDiscoveryUniverse_AgentRunCachedPerBrief: a successful agent run
// caches its companies under the brief hash; the same brief reads the
// cache back and never re-runs the loop.
func TestDiscoveryUniverse_AgentRunCachedPerBrief(t *testing.T) {
	f := db.NewFakeStore()
	cfg := withKeysClients(t, f)
	calls := agentStub(t, func() (*discovery.AgentUniverseResult, error) {
		return agentUniverseOK(), nil
	})

	brief, _ := f.GetBrief()
	text := brief.LLMInput().Text()

	out, err := discoveryUniverse(context.Background(), cfg, text)
	if err != nil {
		t.Fatalf("discoveryUniverse: %v", err)
	}
	if out.Source != "agent" || out.Fallback != "" {
		t.Errorf("first run: source=%q fallback=%q, want agent/none", out.Source, out.Fallback)
	}
	if len(out.Facets[0].Companies) != 1 {
		t.Errorf("facets = %+v, want the agent universe", out.Facets)
	}

	hash := discovery.BriefHash(text)
	saved, ok, _ := f.DiscoveryCompanies(hash)
	if !ok || len(saved) != 1 || saved[0].Why != "deploy platform" {
		t.Errorf("cache = %+v (ok=%v), want the submission with rationale", saved, ok)
	}

	out2, err := discoveryUniverse(context.Background(), cfg, text)
	if err != nil {
		t.Fatalf("second discoveryUniverse: %v", err)
	}
	if out2.Source != "cached" {
		t.Errorf("second run: source=%q, want cached (no agent loop for the same brief)", out2.Source)
	}
	if *calls != 1 {
		t.Errorf("agent runs = %d, want 1", *calls)
	}
}

// TestDiscoveryUniverse_LoopFailureFallsBack: a failed loop comes back as
// the hardcoded universe with the fallback named — never as a hard error.
func TestDiscoveryUniverse_LoopFailureFallsBack(t *testing.T) {
	f := db.NewFakeStore()
	cfg := withKeysClients(t, f)
	agentStub(t, func() (*discovery.AgentUniverseResult, error) {
		return nil, errors.New("zen unavailable")
	})

	brief, _ := f.GetBrief()
	out, err := discoveryUniverse(context.Background(), cfg, brief.LLMInput().Text())
	if err != nil {
		t.Fatalf("fallback must not error: %v", err)
	}
	if out.Fallback == "" || !strings.Contains(out.Fallback, "zen unavailable") {
		t.Errorf("fallback = %q, want the loop failure named", out.Fallback)
	}
	if len(out.Facets) == 0 {
		t.Error("no facets — the hardcoded universe must carry the run")
	}

	// A failure is never cached: the next run attempts the loop again.
	if _, ok, _ := f.DiscoveryCompanies(discovery.BriefHash(brief.LLMInput().Text())); ok {
		t.Error("failed loop cached itself")
	}
}

// TestDiscoveryUniverse_NoKeysSilent: without keys the hardcoded universe
// runs by design — not a fallback, not an error, the loop never starts.
func TestDiscoveryUniverse_NoKeysSilent(t *testing.T) {
	f := db.NewFakeStore()
	cfg := newCycle(f, nil) // no keys, nil clients
	calls := agentStub(t, func() (*discovery.AgentUniverseResult, error) {
		return nil, errors.New("the loop must not run without keys")
	})

	out, err := discoveryUniverse(context.Background(), cfg, "brief")
	if err != nil {
		t.Fatalf("no-key path errored: %v", err)
	}
	if out.Source != "hardcoded" || out.Fallback != "" {
		t.Errorf("source=%q fallback=%q, want silent hardcoded", out.Source, out.Fallback)
	}
	if *calls != 0 {
		t.Error("agent loop started without keys")
	}
}

// TestCycle_FallbackNoted: a loop failure inside a full cycle lands the
// fallback note in the runlog, persists last-run state (the fallback ran
// discovery), and leaves the sweep untouched.
func TestCycle_FallbackNoted(t *testing.T) {
	f := db.NewFakeStore()
	s := seedSweeperScraper()
	cfg := withKeysClients(t, f)
	cfg.Scrapers = []scraper.Scraper{s}

	agentStub(t, func() (*discovery.AgentUniverseResult, error) {
		return nil, errors.New("no submit in budget")
	})
	origRun := runAutoDiscovery
	runAutoDiscovery = func(_ context.Context, _ CycleConfig, _ []discovery.Facet) (discoveryRun, error) {
		return discoveryRun{}, nil
	}
	t.Cleanup(func() { runAutoDiscovery = origRun })

	entry := Run(context.Background(), cfg)
	if entry.PostingsNew != 1 {
		t.Errorf("sweep blocked by discovery fallback: new=%d", entry.PostingsNew)
	}

	noted := false
	for _, e := range entry.StageErrors {
		if strings.Contains(e, "agent loop unavailable") && strings.Contains(e, "hardcoded universe") {
			noted = true
		}
	}
	if !noted {
		t.Errorf("fallback not noted in the runlog: %v", entry.StageErrors)
	}

	_, _, has, _ := f.DiscoveryLastRun()
	if !has {
		t.Error("fallback run did not persist last-run state — it would re-fire next cycle")
	}
}
