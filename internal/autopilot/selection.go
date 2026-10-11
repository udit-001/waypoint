package autopilot

// The source-selection stage (WP-179): at cycle start, zen reads the brief
// and decides which scrapers are worth sweeping for this person. Normal
// cycles make zero LLM calls here — the decision is cached until the brief
// hash or the scraper-registry fingerprint changes. Zen is a decider, not
// a load-bearing path: no key, an outage, or unusable output degrades to
// the aggregators-only fallback, and every unselected source lands in the
// runlog as a skipped row with its reason.

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/udit-001/waypoint/internal/config"
	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
	"github.com/udit-001/waypoint/internal/scraper"
)

// SourceSelection is one selection decision: the scrapers the cycle
// sweeps, in priority order (the sweep runs them in this order), with one
// reason per id. FallbackReason is non-empty when zen did not decide —
// the aggregators-only path ran instead — and names why.
type SourceSelection struct {
	IDs            []string
	Reasons        map[string]string
	Note           string
	FallbackReason string
}

// selectionSystemPrompt is the selection contract's system prompt. Written
// for the model as an agent-consumed document: the priority order is the
// single source of truth, phrased positively, with an explicit completion
// criterion, and the JSON shape stated once at the end.
const selectionSystemPrompt = `You select which job sources to sweep for this person's job search.

The user turn carries the search brief and the source table. Choose the sources worth sweeping, in priority order — your order is the order the sweep runs.

Priority order:
1. Preferences are the search intent. The industries, locations, and companies in the brief's preferences decide what the search targets.
2. Facts and work history judge fit only: they tell you which domains to cover, they never widen the search beyond the preferences.
3. Aggregator sources serve any industry. Exclude one only with cause: a conflict with the preferences, or the selected specialist sources already cover the person's domains.

Every selected source carries a one-line reason naming the rule that picked it.

Done means: the selected set covers every domain the preferences name, and every source left out is excluded by the priority order.

Respond with JSON only:
{"scrapers": [{"id": "<source id>", "reason": "<one line>"}], "note": "<one line on the selection>"}`

// stageSelectSource runs the cached selection. It always returns a usable
// selection — the fallback answers inside it — and records nothing as a
// stage error: the fallback is a designed path, not a broken stage. Only
// a real zen selection is cached, so an outage never pins itself.
//
// A package var (the runAutoDiscovery precedent) so tests stub the stage
// without standing up a second fake upstream.
var stageSelectSource = func(ctx context.Context, cfg CycleConfig, rec *runRecord) *SourceSelection {
	briefText := cfg.LLMInput.Text()
	fingerprint := scraper.Fingerprint(cfg.Scrapers)
	key := selectionKey(discovery.BriefHash(briefText), fingerprint)

	if cached, found, err := cfg.Store.SourceSelection(key); err != nil {
		log.Printf("autopilot: selection cache read failed: %v — selecting fresh", err)
	} else if found {
		return &SourceSelection{IDs: cached.IDs, Reasons: cached.Reasons, Note: cached.Note}
	}

	disabled := disabledSet(cfg)
	byID := scraperMap(visibleScrapers(cfg.Scrapers, disabled))

	if cfg.ZenClient == nil {
		return aggregatorsOnly(cfg.Scrapers, disabled, "no zen key configured")
	}
	// The table zen sees is already the visible set: a disabled scraper is
	// structurally unselectable, and the sweep filters again on its own read.
	visible := visibleScrapers(cfg.Scrapers, disabled)
	out, err := cfg.ZenClient.Complete(ctx, selectionSystemPrompt, buildSelectionInput(cfg.LLMInput, visible, watchedBoards()))
	if err != nil {
		log.Printf("autopilot: source selection failed: %v — aggregators only", err)
		return aggregatorsOnly(cfg.Scrapers, disabled, "zen unavailable")
	}
	sel, perr := parseSelection(out, byID)
	if perr != nil {
		log.Printf("autopilot: source selection unusable: %v — aggregators only", perr)
		return aggregatorsOnly(cfg.Scrapers, disabled, "zen returned an unusable selection")
	}
	if len(sel.IDs) == 0 {
		return aggregatorsOnly(cfg.Scrapers, disabled, "zen selected nothing usable")
	}

	if err := cfg.Store.SaveSourceSelection(key, db.SourceSelection{
		IDs: sel.IDs, Reasons: sel.Reasons, Note: sel.Note,
	}); err != nil {
		// A cache-write failure costs one extra LLM call next cycle — log
		// and carry the in-memory selection either way.
		log.Printf("autopilot: selection cache write failed: %v", err)
	}
	return sel
}

// selectionKey derives the cache key: brief hash + registry fingerprint.
// Either changing means the world the selection was made for changed.
func selectionKey(briefHash, fingerprint string) string {
	sum := sha1.Sum([]byte(briefHash + "|" + fingerprint))
	return hex.EncodeToString(sum[:])
}

// disabledSet reads the user's scraper opt-out (WP-175). A read failure
// disables nothing — the sweep-level guard still enforces the stored list
// at its own read.
func disabledSet(cfg CycleConfig) map[string]bool {
	settings, err := cfg.Store.GetSettings()
	if err != nil {
		return nil
	}
	out := make(map[string]bool)
	for _, name := range db.ParseDisabledScrapers(settings.AutopilotDisabledScrapers) {
		out[name] = true
	}
	return out
}

// visibleScrapers drops disabled scrapers from the table zen sees: a
// constraint enforced structurally — zen cannot select what it is never
// shown — and the sweep filters again on its own read (belt and braces).
func visibleScrapers(all []scraper.Scraper, disabled map[string]bool) []scraper.Scraper {
	if len(disabled) == 0 {
		return all
	}
	out := make([]scraper.Scraper, 0, len(all))
	for _, s := range all {
		if !disabled[s.Name()] {
			out = append(out, s)
		}
	}
	return out
}

func scraperMap(all []scraper.Scraper) map[string]scraper.Scraper {
	out := make(map[string]scraper.Scraper, len(all))
	for _, s := range all {
		out[s.Name()] = s
	}
	return out
}

// boards loads the enabled board names for context. A failure contributes
// nothing — boards are context, never a decision input zen needs.
func watchedBoards() []config.BoardEntry {
	bf, err := loadBoardsForDiscovery()
	if err != nil || bf == nil {
		return nil
	}
	return bf.Boards
}

// buildSelectionInput renders the selection user turn: the brief (the
// allowlist projection — the PII seam crossed in one place), the source
// table, and the watched boards as context. Target ~1KB.
func buildSelectionInput(in db.LLMInput, all []scraper.Scraper, boards []config.BoardEntry) string {
	var sb strings.Builder
	sb.WriteString("BRIEF\n")
	sb.WriteString(in.Text())
	sb.WriteString("\n\nSOURCES\nid | kind | categories\n")
	for _, s := range all {
		kind := "listing"
		if scraper.IsAPI(s.Name()) {
			kind = "api"
		}
		fmt.Fprintf(&sb, "%s | %s | %s\n", s.Name(), kind, strings.Join(s.Categories(), ", "))
	}
	if len(boards) > 0 {
		names := make([]string, 0, len(boards))
		for _, b := range boards {
			names = append(names, b.Company)
		}
		sb.WriteString("\nWATCHED BOARDS (companies already swept directly, context only)\n")
		sb.WriteString(strings.Join(names, ", "))
		sb.WriteString("\n")
	}
	return sb.String()
}

// selectionReply is the JSON shape the prompt locks.
type selectionReply struct {
	Scrapers []struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	} `json:"scrapers"`
	Note string `json:"note"`
}

// parseSelection validates the model's JSON against the registry: unknown
// ids drop, priority order is preserved, and a reply that selects nothing
// usable is an error — the caller answers with the fallback. Code fences
// are stripped; the prompt asks for JSON only, models wrap anyway.
func parseSelection(raw string, byID map[string]scraper.Scraper) (*SourceSelection, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var reply selectionReply
	if err := json.Unmarshal([]byte(raw), &reply); err != nil {
		return nil, fmt.Errorf("parse selection: %w (output: %.200s)", err, raw)
	}
	sel := &SourceSelection{Reasons: make(map[string]string), Note: reply.Note}
	for _, sc := range reply.Scrapers {
		if _, ok := byID[sc.ID]; !ok {
			continue
		}
		sel.IDs = append(sel.IDs, sc.ID)
		sel.Reasons[sc.ID] = sc.Reason
	}
	if len(sel.IDs) == 0 {
		return nil, fmt.Errorf("selection named no registered source")
	}
	return sel, nil
}

// aggregators is the fallback set: the scrapers whose categories include
// "aggregator" — by definition the sources that serve any industry, so an
// unusable selection never widens past them and never zeroes out.
func aggregators(all []scraper.Scraper) []string {
	var out []string
	for _, s := range all {
		for _, c := range s.Categories() {
			if c == "aggregator" {
				out = append(out, s.Name())
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// aggregatorsOnly answers with the fallback set and the reason noted —
// the runlog's skipped rows carry it to the user.
func aggregatorsOnly(all []scraper.Scraper, disabled map[string]bool, why string) *SourceSelection {
	visible := visibleScrapers(all, disabled)
	ids := aggregators(visible)
	reasons := make(map[string]string, len(ids))
	for _, id := range ids {
		reasons[id] = "aggregator — fallback selection"
	}
	return &SourceSelection{
		IDs:            ids,
		Reasons:        reasons,
		Note:           "aggregators only: " + why,
		FallbackReason: why,
	}
}
