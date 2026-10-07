package db

import (
	"strings"
	"testing"
)

// TestBriefText_Allowlist pins the PII seam (spec WP-DOC-8, hard constraint
// "PII never reaches an LLM"). Brief.Text is the single renderer for every
// LLM input in the system, so this one test covers every LLM-input builder:
// a fixture profile carrying the never-leaves set must not leak a byte of it,
// and every allowlisted field must come out.
//
// Default-deny is the rule, so the absence list is the interesting half. If a
// future change wants to render something here, this test is the conversation.
func TestBriefText_Allowlist(t *testing.T) {
	p := Profile{
		// Never leaves the machine.
		Name:            "Ada Lovelace",
		Email:           "ada.lovelace@example.com",
		Phone:           "+91 98765 43210",
		CurrentLocation: "12 MG Road, Bengaluru 560001",
		VisaSponsorship: "citizen-only",
		SalaryFloor:     `[{"region":"IN","amount":4000000}]`,

		// The allowlist — every one of these must reach the model.
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

		// Not on the allowlist either — default-deny keeps it home.
		Dealbreakers: `["night shift"]`,
	}

	f := NewFakeStore()
	f.Profile = p
	brief, err := f.GetBrief()
	if err != nil {
		t.Fatalf("GetBrief: %v", err)
	}
	// The caller's path: profile -> brief -> allowlist projection -> text.
	out := brief.LLMInput().Text()

	mustNotAppear := []struct{ what, value string }{
		{"name", p.Name},
		{"email", p.Email},
		{"phone", p.Phone},
		{"street", "12 MG Road"},
		{"postcode", "560001"},
		{"visa status", p.VisaSponsorship},
		{"salary floor", "4000000"},
		{"free-text role description", "40M users"},
		{"dealbreaker (not on the allowlist)", "night shift"},
	}
	for _, c := range mustNotAppear {
		if strings.Contains(out, c.value) {
			t.Errorf("%s leaked into the LLM input: %q found in:\n%s", c.what, c.value, out)
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
		if !strings.Contains(out, c.value) {
			t.Errorf("%s missing from the LLM input: %q not found in:\n%s", c.what, c.value, out)
		}
	}
}
