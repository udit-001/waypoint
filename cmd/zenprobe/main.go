// zenprobe is a throwaway standalone harness for the waypoint zen connector
// (internal/zen). It calls the live gateway exactly the way the daemon does
// and prints the outcome. Deleted after use — never committed.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/udit-001/waypoint/internal/zen"
)

func main() {
	key := os.Getenv("ZEN_API_KEY")
	if key == "" {
		fmt.Println("ZEN_API_KEY not set")
		os.Exit(1)
	}

	cfg := zen.DefaultConfig()
	cfg.APIKey = key
	cfg.ProjectID = "probe-project"
	cfg.Catalog = zen.SharedCatalog() // UA version + family routing from the curated metadata
	cfg.HTTPClient = &http.Client{Timeout: 45 * time.Second}

	c := zen.New(cfg)
	fmt.Printf("wire: baseURL=%s model=%s ua=%q project=%q\n",
		cfg.BaseURL, cfg.Model, cfg.UserAgent, cfg.ProjectID)

	// Path 1: Complete — no tools, no stream.
	{
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		out, err := c.Complete(ctx, "You are a terse assistant.", "Reply with exactly: ok")
		cancel()
		printErr("Complete", out, err)
	}

	// Path 2: Curate — tools present, streamed.
	{
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		s := c.NewSession("You are a job-curation judge. Call curate_posting when done.")
		v, err := s.Curate(ctx, zen.Posting{
			URL:      "https://example.com/jobs/senior-go-dev",
			Markdown: "Acme Corp: Senior Go engineer, 5+ years Go, distributed systems, remote.",
		})
		cancel()
		printErr("Curate", fmt.Sprintf("%+v", v), err)
	}
}

func printErr(label, out string, err error) {
	if err == nil {
		fmt.Printf("[%s] OK -> %q\n", label, out)
		return
	}
	if ze, ok := err.(*zen.Error); ok {
		fmt.Printf("[%s] FAIL status=%d fatal=%v msg=%q\n", label, ze.Status, ze.Fatal, ze.Msg)
		return
	}
	fmt.Printf("[%s] FAIL %v\n", label, err)
}
