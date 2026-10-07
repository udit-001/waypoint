package db

import (
	"encoding/json"
	"sort"
	"strings"
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
	Title     string   `json:"title"`
	Seniority string   `json:"seniority"`
	Industry  string   `json:"industry"`
	Skills    []string `json:"skills"`
	// WorkHistory is the compressed employment history: role and company per
	// entry, in profile order, deduped and capped. Dates and the free-text
	// description are dropped — a description is uncontrolled text, and
	// neither is on the LLM allowlist.
	WorkHistory []WorkHistoryEntry `json:"work_history"`

	// CurrentLocation is the user's own readout of where they are. It is NOT
	// on the LLM allowlist (the allowlist carries a city-granularity location
	// *preference*): it is free text — the LinkedIn import and a hand-typed
	// field can both hold a street address — so it has no counterpart on
	// LLMInput and no renderer can reach it. It stays on the brief because
	// the profile readout and the web need it.
	CurrentLocation string `json:"current_location"`
}

// WorkHistoryEntry is one past role as an LLM may see it. Deliberately no
// dates and no description: role and company are the fit signal, and both
// are on the allowlist.
type WorkHistoryEntry struct {
	Title   string `json:"title"`
	Company string `json:"company"`
}

// maxWorkHistory caps the compressed history. Beyond a handful of roles the
// signal stops adding and the tokens keep costing.
const maxWorkHistory = 10

// BriefConstraints holds one-time user constraints.
type BriefConstraints struct {
	VisaSponsorship string             `json:"visa_sponsorship"`
	SalaryFloor     []SalaryFloorEntry `json:"salary_floor"`
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
	// Seniority: stored value wins; otherwise derive from experience. A fact
	// is never gated, but when experience can answer it, show the answer.
	seniority := p.Seniority
	if seniority == "" {
		seniority = DeriveSeniority(p.Experience)
	}

	facts := BriefFacts{
		Title:           p.Title,
		Seniority:       seniority,
		Industry:        p.Industry,
		CurrentLocation: p.CurrentLocation,
		Skills:          stringList(p.Skills),
		WorkHistory:     compressWorkHistory(p.Experience, maxWorkHistory),
	}

	constraints := BriefConstraints{
		VisaSponsorship: p.VisaSponsorship,
		SalaryFloor:     salaryFloorBrief(p.SalaryFloor),
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
	// The gate covers the *essential* search brief: location, companies,
	// keywords. `remote` is optional (empty = "any", matching the scraper
	// convention), `dealbreakers` and `avoid_companies` are optional
	// refinements — an empty value is a valid settled answer ("none"), so they
	// are surfaced in the brief but never gate `Complete`.
	open := []string{}

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

// LLMInput is the allowlist, materialized — the only shape an LLM-input
// renderer may read.
//
// Default-deny is structural here, not a matter of callsite discipline: a
// Profile or Brief field with no counterpart in this type is unreachable
// from a renderer, so it cannot leak by a later edit. Name, Email and Phone
// have no counterpart. Neither does Facts.CurrentLocation (free text; may
// hold a street address) or Constraints (visa status, salary floor), both of
// which sit on Brief but are simply not projected here.
//
// Every field below is on the allowlist in spec WP-DOC-8's hard constraint.
// Adding one to the prompt means adding it here — which is the decision that
// should be surfaced, not slipped in.
type LLMInput struct {
	// Facts.
	Title       string
	Seniority   string
	Industry    string
	Skills      []string
	WorkHistory []WorkHistoryEntry

	// Preferences.
	Remote         string
	LocationPref   []string
	Companies      []string
	AvoidCompanies []string
	Keywords       []string

	// Open names the brief fields still unsettled ("companies",
	// "keywords"). Field names only, never values — brief state the model
	// uses to know what it is guessing at, not personal data.
	Open []string
}

// LLMInput projects a Brief onto the allowlist. This function is the whole
// PII decision in one readable place; everything downstream is formatting
// over a type that has nothing to leak.
func (b Brief) LLMInput() LLMInput {
	return LLMInput{
		Title:          b.Facts.Title,
		Seniority:      b.Facts.Seniority,
		Industry:       b.Facts.Industry,
		Skills:         b.Facts.Skills,
		WorkHistory:    b.Facts.WorkHistory,
		Remote:         b.Preferences.Remote,
		LocationPref:   b.Preferences.LocationPref,
		Companies:      b.Preferences.Companies,
		AvoidCompanies: b.Preferences.AvoidCompanies,
		Keywords:       b.Preferences.Keywords,
		Open:           b.Open,
	}
}

// Text renders the allowlist as compact prompt text — the single renderer for
// every LLM input in the system (facet expansion, curation, and anything
// added later). Callers add their own framing around it; the facts are
// rendered here and nowhere else.
//
// This is the PII seam (spec WP-DOC-8, hard constraint "PII never reaches an
// LLM"). It is a method on the allowlist rather than on Brief so that the
// never-leaves fields are not merely unread but unreachable.
//
// TestBriefText_Allowlist asserts the allowlist against a fixture profile
// carrying the whole never-leaves set; it covers every LLM-input builder,
// because they all cross this one seam.
func (i LLMInput) Text() string {
	var sb strings.Builder
	if i.Title != "" {
		sb.WriteString("title: " + i.Title + "\n")
	}
	if i.Seniority != "" {
		sb.WriteString("seniority: " + i.Seniority + "\n")
	}
	if i.Industry != "" {
		sb.WriteString("industry: " + i.Industry + "\n")
	}
	if len(i.Skills) > 0 {
		sb.WriteString("skills: " + strings.Join(i.Skills, ", ") + "\n")
	}
	if roles := renderWorkHistory(i.WorkHistory); roles != "" {
		sb.WriteString("past roles: " + roles + "\n")
	}
	if len(i.Keywords) > 0 {
		sb.WriteString("interests: " + strings.Join(i.Keywords, ", ") + "\n")
	}
	if len(i.Companies) > 0 {
		sb.WriteString("companies liked: " + strings.Join(i.Companies, ", ") + "\n")
	}
	if len(i.AvoidCompanies) > 0 {
		sb.WriteString("companies avoided: " + strings.Join(i.AvoidCompanies, ", ") + "\n")
	}
	if len(i.LocationPref) > 0 {
		sb.WriteString("location: " + strings.Join(i.LocationPref, ", ") + "\n")
	}
	if i.Remote != "" {
		sb.WriteString("remote: " + i.Remote + "\n")
	}
	if len(i.Open) > 0 {
		sb.WriteString("open questions: " + strings.Join(i.Open, "; ") + "\n")
	}
	return sb.String()
}

// renderWorkHistory flattens the compressed history to "Role @ Company"
// fragments, tolerating an entry missing either half.
func renderWorkHistory(history []WorkHistoryEntry) string {
	parts := make([]string, 0, len(history))
	for _, w := range history {
		switch {
		case w.Title != "" && w.Company != "":
			parts = append(parts, w.Title+" @ "+w.Company)
		case w.Company != "":
			parts = append(parts, w.Company)
		case w.Title != "":
			parts = append(parts, w.Title)
		}
	}
	return strings.Join(parts, "; ")
}

// compressWorkHistory reduces structured experience to the LLM-visible
// history. Entries keep their profile order, duplicates collapse, and the
// list is capped — the three things "compressed" means here.
func compressWorkHistory(stored string, max int) []WorkHistoryEntry {
	entries := ParseExperienceEntries(stored)
	out := make([]WorkHistoryEntry, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		title, company := strings.TrimSpace(e.Title), strings.TrimSpace(e.Company)
		if title == "" && company == "" {
			continue
		}
		key := strings.ToLower(title + "@" + company)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, WorkHistoryEntry{Title: title, Company: company})
		if len(out) == max {
			break
		}
	}
	return out
}
