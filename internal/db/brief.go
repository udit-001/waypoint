package db

import (
	"encoding/json"
	"sort"
)

// Brief is the worked readout of the curation brief for the agent. It
// groups the profile into three buckets and exposes the frontier.
//
//	Facts       — seeded (resume / public LinkedIn via Exa) or derived; never gated.
//	Constraints — visa sponsorship, salary floor; told once.
//	Preferences — the brief itself; what the agent interviews and confirms.
//
// `Open` is the frontier: the preferences (and, at setup, derivable facts)
// still to be settled. `Complete` is true iff the preferences bucket is full
// — recency/volume/query are per-run flags and never count here.
type Brief struct {
	Facts       BriefFacts       `json:"facts"`
	Constraints BriefConstraints `json:"constraints"`
	Preferences BriefPreferences `json:"preferences"`
	Open        []string         `json:"open"`
	Complete    bool             `json:"complete"`
}

// BriefFacts holds facts sourced from a resume or public profile. They are
// read-only on the web and never gate `Complete`.
type BriefFacts struct {
	Title           string   `json:"title"`
	Seniority       string   `json:"seniority"`
	CurrentLocation string   `json:"current_location"`
	Skills          []string `json:"skills"`
}

// BriefConstraints holds one-time user constraints.
type BriefConstraints struct {
	VisaSponsorship string `json:"visa_sponsorship"`
	SalaryFloor     string `json:"salary_floor"`
}

// BriefPreferences is the brief — the search intent the agent uses.
type BriefPreferences struct {
	Remote         string   `json:"remote"`
	LocationPref   []string `json:"location_preference"`
	Companies      []string `json:"companies"`
	AvoidCompanies []string `json:"avoid_companies"`
	Keywords       []string `json:"keywords"`
	Dealbreakers   []string `json:"dealbreakers"`
}

// getBrief derives a Brief from a Profile. Purely functional so both
// SQLiteStore and FakeStore share the exact same logic.
func getBrief(p Profile) Brief {
	facts := BriefFacts{
		Title:           p.Title,
		Seniority:       p.Seniority,
		CurrentLocation: p.CurrentLocation,
		Skills:          stringList(p.Skills),
	}

	constraints := BriefConstraints{
		VisaSponsorship: p.VisaSponsorship,
		SalaryFloor:     p.SalaryFloor,
	}

	prefs := BriefPreferences{
		Remote:         p.Remote,
		LocationPref:   stringList(p.LocationPref),
		Companies:      stringList(p.Companies),
		AvoidCompanies: stringList(p.AvoidCompanies),
		Keywords:       stringList(p.Keywords),
		Dealbreakers:   stringList(p.Dealbreakers),
	}

	// The frontier is the preferences bucket. A preference is "open" when it
	// is not settled. Facts at setup are handled by the seeding flow; they
	// never enter `open` and never gate `Complete` — a missing fact means the
	// seed has not arrived, not that the user must be interviewed.
	//
	// The gate covers the *essential* search brief: remote, location,
	// companies, keywords. `dealbreakers` and `avoid_companies` are optional
	// refinements — an empty value is a valid settled answer ("none"), so they
	// are surfaced in the brief but never gate `Complete`.
	open := []string{}
	if prefs.Remote == "" {
		open = append(open, "remote")
	}
	if len(prefs.LocationPref) == 0 {
		open = append(open, "location_preference")
	}
	if len(prefs.Companies) == 0 {
		open = append(open, "companies")
	}
	if len(prefs.Keywords) == 0 {
		open = append(open, "keywords")
	}
	sort.Strings(open)

	return Brief{
		Facts:       facts,
		Constraints: constraints,
		Preferences: prefs,
		Open:        open,
		Complete:    len(open) == 0,
	}
}

// stringList parses a stored JSON-array string into a Go slice, tolerating an
// empty value.
func stringList(s string) []string {
	if s == "" || s == "[]" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}

// GetBrief returns the worked curation-brief readout from the profile.
func (s *SQLiteStore) GetBrief() (Brief, error) {
	p, err := s.GetProfile()
	if err != nil {
		return Brief{}, err
	}
	return getBrief(p), nil
}
