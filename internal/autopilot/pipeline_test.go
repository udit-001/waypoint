package autopilot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
	"github.com/udit-001/waypoint/internal/zen"
)

// --- fixtures ---

// gatedScraper parks in Search until released or the context is
// cancelled, so a test controls exactly when a source's fetch returns.
type gatedScraper struct {
	name      string
	results   []scraper.Result
	entered   chan struct{}
	release   chan struct{}
	sawCancel bool // set when Search observed ctx cancellation

	enteredOnce sync.Once
}

func (g *gatedScraper) Name() string   { return g.name }
func (g *gatedScraper) Source() string { return g.name }
func (g *gatedScraper) Categories() []string {
	return []string{"test", "aggregator"}
}
func (g *gatedScraper) Search(ctx context.Context, _ scraper.SearchOpts) ([]scraper.Result, error) {
	g.enteredOnce.Do(func() { close(g.entered) })
	// Watchdog: a broken stop signal must fail assertions, not hang the run.
	watchdog := time.After(5 * time.Second)
	select {
	case <-ctx.Done():
		g.sawCancel = true
		return nil, ctx.Err()
	case <-g.release:
		return g.results, nil
	case <-watchdog:
		return nil, errors.New("test watchdog: fetch was never released or cancelled")
	}
}

// curateSignal wraps a Progress callback and reports, on ch, the first
// curate-stage fact — the observable moment curation work begins.
func curateSignal() (func(stage, of, label string), <-chan struct{}) {
	ch := make(chan struct{}, 8)
	return func(stage, of, label string) {
		if stage == "curate" {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}, ch
}

// curateSSEWithScore renders one curate turn whose verdict carries the
// given score — the wire shape of verdict_test.go's curateSSE, scored.
func curateSSEWithScore(score int) string {
	args := fmt.Sprintf(`{\"verdict\":\"shortlist\",\"score\":%d,\"note\":\"n\",\"overview\":\"o\",\"reasons\":[{\"kind\":\"match\",\"field\":\"role\",\"text\":\"r\"}]}`, score)
	return `{"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"curate_posting","arguments":"` + args + `"}}]},"finish_reason":null}]}` + "\n" +
		`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`
}

// writeSSEPayload serves a payload in the wire's event format.
func writeSSEPayload(w http.ResponseWriter, payload string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, line := range strings.Split(payload, "\n") {
		if line != "" {
			_, _ = w.Write([]byte("data: " + line + "\n\n"))
		}
	}
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
}

// zenStubByURL serves a zen upstream whose curate verdict's score depends
// on the request body — the posting URL rides in the user turn.
func zenStubByURL(t *testing.T, scoreFor func(body []byte) int) *zen.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		writeSSEPayload(w, curateSSEWithScore(scoreFor(body)))
	}))
	t.Cleanup(srv.Close)
	return zen.New(zen.Config{BaseURL: srv.URL, APIKey: "k", Model: "m"})
}

// --- tests ---

// TestPipeline_OverlapsSweepAndCuration: the consumer's curation of an
// earlier batch runs while the producer is still fetching a later source.
// The slow source stays parked until curation has begun — a sequential
// cycle would deadlock against this release order, so reaching the end
// proves the overlap.
func TestPipeline_OverlapsSweepAndCuration(t *testing.T) {
	stubDiscovery(t)
	quietSleeper(t)
	f := db.NewFakeStore()
	fast := &fixtureScraper{
		name:    "fast",
		results: []scraper.Result{{URL: "https://example.com/fast", Title: "Fast"}},
	}
	slow := &gatedScraper{
		name:    "slow",
		results: []scraper.Result{{URL: "https://example.com/slow", Title: "Slow"}},
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	progress, curateBegan := curateSignal()

	cfg := newCycle(f, []scraper.Scraper{fast, slow})
	cfg.Progress = progress

	run := make(chan db.RunLog, 1)
	go func() { run <- Run(context.Background(), cfg) }()

	select {
	case <-curateBegan:
	case <-time.After(3 * time.Second):
		t.Fatal("curation of the first batch did not begin before the slow source's fetch returned")
	}
	close(slow.release)

	select {
	case entry := <-run:
		if entry.PostingsShortlisted != 2 {
			t.Errorf("postingsShortlisted = %d, want 2 (both batches curated)", entry.PostingsShortlisted)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run did not close after the slow source returned")
	}
}

// TestPipeline_NoLossOrDuplication: every posting — swept or backlog —
// escalates exactly once. A URL returned by two sources is deduped by the
// ledger (earlier batches are added before later sources scrape), and the
// backlog never re-escalates a URL this cycle already swept.
func TestPipeline_NoLossOrDuplication(t *testing.T) {
	stubDiscovery(t)
	quietSleeper(t)
	f := db.NewFakeStore()
	seedBacklog(t, f, 2) // old1..old2 land as p00, p01

	s1 := &fixtureScraper{
		name: "s1",
		results: []scraper.Result{
			{URL: "https://example.com/a1", Title: "A1"},
			{URL: "https://example.com/a2", Title: "A2"},
		},
	}
	// s2 re-returns s1's a1: it must not escalate twice.
	s2 := &fixtureScraper{
		name: "s2",
		results: []scraper.Result{
			{URL: "https://example.com/a1", Title: "A1 dup"},
			{URL: "https://example.com/b1", Title: "B1"},
		},
	}

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{s1, s2}))

	// Exactly five unique postings escalated: a1, a2, b1 and the two
	// backlog rows. A re-escalated URL would inflate the count without
	// adding a ledger row.
	if entry.PostingsShortlisted != 5 {
		t.Errorf("postingsShortlisted = %d, want 5 (each posting escalated once)", entry.PostingsShortlisted)
	}
	if got := countByStatus(f, db.StatusShortlisted); got != 5 {
		t.Errorf("ledger shortlisted rows = %d, want 5", got)
	}
	if got := countByStatus(f, db.StatusNew); got != 0 {
		t.Errorf("ledger new rows = %d, want 0 (nothing dropped or stranded)", got)
	}
	for _, u := range []string{
		"https://example.com/a1", "https://example.com/a2", "https://example.com/b1",
		"https://example.com/p00", "https://example.com/p01",
	} {
		p, ok, _ := f.GetPosting(u)
		if !ok || p.Status != db.StatusShortlisted {
			t.Errorf("%s status = %q (ok=%v), want shortlisted", u, p.Status, ok)
		}
	}
}

// TestPipeline_InterruptMidCycle: cancelling the run mid-pipeline closes
// it, keeps already-scored postings scored, and aborts the producer's
// in-flight fetch — no goroutine blocks past the cancel.
func TestPipeline_InterruptMidCycle(t *testing.T) {
	stubDiscovery(t)
	quietSleeper(t)
	f := db.NewFakeStore()
	fast := &fixtureScraper{
		name:    "fast",
		results: []scraper.Result{{URL: "https://example.com/fast", Title: "Fast"}},
	}
	slow := &gatedScraper{
		name:    "slow",
		results: []scraper.Result{{URL: "https://example.com/slow", Title: "Slow"}},
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	progress, curateBegan := curateSignal()

	cfg := newCycle(f, []scraper.Scraper{fast, slow})
	cfg.Progress = progress

	ctx, cancel := context.WithCancel(context.Background())
	run := make(chan db.RunLog, 1)
	go func() { run <- Run(ctx, cfg) }()

	select {
	case <-curateBegan:
	case <-time.After(3 * time.Second):
		t.Fatal("curation of the first batch did not begin")
	}
	cancel()

	select {
	case entry := <-run:
		if entry.FinishedAt == "" {
			t.Error("interrupted run did not close")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run did not return after cancel — a goroutine is stuck")
	}
	// The producer's fetch observed the stop signal.
	<-slow.entered
	if !slow.sawCancel {
		t.Error("producer's slow fetch was not cancelled")
	}
	// Already-scored stays scored.
	p, ok, _ := f.GetPosting("https://example.com/fast")
	if !ok || p.Status != db.StatusShortlisted {
		t.Errorf("fast posting status = %q (ok=%v), want shortlisted", p.Status, ok)
	}
}

// TestPipeline_ScoredCapAcrossBatches: MaxScored bounds the cycle's total
// escalations across per-source batches, and a truncated batch keeps its
// newest postings.
func TestPipeline_ScoredCapAcrossBatches(t *testing.T) {
	stubDiscovery(t)
	quietSleeper(t)
	f := db.NewFakeStore()

	s1 := &fixtureScraper{
		name: "s1",
		results: []scraper.Result{
			{URL: "https://example.com/a1", Title: "A1"},
			{URL: "https://example.com/a2", Title: "A2"},
		},
	}
	s2 := &fixtureScraper{
		name: "s2",
		results: []scraper.Result{
			{URL: "https://example.com/n1", Title: "N1", Date: "2026-03-01"},
			{URL: "https://example.com/n2", Title: "N2", Date: "2026-02-01"},
		},
	}

	cfg := newCycle(f, []scraper.Scraper{s1, s2})
	cfg.ZenClient = zenStubByURL(t, func([]byte) int { return 80 })
	cfg.Budget = &Budget{MaxScored: 3}

	entry := Run(context.Background(), cfg)

	if entry.PostingsShortlisted != 3 {
		t.Errorf("postingsShortlisted = %d, want 3 (cycle-wide cap)", entry.PostingsShortlisted)
	}
	// Batch 2's single remaining escalation keeps the newest posting.
	st := statuses(f)
	if st["https://example.com/n1"] != db.StatusShortlisted {
		t.Errorf("n1 status = %q, want shortlisted (newest of the truncated batch)", st["https://example.com/n1"])
	}
	if st["https://example.com/n2"] != db.StatusNew {
		t.Errorf("n2 status = %q, want new (dropped by the cap)", st["https://example.com/n2"])
	}
	if got := countByStatus(f, db.StatusNew); got != 1 {
		t.Errorf("ledger new rows = %d, want 1", got)
	}
}

// TestPipeline_DeadlineStopsProducer: a wall budget spent mid-pipeline
// stops consumption, cancels the derived context (aborting the producer's
// in-flight fetch), and closes the run with a budget stage error.
func TestPipeline_DeadlineStopsProducer(t *testing.T) {
	stubDiscovery(t)
	quietSleeper(t)
	f := db.NewFakeStore()
	fast := &fixtureScraper{
		name:    "fast",
		results: []scraper.Result{{URL: "https://example.com/fast", Title: "Fast"}},
	}
	slow := &gatedScraper{
		name:    "slow",
		results: []scraper.Result{{URL: "https://example.com/slow", Title: "Slow"}},
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	progress, _ := curateSignal()

	cfg := newCycle(f, []scraper.Scraper{fast, slow})
	cfg.Progress = progress
	cfg.Budget = &Budget{Deadline: time.Now().Add(500 * time.Millisecond)}

	entry := Run(context.Background(), cfg)

	if entry.FinishedAt == "" {
		t.Fatal("run did not close after the budget expired")
	}
	joined := strings.Join(entry.StageErrors, "\n")
	if !strings.Contains(joined, "wall budget") || !strings.Contains(joined, "detail") {
		t.Errorf("stageErrors = %v, want a wall-budget error naming the detail stage", entry.StageErrors)
	}
	<-slow.entered
	if !slow.sawCancel {
		t.Error("producer's slow fetch was not aborted by the derived context")
	}
	p, ok, _ := f.GetPosting("https://example.com/fast")
	if !ok || p.Status != db.StatusShortlisted {
		t.Errorf("fast posting status = %q (ok=%v), want shortlisted", p.Status, ok)
	}
	if entry.Verdict != db.VerdictDegraded {
		t.Errorf("verdict = %q, want degraded (budget stage error)", entry.Verdict)
	}
}

// TestPipeline_DegradedCapAcrossBatches: without a zen client the
// UnscoredCapPerCycle bounds the cycle's total escalations across batches.
func TestPipeline_DegradedCapAcrossBatches(t *testing.T) {
	stubDiscovery(t)
	quietSleeper(t)
	f := db.NewFakeStore()

	var mk = func(prefix string, n int) *fixtureScraper {
		results := make([]scraper.Result, 0, n)
		for i := 0; i < n; i++ {
			results = append(results, scraper.Result{
				URL:   fmt.Sprintf("https://example.com/%s%02d", prefix, i),
				Title: fmt.Sprintf("Role %s%02d", prefix, i),
			})
		}
		return &fixtureScraper{name: prefix, results: results}
	}

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{mk("s1", 15), mk("s2", 15)}))

	if entry.PostingsShortlisted != UnscoredCapPerCycle {
		t.Errorf("postingsShortlisted = %d, want %d (unscored cap, cycle-wide)",
			entry.PostingsShortlisted, UnscoredCapPerCycle)
	}
	if got := countByStatus(f, db.StatusNew); got != 5 {
		t.Errorf("ledger new rows = %d, want 5 (above the cap)", got)
	}
}

// TestPipeline_NoticeFromMidPipelineBatches: the final notice is built
// from what the consumer escalated across all batches — count reflects
// every batch and the top pick is the highest score, though the batches
// were curated while later sources were still being swept.
func TestPipeline_NoticeFromMidPipelineBatches(t *testing.T) {
	stubDiscovery(t)
	quietSleeper(t)
	f := db.NewFakeStore()
	low := &fixtureScraper{
		name:    "s1",
		results: []scraper.Result{{URL: "https://example.com/low", Title: "Low", Company: "LoCo"}},
	}
	high := &fixtureScraper{
		name:    "s2",
		results: []scraper.Result{{URL: "https://example.com/high", Title: "High", Company: "HiCo"}},
	}
	n := &recordingNotifier{}

	cfg := newCycle(f, []scraper.Scraper{low, high})
	cfg.ZenClient = zenStubByURL(t, func(body []byte) int {
		if strings.Contains(string(body), "/high") {
			return 90
		}
		return 60
	})
	cfg.Notifier = n

	entry := Run(context.Background(), cfg)

	batches := n.batches()
	if len(batches) != 1 {
		t.Fatalf("deliveries = %d, want exactly 1", len(batches))
	}
	if len(batches[0]) != 2 {
		t.Errorf("delivered %d events, want 2 (both mid-pipeline batches)", len(batches[0]))
	}
	var top *ShortlistEvent
	for i := range batches[0] {
		if batches[0][i].Title == "High" {
			top = &batches[0][i]
		}
	}
	if top == nil || top.Company != "HiCo" || top.Score != 90 {
		t.Errorf("High/HiCo score 90 not delivered: %+v", batches[0])
	}
	if entry.Verdict != db.VerdictWaitingOnYou {
		t.Errorf("verdict = %q, want waiting-on-you", entry.Verdict)
	}
}
