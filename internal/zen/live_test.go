package zen

// Live smoke test against the real zen gateway. Opt-in only: set
// ZEN_LIVE=1 plus ZEN_API_KEY. Never part of `make check` — without
// the env gate it skips. One call proves the free-tier UA gate,
// identity headers, and the tool-call verdict end to end.

import (
	"context"
	"os"
	"testing"
	"time"
)

// Live probe of the curated free-models CDN (WP-161). Opt-in only: set
// ZEN_LIVE=1. No key needed — the CDN fetch is anonymous. Proves the real
// jsdelivr file parses and the SWR cold fetch works end to end.
func TestLiveCatalog(t *testing.T) {
	if os.Getenv("ZEN_LIVE") != "1" {
		t.Skip("live catalog test — set ZEN_LIVE=1 to run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	models, err := NewCatalog(CatalogConfig{}).Models(ctx)
	if err != nil {
		t.Fatalf("live catalog: %v", err)
	}
	if len(models) == 0 {
		t.Fatal("live catalog returned no models")
	}
	for _, m := range models {
		if m.ID == "" || m.API == "" {
			t.Errorf("live entry missing id/api: %+v", m)
		}
		t.Logf("live model: %s | %s | %s", m.ID, m.API, m.Name)
	}
}

func TestLiveSmoke(t *testing.T) {
	if os.Getenv("ZEN_LIVE") != "1" {
		t.Skip("live smoke test — set ZEN_LIVE=1 and ZEN_API_KEY to run")
	}
	key := ResolveKey("")
	if key == "" {
		t.Skip("ZEN_LIVE=1 but no ZEN_API_KEY — nothing to authenticate with")
	}

	cfg := DefaultConfig()
	cfg.APIKey = key
	c := New(cfg)
	sess := c.NewSession("You are a job-posting curator. The user's curation brief (JSON) is your single source of truth. For each posting you receive, call curate_posting with your verdict. Judge ONLY against the brief; unset brief fields are unset — never guess. Reply with the tool call only.\n\nBRIEF:\n" +
		`{"facts":{"title":"Backend Engineer","seniority":"mid","skills":["Go","distributed systems"]},"preferences":{"remote":"yes","keywords":["backend","Go"]}}`)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	v, err := sess.Curate(ctx, Posting{
		URL:      "https://boards.greenhouse.io/spacex/jobs/7690206002",
		Markdown: "SOFTWARE ENGINEER, BACKEND (STARLINK). Location: Redmond, WA / Hawthorne, CA. Responsibilities: design and build distributed backend services in C++/Python/Go for satellite networking...",
	})
	if err != nil {
		t.Fatalf("live Curate: %v", err)
	}
	t.Logf("live verdict: %+v", v)
	if v.Decision != DecisionShortlist && v.Decision != DecisionDismiss {
		t.Errorf("live verdict decision = %q", v.Decision)
	}
	if len(v.Reasons) == 0 {
		t.Error("live verdict has no reasons")
	}
}
