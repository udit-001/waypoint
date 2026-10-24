package autopilot

import (
	"context"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
)

// quietSleeper stubs the sweep's politeness sleep so progress tests run
// at full speed. Restored on cleanup.
func quietSleeper(t *testing.T) {
	t.Helper()
	origSleep, origAfter := sleep, after
	sleep = func(time.Duration) {}
	after = func(time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}
	t.Cleanup(func() { sleep, after = origSleep, origAfter })
}

// TestCycle_ProgressRecordsStageFacts: the progress seam carries the
// cycle's live facts in order — the fixed stage ids, a k/n counter that
// counts only what actually runs, and the per-item name (source display
// name for the sweep, company for read/curate). The stages state facts;
// the seam's caller decides where they land.
func TestCycle_ProgressRecordsStageFacts(t *testing.T) {
	stubDiscovery(t)
	quietSleeper(t)

	f := db.NewFakeStore()
	s := &fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Engineer", Company: "Acme"},
			{URL: "https://example.com/b", Title: "Analyst", Company: "Beta"},
		},
	}
	cfg := newCycle(f, []scraper.Scraper{s})

	var facts []string
	cfg.Progress = func(stage, of, label string) {
		facts = append(facts, stage+" "+of+" "+label)
	}
	Run(context.Background(), cfg)

	want := []string{
		"discover  ",
		"select  ",
		"sweep 1/1 test-scraper",
		"read 1/2 Acme",
		"read 2/2 Beta",
		"curate 1/2 Acme",
		"curate 2/2 Beta",
	}
	if len(facts) != len(want) {
		t.Fatalf("got %d facts, want %d: %v", len(facts), len(want), facts)
	}
	for i, w := range want {
		if facts[i] != w {
			t.Errorf("fact %d = %q, want %q", i, facts[i], w)
		}
	}
}

// blockingScraper parks its Search until released, so the test can read
// the run row mid-cycle — the state /api/autopilot reports while a real
// cycle runs.
type blockingScraper struct {
	name    string
	release chan struct{}
	entered chan struct{}
}

func (b *blockingScraper) Name() string   { return b.name }
func (b *blockingScraper) Source() string { return b.name }
func (b *blockingScraper) Categories() []string {
	return []string{"test", "aggregator"}
}
func (b *blockingScraper) Search(ctx context.Context, _ scraper.SearchOpts) ([]scraper.Result, error) {
	close(b.entered)
	<-b.release
	return nil, nil
}

// TestRunLogged_ProgressWiredToRunRow: RunLogged adapts the cycle's
// progress callback to the run row, so the facts land where /api/autopilot
// reads them — and closing the run clears them again (a finished row is
// the run record, never a progress display).
func TestRunLogged_ProgressWiredToRunRow(t *testing.T) {
	stubDiscovery(t)
	quietSleeper(t)

	f := db.NewFakeStore()
	s := &blockingScraper{
		name:    "slow",
		release: make(chan struct{}),
		entered: make(chan struct{}),
	}
	cfg := newCycle(f, []scraper.Scraper{s})

	done := make(chan db.RunLog, 1)
	go func() { done <- RunLogged(context.Background(), cfg) }()

	// Search is blocked, so no progress write is in flight: reading the
	// row here is race-free. The sweep announced itself before entering.
	<-s.entered
	row := f.RunLogs[len(f.RunLogs)-1]
	if row.Stage != "sweep" || row.StageOf != "1/1" || row.StageLabel != "slow" {
		t.Fatalf("mid-cycle row = %q %q %q, want sweep 1/1 slow",
			row.Stage, row.StageOf, row.StageLabel)
	}

	close(s.release)
	entry := <-done

	if entry.FinishedAt == "" {
		t.Fatal("run did not close")
	}
	if entry.Stage != "" || entry.StageOf != "" || entry.StageLabel != "" {
		t.Fatalf("closed row still carries progress facts: %q %q %q",
			entry.Stage, entry.StageOf, entry.StageLabel)
	}
}

// TestCycle_NilProgressSeam: a cycle without a progress callback (a run
// that lost its row, or a direct Run caller) no-ops — the seam never
// becomes a requirement.
func TestCycle_NilProgressSeam(t *testing.T) {
	stubDiscovery(t)
	quietSleeper(t)

	f := db.NewFakeStore()
	s := &fixtureScraper{
		name:    "test-scraper",
		results: []scraper.Result{{URL: "https://example.com/a", Title: "Engineer", Company: "Acme"}},
	}
	cfg := newCycle(f, []scraper.Scraper{s})
	if cfg.Progress != nil {
		t.Fatal("newCycle must not set Progress")
	}
	entry := Run(context.Background(), cfg)
	if entry.Verdict == "" {
		t.Fatal("cycle without a progress seam produced no verdict")
	}
}
