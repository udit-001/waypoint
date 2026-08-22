package autopilot

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
	"github.com/udit-001/waypoint/internal/exa"
)

// timeNow is the clock seam for trigger decisions.
var timeNow = time.Now

// runAutoDiscovery is the seam for the discovery pipeline; tests stub it.
var runAutoDiscovery = func(ctx context.Context, cfg CycleConfig, facets []discovery.Facet) (int, error) {
	bf, err := loadBoardsForDiscovery()
	if err != nil {
		return 0, err
	}
	watched := make(map[string]bool, len(bf.Boards))
	for _, b := range bf.Boards {
		watched[b.URL] = true
	}

	cands, err := discovery.Discover(ctx, facets, watched, discovery.Options{})
	if err != nil {
		return 0, err
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
		return 0, err
	}
	return len(rows), nil
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

// cycleEnumerator adapts the shared Exa client to the enumerator seam.
type cycleEnumerator struct{ c *exa.Client }

func (e *cycleEnumerator) Companies(ctx context.Context, facet string) ([]discovery.Company, error) {
	hits, err := e.c.SearchCompanies(ctx, facet, 15)
	if err != nil {
		return nil, err
	}
	out := make([]discovery.Company, 0, len(hits))
	for _, h := range hits {
		u, perr := parseURLHost(h.URL)
		if h.Name == "" || u == "" || perr != nil {
			continue
		}
		out = append(out, discovery.Company{Name: h.Name, Domain: u})
	}
	return out, nil
}

func parseURLHost(rawURL string) (string, error) {
	u := strings.TrimPrefix(strings.TrimPrefix(strings.ToLower(rawURL), "https://"), "http://")
	u = strings.SplitN(u, "/", 2)[0]
	if u == "" {
		return "", errEmptyDomain
	}
	return strings.TrimPrefix(u, "www."), nil
}

var errEmptyDomain = errors.New("empty domain")

// stageDiscovery runs discovery before the cycle when a trigger fires:
// first-run (no candidates ever), brief-hash change, or interval elapsed
// (new setting discovery_interval_days, default 30). Discovery never
// blocks scoring — a failure is logged with cause and the cycle proceeds
// to sweep. Trigger state persists in kv so daemon restarts don't re-fire.
func stageDiscovery(ctx context.Context, cfg CycleConfig) string {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("autopilot: discovery panicked: %v — proceeding without it", r)
		}
	}()

	settings, serr := cfg.Store.GetSettings()
	if serr != nil {
		log.Printf("autopilot: discovery skipped (settings): %v", serr)
		return ""
	}
	interval := settings.DiscoveryIntervalDays
	if interval <= 0 {
		interval = 30
	}
	brief, berr := cfg.Store.GetBrief()
	if berr != nil {
		log.Printf("autopilot: discovery skipped (brief): %v", berr)
		return ""
	}
	briefText := brief.Text()

	lastHash, lastAt, hasLast, lerr := cfg.Store.DiscoveryLastRun()
	if lerr != nil {
		log.Printf("autopilot: discovery skipped (trigger state): %v", lerr)
		return ""
	}
	cands, _ := cfg.Store.Candidates("")
	reason := discovery.ShouldRunDiscovery(discovery.TriggerInput{
		Now:           timeNow(),
		IntervalDays:  interval,
		Candidates:    len(cands),
		BriefHash:     discovery.BriefHash(briefText),
		HasLastRun:    hasLast,
		LastBriefHash: lastHash,
		LastRunAt:     parseRFC3339OrZero(lastAt),
	})
	if reason == "" {
		return ""
	}

	facets, source, derr := expandEnumerateFacets(ctx, cfg, briefText)
	if derr != nil {
		log.Printf("autopilot: discovery failed (%s): %v — proceeding with scoring", reason, derr)
		return ""
	}

	n, rerr := runAutoDiscovery(ctx, cfg, facets)
	if rerr != nil {
		log.Printf("autopilot: discovery failed (%s): %v — proceeding with scoring", reason, rerr)
		return ""
	}
	if serr := cfg.Store.SaveDiscoveryLastRun(discovery.BriefHash(briefText), timeNow().UTC().Format(time.RFC3339)); serr != nil {
		log.Printf("autopilot: discovery state save failed: %v", serr)
	}
	log.Printf("autopilot: discovery ran (%s, facets via %s): %d new candidate(s)", reason, source, n)
	return reason
}

// expandEnumerateFacets mirrors the CLI's enumeration path: brief → zen
// facets (cached by hash) → Exa companies, falling back to the built-in
// starter facets when either key is missing.
func expandEnumerateFacets(ctx context.Context, cfg CycleConfig, briefText string) ([]discovery.Facet, string, error) {
	settings, err := cfg.Store.GetSettings()
	if err != nil {
		return nil, "", err
	}
	hasExa := strings.TrimSpace(settings.ExaAPIKey) != "" && cfg.ExaClient != nil
	hasZen := strings.TrimSpace(settings.ZenAPIKey) != "" && cfg.ZenClient != nil
	if !hasExa || !hasZen {
		return discovery.HardcodedFacets(), "hardcoded", nil
	}

	brief, berr := cfg.Store.GetBrief()
	if berr != nil {
		return nil, "", berr
	}
	text := brief.Text()
	hash := discovery.BriefHash(text)
	if cached, ok, _ := cfg.Store.DiscoveryFacets(hash); ok && len(cached) > 0 {
		return toFacetList(cached), "cached", nil
	}

	facets, err := discovery.ExpandFacets(ctx, text, cfg.ZenClient, 12)
	if err != nil {
		return nil, "", err
	}
	if err := cfg.Store.SaveDiscoveryFacets(hash, facets); err != nil {
		return nil, "", err
	}

	enum := &cycleEnumerator{c: cfg.ExaClient}
	out, err := discovery.Enumerate(ctx, facets, enum, 60)
	if err != nil {
		return nil, "", err
	}
	return out, "brief+exa", nil
}

func toFacetList(labels []string) []discovery.Facet {
	out := make([]discovery.Facet, 0, len(labels))
	for _, l := range labels {
		out = append(out, discovery.Facet{Name: l})
	}
	return out
}

func parseRFC3339OrZero(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
