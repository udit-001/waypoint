package zen

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// The catalog tests cross the module's public interface (NewCatalog +
// Models) with an injected clock and an httptest upstream — never the
// SWR internals.

// cdnUpstream is a controllable stand-in for the pi-zen jsdelivr CDN.
type cdnUpstream struct {
	mu      sync.Mutex
	hits    int
	body    string
	etag    string
	lastMod string
	status  int // 0 means 200
	condReq http.Header
	release chan struct{} // when non-nil, the handler parks until closed
	gotCond chan http.Header
}

func (u *cdnUpstream) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		u.hits++
		u.condReq = r.Header.Clone()
		status := u.status
		body := u.body
		etag := u.etag
		lastMod := u.lastMod
		release := u.release // snapshot before parking
		u.mu.Unlock()

		if u.gotCond != nil {
			u.gotCond <- r.Header.Clone()
		}
		if release != nil {
			<-release
		}

		if etag != "" {
			w.Header().Set("ETag", etag)
		}
		if lastMod != "" {
			w.Header().Set("Last-Modified", lastMod)
		}
		if status == http.StatusNotModified {
			w.WriteHeader(304)
			return
		}
		if status != 0 && status != 200 {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}
}

func (u *cdnUpstream) hitCount() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.hits
}

func (u *cdnUpstream) lastHeaders() http.Header {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.condReq
}

// base is the injected clock's origin.
var base = time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)

func newTestCatalog(t *testing.T, u *cdnUpstream, now func() time.Time) *Catalog {
	t.Helper()
	srv := httptest.NewServer(u.handler())
	t.Cleanup(srv.Close)
	return NewCatalog(CatalogConfig{URL: srv.URL, Now: now})
}

// TestCatalog_firstCallFetchesAndParses: a cold cache fetches the curated
// list and serves friendly names + API families. Regression for pi-zen
// fix 0411587: the very first apply must initialise the cache, not panic.
func TestCatalog_firstCallFetchesAndParses(t *testing.T) {
	u := &cdnUpstream{body: `{
		"generatedAt": "2026-09-18T20:17:49.256Z",
		"count": 2,
		"opencodeVersion": "1.18.31",
		"models": [
			{"id": "big-pickle", "name": "Big Pickle", "api": "openai-completions"},
			{"id": "union-alpha-free"}
		]
	}`}
	cat := newTestCatalog(t, u, func() time.Time { return base })

	models, err := cat.Models(context.Background())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if u.hitCount() != 1 {
		t.Fatalf("upstream hits = %d, want 1", u.hitCount())
	}
	if len(models) != 2 {
		t.Fatalf("models = %d entries, want 2", len(models))
	}
	if models[0].ID != "big-pickle" || models[0].Name != "Big Pickle" || models[0].API != "openai-completions" {
		t.Errorf("models[0] = %+v, want big-pickle/Big Pickle/openai-completions", models[0])
	}
	// Absent api defaults to openai-completions; absent name falls back to id.
	if models[1].ID != "union-alpha-free" || models[1].Name != "union-alpha-free" || models[1].API != "openai-completions" {
		t.Errorf("models[1] = %+v, want union-alpha-free with defaults", models[1])
	}
}

// TestCatalog_freshCacheSkipsNetwork: within the TTL the cache answers
// without touching the upstream.
func TestCatalog_freshCacheSkipsNetwork(t *testing.T) {
	u := &cdnUpstream{body: `{"count":1,"models":[{"id":"big-pickle","name":"Big Pickle"}]}`}
	cat := newTestCatalog(t, u, func() time.Time { return base })

	if _, err := cat.Models(context.Background()); err != nil {
		t.Fatalf("first Models: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := cat.Models(context.Background()); err != nil {
			t.Fatalf("Models %d: %v", i, err)
		}
	}
	if got := u.hitCount(); got != 1 {
		t.Fatalf("upstream hits = %d, want 1 (fresh cache must not re-fetch)", got)
	}
}

// TestCatalog_staleServesLastKnownGoodThenRevalidates: an expired cache
// answers instantly with the old list while one background conditional
// revalidation swaps in the new list.
func TestCatalog_staleServesLastKnownGoodThenRevalidates(t *testing.T) {
	release := make(chan struct{})
	u := &cdnUpstream{body: `{"count":1,"models":[{"id":"big-pickle","name":"Big Pickle"}]}`}
	cat := newTestCatalog(t, u, func() time.Time { return base })

	if _, err := cat.Models(context.Background()); err != nil {
		t.Fatalf("cold Models: %v", err)
	}
	// Arm the parking gate and swap the body only now: the revalidation
	// request parks, the cold fetch above must not.
	u.mu.Lock()
	u.release = release
	u.body = `{"count":1,"models":[{"id":"ling-3.0-flash-fin-free","name":"Ling 3.0 Flash Fin Free"}]}`
	u.mu.Unlock()

	// Expire the entry. The stale list must come back immediately —
	// long before the parked upstream could ever answer.
	later := base.Add(2 * time.Hour)
	cat.now = func() time.Time { return later }
	started := time.Now()
	stale, err := cat.Models(context.Background())
	if err != nil {
		t.Fatalf("stale Models: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 500*time.Millisecond {
		t.Fatalf("stale serve blocked %v — must answer from cache while the revalidation runs", elapsed)
	}
	if len(stale) != 1 || stale[0].ID != "big-pickle" {
		t.Fatalf("stale serve = %+v, want the old list", stale)
	}

	// Let the parked revalidation complete with the new body.
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for u.hitCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if u.hitCount() != 2 {
		t.Fatalf("upstream hits = %d, want 2 (one revalidation)", u.hitCount())
	}
	fresh, err := cat.Models(context.Background())
	if err != nil {
		t.Fatalf("Models after revalidation: %v", err)
	}
	if len(fresh) != 1 || fresh[0].ID != "ling-3.0-flash-fin-free" {
		t.Fatalf("post-revalidation list = %+v, want the new list", fresh)
	}
}

// TestCatalog_304RefreshesTimerOnly: an unchanged list revalidates as a
// body-less 304 — the list stays, only freshness is restored.
func TestCatalog_304RefreshesTimerOnly(t *testing.T) {
	u := &cdnUpstream{
		body: `{"count":1,"models":[{"id":"big-pickle","name":"Big Pickle"}]}`,
		etag: `"abc123"`,
	}
	cat := newTestCatalog(t, u, func() time.Time { return base })

	if _, err := cat.Models(context.Background()); err != nil {
		t.Fatalf("cold Models: %v", err)
	}
	// Expire; the revalidation must send the stored ETag and get a 304.
	later := base.Add(2 * time.Hour)
	cat.now = func() time.Time { return later }
	u.status = http.StatusNotModified
	if _, err := cat.Models(context.Background()); err != nil {
		t.Fatalf("stale Models: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for u.hitCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	h := u.lastHeaders()
	if got := h.Get("If-None-Match"); got != `"abc123"` {
		t.Errorf("If-None-Match = %q, want the stored etag", got)
	}
	// Fresh again: the timer advanced, no further upstream hits.
	if _, err := cat.Models(context.Background()); err != nil {
		t.Fatalf("Models after 304: %v", err)
	}
	if got := u.hitCount(); got != 2 {
		t.Fatalf("upstream hits = %d, want 2 (304 restored freshness)", got)
	}
}

// TestCatalog_failureKeepsStaleAndAdvancesTimer: a broken upstream leaves
// the last-known-good list servable and must not become a busy retry loop.
func TestCatalog_failureKeepsStaleAndAdvancesTimer(t *testing.T) {
	u := &cdnUpstream{body: `{"count":1,"models":[{"id":"big-pickle","name":"Big Pickle"}]}`}
	cat := newTestCatalog(t, u, func() time.Time { return base })

	if _, err := cat.Models(context.Background()); err != nil {
		t.Fatalf("cold Models: %v", err)
	}
	later := base.Add(2 * time.Hour)
	cat.now = func() time.Time { return later }
	u.status = http.StatusInternalServerError

	stale, err := cat.Models(context.Background())
	if err != nil {
		t.Fatalf("stale Models during outage: %v", err)
	}
	if len(stale) != 1 || stale[0].ID != "big-pickle" {
		t.Fatalf("stale serve = %+v, want the old list", stale)
	}
	deadline := time.Now().Add(2 * time.Second)
	for u.hitCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}

	// The failure advanced the timer: further calls stay fresh (no retry
	// storm) and keep serving the old list.
	for i := 0; i < 5; i++ {
		if _, err := cat.Models(context.Background()); err != nil {
			t.Fatalf("Models %d during outage: %v", i, err)
		}
	}
	if got := u.hitCount(); got != 2 {
		t.Fatalf("upstream hits = %d, want 2 (failure must advance the timer, not retry)", got)
	}
}

// TestCatalog_singleFlightDedupesRevalidation: N concurrent stale-cache
// triggers share one background revalidation.
func TestCatalog_singleFlightDedupesRevalidation(t *testing.T) {
	u := &cdnUpstream{body: `{"count":1,"models":[{"id":"big-pickle","name":"Big Pickle"}]}`}
	cat := newTestCatalog(t, u, func() time.Time { return base })

	if _, err := cat.Models(context.Background()); err != nil {
		t.Fatalf("cold Models: %v", err)
	}
	later := base.Add(2 * time.Hour)
	cat.now = func() time.Time { return later }

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cat.Models(context.Background()); err != nil {
				t.Errorf("concurrent Models: %v", err)
			}
		}()
	}
	wg.Wait()
	deadline := time.Now().Add(2 * time.Second)
	for u.hitCount() < 2 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond) // let any duplicate revalidation land
	if got := u.hitCount(); got != 2 {
		t.Fatalf("upstream hits = %d, want exactly 2 (single-flight revalidation)", got)
	}
}

// TestCatalog_coldFetchFailureReturnsError: with no last-known-good and a
// dead CDN there is nothing to serve — the caller gets an error and
// decides the fallback. A later recovery repairs the cache.
func TestCatalog_coldFetchFailureReturnsError(t *testing.T) {
	u := &cdnUpstream{body: `{"count":1,"models":[{"id":"big-pickle","name":"Big Pickle"}]}`, status: http.StatusServiceUnavailable}
	cat := newTestCatalog(t, u, func() time.Time { return base })

	if _, err := cat.Models(context.Background()); err == nil {
		t.Fatal("cold Models with dead CDN = nil error, want an error (nothing to serve)")
	}
	u.status = 0
	models, err := cat.Models(context.Background())
	if err != nil {
		t.Fatalf("Models after recovery: %v", err)
	}
	if len(models) != 1 || models[0].ID != "big-pickle" {
		t.Fatalf("recovered list = %+v, want big-pickle", models)
	}
}

// TestCatalog_versionExposedForWire: the curated opencodeVersion is
// readable for the connector's UA gate (WP-162), passively — no network.
func TestCatalog_versionExposedForWire(t *testing.T) {
	u := &cdnUpstream{body: `{
		"count":1,
		"opencodeVersion": "1.18.31",
		"models": [{"id":"big-pickle","name":"Big Pickle"}]
	}`}
	cat := newTestCatalog(t, u, func() time.Time { return base })

	if got := cat.Version(); got != "" {
		t.Fatalf("Version before any fetch = %q, want empty (passive accessor)", got)
	}
	if _, err := cat.Models(context.Background()); err != nil {
		t.Fatalf("Models: %v", err)
	}
	if got := cat.Version(); got != "1.18.31" {
		t.Fatalf("Version = %q, want 1.18.31", got)
	}
	if u.hitCount() != 1 {
		t.Fatalf("upstream hits = %d, want 1 (Version must not fetch)", u.hitCount())
	}
}

// TestCatalog_coldSingleFlight: concurrent cold-cache callers share one
// upstream fetch instead of stampeding the CDN.
func TestCatalog_coldSingleFlight(t *testing.T) {
	release := make(chan struct{})
	u := &cdnUpstream{body: `{"count":1,"models":[{"id":"big-pickle","name":"Big Pickle"}]}`}
	cat := newTestCatalog(t, u, func() time.Time { return base })

	u.mu.Lock()
	u.release = release
	u.mu.Unlock()

	var wg sync.WaitGroup
	results := make([][]Model, 5)
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = cat.Models(context.Background())
		}(i)
	}

	// All five share the first (parked) fetch: while it holds, exactly one
	// upstream request exists and no second one arrives.
	deadline := time.Now().Add(2 * time.Second)
	for u.hitCount() < 1 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if got := u.hitCount(); got != 1 {
		t.Fatalf("upstream hits while parked = %d, want 1 (cold callers must share one fetch)", got)
	}
	close(release)
	wg.Wait()

	for i := 0; i < 5; i++ {
		if errs[i] != nil {
			t.Fatalf("call %d: %v", i, errs[i])
		}
		if len(results[i]) != 1 || results[i][0].ID != "big-pickle" {
			t.Fatalf("call %d: results = %+v", i, results[i])
		}
	}
	if got := u.hitCount(); got != 1 {
		t.Fatalf("upstream hits = %d, want exactly 1", got)
	}
}
