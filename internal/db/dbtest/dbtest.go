// Package dbtest holds fixtures shared by tests that assert the PII seam
// (spec WP-DOC-8, hard constraint "PII never reaches an LLM").
//
// The fixture lives here, not in each test file, because the invariant is
// only as good as its coverage: when Profile gains a personal field, the
// never-leaves set has to grow in one place, not three.
package dbtest

import "github.com/udit-001/waypoint/internal/db"

// PIIProfile is a profile carrying the whole never-leaves set alongside every
// allowlisted field, so one fixture exercises both halves of the allowlist.
//
// The values are distinctive on purpose: each must be findable by substring
// in a rendered prompt, which is how the leak assertions work.
func PIIProfile() db.Profile {
	return db.Profile{
		// Never leaves the machine (spec: name, email, phone, street/precise
		// address, salary floor, visa status).
		Name:            "Ada Lovelace",
		Email:           "ada.lovelace@example.com",
		Phone:           "+91 98765 43210",
		CurrentLocation: "12 MG Road, Bengaluru 560001",
		VisaSponsorship: "citizen-only",
		SalaryFloor:     `[{"region":"IN","amount":4000000}]`,

		// The allowlist: title, seniority, industry, skills, interests,
		// companies liked/avoided, remote, city-granularity location
		// preference, past roles/companies.
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
}

// Leak is one value that must never appear in an LLM input. What names the
// kind ("email"); Value is the string to search for.
type Leak struct {
	What  string
	Value string
}

// NeverLeaves returns the values from PIIProfile that must never appear in an
// LLM input.
//
// It also carries two things that are not PII but are off the allowlist:
// a free-text role description (uncontrolled text) and a dealbreaker. Default
// deny means they stay home too.
func NeverLeaves() []Leak {
	return []Leak{
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
}
