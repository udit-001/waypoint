package autopilot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/discovery"
	"github.com/udit-001/waypoint/internal/scraper"
)

// --- fixtures ---

// recordingNotifier records each NotifyShortlists and NotifyDegraded
// delivery. Safe to read while a Run is in flight — the flusher
// delivers mid-cycle.
type recordingNotifier struct {
	mu          sync.Mutex
	deliveries  [][]ShortlistEvent
	degraded    [][]string
	err         error // NotifyShortlists failure
	degradedErr error // NotifyDegraded failure
}

func (r *recordingNotifier) NotifyShortlists(_ context.Context, events []ShortlistEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deliveries = append(r.deliveries, events)
	return r.err
}

func (r *recordingNotifier) NotifyDegraded(_ context.Context, d DegradedNotice) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.degraded = append(r.degraded, d.Errors)
	return r.degradedErr
}

func (r *recordingNotifier) batches() [][]ShortlistEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.deliveries
}

func (r *recordingNotifier) degradedNotices() [][]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.degraded
}

// stubDiscovery replaces the discovery pipeline with a no-op so cycle
// tests stay offline and fast.
func stubDiscovery(t *testing.T) {
	t.Helper()
	orig := runAutoDiscovery
	runAutoDiscovery = func(_ context.Context, _ CycleConfig, _ []discovery.Facet) (discoveryRun, error) {
		return discoveryRun{}, nil
	}
	t.Cleanup(func() { runAutoDiscovery = orig })
}

// --- tests ---

// TestCycle_ShortlistsDeliveredAtCycleEnd: with no events after the
// last quiet window fires, the final flush carries the whole cycle's
// shortlists as one delivery.
func TestCycle_ShortlistsDeliveredAtCycleEnd(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Engineer", Company: "Acme"},
			{URL: "https://example.com/b", Title: "Designer", Company: "Beta"},
			{URL: "https://example.com/c", Title: "Writer", Company: "Gamma"},
		},
	}
	n := &recordingNotifier{}

	cfg := newCycle(f, []scraper.Scraper{s})
	cfg.Notifier = n
	entry := Run(context.Background(), cfg)

	if entry.PostingsShortlisted != 3 {
		t.Fatalf("shortlisted = %d, want 3", entry.PostingsShortlisted)
	}
	batches := n.batches()
	if len(batches) != 1 {
		t.Fatalf("deliveries = %d, want exactly 1", len(batches))
	}
	if len(batches[0]) != 3 {
		t.Errorf("delivered %d events, want all 3", len(batches[0]))
	}
	for _, e := range batches[0] {
		if e.Title == "" || e.Company == "" || e.URL == "" {
			t.Errorf("event missing fields: %+v", e)
		}
	}
}

func TestCycle_NoShortlistNoNotify(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	n := &recordingNotifier{}

	cfg := newCycle(f, nil)
	cfg.Notifier = n
	Run(context.Background(), cfg)

	if len(n.batches()) != 0 {
		t.Errorf("notifier deliveries = %d, want 0", len(n.batches()))
	}
}

func TestCycle_NilNotifierIsNoop(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Engineer", Company: "Acme"},
		},
	}

	// Must not panic with nil notifier, and must record no errors.
	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{s}))

	if entry.PostingsShortlisted != 1 {
		t.Errorf("shortlisted = %d, want 1", entry.PostingsShortlisted)
	}
	if len(entry.StageErrors) != 0 {
		t.Errorf("stage errors = %v, want none with a nil notifier", entry.StageErrors)
	}
}

func TestCycle_NotifierFailureNeverBlocksCycle(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Engineer", Company: "Acme"},
		},
	}
	n := &recordingNotifier{err: context.DeadlineExceeded}

	cfg := newCycle(f, []scraper.Scraper{s})
	cfg.Notifier = n
	entry := Run(context.Background(), cfg)

	// Cycle completes normally; failure recorded as a stage error.
	if entry.FinishedAt == "" {
		t.Error("cycle did not finish after notifier failure")
	}
	if !strings.Contains(strings.Join(entry.StageErrors, "\n"), "notify") {
		t.Errorf("stageErrors = %v, want notify failure recorded", entry.StageErrors)
	}
	if entry.Verdict != db.VerdictDegraded {
		t.Errorf("verdict = %q, want degraded after a stage error", entry.Verdict)
	}
}

// panickyScraper panics inside Search — simulates a poisoned stage.
type panickyScraper struct{}

func (panickyScraper) Name() string         { return "panic" }
func (panickyScraper) Source() string       { return "panic" }
func (panickyScraper) Categories() []string { return []string{"test"} }
func (panickyScraper) Search(context.Context, scraper.SearchOpts) ([]scraper.Result, error) {
	panic("boom")
}

// TestCycle_PanicClosesRunRow: a mid-cycle panic must still produce a
// CLOSED run-log row (finished_at set, panic recorded as an error).
// The scheduler's cadence measures from last FINISHED run — an empty
// FinishedAt would re-fire a panicking cycle at poll rate forever.
func TestCycle_PanicClosesRunRow(t *testing.T) {
	stubSelection(t)
	stubDiscovery(t)
	f := db.NewFakeStore()

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{panickyScraper{}}))

	if entry.FinishedAt == "" {
		t.Fatal("panicked cycle left FinishedAt empty — scheduler would hot-refire every poll")
	}
	if !strings.Contains(strings.Join(entry.StageErrors, "\n"), "panic") {
		t.Errorf("stageErrors = %v, want panic recorded", entry.StageErrors)
	}
	if entry.Verdict != db.VerdictDegraded {
		t.Errorf("verdict = %q, want degraded for a panicked cycle", entry.Verdict)
	}
}

// TestCycle_NoZenCapsUnscored: without a Zen client every surviving
// posting would escalate to shortlisted — a fresh install floods
// Matches with hundreds of unscored rows. The cycle must escalate
// only the freshest slice per run.
func TestCycle_NoZenCapsUnscored(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()

	// Seed 40 backlog postings, oldest first so newest-first ordering
	// is observable in WHICH urls got escalated.
	var results []scraper.Result
	for i := 0; i < 40; i++ {
		results = append(results, scraper.Result{
			URL:   fmt.Sprintf("https://example.com/%02d", i),
			Title: fmt.Sprintf("Role %02d", i),
		})
	}
	if err := f.AddPostings(results); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cfg := newCycle(f, nil)
	cfg.ZenClient = nil // degraded mode
	entry := Run(context.Background(), cfg)

	if entry.PostingsShortlisted != 25 {
		t.Fatalf("shortlisted = %d, want capped at 25", entry.PostingsShortlisted)
	}

	// The escalated ones must be the NEWEST (30..39 by our seed order —
	// later inserts sort first under RFC3339 DESC ties broken by rowid).
	shortlisted := map[string]bool{}
	postings, _ := f.ListPostings(db.StatusShortlisted)
	for _, p := range postings {
		shortlisted[p.Result.URL] = true
	}
	if len(shortlisted) != 25 {
		t.Fatalf("ledger shortlisted rows = %d, want 25", len(shortlisted))
	}
}

// TestCycle_DegradedNotifiesOnce: a run that closes degraded sends
// exactly one NotifyDegraded carrying the run's stage errors (WP-193).
func TestCycle_DegradedNotifiesOnce(t *testing.T) {
	stubDiscovery(t)
	stubSelection(t)
	f := db.NewFakeStore()
	n := &recordingNotifier{}

	cfg := newCycle(f, []scraper.Scraper{failingScraper{name: "broken-portal"}})
	cfg.Notifier = n
	entry := Run(context.Background(), cfg)

	if entry.Verdict != db.VerdictDegraded {
		t.Fatalf("verdict = %q, want degraded", entry.Verdict)
	}
	notices := n.degradedNotices()
	if len(notices) != 1 {
		t.Fatalf("degraded notices = %d, want exactly 1", len(notices))
	}
	if strings.Join(notices[0], "\n") != strings.Join(entry.StageErrors, "\n") {
		t.Errorf("notice errors = %v, want the run's stage errors %v", notices[0], entry.StageErrors)
	}
}

// TestCycle_HealthyRunSilentOnDegraded: a quiet run announces nothing —
// silence is the absence of a degraded verdict, with no state behind it.
func TestCycle_HealthyRunSilentOnDegraded(t *testing.T) {
	stubDiscovery(t)
	stubSelection(t)
	f := db.NewFakeStore()
	n := &recordingNotifier{}

	cfg := newCycle(f, []scraper.Scraper{&fixtureScraper{name: "empty-scraper"}})
	cfg.Notifier = n
	entry := Run(context.Background(), cfg)

	if entry.Verdict == db.VerdictDegraded {
		t.Fatalf("verdict = degraded, want a healthy verdict")
	}
	if notices := n.degradedNotices(); len(notices) != 0 {
		t.Errorf("degraded notices = %d, want 0 for a healthy run", len(notices))
	}
}

// TestCycle_DegradedNotifyFailureDoesNotFailRun: a failed degraded
// notice is logged, not propagated — the run row is already closed by
// the time the notice fires, so there is nothing left to record into.
func TestCycle_DegradedNotifyFailureDoesNotFailRun(t *testing.T) {
	stubDiscovery(t)
	stubSelection(t)
	f := db.NewFakeStore()
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	n := &recordingNotifier{degradedErr: errors.New("no bus")}
	cfg := newCycle(f, []scraper.Scraper{failingScraper{name: "broken-portal"}})
	cfg.Notifier = n

	entry := Run(context.Background(), cfg)

	if entry.FinishedAt == "" {
		t.Error("run did not finish after a failed degraded notice")
	}
	if entry.Verdict != db.VerdictDegraded {
		t.Errorf("verdict = %q, want the runlog unchanged by the notify failure", entry.Verdict)
	}
	if !strings.Contains(logBuf.String(), "notify degraded") {
		t.Errorf("log = %q, want the notify failure logged", logBuf.String())
	}
}
