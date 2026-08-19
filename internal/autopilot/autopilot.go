// Package autopilot implements the six-stage curation cycle:
// sweep → detail → prefilter → curate → store+notify → runlog.
//
// The cycle is driven by a ticker goroutine inside the daemon. Each
// stage commits per-posting — zen outage mid-cycle leaves postings
// "new" for the next tick. Panics are recovered and logged to the
// run row.
package autopilot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/detail"
	"github.com/udit-001/waypoint/internal/exa"
	"github.com/udit-001/waypoint/internal/mdsvc"
	"github.com/udit-001/waypoint/internal/prefilter"
	"github.com/udit-001/waypoint/internal/scraper"
	"github.com/udit-001/waypoint/internal/zen"
)

// sleep is the test seam for backoff pauses.
var sleep = time.Sleep

// CycleConfig holds the configuration for one autopilot cycle.
type CycleConfig struct {
	Store     db.Store
	ZenClient *zen.Client
	Scrapers  []scraper.Scraper
	// ExaClient is the shared Exa seam: one budget and one cache for
	// company research (zen) and posting fetch (detail tier 4). Created
	// by Run when nil; the budget resets per cycle to ExaCap.
	ExaClient *exa.Client
	ExaCap    int           // max live Exa calls per cycle (default 10; anonymous MCP allows ~50/day)
	FetchCap  int           // max webfetch calls per cycle (default 10)
	Recency   int           // days for recency filter (default 14)
	Cadence   time.Duration // time between cycles
	Limit     int           // max new postings to curate (0 = all)
}

// Run executes one full autopilot cycle. It is safe to call from a
// goroutine — panics are recovered and logged.
func Run(ctx context.Context, cfg CycleConfig) db.RunLog {
	started := time.Now().UTC()
	logEntry := db.RunLog{
		StartedAt: started.Format(time.RFC3339),
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("autopilot: cycle panicked: %v", r)
			logEntry.Errors = addError(logEntry.Errors, fmt.Sprintf("panic: %v", r))
		}
	}()

	// Shared Exa seam: one client, one budget, one cache across company
	// research and detail tier 4. Anonymous by default — an invalid
	// Bearer is worse than none on the hosted MCP server.
	if cfg.ExaClient == nil {
		cfg.ExaClient = exa.New("", nil)
	}
	exaCap := cfg.ExaCap
	if exaCap == 0 {
		exaCap = 10
	}
	cfg.ExaClient.SetBudget(exaCap)

	// Stage 1: Sweep — scrape new postings from relevant sources.
	newPostings := stageSweep(ctx, cfg)
	logEntry.PostingsNew = len(newPostings)

	// Collect swept URLs to avoid double-processing.
	sweptURLs := make(map[string]bool, len(newPostings))
	for _, p := range newPostings {
		sweptURLs[p.URL] = true
	}

	// Also curate existing "new" postings from the ledger backlog,
	// excluding any that were just swept.
	backlog := stageBacklog(ctx, cfg, sweptURLs)

	// Merge: sweep results first, then backlog. Apply limit.
	allPostings := append(newPostings, backlog...)
	if cfg.Limit > 0 && len(allPostings) > cfg.Limit {
		allPostings = allPostings[:cfg.Limit]
	}

	// Stage 2: Detail — enrich postings with full job body.
	stageDetail(ctx, cfg, allPostings)

	// Stage 3+4: Prefilter + Curate — per-posting commit.
	shortlisted, dismissed, errored := stagePrefilterCurate(ctx, cfg, allPostings)
	logEntry.PostingsShortlisted = shortlisted
	logEntry.PostingsDismissed = dismissed
	logEntry.PostingsErrored = errored

	// Stage 5+6: Already done per-posting in stage 4.

	finished := time.Now().UTC()
	logEntry.FinishedAt = finished.Format(time.RFC3339)
	logEntry.DurationMs = finished.Sub(started).Milliseconds()

	return logEntry
}

// stageSweep scrapes all enabled scrapers and adds new postings to the ledger.
func stageSweep(ctx context.Context, cfg CycleConfig) []scraper.Result {
	var allNew []scraper.Result

	for _, s := range cfg.Scrapers {
		// Politeness: jittered delay between sources.
		jitter := time.Duration(2000+rand.Intn(1000)) * time.Millisecond
		select {
		case <-ctx.Done():
			return allNew
		case <-time.After(jitter):
		}

		// 30s timeout per source.
		srcCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		results, err := s.Search(srcCtx, scraper.SearchOpts{
			Limit:  50,
			JobAge: cfg.Recency,
		})
		cancel()

		if err != nil {
			log.Printf("autopilot: sweep %s failed: %v", s.Name(), err)
			continue
		}

		// Dedup against existing postings and jobs.
		for _, r := range results {
			seen, _ := cfg.Store.HasPosting(r.URL)
			if seen {
				continue
			}
			tracked, _ := cfg.Store.JobExists(r.URL)
			if tracked {
				continue
			}
			allNew = append(allNew, r)
		}
	}

	// Batch-add all new results to the postings ledger.
	if len(allNew) > 0 {
		if err := cfg.Store.AddPostings(allNew); err != nil {
			log.Printf("autopilot: sweep add postings: %v", err)
		}
	}

	return allNew
}

// stageBacklog fetches existing "new" postings from the ledger and returns
// them as scraper.Result for processing. This allows the autopilot to curate
// postings that were swept in previous cycles but never curated.
// exclude contains URLs already swept this cycle to avoid double-processing.
func stageBacklog(ctx context.Context, cfg CycleConfig, exclude map[string]bool) []scraper.Result {
	postings, err := cfg.Store.ListPostings(db.StatusNew)
	if err != nil {
		log.Printf("autopilot: backlog fetch failed: %v", err)
		return nil
	}
	results := make([]scraper.Result, 0, len(postings))
	for _, p := range postings {
		if exclude[p.Result.URL] {
			continue
		}
		results = append(results, p.Result)
	}
	if len(results) > 0 {
		log.Printf("autopilot: backlog found %d uncured postings", len(results))
	}
	return results
}

// stageDetail enriches postings with full job body using the detail chain.
// It writes enriched data to the DB AND updates the in-memory Result so
// subsequent stages (prefilter, zen) see the full description.
func stageDetail(ctx context.Context, cfg CycleConfig, postings []scraper.Result) {
	chain := BuildDetailChain()
	// Tier 3: direct HTTP fetch (free). Tier 4: shared Exa client — its
	// internal budget spans detail fetches AND zen company research, so
	// the chain-level cap is left unset here.
	chain.Direct = &scraper.HTTPFetcher{}
	chain.Exa = cfg.ExaClient

	for i := range postings {
		p := &postings[i]
		select {
		case <-ctx.Done():
			return
		default:
		}

		detailCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		result, err := chain.FetchDetail(detailCtx, p.URL, FindBoardURL(p.URL), p.ID, p.Description, p.Metadata)
		cancel()

		if err != nil {
			log.Printf("autopilot: detail %s failed: %v", p.URL, err)
			continue
		}

		if result.Description != "" || result.Body != "" {
			newDesc, newMeta := detail.MergeDetailResultInto(p.Description, p.Metadata, result)
			// Update in-memory so zen sees the full description.
			p.Description = newDesc
			if len(newMeta) > 0 {
				if p.Metadata == nil {
					p.Metadata = make(map[string]string)
				}
				for k, v := range newMeta {
					p.Metadata[k] = v
				}
			}
			// Persist to DB.
			_ = cfg.Store.EnrichPosting(p.URL, newDesc, newMeta)
		}
	}
}

// stagePrefilterCurate runs the prefilter and zen curation per-posting.
// Returns counts of shortlisted, dismissed, and errored postings.
func stagePrefilterCurate(ctx context.Context, cfg CycleConfig, postings []scraper.Result) (shortlisted, dismissed, errored int) {
	// Build profile for prefilter.
	profile := buildPrefilterProfile(cfg.Store)

	// Build a portrait of the person from profile data.
	portrait := buildPortrait(cfg.Store)
	systemPrompt := fmt.Sprintf(`You are a talent scout. Be **grounded** — every judgment must cite something concrete from the posting. No vibes, no assumptions.

The person you're scouting for:
%s

For each posting:
1. Read the title, company, location, description.
2. If you don't know what the company does, call search_company.
3. Call curate_posting with:
   - verdict: shortlist or dismiss
   - score: 0-100
   - reasons: 1-3 grounded facts about the ROLE. Each reason must name a specific thing from the posting — the responsibilities, the tech, the level, the domain, the location. Reason format: "<what the job is> — <why it fits or doesn't>".

Scoring guide: 80+ strong fit (role + domain + level + location all align), 60-79 decent (most align, one gap), below 60 weak (major mismatch).`, portrait)

	// Create zen session (one per cycle).
	var session *zen.Session
	if cfg.ZenClient != nil {
		session = cfg.ZenClient.NewSession(systemPrompt)
		// Company research through the shared Exa client (same budget
		// as detail tier 4); posting-page fetch through our free fetcher.
		cfg.ZenClient.SetCompanySearcher(cfg.ExaClient)
		fetchCap := cfg.FetchCap
		if fetchCap == 0 {
			fetchCap = 10
		}
		wf := zen.NewWebFetcher(fetchCap)
		// A fresh mdsvc client here re-reads the quota state file, so it
		// inherits the detail stage's usage written earlier this cycle.
		wf.Markdown = mdsvc.New(MdsvcStatePath())
		cfg.ZenClient.SetPageFetcher(wf)
	}

	for _, p := range postings {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Stage 3: Prefilter — deterministic Go rules.
		verdict := prefilter.Filter(p, profile, cfg.Store)
		if verdict.Action == "dismiss" {
			_ = cfg.Store.SetPostingStatus(p.URL, db.StatusDismissed)
			dismissed++
			continue
		}
		if verdict.Action == "shortlist" {
			_ = cfg.Store.SetPostingStatus(p.URL, db.StatusShortlisted)
			shortlisted++
			continue
		}

		// Stage 4: Curate — zen LLM (when available).
		if session == nil {
			// No zen client — escalate to shortlist for manual review.
			_ = cfg.Store.SetPostingStatus(p.URL, db.StatusShortlisted)
			shortlisted++
			continue
		}

		v, err := session.Curate(ctx, zen.Posting{
			URL:      p.URL,
			Markdown: p.Description,
		})
		if err != nil {
			log.Printf("autopilot: curate %s failed: %v", p.URL, err)
			errored++
			continue
		}

		// Apply verdict — store score + reasons in posting metadata.
		meta := map[string]string{
			"score":   fmt.Sprintf("%d", v.Score),
			"reasons": mustJSON(v.Reasons),
		}
		_ = cfg.Store.EnrichPosting(p.URL, "", meta)

		if v.Decision == zen.DecisionDismiss {
			_ = cfg.Store.SetPostingStatus(p.URL, db.StatusDismissed)
			dismissed++
		} else {
			_ = cfg.Store.SetPostingStatus(p.URL, db.StatusShortlisted)
			shortlisted++
		}
	}

	return
}

// buildPortrait synthesizes a readable portrait from the person's profile.
// The portrait is what zen sees — it should read like a recruiter's brief,
// not a JSON dump.
func buildPortrait(store db.Store) string {
	p, _ := store.GetProfile()

	var b strings.Builder

	// Title + seniority.
	if p.Title != "" {
		b.WriteString(fmt.Sprintf("Role focus: %s\n", p.Title))
	}
	// Seniority is derived from experience, not stored.
	seniority := p.Seniority
	if seniority == "" {
		seniority = db.DeriveSeniority(p.Experience)
	}
	if seniority != "" {
		b.WriteString(fmt.Sprintf("Level: %s\n", seniority))
	}

	// Location.
	if p.CurrentLocation != "" {
		b.WriteString(fmt.Sprintf("Current location: %s\n", p.CurrentLocation))
	}
	if p.Remote == "yes" {
		b.WriteString("Prefers: remote\n")
	}
	if p.LocationPref != "" {
		var locs []string
		_ = json.Unmarshal([]byte(p.LocationPref), &locs)
		if len(locs) > 0 {
			b.WriteString(fmt.Sprintf("Location preferences: %s\n", strings.Join(locs, ", ")))
		}
	}

	// Experience — the real signal.
	if p.Experience != "" {
		e := db.ParseExperienceEntries(p.Experience)
		if len(e) > 0 {
			b.WriteString("\nExperience:\n")
			for _, exp := range e {
				end := exp.End
				if end == "" {
					end = "present"
				}
				b.WriteString(fmt.Sprintf("  %s–%s  %s @ %s\n", exp.Start, end, exp.Title, exp.Company))
			}
		}
	}

	// Skills — curated, not raw tags.
	if p.Skills != "" {
		var skills []string
		_ = json.Unmarshal([]byte(p.Skills), &skills)
		if len(skills) > 0 {
			// Dedupe and pick the meaningful ones.
			seen := make(map[string]bool)
			var curated []string
			for _, s := range skills {
				s = strings.TrimSpace(s)
				lower := strings.ToLower(s)
				if seen[lower] {
					continue
				}
				seen[lower] = true
				// Skip generic filler tags.
				if lower == "developer" || lower == "engineer" || lower == "software developer" ||
					lower == "software engineer" || lower == "software development" ||
					lower == "development engineering" || lower == "development" {
					continue
				}
				curated = append(curated, s)
			}
			if len(curated) > 0 {
				b.WriteString(fmt.Sprintf("Skills: %s\n", strings.Join(curated, ", ")))
			}
		}
	}

	// Target companies.
	if p.Companies != "" {
		var cos []string
		_ = json.Unmarshal([]byte(p.Companies), &cos)
		if len(cos) > 0 {
			b.WriteString(fmt.Sprintf("\nTarget companies: %s\n", strings.Join(cos, ", ")))
		}
	}

	// Avoid companies.
	if p.AvoidCompanies != "" {
		var avoid []string
		_ = json.Unmarshal([]byte(p.AvoidCompanies), &avoid)
		if len(avoid) > 0 {
			b.WriteString(fmt.Sprintf("Avoid: %s\n", strings.Join(avoid, ", ")))
		}
	}

	// Keywords.
	if p.Keywords != "" {
		var kws []string
		_ = json.Unmarshal([]byte(p.Keywords), &kws)
		if len(kws) > 0 {
			b.WriteString(fmt.Sprintf("Interests: %s\n", strings.Join(kws, ", ")))
		}
	}

	return b.String()
}

// buildPrefilterProfile extracts the curation-relevant fields from the store.
func buildPrefilterProfile(store db.Store) prefilter.Profile {
	p, _ := store.GetProfile()
	return prefilter.Profile{
		AvoidCompanies: prefilter.ParseProfileCompanies(p.AvoidCompanies),
		Companies:      prefilter.ParseProfileCompanies(p.Companies),
		SalaryFloor:    prefilter.ParseProfileFloors(p.SalaryFloor),
	}
}

// mustJSON marshals v to a JSON string. Panics on failure (should never
// happen for []string).
func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// addError appends an error string to a JSON array.
func addError(errorsJSON, msg string) string {
	var errs []string
	if errorsJSON != "" && errorsJSON != "[]" {
		_ = json.Unmarshal([]byte(errorsJSON), &errs)
	}
	errs = append(errs, msg)
	b, _ := json.Marshal(errs)
	return string(b)
}

// Ticker runs the autopilot cycle on a ticker. It blocks until ctx is done.
func Ticker(ctx context.Context, cfg CycleConfig, store db.Store) {
	// Get initial settings.
	settings, _ := store.GetSettings()
	cadence := time.Duration(settings.AutopilotCadence) * time.Hour
	if cadence <= 0 {
		cadence = 6 * time.Hour
	}

	ticker := time.NewTicker(cadence)
	defer ticker.Stop()

	log.Printf("autopilot: ticker started (cadence: %s)", cadence)

	for {
		select {
		case <-ctx.Done():
			log.Println("autopilot: ticker stopped")
			return
		case <-ticker.C:
			// Re-read settings in case cadence changed.
			settings, _ = store.GetSettings()
			if settings.AutopilotEnabled == 0 {
				continue
			}

			newCadence := time.Duration(settings.AutopilotCadence) * time.Hour
			if newCadence <= 0 {
				newCadence = 6 * time.Hour
			}
			if newCadence != cadence {
				ticker.Reset(newCadence)
				cadence = newCadence
				log.Printf("autopilot: cadence changed to %s", cadence)
			}

			// Run one cycle.
			log.Println("autopilot: starting cycle")
			entry := Run(ctx, cfg)

			// Store run log.
			id, err := store.AddRunLog(entry)
			if err != nil {
				log.Printf("autopilot: failed to log run: %v", err)
			}

			log.Printf("autopilot: cycle complete (id=%d, new=%d, shortlisted=%d, dismissed=%d, errored=%d, duration=%dms)",
				id, entry.PostingsNew, entry.PostingsShortlisted,
				entry.PostingsDismissed, entry.PostingsErrored, entry.DurationMs)
		}
	}
}

// --- helpers ---

// Ensure strings is used (for contains checks in future).
var _ = strings.Contains
