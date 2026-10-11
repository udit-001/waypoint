package autopilot

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/db/dbtest"
	"github.com/udit-001/waypoint/internal/scraper"
	"github.com/udit-001/waypoint/internal/zen"
)

// captureUpstream is a fake zen that records every request body and answers
// each one with a successful curation turn — so a test can assert what was
// actually put on the wire.
type captureUpstream struct {
	mu     sync.Mutex
	bodies []string
}

func (u *captureUpstream) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	u.mu.Lock()
	u.bodies = append(u.bodies, string(body))
	u.mu.Unlock()
	writeSSE(w)
}

func (u *captureUpstream) sent() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return strings.Join(u.bodies, "\n")
}

// TestCurationInput_IsPIIFree: the curation call builds its message from the
// shared renderer, not from raw profile rows. Asserted at the wire — this is
// what zen was actually handed.
func TestCurationInput_IsPIIFree(t *testing.T) {
	stubDiscovery(t)
	f := db.NewFakeStore()
	f.Profile = dbtest.PIIProfile()

	up := &captureUpstream{}
	srv := httptest.NewServer(http.HandlerFunc(up.handler))
	t.Cleanup(srv.Close)

	cfg := newCycle(f, []scraper.Scraper{&fixtureScraper{
		name: "test-scraper",
		results: []scraper.Result{
			{URL: "https://example.com/a", Title: "Backend Engineer", Company: "Globex"},
		},
	}})
	cfg.ZenClient = zen.New(zen.Config{BaseURL: srv.URL, APIKey: "k", Model: "m"})

	stubSelection(t)
	entry := Run(context.Background(), cfg)
	if entry.PostingsShortlisted+entry.PostingsDismissed == 0 {
		t.Fatalf("curation never ran — nothing to assert against: %+v", entry)
	}
	sent := up.sent()
	if sent == "" {
		t.Fatal("no zen request captured")
	}

	for _, c := range dbtest.NeverLeaves() {
		if strings.Contains(sent, c.Value) {
			t.Errorf("%s reached the wire: %q", c.What, c.Value)
		}
	}

	mustAppear := []dbtest.Leak{
		{What: "title", Value: "Backend Engineer"},
		{What: "seniority", Value: "senior"},
		{What: "industry", Value: "fintech"},
		{What: "skill", Value: "PostgreSQL"},
		{What: "interest", Value: "distributed systems"},
		{What: "company liked", Value: "Gojek"},
		{What: "company avoided", Value: "Acme"},
		{What: "remote preference", Value: "remote"},
		{What: "city-granularity location preference", Value: "Berlin"},
		{What: "past role", Value: "Senior Engineer"},
		{What: "past company", Value: "Stripe"},
	}
	for _, c := range mustAppear {
		if !strings.Contains(sent, c.Value) {
			t.Errorf("%s missing from the curation input: %q", c.What, c.Value)
		}
	}
}
