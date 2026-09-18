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
}
