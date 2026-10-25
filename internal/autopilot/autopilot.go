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
	"sort"
	"sync"
	"time"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/detail"
	"github.com/udit-001/waypoint/internal/exa"
	"github.com/udit-001/waypoint/internal/mdsvc"
	"github.com/udit-001/waypoint/internal/prefilter"
	"github.com/udit-001/waypoint/internal/scraper"
	"github.com/udit-001/waypoint/internal/zen"
)

// UnscoredCapPerCycle bounds the degraded path: without a Zen client
// every surviving posting would escalate to shortlisted, flooding
// Matches on fresh installs. Only the freshest slice per cycle is
// escalated; the rest stay queued for later cycles (or for scoring
// once a key lands).
const UnscoredCapPerCycle = 25

// sleep is the test seam for backoff pauses.
var sleep = time.Sleep
var after = time.After

// CycleConfig holds the configuration for one autopilot cycle.
type CycleConfig struct {
	Store     db.Store
	ZenClient *zen.Client
	Scrapers  []scraper.Scraper
	// Notifier delivers the one per-cycle nudge when shortlists exist.
	// Nil (or any error it returns) never blocks scoring.
	Notifier Notifier
	// ExaClient is the shared Exa seam: one budget and one cache for
	// company research (zen) and posting fetch (detail tier 4). Created
	// by Run when nil; the budget resets per cycle to ExaCap.
	ExaClient *exa.Client
	// LLMInput is the allowlist projection of the brief (db.LLMInput),
	// loaded once by Run. Every LLM input the cycle builds — curation now,
	// source selection next — renders from it, so the PII seam is crossed
	// in one place. Empty until Run fills it.
	LLMInput db.LLMInput
	ExaCap   int           // max live Exa calls per cycle (default 10; anonymous MCP allows ~50/day)
	FetchCap int           // max webfetch calls per cycle (default 10)
	Recency  int           // days for recency filter (default 14)
	Cadence  time.Duration // time between cycles
	Limit    int           // max new postings to curate (0 = all)
	// Budget is the cycle's own resource envelope (wall clock + scored
	// cap), distinct from the user-facing Limit. Nil means defaults; zero
	// fields inside a provided Budget keep their "no limit" meaning.
	Budget *Budget
	// Direct overrides the detail stage's tier-3 HTTP fetcher. When nil,
	// the cycle falls back to a real scraper.HTTPFetcher. Tests inject a
	// fake so the cycle never touches the network.
	Direct detail.Fetcher
	// Progress receives the cycle's live stage facts as they happen:
	// stage is one of the fixed stage ids (discover/select/sweep/read/
	// curate), of is a "k/n" counter, label the per-item fact (source
	// display name, posting company). The stages report facts and never
	// know where they land; RunLogged adapts the callback to the run row
	// (store.SetRunProgress), so the web's progress line and the cycle
	// stay decoupled. Since WP-189 the sweep runs concurrently with
	// detail+curate, so facts from different stages may interleave and
	// the callback may be invoked from two goroutines — implementations
	// must tolerate that. Nil is legal — stages no-op — so a run without
	// a row (RunLogged's fallback) still works.
	Progress func(stage, of, label string)
}

// progress reports a stage fact when the cycle has a progress seam. The
// nil check lives here once — stages state what they are doing, not
// whether anyone is listening.
func (cfg CycleConfig) progress(stage, of, label string) {
	if cfg.Progress != nil {
		cfg.Progress(stage, of, label)
	}
}

// itemLabel is a posting's per-item progress fact: the company when the
// ledger knows it, else the title — the skimmable name, not the URL.
func itemLabel(p scraper.Result) string {
	if p.Company != "" {
		return p.Company
	}
	return p.Title
}

// runRecord accumulates a cycle's evidence as the stages execute: the
// per-source rows and the stage-level errors. It is the writer's side of
// the runlog record; the verdict is computed from it when the run closes,
// never stored as a parallel truth.
type runRecord struct {
	db.RunLog
	outcomes map[string]*db.SourceOutcome
	closed   bool
}

func newRunRecord(started time.Time) *runRecord {
	return &runRecord{
		RunLog:   db.RunLog{StartedAt: started.Format(time.RFC3339)},
		outcomes: map[string]*db.SourceOutcome{},
	}
}

// outcome returns a source's row, creating it on first touch.
func (r *runRecord) outcome(source string) *db.SourceOutcome {
	if row, ok := r.outcomes[source]; ok {
		return row
	}
	row := &db.SourceOutcome{Source: source}
	r.outcomes[source] = row
	return row
}

// record moves a posting to a status and attributes the outcome to the
// source that produced it. The ledger write and the per-source count move
// together here; the run totals are summed from the rows when the record
// closes, so there is one place a new outcome has to be taught.
func (r *runRecord) record(store db.Store, p scraper.Result, status string) {
	_ = store.SetPostingStatus(p.URL, status)
	row := r.outcome(p.Source)
	switch status {
	case db.StatusShortlisted:
		row.Shortlisted++
	case db.StatusDismissed:
		row.Dismissed++
	}
}

// stageError records a stage-level failure (a fetch that never ran, a
// panicked stage, an interrupted cycle). Per-posting failures are counted
// per source instead — that split is what keeps signal out of noise.
func (r *runRecord) stageError(format string, args ...any) {
	r.StageErrors = append(r.StageErrors, fmt.Sprintf(format, args...))
}

// finish closes the record: it stamps the end, materializes the per-source
// rows, and computes the verdict. Called once from Run's defer, so a
// panicked or interrupted cycle still ends with a verdict.
func (r *runRecord) finish(ctx context.Context, store db.Store) db.RunLog {
	if r.closed {
		return r.RunLog
	}
	r.closed = true

	finished := time.Now().UTC()
	r.FinishedAt = finished.Format(time.RFC3339)
	started, _ := time.Parse(time.RFC3339, r.StartedAt)
	r.DurationMs = finished.Sub(started).Milliseconds()

	// An interrupted cycle is a degraded one: its coverage is partial.
	if err := ctx.Err(); err != nil {
		r.stageError("cycle interrupted: %v", err)
	}

	r.PerSource = r.rows()
	r.sumTotals()
	r.Verdict = verdictFor(r.RunLog, queueDepth(store, r))
	return r.RunLog
}

// sumTotals sets the run totals from the per-source evidence — never a
// parallel count that could drift from it. Idempotent, so the cycle can ask
// mid-run (the notifier needs the shortlist count) and finish() can ask
// again after the last outcome landed.
func (r *runRecord) sumTotals() {
	r.PostingsShortlisted, r.PostingsDismissed, r.PostingsErrored = 0, 0, 0
	for _, row := range r.outcomes {
		r.PostingsShortlisted += row.Shortlisted
		r.PostingsDismissed += row.Dismissed
		r.PostingsErrored += row.Errored
	}
}

// rows materializes the per-source record in a stable order (by source
// id), so two renderings of one log agree byte for byte.
func (r *runRecord) rows() []db.SourceOutcome {
	out := make([]db.SourceOutcome, 0, len(r.outcomes))
	for _, row := range r.outcomes {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}

// verdictFor is the run's outcome — the answer to "why did nothing show
// up?". Precedence: a stage error outranks everything (the run's coverage
// is partial, so it must never read as dry — §7's whole point), then a
// non-empty review queue (the ball is with the user), then whether anything
// survived curation, then quiet.
//
// Only a stage error degrades a run. Per-posting failures are a count under
// the verdict, never the verdict itself: the run did look everywhere, so
// "1 of 47 failed to score" must not outrank a shortlist waiting to be
// reviewed. A curate stage that failed outright is a stage error, recorded
// as one by the curate stage — not inferred from the count here.
func verdictFor(entry db.RunLog, queued int) db.Verdict {
	switch {
	case len(entry.StageErrors) > 0:
		return db.VerdictDegraded
	case queued > 0:
		return db.VerdictWaitingOnYou
	case entry.PostingsDismissed > 0:
		return db.VerdictNothingSurvived
	default:
		return db.VerdictQuiet
	}
}

// queueDepth is the review queue's depth: postings shortlisted and not yet
// reviewed. A read failure is a stage error — without the count the verdict
// could claim quiet while postings wait.
func queueDepth(store db.Store, rec *runRecord) int {
	n, err := store.CountPostings(db.StatusShortlisted)
	if err != nil {
		rec.stageError("runlog: read review queue: %v", err)
		return 0
	}
	return n
}

// loadLLMInput projects the store's brief onto the LLM allowlist. Called
// once per cycle so every LLM input renders from one projection. A read
// failure leaves the projection empty — curation then judges without the
// person's context, which is logged rather than fatal (same tolerance as
// the prefilter profile).
func loadLLMInput(store db.Store) db.LLMInput {
	brief, err := store.GetBrief()
	if err != nil {
		log.Printf("autopilot: brief read failed: %v — LLM input will be empty", err)
		return db.LLMInput{}
	}
	return brief.LLMInput()
}

// Run executes one full autopilot cycle. It is safe to call from a
// goroutine — panics are recovered and logged, and the run-log row is
// always CLOSED (FinishedAt set, verdict computed) even on panic: the
// scheduler's cadence measures from the last finished run, so an open row
// would re-fire a poisoned cycle at poll rate forever.
func Run(ctx context.Context, cfg CycleConfig) (logEntry db.RunLog) {
	start := time.Now().UTC()
	rec := newRunRecord(start)

	defer func() {
		if r := recover(); r != nil {
			log.Printf("autopilot: cycle panicked: %v", r)
			rec.stageError("panic: %v", r)
		}
		// One place closes the record: a panicked cycle still ends with a
		// verdict rather than an open row.
		logEntry = rec.finish(ctx, cfg.Store)
	}()

	// One allowlist projection for the whole cycle. Every LLM input the
	// cycle builds renders from cfg.LLMInput — see db.LLMInput for why that
	// shape is the seam rather than a callsite discipline.
	cfg.LLMInput = loadLLMInput(cfg.Store)

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

	// Cycle budget: the autopilot's own envelope, separate from the
	// user-facing Limit. Nil config takes the defaults; a provided
	// Budget's zero fields stay "no limit" so tests can build partial
	// budgets.
	defaultedBudget := false
	if cfg.Budget == nil {
		cfg.Budget = &Budget{Deadline: start.Add(DefaultWall), MaxScored: DefaultScoredCap}
		defaultedBudget = true
	}
	// gate skips a stage the wall budget spent, recording why coverage
	// is partial. Postings a skip leaves behind stay "new" in the
	// ledger; the next cycle's backlog picks them up.
	gate := func(stage string) bool {
		if !cfg.Budget.Expired() {
			return false
		}
		wall := DefaultWall
		if !defaultedBudget {
			wall = time.Since(start).Round(time.Second)
		}
		rec.stageError("wall budget (%s) spent before %s stage", wall, stage)
		return true
	}

	// Stage 0: Discovery — widen the company universe when a trigger
	// fires (first-run / brief change / interval). Never blocks scoring:
	// a failure is recorded as a stage error and the cycle proceeds.
	// Landed boards ride the runlog as evidence, never a verdict input.
	cfg.progress("discover", "", "")
	_, landed, derr := stageDiscovery(ctx, cfg)
	if derr != nil {
		rec.stageError("%v", derr)
	}
	rec.DiscoveredBoards = landed

	// Stage 1: Source selection — zen picks which sources are worth
	// sweeping for this brief (cached; aggregators-only fallback). The
	// runlog's skipped rows carry the decision to the user.
	cfg.progress("select", "", "")
	sel := stageSelectSource(ctx, cfg, rec)

	// Stages 2-5, pipelined (WP-189): a producer goroutine scrapes the
	// selected sources and pushes each source's new postings as a batch —
	// then the ledger backlog as the final batch — while this goroutine
	// consumes batches through detail and prefilter+curate. The derived
	// context is the consumer's stop signal: a spent budget or a full
	// escalation bound cancels it, so the sweep stops fetching once
	// nothing more can be curated this cycle.
	pctx, cancel := context.WithCancel(ctx)
	defer cancel()
	pipe := newPipeline(cancel)

	sweepGated := gate("sweep")
	buf := newRunRecord(start) // producer's sweep evidence, merged after the consumer's loop ends
	go pipe.produce(pctx, cfg, rec, buf, sel, sweepGated)

	pipe.consume(ctx, pctx, cfg, rec, gate)
	close(pipe.consumerDone)
	<-pipe.producerDone
	rec.sumTotals()

	// Stage 6: Notify — one nudge per cycle when shortlists exist. The
	// URL set is what the consumer escalated, however the batches landed;
	// the notice reads the ledger, so it still runs on a spent budget.
	notifyShortlist(ctx, cfg, rec, pipe.escalatedSoFar())

	// Stage 7: Run log — closed by the deferred finish above; written by
	// RunLogged by the caller.
	return
}

// skipReason answers why a source is not swept this cycle, and whether
// that is the case. The user's disabled list wins over everything — the
// sweep enforces it on its own read, regardless of what selection said.
func skipReason(name string, sel *SourceSelection, disabled map[string]bool) (string, bool) {
	if disabled[name] {
		return "disabled by user", true
	}
	for _, id := range sel.IDs {
		if id == name {
			return "", false
		}
	}
	if sel.FallbackReason != "" {
		return "selection fallback: aggregators only (" + sel.FallbackReason + ")", true
	}
	return "not selected by source selection", true
}

// stageBacklog fetches existing "new" postings from the ledger and returns
// them as scraper.Result for processing. This allows the autopilot to curate
// postings that were swept in previous cycles but never curated.
// exclude contains URLs already swept this cycle to avoid double-processing.
func stageBacklog(ctx context.Context, cfg CycleConfig, rec *runRecord, exclude map[string]bool) []scraper.Result {
	postings, err := cfg.Store.ListPostings(db.StatusNew)
	if err != nil {
		log.Printf("autopilot: backlog fetch failed: %v", err)
		rec.stageError("backlog: %v", err)
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
	// Accept the injected fetcher (test seam); fall back to the real one.
	chain.Direct = cfg.Direct
	if chain.Direct == nil {
		chain.Direct = &scraper.HTTPFetcher{}
	}
	chain.Exa = cfg.ExaClient

	for i := range postings {
		p := &postings[i]
		cfg.progress("read", fmt.Sprintf("%d/%d", i+1, len(postings)), itemLabel(*p))
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
// Every outcome is written to the ledger and attributed to the posting's
// source; the run totals are summed from those rows when the record closes.
func stagePrefilterCurate(ctx context.Context, cfg CycleConfig, postings []scraper.Result, rec *runRecord) {
	// Build the prefilter profile from the store.
	profile := buildPrefilterProfile(cfg.Store)

	systemPrompt := fmt.Sprintf(`You are a talent scout. Be **grounded** — every judgment must cite something concrete from the posting.

The person you're scouting for:
%s

For each posting (title, company, location, description):
1. If you don't know what the company does, call search_company.
2. Call curate_posting with:
   - verdict: shortlist or dismiss
   - score: 0-100
   - overview: 2-3 neutral sentences describing THIS posting only — what
     the team builds, core stack/systems, work model and location. Facts
     from the posting; never compare to the person (note/reasons do that).
     This replaces the raw description on the review page. Max 60 words.
   - note: 1-2 sentences saying what the row doesn't — the stack and
     systems, the team's remit, the product, the hiring bar, or the
     real dealbreaker. The title/company/location/score/chips are
     already displayed; your note is the delta. Lead with the decisive
     fact, max 200 chars, in your own scout's voice.
   - reasons: 1-3 fit facts, one per dimension. Each is an object:
       kind: "match" (supports the fit) or "gap" (works against it)
       field: one of role, domain, level, location, company
       text: a terse fragment, max 60 chars, naming the specific thing —
             e.g. "Senior distributed-systems role" (role/match),
             "Hybrid — conflicts with remote preference" (location/gap).
     Fragments the user can skim.

Scoring guide: 80+ strong fit (role + domain + level + location all align), 60-79 decent (most align, one gap), below 60 weak (major mismatch).`, cfg.LLMInput.Text())

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

	// Curate attempts, counted so "the stage failed outright" can be told
	// from ordinary per-posting noise — see the stage error after the loop.
	curateAttempts, curateFailures := 0, 0

	for i, p := range postings {
		cfg.progress("curate", fmt.Sprintf("%d/%d", i+1, len(postings)), itemLabel(p))
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Stage 4: Prefilter — deterministic Go rules.
		verdict := prefilter.Filter(p, profile, cfg.Store)
		if verdict.Action == "dismiss" {
			rec.record(cfg.Store, p, db.StatusDismissed)
			continue
		}
		if verdict.Action == "shortlist" {
			rec.record(cfg.Store, p, db.StatusShortlisted)
			continue
		}

		// Stage 5: Curate — zen LLM (when available).
		if session == nil {
			// No zen client — escalate to shortlist for manual review.
			rec.record(cfg.Store, p, db.StatusShortlisted)
			continue
		}

		curateAttempts++
		v, err := session.Curate(ctx, zen.Posting{
			URL:      p.URL,
			Markdown: p.Description,
		})
		if err != nil {
			log.Printf("autopilot: curate %s failed: %v", p.URL, err)
			rec.outcome(p.Source).Errored++
			curateFailures++
			continue
		}

		// Apply verdict — store score, overview, note, reasons in posting metadata.
		meta := map[string]string{
			"score":    fmt.Sprintf("%d", v.Score),
			"overview": v.Overview,
			"note":     v.Note,
			"reasons":  mustJSON(v.Reasons),
		}
		_ = cfg.Store.EnrichPosting(p.URL, "", meta)

		if v.Decision == zen.DecisionDismiss {
			rec.record(cfg.Store, p, db.StatusDismissed)
		} else {
			rec.record(cfg.Store, p, db.StatusShortlisted)
		}
	}

	// A curate stage that failed every posting it attempted is a broken
	// stage, not noise: without this the run would claim a dry verdict for a
	// cycle that judged nothing. A partial failure stays a count under the
	// verdict — which is what keeps one flaky posting from reading as a
	// coverage gap and resetting the user's quiet streak.
	if curateAttempts > 0 && curateFailures == curateAttempts {
		rec.stageError("curate: all %d postings failed", curateAttempts)
	}
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

// RunLogged executes one cycle with a live run ledger: the RunLog row
// is inserted before the cycle starts, so the unfinished row
// (finished_at empty) is the "running now" signal surfaced by
// GET /api/autopilot while the cycle is in flight. It is updated with
// the result when the cycle ends. Falls back to append-at-end if the
// seed insert fails — a missing in-flight signal beats a lost result.
func RunLogged(ctx context.Context, cfg CycleConfig) db.RunLog {
	entry := db.RunLog{StartedAt: time.Now().UTC().Format(time.RFC3339)}
	id, err := cfg.Store.AddRunLog(entry)
	if err != nil {
		log.Printf("autopilot: failed to open run log: %v", err)
		return Run(ctx, cfg)
	}
	// Live-sync (WP-144): the unfinished row is the "running now" signal;
	// a "runs" event flips the chip immediately.
	_ = cfg.Store.AddChangeEvent("runs")
	// Live progress (migration 00021): the cycle reports what it is doing
	// right now and the run row carries it to /api/autopilot, which already
	// returns the full RunLog. Best effort — a failed progress write costs
	// the page one stale line, never a result. The pipelined cycle calls
	// this from two goroutines (sweep and consumer), so the write is
	// serialized here.
	var progressMu sync.Mutex
	cfg.Progress = func(stage, of, label string) {
		progressMu.Lock()
		defer progressMu.Unlock()
		_ = cfg.Store.SetRunProgress(id, stage, of, label)
	}
	entry = Run(ctx, cfg)
	if err := cfg.Store.UpdateRunLog(id, entry); err != nil {
		log.Printf("autopilot: failed to close run log: %v", err)
	}
	// ...and the matches/queue moved during the cycle.
	_ = cfg.Store.AddChangeEvent("matches")
	_ = cfg.Store.AddChangeEvent("runs")
	entry.ID = id
	return entry
}

// --- helpers ---
