package autopilot

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
)

// waitFor delivers on ch with a timeout, so a broken stream fails the
// test instead of hanging it.
func waitFor(ch <-chan struct{}, t *testing.T) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a notification delivery")
	}
}

// waitForBatches resolves when the notifier has recorded n deliveries.
func waitForBatches(n *recordingNotifier, want int, t *testing.T) [][]ShortlistEvent {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		if batches := n.batches(); len(batches) >= want {
			return batches
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %d deliveries, have %d", want, len(n.batches()))
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// TestStream_CoalescesBurst: events inside one quiet window deliver as
// exactly one batch containing all of them.
func TestStream_CoalescesBurst(t *testing.T) {
	n := &recordingNotifier{}
	s := newShortlistStream(200 * time.Millisecond)
	go s.run(context.Background(), n)

	for i := 0; i < 10; i++ {
		s.Emit(ShortlistEvent{URL: fmtURL(i), Title: "Role", Company: "Co"})
	}

	batches := waitForBatches(n, 1, t)
	if len(batches) != 1 {
		t.Fatalf("deliveries = %d, want 1", len(batches))
	}
	if len(batches[0]) != 10 {
		t.Errorf("batch size = %d, want 10", len(batches[0]))
	}

	// The window already fired, the buffer is empty: closing must not
	// re-deliver anything.
	close(s.events)
	<-s.done
	if got := len(n.batches()); got != 1 {
		t.Errorf("deliveries after close = %d, want still 1", got)
	}
}

// TestStream_WindowBoundary: events spaced across a quiet window
// deliver as separate batches, none lost, none duplicated.
func TestStream_WindowBoundary(t *testing.T) {
	n := &recordingNotifier{}
	s := newShortlistStream(150 * time.Millisecond)
	go s.run(context.Background(), n)

	s.Emit(ShortlistEvent{URL: "https://example.com/a", Title: "A"})
	s.Emit(ShortlistEvent{URL: "https://example.com/b", Title: "B"})
	batches := waitForBatches(n, 1, t)

	s.Emit(ShortlistEvent{URL: "https://example.com/c", Title: "C"})
	s.Emit(ShortlistEvent{URL: "https://example.com/d", Title: "D"})
	batches = waitForBatches(n, 2, t)

	close(s.events)
	<-s.done

	if len(batches) != 2 {
		t.Fatalf("deliveries = %d, want 2", len(batches))
	}
	if len(batches[0]) != 2 || len(batches[1]) != 2 {
		t.Errorf("batch sizes = %d, %d; want 2, 2", len(batches[0]), len(batches[1]))
	}
	seen := map[string]int{}
	for _, batch := range batches {
		for _, e := range batch {
			seen[e.URL]++
		}
	}
	for url, count := range seen {
		if count != 1 {
			t.Errorf("event %s delivered %d times, want exactly 1", url, count)
		}
	}
}

// TestStream_FinalFlushOnClose: events still buffered when the cycle
// ends deliver exactly once — the final flush, not the window.
func TestStream_FinalFlushOnClose(t *testing.T) {
	n := &recordingNotifier{}
	s := newShortlistStream(10 * time.Second) // window never fires in this test
	go s.run(context.Background(), n)

	for i := 0; i < 5; i++ {
		s.Emit(ShortlistEvent{URL: fmtURL(i), Title: "Role"})
	}
	close(s.events)
	<-s.done

	batches := n.batches()
	if len(batches) != 1 {
		t.Fatalf("deliveries = %d, want exactly 1 final flush", len(batches))
	}
	if len(batches[0]) != 5 {
		t.Errorf("final flush carried %d events, want all 5", len(batches[0]))
	}
}

// TestRun_DeliversMidCycle: a shortlist committed mid-cycle is
// announced before Run returns, not held to the end.
func TestRun_DeliversMidCycle(t *testing.T) {
	stubDiscovery(t)
	quietSleeper(t)
	f := db.NewFakeStore()
	s1 := &fixtureScraper{
		name:    "s1",
		results: []scraper.Result{{URL: "https://example.com/a", Title: "Early", Company: "EarlyCo"}},
	}
	gated := &gatedScraper{
		name:    "s2",
		results: []scraper.Result{{URL: "https://example.com/b", Title: "Late", Company: "LateCo"}},
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	n := &recordingNotifier{}

	cfg := newCycle(f, []scraper.Scraper{s1, gated})
	cfg.Notifier = n
	cfg.NotifyWindow = 50 * time.Millisecond

	done := make(chan db.RunLog, 1)
	go func() { done <- Run(context.Background(), cfg) }()

	// Wait until s2's Search is parked — the cycle is mid-flight with
	// s1's shortlist already committed — and require the notification
	// for it to arrive before we let the cycle proceed.
	<-gated.entered
	batches := waitForBatches(n, 1, t)
	if len(batches[0]) != 1 || batches[0][0].URL != "https://example.com/a" {
		t.Fatalf("mid-cycle delivery = %+v, want exactly the first batch's shortlist", batches[0])
	}

	close(gated.release)
	entry := <-done

	batches = n.batches()
	if len(batches) != 2 {
		t.Fatalf("deliveries = %d, want 2 (mid-cycle + final)", len(batches))
	}
	if len(batches[1]) != 1 || batches[1][0].URL != "https://example.com/b" {
		t.Errorf("final delivery = %+v, want the second batch's shortlist", batches[1])
	}
	if entry.PostingsShortlisted != 2 || entry.FinishedAt == "" {
		t.Errorf("entry = shortlisted %d, finished %q; want 2 and a close", entry.PostingsShortlisted, entry.FinishedAt)
	}
}

// panickyNotifier simulates a delivery mechanism that blows up.
type panickyNotifier struct{}

func (panickyNotifier) NotifyShortlists(context.Context, []ShortlistEvent) error {
	panic("boom")
}

// TestRun_FlusherPanicContained: a panicking Notifier must not take
// down the cycle — the run closes with its normal shape and the panic
// surfaces as a stage error.
func TestRun_FlusherPanicContained(t *testing.T) {
	stubDiscovery(t)
	quietSleeper(t)
	f := db.NewFakeStore()
	s := &fixtureScraper{
		name:    "test-scraper",
		results: []scraper.Result{{URL: "https://example.com/a", Title: "Engineer", Company: "Acme"}},
	}

	cfg := newCycle(f, []scraper.Scraper{s})
	cfg.Notifier = panickyNotifier{}
	entry := Run(context.Background(), cfg)

	if entry.FinishedAt == "" {
		t.Error("cycle did not finish after a flusher panic")
	}
	joined := strings.Join(entry.StageErrors, "\n")
	if !strings.Contains(joined, "notify") || !strings.Contains(joined, "boom") {
		t.Errorf("stage errors = %v, want the flusher panic recorded as a notify error", entry.StageErrors)
	}
}

func fmtURL(i int) string {
	return fmt.Sprintf("https://example.com/%02d", i)
}
