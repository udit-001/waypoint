package zen

// Throwaway anonymous-path probe (WP-162 verification). Deleted after use.

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestAnonLiveWire(t *testing.T) {
	if os.Getenv("ZEN_LIVE") != "1" {
		t.Skip("live probe — set ZEN_LIVE=1")
	}
	cfg := DefaultConfig()
	cfg.APIKey = "" // Bearer public — the anonymous free-tier path
	cfg.ProjectID = "probe-project"
	cfg.Catalog = SharedCatalog()
	cfg.HTTPClient = &http.Client{Timeout: 45 * time.Second}
	c := New(cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	out, err := c.Complete(ctx, "You are a terse assistant.", "Reply with exactly: ok")
	if err != nil {
		if ze, ok := err.(*Error); ok {
			t.Fatalf("Complete: status=%d fatal=%v msg=%q", ze.Status, ze.Fatal, ze.Msg)
		}
		t.Fatalf("Complete: %v", err)
	}
	fmt.Printf("Complete OK -> %q\n", out)

	// The full curation tool-loop: streamed, gate tools + curate_posting,
	// verdict assembled from SSE deltas — no key required.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel2()
	sess := c.NewSession("You are a job-posting curator. Call curate_posting with your verdict for each posting. Judge ONLY against the brief.\n\nBRIEF:\n" +
		`{"facts":{"title":"Backend Engineer","skills":["Go"]},"preferences":{"keywords":["backend","Go"]}}`)
	v, err := sess.Curate(ctx2, Posting{
		URL:      "https://example.com/jobs/senior-go-dev",
		Markdown: "Acme Corp: Senior Go engineer, 5+ years Go, distributed systems, remote-first. Competitive salary.",
	})
	if err != nil {
		if ze, ok := err.(*Error); ok {
			t.Fatalf("Curate: status=%d fatal=%v msg=%q", ze.Status, ze.Fatal, ze.Msg)
		}
		t.Fatalf("Curate: %v", err)
	}
	fmt.Printf("Curate OK -> %+v\n", v)
	if v.Decision != DecisionShortlist && v.Decision != DecisionDismiss {
		t.Errorf("verdict decision = %q", v.Decision)
	}
}
