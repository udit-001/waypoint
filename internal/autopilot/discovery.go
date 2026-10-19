package autopilot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
)

// timeNow is the clock seam for trigger decisions.
var timeNow = time.Now

// discoverCandidates is the Discover seam: a package var so the stage's
// body (filtering, landing, ledger writes) runs offline in tests.
var discoverCandidates = discovery.Discover

// discoveryRun is one discovery stage's yield: candidates that survived
// verification, and boards that auto-landed in boards.toml (WP-181).
type discoveryRun struct {
	Discovered int
	Landed     int
}

// runAutoDiscovery is the seam for the discovery pipeline; tests stub it.
var runAutoDiscovery = func(ctx context.Context, cfg CycleConfig, facets []discovery.Facet) (discoveryRun, error) {
	blocked, berr := cfg.Store.BlockedDomains()
	if berr != nil {
		return discoveryRun{}, fmt.Errorf("blocklist: %w", berr)
	}
	// Blocked domains never reach the prober: re-discovery skips them
	// entirely, whatever board link they resurface under (WP-181).
	facets = filterBlockedDomains(facets, blocked)

	bf, err := loadBoardsForDiscovery()
	if err != nil {
		return discoveryRun{}, err
	}
	watched := make(map[string]bool, len(bf.Boards))
	for _, b := range bf.Boards {
		watched[b.URL] = true
	}

	cands, err := discoverCandidates(ctx, facets, watched, discovery.Options{})
	if err != nil {
		return discoveryRun{}, err
	}
	rows := make([]db.CompanyCandidate, 0, len(cands))
	for _, c := range cands {
		bs := make([]db.CandidateBoard, 0, len(c.Boards))
		for _, l := range c.Boards {
			bs = append(bs, db.CandidateBoard{Provider: l.Provider, URL: l.URL})
		}
		rows = append(rows, db.CompanyCandidate{
			Name: c.Name, Domain: c.Domain, Facet: c.Facet,
			Boards: bs, Status: db.StatusCandidateSuggested,
		})
	}
	if err := cfg.Store.SaveCandidates(rows); err != nil {
		return discoveryRun{}, err
	}

	// Auto-promote (WP-181): verified candidates land now. A company the
	// ledger holds dismissed (a pre-auto-promote review decision) is a
	// standing no — it never re-enters boards.toml.
	status, serr := candidateStatuses(cfg.Store)
	if serr != nil {
		return discoveryRun{}, serr
	}
	landable := make([]discovery.Candidate, 0, len(cands))
	for _, c := range cands {
		if cand, ok := status[c.Name]; ok && cand.Status == db.StatusCandidateDismissed {
			continue
		}
		landable = append(landable, c)
	}
	landed, skipped, lerr := discovery.LandVerified(landable, bf)
	if lerr != nil {
		return discoveryRun{Discovered: len(cands)}, fmt.Errorf("auto-promote: %w", lerr)
	}
	if skipped > 0 {
		log.Printf("autopilot: auto-promote skipped %d extra verified board(s) — one entry per company; boards add can add the rest", skipped)
	}
	if len(landed) > 0 {
		if serr := saveBoardsForDiscovery(bf); serr != nil {
			return discoveryRun{Discovered: len(cands)}, fmt.Errorf("auto-promote: %w", serr)
		}
		if serr := markCandidatesAdded(cfg.Store, status, landed); serr != nil {
			return discoveryRun{Discovered: len(cands)}, fmt.Errorf("auto-promote ledger: %w", serr)
		}
	}
	return discoveryRun{Discovered: len(cands), Landed: len(landed)}, nil
}

// filterBlockedDomains drops blocked companies from the facet universe
// before probing — a blocked domain costs zero fetches. Passes the input
// through untouched when nothing is blocked.
func filterBlockedDomains(facets []discovery.Facet, blocked []string) []discovery.Facet {
	if len(blocked) == 0 {
		return facets
	}
	set := make(map[string]bool, len(blocked))
	for _, d := range blocked {
		set[db.NormalizeDomain(d)] = true
	}
	out := make([]discovery.Facet, 0, len(facets))
	for _, f := range facets {
		kept := make([]discovery.Company, 0, len(f.Companies))
		for _, c := range f.Companies {
			if !set[db.NormalizeDomain(c.Domain)] {
				kept = append(kept, c)
			}
		}
		out = append(out, discovery.Facet{Name: f.Name, Companies: kept})
	}
	return out
}

// candidateStatuses reads the ledger once and indexes it by company
// name, so the dismissed check and the status flip inside one
// runAutoDiscovery call share a single read.
func candidateStatuses(store db.Store) (map[string]db.CompanyCandidate, error) {
	cands, err := store.Candidates("")
	if err != nil {
		return nil, fmt.Errorf("read candidates: %w", err)
	}
	out := make(map[string]db.CompanyCandidate, len(cands))
	for _, c := range cands {
		out[c.Name] = c
	}
	return out, nil
}

// markCandidatesAdded flips a landed company's ledger row from suggested
// to added — the auto-promoted twin of the manual promote's status flip.
// statuses is the name->candidate map already read by candidateStatuses
// in the same run, so no second full ledger read is needed.
func markCandidatesAdded(store db.Store, statuses map[string]db.CompanyCandidate, landed []string) error {
	want := make(map[string]bool, len(landed))
	for _, n := range landed {
		want[n] = true
	}
	for name, c := range statuses {
		if want[name] && c.Status == db.StatusCandidateSuggested {
			if err := store.SetCandidateStatus(c.ID, db.StatusCandidateAdded); err != nil {
				return err
			}
		}
	}
	return nil
}

// loadBoardsForDiscovery reads boards.toml from the data dir (the daemon
// is the CLI process family — same config resolution).
func loadBoardsForDiscovery() (*config.BoardsFile, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return config.LoadBoards(cfg)
}

// saveBoardsForDiscovery writes boards.toml back through the same config
// resolution loadBoardsForDiscovery reads from.
func saveBoardsForDiscovery(bf *config.BoardsFile) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	return config.SaveBoards(cfg, bf)
}

// stageDiscovery runs discovery before the cycle when a trigger fires:
// first-run (no candidates ever), brief-hash change, or interval elapsed
// (new setting discovery_interval_days, default 30). Discovery never
// blocks scoring — a failure comes back as an error for the run record
// and the cycle proceeds to sweep. Trigger state persists in kv so daemon
// restarts don't re-fire.
//
// The returned reason names the trigger that fired (empty when no trigger
// did); the error is a stage failure, never a reason to skip the sweep.
func stageDiscovery(ctx context.Context, cfg CycleConfig) (reason string, landed int, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("autopilot: discovery panicked: %v — proceeding without it", r)
			err = fmt.Errorf("discovery: panic: %v", r)
		}
	}()

	settings, serr := cfg.Store.GetSettings()
	if serr != nil {
		log.Printf("autopilot: discovery skipped (settings): %v", serr)
		return "", 0, nil
	}
	// The Sources tab's discovery toggle is a hard off (WP-183): no
	// trigger fires and no trigger state is written, so re-enabling
	// starts from "first run", not a stale clock.
	if settings.DiscoveryEnabled == 0 {
		return "", 0, nil
	}
	interval := settings.DiscoveryIntervalDays
	if interval <= 0 {
		interval = 30
	}
	// Read the brief here rather than trusting cfg.LLMInput: this stage has to
	// tell "the brief is unreadable" (skip — the trigger state would be
	// poisoned by an empty hash) from "the brief is empty" (first run — the
	// trigger should fire). The cycle's projection has already collapsed that
	// distinction. Same renderer either way.
	brief, berr := cfg.Store.GetBrief()
	if berr != nil {
		log.Printf("autopilot: discovery skipped (brief): %v", berr)
		return "", 0, nil
	}
	briefText := brief.LLMInput().Text()

	lastHash, lastAt, hasLast, lerr := cfg.Store.DiscoveryLastRun()
	if lerr != nil {
		log.Printf("autopilot: discovery skipped (trigger state): %v", lerr)
		return "", 0, nil
	}
	cands, _ := cfg.Store.Candidates("")
	reason = discovery.ShouldRunDiscovery(discovery.TriggerInput{
		Now:           timeNow(),
		IntervalDays:  interval,
		Candidates:    len(cands),
		BriefHash:     discovery.BriefHash(briefText),
		HasLastRun:    hasLast,
		LastBriefHash: lastHash,
		LastRunAt:     parseRFC3339OrZero(lastAt),
	})
	if reason == "" {
		return "", 0, nil
	}

	out, derr := discoveryUniverse(ctx, cfg, briefText)
	if derr != nil {
		log.Printf("autopilot: discovery failed (%s): %v — proceeding with scoring", reason, derr)
		return "", 0, fmt.Errorf("discovery (%s): %w", reason, derr)
	}

	run, rerr := runAutoDiscovery(ctx, cfg, out.Facets)
	if rerr != nil {
		log.Printf("autopilot: discovery failed (%s): %v — proceeding with scoring", reason, rerr)
		return "", 0, fmt.Errorf("discovery (%s): %w", reason, rerr)
	}
	if serr := cfg.Store.SaveDiscoveryLastRun(discovery.BriefHash(briefText), timeNow().UTC().Format(time.RFC3339)); serr != nil {
		log.Printf("autopilot: discovery state save failed: %v", serr)
	}
	log.Printf("autopilot: discovery ran (%s, via %s): %d new candidate(s), %d board(s) landed", reason, out.Source, run.Discovered, run.Landed)
	// A loop failure that fell back to the hardcoded universe is the run
	// record's one discovery note: the stage errored (coverage narrowed to
	// the built-in universe), the cycle ran on. Silent by design when keys
	// are missing — the hardcoded path there is the no-key contract, not a
	// failure.
	if out.Fallback != "" {
		return reason, run.Landed, fmt.Errorf("discovery (%s): agent loop unavailable (%s) — hardcoded universe used, sweep unaffected", reason, out.Fallback)
	}
	return reason, run.Landed, nil
}

// universeOutcome carries one discovery run's universe and where it came
// from. Fallback is non-empty only when the agent loop failed and the
// hardcoded universe replaced it: the caller notes that in the run record.
type universeOutcome struct {
	Facets   []discovery.Facet
	Source   string
	Fallback string
}

// runAgentUniverse is the agent-loop producer seam: a package var (the
// runAutoDiscovery precedent) so tests stub the loop without standing up a
// second fake upstream.
var runAgentUniverse = discovery.AgentUniverse

// discoveryUniverse produces the company universe for one discovery run
// (WP-180): cached companies when the brief hash matches, else the zen
// agent loop. No keys, or an agent-loop failure, degrades to the built-in
// hardcoded facets — silent by design without keys (the selection
// contract's precedent), noted when the loop itself failed. Only a real
// agent run is cached, so a failure never pins itself.
func discoveryUniverse(ctx context.Context, cfg CycleConfig, briefText string) (*universeOutcome, error) {
	settings, err := cfg.Store.GetSettings()
	if err != nil {
		return nil, err
	}
	hasZen := strings.TrimSpace(settings.ZenAPIKey) != "" && cfg.ZenClient != nil
	hasExa := strings.TrimSpace(settings.ExaAPIKey) != "" && cfg.ExaClient != nil
	if !hasZen || !hasExa {
		return &universeOutcome{Facets: discovery.HardcodedFacets(), Source: "hardcoded"}, nil
	}

	hash := discovery.BriefHash(briefText)
	if cached, ok, cerr := cfg.Store.DiscoveryCompanies(hash); cerr != nil {
		// A cache-read failure costs one agent run — log and run fresh.
		log.Printf("autopilot: discovery cache read failed: %v — running the loop", cerr)
	} else if ok && len(cached) > 0 {
		return &universeOutcome{Facets: discovery.CachedFacets(cached), Source: "cached"}, nil
	}

	uni, aerr := runAgentUniverse(ctx, briefText, cfg.ZenClient, cfg.ExaClient)
	if aerr != nil {
		log.Printf("autopilot: discovery agent loop failed: %v — hardcoded facets", aerr)
		return &universeOutcome{
			Facets:   discovery.HardcodedFacets(),
			Source:   "hardcoded",
			Fallback: aerr.Error(),
		}, nil
	}
	if serr := cfg.Store.SaveDiscoveryCompanies(hash, discovery.ToStoredCompanies(uni.Companies)); serr != nil {
		log.Printf("autopilot: discovery cache write failed: %v", serr)
	}
	return &universeOutcome{Facets: uni.Facets, Source: "agent"}, nil
}

func parseRFC3339OrZero(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
