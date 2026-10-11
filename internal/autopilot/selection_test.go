package autopilot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/db/dbtest"
	"github.com/udit-001/waypoint/internal/discovery"
	"github.com/udit-001/waypoint/internal/scraper"
	"github.com/udit-001/waypoint/internal/zen"
)

// The selection tests cross the stage through the cycle's own seams: the
// input builder and parser as pure units, the cache through the Store, the
// stage through a fake zen upstream — the same fake-SSE pattern the
// curation tests use. No live network.

// textUpstream is a fake zen that records every request body and answers
// each one with the configured plain-text completion (the shape zen's
// Complete path reads). hits counts selection calls, so the cache test can
// assert a second cycle made zero LLM calls.
type textUpstream struct {
	mu     sync.Mutex
	bodies []string
	reply  string
	status int // 0 = 200
}

func (u *textUpstream) handler(w http.ResponseWriter, r *http.Request) {
	buf := make([]byte, 1<<16)
	n, _ := r.Body.Read(buf)
	u.mu.Lock()
	u.bodies = append(u.bodies, string(buf[:n]))
	u.mu.Unlock()

	if u.status != 0 && u.status != http.StatusOK {
		w.WriteHeader(u.status)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	half := len(u.reply) / 2
	w.Write([]byte("data: " + jsonStr(map[string]any{
		"choices": []map[string]any{{"delta": map[string]any{"role": "assistant", "content": u.reply[:half]}}},
	}) + "\n\n"))
	w.Write([]byte("data: " + jsonStr(map[string]any{
		"choices": []map[string]any{{"delta": map[string]any{"content": u.reply[half:]}, "finish_reason": "stop"}},
	}) + "\n\n"))
	w.Write([]byte("data: [DONE]\n\n"))
}

func jsonStr(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func (u *textUpstream) start(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(u.handler))
	t.Cleanup(srv.Close)
	return srv
}

func (u *textUpstream) hits() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.bodies)
}

func (u *textUpstream) sent() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return strings.Join(u.bodies, "\n")
}

// selScrapers builds the stage's fixture: one specialist portal, one
// aggregator, one source the model never selects — the shape the fallback,
// skip-reason, and runlog tests need.
func selScrapers() []scraper.Scraper {
	return []scraper.Scraper{
		&fixtureScraper{name: "spec", cats: []string{"academic", "biotech"}},
		&fixtureScraper{name: "aggr", cats: []string{"aggregator", "academic"}},
		&fixtureScraper{name: "unsel", cats: []string{"tech"}},
	}
}

// --- input builder + prompt (slices 2-3) ---

// TestSelectionInput_IsPIIFree: the selection user-turn is built from the
// allowlist projection, never from raw profile rows — the same seam the
// curation call crosses, asserted at the rendered text.
func TestSelectionInput_IsPIIFree(t *testing.T) {
	in := db.BriefFromProfile(dbtest.PIIProfile()).LLMInput()
	all := selScrapers()

	out := buildSelectionInput(in, all, nil)

	for _, c := range dbtest.NeverLeaves() {
		if strings.Contains(out, c.Value) {
			t.Errorf("%s reached the selection input: %q", c.What, c.Value)
		}
	}
	must := []dbtest.Leak{
		{What: "title", Value: "Backend Engineer"},
		{What: "industry", Value: "fintech"},
		{What: "keyword", Value: "payments"},
		{What: "scraper id", Value: "spec"},
		{What: "scraper id", Value: "aggr"},
	}
	for _, m := range must {
		if !strings.Contains(out, m.Value) {
			t.Errorf("selection input missing %s: %q", m.What, m.Value)
		}
	}
}

// TestSelectionPrompt_styleContract: the prompt is agent-consumed text, so
// it follows the writing-for-agents contract — priority order as the single
// source of truth, a completion criterion, and the JSON shape stated once
// at the end.
func TestSelectionPrompt_styleContract(t *testing.T) {
	must := []string{
		"Priority order:",                   // the single source of truth
		"Preferences are the search intent", // rule 1
		"judge fit only",                    // rule 2: facts never widen
		"Exclude one only with cause",       // rule 3: aggregators
		"Done means:",                       // the completion criterion
		`{"scrapers": [{"id": "`,            // the JSON shape
	}
	for _, m := range must {
		if !strings.Contains(selectionSystemPrompt, m) {
			t.Errorf("selection prompt missing %q", m)
		}
	}
}

// --- parser (slice 2) ---

// TestParseSelection_dropsUnknownKeepsOrder: ids outside the registry are
// dropped; selected ids keep the model's priority order; reasons and the
// note ride along.
func TestParseSelection_dropsUnknownKeepsOrder(t *testing.T) {
	byID := scraperMap(selScrapers())
	raw := `{"scrapers": [
		{"id": "spec", "reason": "matches the academic preference"},
		{"id": "ghost", "reason": "not in the registry"},
		{"id": "aggr", "reason": "covers the rest"}
	], "note": "two sources cover the domains"}`

	sel, err := parseSelection(raw, byID)
	if err != nil {
		t.Fatalf("parseSelection: %v", err)
	}
	if len(sel.IDs) != 2 || sel.IDs[0] != "spec" || sel.IDs[1] != "aggr" {
		t.Errorf("IDs = %v, want [spec aggr] in model order, ghost dropped", sel.IDs)
	}
	if sel.Reasons["spec"] != "matches the academic preference" || sel.Note == "" {
		t.Errorf("reasons/note lost: %+v", sel)
	}
}

// TestParseSelection_rejectsUnusable: bad JSON, wrong shapes, and
// selections that are empty after dropping unknowns all fail — the caller
// answers with the aggregators-only fallback.
func TestParseSelection_rejectsUnusable(t *testing.T) {
	byID := scraperMap(selScrapers())
	for _, raw := range []string{
		"not json at all",
		`[]`,
		`{}`,
		`{"scrapers": []}`,
		`{"scrapers": [{"id": "ghost"}, {"id": "phantom"}]}`,
	} {
		if _, err := parseSelection(raw, byID); err == nil {
			t.Errorf("parseSelection(%q) = nil error, want rejection", raw)
		}
	}
}

// --- stage (slice 5): fallback, cache, runlog rows ---

func selCycle(f db.Store, scrapers []scraper.Scraper) CycleConfig {
	cfg := newCycle(f, scrapers)
	cfg.LLMInput = db.BriefFromProfile(dbtest.PIIProfile()).LLMInput()
	return cfg
}

// TestSelectSources_noZenKey_fallsBackToAggregators: a keyless install
// still sweeps — the aggregators, with the fallback noted on the selection.
func TestSelectSources_noZenKey_fallsBackToAggregators(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	cfg := selCycle(f, selScrapers())

	sel := stageSelectSource(context.Background(), cfg, newRunRecord(time.Now().UTC()))

	if len(sel.IDs) != 1 || sel.IDs[0] != "aggr" {
		t.Errorf("IDs = %v, want the aggregator only", sel.IDs)
	}
	if sel.FallbackReason == "" {
		t.Error("fallback not noted on the selection")
	}
}

// TestSelectSources_zenUnusable_fallsBackToAggregators: an outage and a
// broken reply both degrade to the aggregators — never all sources, never
// a zero-source sweep.
func TestSelectSources_zenUnusable_fallsBackToAggregators(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		reply  string
	}{
		{"outage", http.StatusInternalServerError, ""},
		{"bad json", 0, "not json at all"},
		{"all unknown", 0, `{"scrapers": [{"id": "ghost"}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubDiscovery(t)
			f := db.NewFakeStore()
			up := &textUpstream{reply: tc.reply, status: tc.status}
			cfg := selCycle(f, selScrapers())
			cfg.ZenClient = zen.New(zen.Config{BaseURL: up.start(t).URL, APIKey: "k", Model: "m"})

			sel := stageSelectSource(context.Background(), cfg, newRunRecord(time.Now().UTC()))

			if len(sel.IDs) != 1 || sel.IDs[0] != "aggr" {
				t.Errorf("IDs = %v, want the aggregator only", sel.IDs)
			}
			if sel.FallbackReason == "" {
				t.Error("fallback not noted on the selection")
			}
			// The fallback must not pin itself: nothing was cached.
			key := selectionKey(discovery.BriefHash(cfg.LLMInput.Text()), scraper.Fingerprint(selScrapers()))
			if _, found, _ := f.SourceSelection(key); found {
				t.Error("fallback was cached — an outage would persist past recovery")
			}
		})
	}
}

// TestSelectSources_cachesUntilInputsChange: the selection runs once per
// brief-hash + registry fingerprint — the second cycle makes zero
// selection calls; a brief change re-runs.
func TestSelectSources_cachesUntilInputsChange(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	up := &textUpstream{reply: `{"scrapers": [{"id": "spec", "reason": "matches"}], "note": "n"}`}
	cfg := selCycle(f, selScrapers())
	cfg.ZenClient = zen.New(zen.Config{BaseURL: up.start(t).URL, APIKey: "k", Model: "m"})

	sel := stageSelectSource(context.Background(), cfg, newRunRecord(time.Now().UTC()))
	if sel.FallbackReason != "" || len(sel.IDs) != 1 || sel.IDs[0] != "spec" {
		t.Fatalf("first selection = %+v, want zen's pick", sel)
	}
	if up.hits() != 1 {
		t.Fatalf("zen hits = %d, want 1", up.hits())
	}

	// Same inputs: served from the cache.
	sel2 := stageSelectSource(context.Background(), cfg, newRunRecord(time.Now().UTC()))
	if len(sel2.IDs) != 1 || sel2.IDs[0] != "spec" || sel2.FallbackReason != "" {
		t.Errorf("cached selection = %+v, want the cached pick", sel2)
	}
	if up.hits() != 1 {
		t.Errorf("zen hits = %d after a cached cycle, want 1 (cache hit)", up.hits())
	}

	// Brief change: selection re-runs.
	cfg.LLMInput.Title = "Data Engineer"
	sel3 := stageSelectSource(context.Background(), cfg, newRunRecord(time.Now().UTC()))
	if up.hits() != 2 {
		t.Errorf("zen hits = %d after a brief change, want 2 (re-selection)", up.hits())
	}
	if len(sel3.IDs) != 1 || sel3.IDs[0] != "spec" {
		t.Errorf("re-selected = %v, want the fresh pick", sel3.IDs)
	}
}

// TestCycle_skippedSourcesInRunlog: the full cycle records every
// unselected source as a per-source skipped row with a reason, and a
// zen-selected-but-user-disabled source is never swept regardless.
func TestCycle_skippedSourcesInRunlog(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	f.Settings.AutopilotDisabledScrapers = db.DisabledScrapersJSON([]string{"spec"})
	up := &textUpstream{reply: `{"scrapers": [
		{"id": "spec", "reason": "matches"},
		{"id": "aggr", "reason": "covers the rest"}
	], "note": "n"}`}
	cfg := selCycle(f, selScrapers())
	cfg.ZenClient = zen.New(zen.Config{BaseURL: up.start(t).URL, APIKey: "k", Model: "m"})

	entry := Run(context.Background(), cfg)

	bySource := map[string]db.SourceOutcome{}
	for _, row := range entry.PerSource {
		bySource[row.Source] = row
	}
	spec := bySource["spec"]
	if !spec.Skipped || !strings.Contains(spec.Reason, "disabled") {
		t.Errorf("disabled source row = %+v, want skipped with a disabled reason", spec)
	}
	unsel := bySource["unsel"]
	if !unsel.Skipped || unsel.Reason == "" {
		t.Errorf("unselected source row = %+v, want skipped with a reason", unsel)
	}
	aggr := bySource["aggr"]
	if aggr.Skipped {
		t.Errorf("selected source was skipped: %+v", aggr)
	}
	if spec.Scraped != 0 {
		t.Errorf("disabled source was swept anyway: %+v", spec)
	}
	// Structural enforcement: the disabled id never reached zen's table —
	// the SOURCES block would carry "spec | <kind> | ..." if it had.
	if strings.Contains(up.sent(), "spec |") {
		t.Errorf("disabled source appeared in the selection table zen saw")
	}
}

// stubSelection replaces the selection stage with a pass-through that
// selects every scraper — for tests that exercise other stages against a
// fake zen upstream and must not pay the selection path's retry backoff.
func stubSelection(t *testing.T) {
	t.Helper()
	orig := stageSelectSource
	stageSelectSource = func(_ context.Context, cfg CycleConfig, _ *runRecord) *SourceSelection {
		ids := make([]string, 0, len(cfg.Scrapers))
		reasons := make(map[string]string, len(cfg.Scrapers))
		for _, s := range cfg.Scrapers {
			ids = append(ids, s.Name())
			reasons[s.Name()] = "stubbed selection"
		}
		return &SourceSelection{IDs: ids, Reasons: reasons, Note: "stubbed"}
	}
	t.Cleanup(func() { stageSelectSource = orig })
}
