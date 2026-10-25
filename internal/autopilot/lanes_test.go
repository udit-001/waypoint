package autopilot

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/scraper"
	"github.com/udit-001/waypoint/internal/zen"
)

// --- lanes.go unit tests ---

// TestLaneWidthConfig: 0 resolves to the (provisional) default, values
// clamp at the ceiling, negatives fall back to the default.
func TestLaneWidthConfig(t *testing.T) {
	cases := []struct{ in, want int }{
		{0, DefaultLaneWidth},
		{-3, DefaultLaneWidth},
		{1, 1},
		{3, 3},
		{MaxLaneWidth, MaxLaneWidth},
		{99, MaxLaneWidth},
	}
	for _, c := range cases {
		if got := laneWidth(c.in); got != c.want {
			t.Errorf("laneWidth(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestRunLanes_RespectsWidthAndOrder: with width 2 both lanes hold an
// item at once (barrier, so no timing flake), and results come back in
// input order regardless of finish order.
func TestRunLanes_RespectsWidthAndOrder(t *testing.T) {
	items := make([]int, 8)
	for i := range items {
		items[i] = i
	}

	var cur, max atomic.Int64
	deadline := time.Now().Add(2 * time.Second)
	res := runLanes(context.Background(), items, 2, func(_ context.Context, n int) (int, error) {
		c := cur.Add(1)
		for {
			m := max.Load()
			if c <= m || max.CompareAndSwap(m, c) {
				break
			}
		}
		for max.Load() < 2 && time.Now().Before(deadline) {
			time.Sleep(500 * time.Microsecond)
		}
		cur.Add(-1)
		return n * 10, nil
	})

	if got := max.Load(); got != 2 {
		t.Fatalf("max observed concurrency = %d, want 2", got)
	}
	for i, r := range res {
		if r.err != nil || r.val != i*10 {
			t.Fatalf("results[%d] = (%d, %v), want (%d, nil)", i, r.val, r.err, i*10)
		}
	}
}

// TestRunLanes_SerialFallback: width <= 1 runs on the caller's
// goroutine and returns identical ordered results.
func TestRunLanes_SerialFallback(t *testing.T) {
	items := []int{3, 1, 2}
	for _, width := range []int{0, 1, -2} {
		res := runLanes(context.Background(), items, width, func(_ context.Context, n int) (int, error) {
			return n * 10, nil
		})
		for i, r := range res {
			if r.err != nil || r.val != items[i]*10 {
				t.Fatalf("width %d: results[%d] = (%d, %v)", width, i, r.val, r.err)
			}
		}
	}
}

// TestRunLanes_CanceledCtx: a canceled context stops new work — every
// unstarted item carries ctx.Err(), no fn call happens.
func TestRunLanes_CanceledCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	res := runLanes(ctx, []int{1, 2, 3}, 2, func(context.Context, int) (int, error) {
		called = true
		return 0, nil
	})
	if called {
		t.Fatal("fn ran on a canceled context")
	}
	for i, r := range res {
		if !errors.Is(r.err, context.Canceled) {
			t.Fatalf("results[%d].err = %v, want context.Canceled", i, r.err)
		}
	}
}

// TestIsThrottled: the heuristic's accepted shapes live in one function.
func TestIsThrottled(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{&zen.Error{Status: 429, Msg: "slow down"}, true},
		{&zen.Error{Status: 500, Msg: "boom"}, false},
		{errors.New("Rate Limit exceeded"), true},
		{errors.New("being throttled by upstream"), true},
		{errors.New("not found"), false},
		{context.DeadlineExceeded, false},
	}
	for _, c := range cases {
		if got := isThrottled(c.err); got != c.want {
			t.Errorf("isThrottled(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

// TestRetryThrottled_Recovers: two throttle responses then success —
// exactly 3 attempts total, success returned.
func TestRetryThrottled_Recovers(t *testing.T) {
	oldBase := throttleBackoffBase
	throttleBackoffBase = time.Millisecond
	t.Cleanup(func() { throttleBackoffBase = oldBase })

	attempts := 0
	fn := func() error {
		attempts++
		if attempts <= 2 {
			return errors.New("upstream: rate limit hit, back off")
		}
		return nil
	}
	if err := retryThrottled(context.Background(), fn); err != nil {
		t.Fatalf("retryThrottled = %v, want success on attempt 3", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

// TestRetryThrottled_Exhausts: an always-throttling call stands after
// 3 attempts; a non-throttle error returns immediately.
func TestRetryThrottled_Exhausts(t *testing.T) {
	oldBase := throttleBackoffBase
	throttleBackoffBase = time.Millisecond
	t.Cleanup(func() { throttleBackoffBase = oldBase })

	attempts := 0
	err := retryThrottled(context.Background(), func() error {
		attempts++
		return &zen.Error{Status: 429, Msg: "slow down"}
	})
	if err == nil || attempts != maxThrottleAttempts {
		t.Fatalf("err = %v after %d attempts, want the throttle error after 3", err, attempts)
	}

	nonThrottle := 0
	err = retryThrottled(context.Background(), func() error {
		nonThrottle++
		return errors.New("boom")
	})
	if err == nil || nonThrottle != 1 {
		t.Fatalf("non-throttle error retried: %d attempts, err %v", nonThrottle, err)
	}
}

// TestRetryThrottled_CancelMidBackoff: a cancel during a backoff sleep
// exits promptly with ctx.Err(), not stuck in the sleep.
func TestRetryThrottled_CancelMidBackoff(t *testing.T) {
	oldBase := throttleBackoffBase
	throttleBackoffBase = 5 * time.Second // would hang the test if cancel were ignored
	t.Cleanup(func() { throttleBackoffBase = oldBase })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	attempts := 0
	done := make(chan error, 1)
	go func() {
		done <- retryThrottled(ctx, func() error {
			attempts++
			cancel() // kill the ctx while the first backoff sleep is pending
			return errors.New("rate limited")
		})
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		if attempts != 1 {
			t.Fatalf("attempts = %d, want 1", attempts)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("retryThrottled did not exit promptly on cancel")
	}
}

// --- stage integration ---

// curateSSEVerdict renders one curate turn with the given decision and
// score — the wire shape of pipeline_test.go's curateSSEWithScore,
// generalized to both decisions.
func curateSSEVerdict(decision string, score int) string {
	args := fmt.Sprintf(`{\"verdict\":\"%s\",\"score\":%d,\"note\":\"n\",\"overview\":\"o\",\"reasons\":[{\"kind\":\"match\",\"field\":\"role\",\"text\":\"r\"}]}`, decision, score)
	return `{"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"curate_posting","arguments":"` + args + `"}}]},"finish_reason":null}]}` + "\n" +
		`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`
}

// curateServerByURL serves a zen upstream whose verdict depends on the
// request body (the posting URL rides in the user turn).
func curateServerByURL(t *testing.T, verdictFor func(body string) (string, int)) *zen.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		decision, score := verdictFor(string(body))
		writeSSEPayload(w, curateSSEVerdict(decision, score))
	}))
	t.Cleanup(srv.Close)
	return zen.New(zen.Config{BaseURL: srv.URL, APIKey: "k", Model: "m"})
}

// mixedBatch: an avoid-list dismissal, then three postings that escalate
// to zen — shortlist, dismiss, shortlist. Input order matters: the
// events and store outcomes must come back in exactly this order.
func mixedBatch() []scraper.Result {
	return []scraper.Result{
		{URL: "https://example.com/a", Title: "A", Company: "Evil Corp", Source: "src"},
		{URL: "https://example.com/b", Title: "B", Company: "Beta", Source: "src"},
		{URL: "https://example.com/c", Title: "C", Company: "Gamma", Source: "src"},
		{URL: "https://example.com/d", Title: "D", Company: "Delta", Source: "src"},
	}
}

// TestStagePrefilterCurate_WidthZeroMatchesSerialPath: the default width
// (0 -> DefaultLaneWidth) produces exactly the serial path's outcomes —
// statuses, ledger counts, and shortlist events in input order.
func TestStagePrefilterCurate_WidthZeroMatchesSerialPath(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	_ = f.UpsertProfile(map[string]any{"avoid_companies": `["evil corp"]`})

	client := curateServerByURL(t, func(body string) (string, int) {
		switch {
		case strings.Contains(body, "example.com/b"):
			return zen.DecisionShortlist, 90
		case strings.Contains(body, "example.com/c"):
			return zen.DecisionDismiss, 30
		default:
			return zen.DecisionShortlist, 70
		}
	})

	batch := mixedBatch()
	_ = f.AddPostings(batch)
	cfg := newCycle(f, nil)
	cfg.ZenClient = client
	rec := newRunRecord(time.Now())
	stream := newShortlistStream(0)

	stagePrefilterCurate(context.Background(), cfg, batch, rec, stream)

	for url, want := range map[string]string{
		"https://example.com/a": db.StatusDismissed,
		"https://example.com/b": db.StatusShortlisted,
		"https://example.com/c": db.StatusDismissed,
		"https://example.com/d": db.StatusShortlisted,
	} {
		p, ok, err := f.GetPosting(url)
		if err != nil || !ok {
			t.Fatalf("GetPosting(%s): ok=%v err=%v", url, ok, err)
		}
		if p.Status != want {
			t.Errorf("%s status = %q, want %q", url, p.Status, want)
		}
	}
	row := rec.outcome("src")
	if row.Shortlisted != 2 || row.Dismissed != 2 {
		t.Errorf("src row = shortlisted %d / dismissed %d, want 2 / 2", row.Shortlisted, row.Dismissed)
	}

	// Events in input order, scores attached: b, then d.
	var got []ShortlistEvent
	for len(stream.events) > 0 {
		got = append(got, <-stream.events)
	}
	wantEvents := []ShortlistEvent{
		{URL: "https://example.com/b", Title: "B", Company: "Beta", Score: 90},
		{URL: "https://example.com/d", Title: "D", Company: "Delta", Score: 70},
	}
	if len(got) != len(wantEvents) {
		t.Fatalf("emitted %d events, want %d (%v)", len(got), len(wantEvents), got)
	}
	for i, e := range wantEvents {
		if got[i] != e {
			t.Errorf("event[%d] = %+v, want %+v (input order required)", i, got[i], e)
		}
	}
}

// TestStagePrefilterCurate_DegradedStaysSerial: no zen client — every
// escalation shortlists for manual review, dismissals still fire.
func TestStagePrefilterCurate_DegradedStaysSerial(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	_ = f.UpsertProfile(map[string]any{"avoid_companies": `["evil corp"]`})

	cfg := newCycle(f, nil) // ZenClient stays nil
	rec := newRunRecord(time.Now())
	stream := newShortlistStream(0)

	batch := mixedBatch()
	_ = f.AddPostings(batch)
	stagePrefilterCurate(context.Background(), cfg, batch, rec, stream)

	row := rec.outcome("src")
	if row.Shortlisted != 3 || row.Dismissed != 1 {
		t.Errorf("src row = shortlisted %d / dismissed %d, want 3 / 1", row.Shortlisted, row.Dismissed)
	}
	if len(rec.StageErrors) != 0 {
		t.Errorf("stageErrors = %v, want none", rec.StageErrors)
	}
}

// runThrottleStage runs the curate stage against an upstream that
// throttles the marked posting with a fatal-status body carrying the
// rate-limit message: zen returns it immediately (no internal retry
// ladder), so the request count isolates THIS layer's retry ladder.
func runThrottleStage(t *testing.T, f *db.FakeStore, batch []scraper.Result) (*runRecord, *shortlistStream, *atomic.Int32) {
	t.Helper()
	var throttledReqs atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "throttle.example.com") {
			throttledReqs.Add(1)
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"message":"rate limit exceeded"}}`))
			return
		}
		writeSSEPayload(w, curateSSEVerdict(zen.DecisionShortlist, 88))
	}))
	t.Cleanup(srv.Close)

	_ = f.AddPostings(batch)
	cfg := newCycle(f, nil)
	cfg.ZenClient = zen.New(zen.Config{BaseURL: srv.URL, APIKey: "k", Model: "m"})
	rec := newRunRecord(time.Now())
	stream := newShortlistStream(0)
	stagePrefilterCurate(context.Background(), cfg, batch, rec, stream)
	return rec, stream, &throttledReqs
}

// TestStagePrefilterCurate_ThrottleRetriesThenErrors: a posting whose
// zen calls always answer throttle-shaped burns exactly 3 attempts
// (retryThrottled's ladder) and records as errored — without stopping
// the batch's other postings.
func TestStagePrefilterCurate_ThrottleRetriesThenErrors(t *testing.T) {
	stubDiscovery(t)
	oldBase := throttleBackoffBase
	throttleBackoffBase = time.Millisecond
	t.Cleanup(func() { throttleBackoffBase = oldBase })

	f := db.NewFakeStore()
	_ = f.UpsertProfile(map[string]any{"avoid_companies": `["evil corp"]`})
	batch := []scraper.Result{
		{URL: "https://example.com/a", Title: "A", Company: "Evil Corp", Source: "src"},
		{URL: "https://throttle.example.com/x", Title: "T", Company: "T", Source: "src"},
		{URL: "https://example.com/ok", Title: "O", Company: "Ok Co", Source: "src"},
	}

	rec, _, reqs := runThrottleStage(t, f, batch)

	if got := reqs.Load(); got != 3 {
		t.Errorf("throttled upstream requests = %d, want 3 (1 call + 2 backoff retries)", got)
	}
	if p, ok, _ := f.GetPosting("https://example.com/ok"); !ok || p.Status != db.StatusShortlisted {
		t.Errorf("ok posting status = %q (ok=%v), want shortlisted — remaining postings must still process", p.Status, ok)
	}
	if p, ok, _ := f.GetPosting("https://example.com/a"); !ok || p.Status != db.StatusDismissed {
		t.Errorf("dismissed posting status = %q (ok=%v), want dismissed", p.Status, ok)
	}
	if got := rec.outcome("src").Errored; got != 1 {
		t.Errorf("src Errored = %d, want 1", got)
	}
	// A partial failure stays a count — no stage error.
	if len(rec.StageErrors) != 0 {
		t.Errorf("stageErrors = %v, want none — one failed posting is not a broken stage", rec.StageErrors)
	}
}

// TestStagePrefilterCurate_AllThrottledRecordsStageError: a curate
// attempt that failed every posting it attempted is a broken stage.
func TestStagePrefilterCurate_AllThrottledRecordsStageError(t *testing.T) {
	stubDiscovery(t)
	oldBase := throttleBackoffBase
	throttleBackoffBase = time.Millisecond
	t.Cleanup(func() { throttleBackoffBase = oldBase })

	f := db.NewFakeStore()
	_ = f.UpsertProfile(map[string]any{"avoid_companies": `["evil corp"]`})
	batch := []scraper.Result{
		{URL: "https://example.com/a", Title: "A", Company: "Evil Corp", Source: "src"},
		{URL: "https://throttle.example.com/x", Title: "T", Company: "T", Source: "src"},
	}

	rec, _, reqs := runThrottleStage(t, f, batch)

	if got := reqs.Load(); got != 3 {
		t.Errorf("throttled upstream requests = %d, want 3", got)
	}
	if got := rec.outcome("src").Errored; got != 1 {
		t.Errorf("src Errored = %d, want 1", got)
	}
	if len(rec.StageErrors) != 1 || !strings.Contains(rec.StageErrors[0], "curate: all 1 postings failed") {
		t.Errorf("stageErrors = %v, want the all-failures stage error", rec.StageErrors)
	}
}
