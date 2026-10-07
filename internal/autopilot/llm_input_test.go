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
	"github.com/udit-001/waypoint/internal/scraper"
	"github.com/udit-001/waypoint/internal/zen"
)

// piiLadenProfile is the never-leaves fixture plus every allowlisted field,
// shared with the db-side renderer test.
func piiLadenProfile() db.Profile {
	return db.Profile{
		// Never leaves the machine.
		Name:            "Ada Lovelace",
		Email:           "ada.lovelace@example.com",
		Phone:           "+91 98765 43210",
		CurrentLocation: "12 MG Road, Bengaluru 560001",
		VisaSponsorship: "citizen-only",
		SalaryFloor:     `[{"region":"IN","amount":4000000}]`,

		// The allowlist.
		Title:     "Backend Engineer",
		Seniority: "senior",
		Industry:  "fintech",
		Skills:    `["Go","PostgreSQL"]`,
		Experience: `[{"title":"Senior Engineer","company":"Stripe","start":"2021-01","end":"",` +
			`"description":"Owned the payment rails for 40M users"}]`,
		Remote:         "remote",
		LocationPref:   `["Bengaluru","Berlin"]`,
		Companies:      `["Stripe","Gojek"]`,
		AvoidCompanies: `["Acme"]`,
		Keywords:       `["payments","distributed systems"]`,

		// Not on the allowlist either.
		Dealbreakers: `["night shift"]`,
	}
}

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
	f.Profile = piiLadenProfile()

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

	entry := Run(context.Background(), cfg)
	if entry.PostingsShortlisted+entry.PostingsDismissed == 0 {
		t.Fatalf("curation never ran — nothing to assert against: %+v", entry)
	}
	sent := up.sent()
	if sent == "" {
		t.Fatal("no zen request captured")
	}

	mustNotAppear := []struct{ what, value string }{
		{"name", "Ada Lovelace"},
		{"email", "ada.lovelace@example.com"},
		{"phone", "+91 98765 43210"},
		{"street", "12 MG Road"},
		{"postcode", "560001"},
		{"visa status", "citizen-only"},
		{"salary floor", "4000000"},
		{"free-text role description", "40M users"},
		{"dealbreaker (not on the allowlist)", "night shift"},
	}
	for _, c := range mustNotAppear {
		if strings.Contains(sent, c.value) {
			t.Errorf("%s reached the wire: %q", c.what, c.value)
		}
	}

	mustAppear := []struct{ what, value string }{
		{"title", "Backend Engineer"},
		{"seniority", "senior"},
		{"industry", "fintech"},
		{"skill", "PostgreSQL"},
		{"interest", "distributed systems"},
		{"company liked", "Gojek"},
		{"company avoided", "Acme"},
		{"remote preference", "remote"},
		{"city-granularity location preference", "Berlin"},
		{"past role", "Senior Engineer"},
		{"past company", "Stripe"},
	}
	for _, c := range mustAppear {
		if !strings.Contains(sent, c.value) {
			t.Errorf("%s missing from the curation input: %q", c.what, c.value)
		}
	}
}
