package autopilot

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/udit-001/waypoint/internal/zen"
)

// DefaultLaneWidth is the cycle's concurrent-call width for the zen
// curation lane. Provisional and deliberately conservative: the
// free-tier concurrency research (WP-191) has NOT run, so this value is
// a safe guess, not a measurement. Re-tune it (and MaxLaneWidth) when
// WP-191's findings land.
const DefaultLaneWidth = 2

// MaxLaneWidth is the hard ceiling — a cycle never runs more than this
// many concurrent calls, whatever config asks for.
const MaxLaneWidth = 8

// laneWidth resolves a configured width: 0 means DefaultLaneWidth,
// anything above MaxLaneWidth clamps to the ceiling.
func laneWidth(n int) int {
	if n <= 0 {
		return DefaultLaneWidth
	}
	if n > MaxLaneWidth {
		return MaxLaneWidth
	}
	return n
}

// laneResult is one item's outcome; results[i] always corresponds to
// items[i], whatever order the calls finished in.
type laneResult[R any] struct {
	val R
	err error
}

// runLanes runs fn over items with at most width calls in flight and
// returns one result per item in input order. width <= 1 or a single
// item runs serially on the caller's goroutine — no pool. A canceled
// context stops new calls from starting (they fail with ctx.Err());
// in-flight calls finish or abort through their own ctx.
func runLanes[T, R any](ctx context.Context, items []T, width int, fn func(context.Context, T) (R, error)) []laneResult[R] {
	results := make([]laneResult[R], len(items))
	if width <= 1 || len(items) <= 1 {
		for i, item := range items {
			results[i].val, results[i].err = fn(ctx, item)
		}
		return results
	}

	var next atomic.Int64
	var wg sync.WaitGroup
	for w := 0; w < width; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				i := int(next.Add(1)) - 1
				if i >= len(items) {
					return
				}
				// Claimed-but-canceled items must still carry an error,
				// or the caller would see a nil-err gap in the results.
				if err := ctx.Err(); err != nil {
					results[i].err = err
					continue
				}
				results[i].val, results[i].err = fn(ctx, items[i])
			}
		}()
	}
	wg.Wait()
	return results
}

// Backoff knobs. throttleBackoffBase is a package var so tests can
// shrink the sleeps; production code must not touch it.
var throttleBackoffBase = 250 * time.Millisecond

const (
	// maxThrottleBackoff caps the doubling backoff between throttle
	// retries; maxThrottleAttempts is the total tries (1 call + 2
	// retries) before the error stands.
	maxThrottleBackoff  = 4 * time.Second
	maxThrottleAttempts = 3
)

// isThrottled reports whether err looks like a rate-limit response:
// an HTTP 429 status or "rate limit"/"throttl" in the message.
// Heuristic by design — WP-191's findings may replace it with typed
// errors from the upstreams; keep the guess in this one function.
func isThrottled(err error) bool {
	if err == nil {
		return false
	}
	var zerr *zen.Error
	if errors.As(err, &zerr) && zerr.Status == http.StatusTooManyRequests {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "rate limit") || strings.Contains(msg, "throttl")
}

// retryThrottled runs fn, retrying throttle-shaped failures with
// exponential backoff — base doubling per attempt, capped at
// maxThrottleBackoff, at most maxThrottleAttempts tries total. Any
// other error returns immediately. Backoff sleeps select on ctx.Done(),
// so a cancel mid-backoff exits promptly with ctx.Err().
func retryThrottled(ctx context.Context, fn func() error) error {
	var err error
	backoff := throttleBackoffBase
	for attempt := 1; ; attempt++ {
		if err = fn(); !isThrottled(err) {
			return err
		}
		if attempt >= maxThrottleAttempts {
			return err
		}
		if serr := sleepCtx(ctx, backoff); serr != nil {
			return serr
		}
		backoff *= 2
		if backoff > maxThrottleBackoff {
			backoff = maxThrottleBackoff
		}
	}
}

// sleepCtx sleeps d or returns ctx.Err() when the context is canceled
// first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
