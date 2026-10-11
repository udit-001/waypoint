package discovery

// The agentic discovery universe (WP-180): one Zen agent loop replaces
// ExpandFacets + Enumerate as the company-universe producer. The loop is
// zen protocol + discovery domain: this file owns the prompt, the tools,
// the submit validation, and the facet wrapping; zen.Client owns turns and
// wire; exa.Client owns one search. The hardcoded universe (HardcodedFacets)
// is the second adapter at the same seam — selected by the caller on loop
// failure.
//
// The brief text arriving here is the LLM allowlist projection (the one
// PII seam), never raw profile rows.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/udit-001/waypoint/internal/db"
	"github.com/udit-001/waypoint/internal/exa"
	"github.com/udit-001/waypoint/internal/zen"
)

// Budgets (WP-DOC-8 §2): ~10 live Exa searches per run, ≤12 loop turns.
// Past the search budget the tool answers with the exhaustion line and the
// model must still submit.
const (
	MaxAgentTurns    = 12
	MaxAgentSearches = 10
)

// AgentFacetName marks candidates as agent-discovered — the provenance the
// discovery ledger shows. The agent's own clusters live in its notes; the
// candidate row keeps one name.
const AgentFacetName = "agentic"

// AgentLooper is the loop seam, satisfied by *zen.Client. Tests stub it
// with a scripted conversation.
type AgentLooper interface {
	AgentLoop(ctx context.Context, system, user string, tools []zen.AgentTool, exec zen.AgentExec, maxTurns int) (args string, turns int, err error)
}

// Searcher is the search seam, satisfied by *exa.Client.
type Searcher interface {
	Search(ctx context.Context, query string, numResults int) ([]exa.CompanyHit, error)
}

// agentSubmission is the strict shape submit_companies must carry.
type agentSubmission struct {
	Companies []AgentCompany `json:"companies"`
	Clusters  []string       `json:"clusters"`
	Notes     []string       `json:"notes"`
}

// AgentCompany is one submitted company with its one-line rationale.
type AgentCompany struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
	Why    string `json:"why,omitempty"`
}

// discoveryTools is the loop's toolset: the search tool (free-form
// queries) and the terminal submit. The gate decoys are zen's, riding
// ahead of these.
func discoveryTools() []zen.AgentTool {
	return []zen.AgentTool{
		{
			Name:        "exa_search",
			Description: "Search the web for companies. Use free-form queries — industry clusters (\"companies doing credit ratings in India\"), product categories (\"market data vendors\"), or specific lookups (\"what does Acuite do\"). Returns titles and URLs.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":      map[string]any{"type": "string", "description": "The search query"},
					"numResults": map[string]any{"type": "integer", "description": "Results to return, 1-25 (default 10)"},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "submit_companies",
			Description: "Submit the final company watchlist. Call this exactly once, when the list is complete.",
			Terminal:    true,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"companies": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"name":   map[string]any{"type": "string"},
								"domain": map[string]any{"type": "string", "description": "Company website domain, e.g. moodys.com"},
								"why":    map[string]any{"type": "string", "description": "One line: which part of the brief this company serves"},
							},
							"required": []string{"name", "domain"},
						},
					},
					"clusters": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "The industry clusters you explored"},
					"notes":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Anything thin or uncertain — clusters where search came back weak"},
				},
				"required": []string{"companies"},
			},
		},
	}
}

// agentSystemPrompt is the discovery contract's system prompt (WP-DOC-8
// §2, validated by the WP-172 prototype live runs): role + brief + rules,
// positive phrasing, the completion criterion in the last rule.
func agentSystemPrompt(brief string) string {
	return `You are a company-discovery agent for a job-search tool. Given the user's brief, build the list of companies worth watching for job postings.

BRIEF:
` + brief + `

Rules:
- Cover the DISTINCT industry clusters the brief implies. Explore several angles before settling — do not stop at the first search.
- 25 to 40 companies total. Prefer companies the brief's "companies liked" pattern suggests plus credible peers in each cluster.
- Every company needs a real website domain (moodys.com, not a careers-page URL).
- Use exa_search freely: broad cluster queries to enumerate, targeted queries to verify or fill gaps. Budget matters — avoid near-duplicate queries.
- When the list is complete, call submit_companies exactly once with the final list. Never answer in prose instead.`
}

// AgentUniverseResult carries the validated universe: the facets Discover
// probes downstream and the company list in its cacheable shape (with the
// per-company rationale).
type AgentUniverseResult struct {
	Facets    []Facet
	Companies []AgentCompany
}

// AgentUniverse runs the agentic discovery loop and returns the validated
// company universe — the facet shape Discover probes downstream, unchanged
// (WP-172: shared board probing, unchanged).
func AgentUniverse(ctx context.Context, brief string, loop AgentLooper, search Searcher) (*AgentUniverseResult, error) {
	sub := &agentSubmission{}
	searches := 0

	exec := func(name, args string) (string, bool) {
		switch name {
		case "exa_search":
			return execSearch(ctx, search, args, &searches), false
		case "submit_companies":
			return execSubmit(args, sub)
		default:
			// Gate decoys and unknown tools: refuse, never spend a search.
			return "Tool not available in this environment.", false
		}
	}

	if _, _, err := loop.AgentLoop(ctx, agentSystemPrompt(brief),
		"Build the company watchlist for this brief.", discoveryTools(), exec, MaxAgentTurns); err != nil {
		return nil, err
	}
	if len(sub.Companies) == 0 {
		return nil, fmt.Errorf("agent submitted no usable companies")
	}
	return &AgentUniverseResult{
		Facets: []Facet{{
			Name:      AgentFacetName,
			Companies: toFacetCompanies(sub.Companies),
		}},
		Companies: sub.Companies,
	}, nil
}

// execSearch answers one exa_search call. Only a successful search
// consumes the loop's search budget — a failed dial is not a live Exa call
// and the model's allowance is not the retry tax (the turn cap bounds a
// pathologically failing query).
func execSearch(ctx context.Context, search Searcher, args string, searches *int) string {
	var a struct {
		Query      string `json:"query"`
		NumResults int    `json:"numResults"`
	}
	_ = json.Unmarshal([]byte(args), &a)
	if *searches >= MaxAgentSearches {
		return "Search budget exhausted. Finish with what you have and call submit_companies."
	}
	hits, err := search.Search(ctx, a.Query, a.NumResults)
	if err != nil {
		// One dead search is not fatal: the retry costs a turn and the
		// exhaustion nudge still lands the submit.
		return fmt.Sprintf("Search failed: %v. Try a different query or finish with what you have.", err)
	}
	*searches++
	var sb strings.Builder
	for _, h := range hits {
		fmt.Fprintf(&sb, "%s — %s\n", h.Name, h.URL)
	}
	return truncateRunes(sb.String(), 6000)
}

// execSubmit answers one submit_companies call: valid non-empty payload
// records it and ends the loop; anything else is an error reply the model
// retries against.
func execSubmit(args string, sub *agentSubmission) (string, bool) {
	parsed, err := parseSubmission(args)
	if err != nil {
		return fmt.Sprintf(`{"error": %q}`, err.Error()), false
	}
	*sub = *parsed
	return `{"status": "recorded"}`, true
}

// parseSubmission validates the submit payload: drop empty names and
// domains, reduce each domain to a bare host (models slip URLs), and dedup
// by host preserving first occurrence. A payload left with nothing is an
// error — the model retries rather than ending the loop empty.
func parseSubmission(args string) (*agentSubmission, error) {
	var sub agentSubmission
	if err := json.Unmarshal([]byte(args), &sub); err != nil {
		return nil, fmt.Errorf("invalid submit_companies JSON: %w", err)
	}
	clean := make([]AgentCompany, 0, len(sub.Companies))
	seen := make(map[string]bool, len(sub.Companies))
	for _, c := range sub.Companies {
		c.Name = strings.TrimSpace(c.Name)
		domain := NormalizeDomain(c.Domain)
		if c.Name == "" || domain == "" {
			continue
		}
		if seen[domain] {
			continue
		}
		seen[domain] = true
		c.Domain = domain
		clean = append(clean, c)
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("companies must be a non-empty array with name and domain")
	}
	sub.Companies = clean
	return &sub, nil
}

// NormalizeDomain reduces a submitted domain (or full URL) to the bare
// host the probe stage builds careers URLs from.
func NormalizeDomain(raw string) string {
	d := strings.ToLower(strings.TrimSpace(raw))
	d = strings.TrimPrefix(strings.TrimPrefix(d, "https://"), "http://")
	d = strings.SplitN(d, "/", 2)[0]
	return strings.TrimPrefix(d, "www.")
}

// toFacetCompanies maps the validated submission onto Facet's Company
// shape.
func toFacetCompanies(in []AgentCompany) []Company {
	out := make([]Company, 0, len(in))
	for _, c := range in {
		out = append(out, Company{Name: c.Name, Domain: c.Domain})
	}
	return out
}

// ToStoredCompanies maps a validated submission onto the cache shape,
// rationale included. One place for the mapping — the autopilot stage and
// the CLI both cache (WP-180).
func ToStoredCompanies(in []AgentCompany) []db.DiscoveredCompany {
	out := make([]db.DiscoveredCompany, 0, len(in))
	for _, c := range in {
		out = append(out, db.DiscoveredCompany{Name: c.Name, Domain: c.Domain, Why: c.Why})
	}
	return out
}

// CachedFacets rebuilds the agent facet from a cached company list — the
// shape Discover probes downstream, identical to a fresh run's facets.
func CachedFacets(cached []db.DiscoveredCompany) []Facet {
	companies := make([]Company, 0, len(cached))
	for _, c := range cached {
		companies = append(companies, Company{Name: c.Name, Domain: c.Domain})
	}
	return []Facet{{Name: AgentFacetName, Companies: companies}}
}

// truncateRunes caps a tool reply at n runes — search text can run long.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
