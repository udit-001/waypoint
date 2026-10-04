package autopilot

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
)

// WP-178: every run ends with a verdict, and the counts are the evidence
// stored under it. These tests pin the four verdicts, the per-source
// record, and the skipped fact.

// failingScraper returns an error from Search — a source that never ran.
type failingScraper struct{ name string }

func (f failingScraper) Name() string         { return f.name }
func (f failingScraper) Source() string       { return f.name }
func (f failingScraper) Categories() []string { return []string{"test"} }
func (f failingScraper) Search(context.Context, scraper.SearchOpts) ([]scraper.Result, error) {
	return nil, errors.New("portal down")
}

// outcomeFor finds a source's row in a run's per-source record.
func outcomeFor(t *testing.T, entry db.RunLog, source string) db.SourceOutcome {
	t.Helper()
	for _, row := range entry.PerSource {
		if row.Source == source {
			return row
		}
	}
	t.Fatalf("no per-source row for %q in %+v", source, entry.PerSource)
	return db.SourceOutcome{}
}

// TestVerdict_Quiet: sources ran fine and nothing new fit the brief.
func TestVerdict_Quiet(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	s := &fixtureScraper{name: "empty-scraper"}

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{s}))

	if entry.Verdict != db.VerdictQuiet {
		t.Errorf("verdict = %q, want quiet", entry.Verdict)
	}
	if !entry.Verdict.IsDry() {
		t.Error("quiet must count as dry")
	}
	if len(entry.StageErrors) != 0 {
		t.Errorf("stageErrors = %v, want none for a clean run", entry.StageErrors)
	}
	row := outcomeFor(t, entry, "empty-scraper")
	if row.Scraped != 0 || row.New != 0 || row.Skipped {
		t.Errorf("row = %+v, want a swept, empty source", row)
	}
}

// TestVerdict_NothingSurvived: postings were found, none survived curation.
func TestVerdict_NothingSurvived(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	_ = f.UpsertProfile(map[string]any{"avoid_companies": `["evil corp"]`})
	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Engineer", Company: "Evil Corp"},
		},
	}

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{s}))

	if entry.Verdict != db.VerdictNothingSurvived {
		t.Errorf("verdict = %q, want nothing-survived", entry.Verdict)
	}
	if !entry.Verdict.IsDry() {
		t.Error("nothing-survived must count as dry")
	}
	if row := outcomeFor(t, entry, "test-scraper"); row.Dismissed != 1 {
		t.Errorf("row = %+v, want 1 dismissed", row)
	}
}

// TestVerdict_WaitingOnYou: shortlists exist and are unreviewed.
func TestVerdict_WaitingOnYou(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	_ = f.UpsertProfile(map[string]any{"companies": `["google"]`})
	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "SWE", Company: "Google"},
		},
	}

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{s}))

	if entry.Verdict != db.VerdictWaitingOnYou {
		t.Errorf("verdict = %q, want waiting-on-you", entry.Verdict)
	}
	if entry.Verdict.IsDry() {
		t.Error("waiting-on-you must never count as dry")
	}
	if row := outcomeFor(t, entry, "test-scraper"); row.Shortlisted != 1 {
		t.Errorf("row = %+v, want 1 shortlisted", row)
	}
}

// TestVerdict_WaitingOnYouFromQueue: the queue is the answer, not this
// cycle's finds — a run that discovers nothing still says waiting-on-you
// while postings sit unreviewed.
func TestVerdict_WaitingOnYouFromQueue(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	_ = f.AddPostings([]scraper.Result{{URL: "https://old/1", Title: "Old"}})
	if err := f.SetPostingStatus("https://old/1", db.StatusShortlisted); err != nil {
		t.Fatalf("SetPostingStatus: %v", err)
	}
	s := &fixtureScraper{name: "empty-scraper"}

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{s}))

	if entry.Verdict != db.VerdictWaitingOnYou {
		t.Errorf("verdict = %q, want waiting-on-you while the queue is unreviewed", entry.Verdict)
	}
}

// TestVerdict_Degraded_SweepFailure: a source failed, so the run's
// coverage is partial and the verdict says so.
func TestVerdict_Degraded_SweepFailure(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	s := failingScraper{name: "broken-portal"}

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{s}))

	if entry.Verdict != db.VerdictDegraded {
		t.Errorf("verdict = %q, want degraded", entry.Verdict)
	}
	if entry.Verdict.IsDry() {
		t.Error("degraded must never count as dry")
	}
	if len(entry.StageErrors) != 1 || !strings.Contains(entry.StageErrors[0], "sweep broken-portal") {
		t.Errorf("stageErrors = %v, want a stage-prefixed sweep failure", entry.StageErrors)
	}
	if row := outcomeFor(t, entry, "broken-portal"); row.Errored != 1 {
		t.Errorf("row = %+v, want the failure attributed to the source", row)
	}
}

// TestVerdict_Degraded_Interrupted: an interrupted cycle has partial
// coverage, so it is degraded — never a claim that nothing showed up.
func TestVerdict_Degraded_Interrupted(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	entry := Run(ctx, newCycle(f, []scraper.Scraper{&fixtureScraper{name: "s"}}))

	if entry.Verdict != db.VerdictDegraded {
		t.Errorf("verdict = %q, want degraded for an interrupted cycle", entry.Verdict)
	}
	if len(entry.StageErrors) == 0 || !strings.Contains(strings.Join(entry.StageErrors, "\n"), "interrupted") {
		t.Errorf("stageErrors = %v, want the interruption recorded", entry.StageErrors)
	}
}

// TestRunRecord_PerSourceAttribution: each outcome lands on the source
// that produced the posting.
func TestRunRecord_PerSourceAttribution(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	_ = f.UpsertProfile(map[string]any{
		"avoid_companies": `["evil corp"]`,
		"companies":       `["google"]`,
	})
	dismisser := &fixtureScraper{
		name: "portal-a",
		results: []scraper.Result{
			{URL: "https://a/1", Title: "Engineer", Company: "Evil Corp"},
		},
	}
	shortlister := &fixtureScraper{
		name: "portal-b",
		results: []scraper.Result{
			{URL: "https://b/1", Title: "SWE", Company: "Google"},
			{URL: "https://b/2", Title: "SWE II", Company: "Google"},
		},
	}

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{dismisser, shortlister}))

	if len(entry.PerSource) != 2 {
		t.Fatalf("perSource = %+v, want one row per swept source", entry.PerSource)
	}
	// Rows are materialized in source-id order — two renderings of one
	// log must agree.
	if entry.PerSource[0].Source != "portal-a" || entry.PerSource[1].Source != "portal-b" {
		t.Errorf("perSource order = %q,%q, want portal-a,portal-b",
			entry.PerSource[0].Source, entry.PerSource[1].Source)
	}

	a := outcomeFor(t, entry, "portal-a")
	if a.Scraped != 1 || a.New != 1 || a.Dismissed != 1 || a.Shortlisted != 0 {
		t.Errorf("portal-a row = %+v, want scraped 1 / new 1 / dismissed 1", a)
	}
	b := outcomeFor(t, entry, "portal-b")
	if b.Scraped != 2 || b.New != 2 || b.Shortlisted != 2 || b.Dismissed != 0 {
		t.Errorf("portal-b row = %+v, want scraped 2 / new 2 / shortlisted 2", b)
	}
}

// TestRunRecord_BacklogKeepsItsSource: a posting swept in an earlier cycle
// is still attributed to its source when the backlog curates it.
func TestRunRecord_BacklogKeepsItsSource(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	_ = f.UpsertProfile(map[string]any{"avoid_companies": `["evil corp"]`})
	// A previous cycle swept this posting and left it uncurated.
	if err := f.AddPostings([]scraper.Result{
		{URL: "https://old/1", Title: "Engineer", Company: "Evil Corp", Source: "portal-a"},
	}); err != nil {
		t.Fatalf("AddPostings: %v", err)
	}

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{}))

	a := outcomeFor(t, entry, "portal-a")
	if a.Scraped != 0 || a.New != 0 || a.Dismissed != 1 {
		t.Errorf("backlog row = %+v, want the dismissal attributed to portal-a", a)
	}
	if entry.Verdict != db.VerdictNothingSurvived {
		t.Errorf("verdict = %q, want nothing-survived", entry.Verdict)
	}
}

// TestRunRecord_DiscoverySuccessIsNotAStageError: discovery running is
// information, not a failure — it must not paint a clean run degraded.
func TestRunRecord_DiscoverySuccessIsNotAStageError(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()

	entry := Run(context.Background(), newCycle(f, []scraper.Scraper{}))

	if len(entry.StageErrors) != 0 {
		t.Errorf("stageErrors = %v, want none", entry.StageErrors)
	}
	if entry.Verdict != db.VerdictQuiet {
		t.Errorf("verdict = %q, want quiet", entry.Verdict)
	}
}

// TestVerdictFor_Precedence: a stage error outranks a non-empty queue,
// which outranks nothing-survived, which outranks quiet.
func TestVerdictFor_Precedence(t *testing.T) {
	cases := []struct {
		name   string
		entry  db.RunLog
		queued int
		want   db.Verdict
	}{
		{"clean and empty", db.RunLog{}, 0, db.VerdictQuiet},
		{"postings dismissed", db.RunLog{PostingsDismissed: 3}, 0, db.VerdictNothingSurvived},
		{"queue holds work", db.RunLog{PostingsDismissed: 3}, 2, db.VerdictWaitingOnYou},
		{"stage error", db.RunLog{StageErrors: []string{"sweep x: boom"}}, 5, db.VerdictDegraded},
		{"curate errors", db.RunLog{PostingsErrored: 1}, 5, db.VerdictDegraded},
	}
	for _, tc := range cases {
		if got := verdictFor(tc.entry, tc.queued); got != tc.want {
			t.Errorf("%s: verdict = %q, want %q", tc.name, got, tc.want)
		}
	}
}
