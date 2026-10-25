package autopilot

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ShortlistEvent is one posting's shortlist moment, announced as it
// lands during the cycle (WP-195) rather than summarized once at the
// end. Score is 0 on the unscored paths (prefilter verdict, degraded
// no-zen escalate).
type ShortlistEvent struct {
	URL     string
	Title   string
	Company string
	Score   int
}

// DegradedNotice announces a run that closed degraded: the stage
// errors, exactly as the runlog shows them (WP-193). A healthy run
// sends nothing — recovery is signaled by the shortlist stream, not by
// a counter-notice.
type DegradedNotice struct {
	Errors []string
}

// Notifier delivers shortlist announcements and degraded-run notices.
// It is the seam that keeps the cycle free of delivery mechanics —
// desktop notifications today, Web Push or anything else behind the
// same methods later. A nil Notifier is a valid noop; failures never
// block scoring.
type Notifier interface {
	NotifyShortlists(ctx context.Context, events []ShortlistEvent) error
	NotifyDegraded(ctx context.Context, d DegradedNotice) error
}

// DefaultNotifyQuietWindow coalesces a burst of shortlists into one
// notification. Provisional: an actual user preference once someone
// complains about the value.
const DefaultNotifyQuietWindow = 30 * time.Second

// shortlistStream carries shortlist announcements from curation to a
// flusher goroutine that delivers one notification per quiet window.
// Events flow one way; the only shared state is the mutex-guarded
// error list, drained by Run after the flusher's done channel closes —
// never the runRecord, which belongs to Run's goroutine.
type shortlistStream struct {
	events chan ShortlistEvent
	window time.Duration
	done   chan struct{} // closed by the flusher when it exits

	mu   sync.Mutex
	errs []error
}

func newShortlistStream(window time.Duration) *shortlistStream {
	if window <= 0 {
		window = DefaultNotifyQuietWindow
	}
	return &shortlistStream{
		events: make(chan ShortlistEvent, 256),
		window: window,
		done:   make(chan struct{}),
	}
}

// Emit hands one shortlist to the flusher. Non-blocking on purpose:
// the ledger queue is the source of truth, so on overflow the event is
// dropped with a log line rather than ever stalling curation. Nil
// receiver is the noop a run without a Notifier gets.
func (s *shortlistStream) Emit(e ShortlistEvent) {
	if s == nil {
		return
	}
	select {
	case s.events <- e:
	default:
		log.Printf("autopilot: notify: buffer full, dropping shortlist %s", e.URL)
	}
}

// drainErrs returns the flusher's delivery failures, once, after done
// has closed. Run records them as stage errors so the run row's
// "notify failed" surface keeps its meaning.
func (s *shortlistStream) drainErrs() []error {
	s.mu.Lock()
	defer s.mu.Unlock()
	errs := s.errs
	s.errs = nil
	return errs
}

// flush delivers one batch. Errors land in the stream, not the run
// record — see drainErrs.
func (s *shortlistStream) flush(ctx context.Context, n Notifier, buf []ShortlistEvent) {
	if len(buf) == 0 {
		return
	}
	// Copy: the notifier may retain its argument past the call, and the
	// flusher reuses buf's backing array for the next window.
	batch := make([]ShortlistEvent, len(buf))
	copy(batch, buf)
	if err := n.NotifyShortlists(ctx, batch); err != nil {
		s.mu.Lock()
		s.errs = append(s.errs, err)
		s.mu.Unlock()
	}
}

// run is the flusher: accumulate events and deliver one batch per
// quiet window. The timer starts at the first event after a flush; on
// close or ctx cancel the remaining buffer goes out as the final flush
// (best effort — the Desktop notifier ignores ctx), replacing the old
// end-of-cycle nudge. A panicking Notifier must not take down the
// cycle: it is recovered, logged, and recorded as a stage error via
// drainErrs; buffered events past the panic are lost.
func (s *shortlistStream) run(ctx context.Context, n Notifier) {
	defer close(s.done)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("autopilot: notify flusher panicked: %v", r)
			s.mu.Lock()
			s.errs = append(s.errs, fmt.Errorf("flusher panic: %v", r))
			s.mu.Unlock()
		}
	}()

	var buf []ShortlistEvent
	timer := time.NewTimer(s.window)
	defer timer.Stop()
	if !timer.Stop() {
		<-timer.C
	}
	ticking := false
	for {
		select {
		case e, ok := <-s.events:
			if !ok {
				s.flush(ctx, n, buf)
				return
			}
			buf = append(buf, e)
			if !ticking {
				timer.Reset(s.window)
				ticking = true
			}
		case <-timer.C:
			ticking = false
			s.flush(ctx, n, buf)
			buf = buf[:0]
		case <-ctx.Done():
			// The cycle is stopping. Run still closes the channel after
			// its last Emit, so draining to close here catches every
			// shortlist that made it past curation.
			for e := range s.events {
				buf = append(buf, e)
			}
			s.flush(ctx, n, buf)
			return
		}
	}
}
