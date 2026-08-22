package discovery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// Completer is the facet-expansion seam: a single-turn LLM completion.
// zen.Session satisfies it; tests use doubles.
type Completer interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// Enumerator lists companies for one facet. The production
// implementation wraps the Exa client; tests use canned slices.
type Enumerator interface {
	Companies(ctx context.Context, facet string) ([]Company, error)
}

// facetSystemPrompt frames the expansion job: short, lowercase,
// industry-level facets — company universes, not job titles.
const facetSystemPrompt = `You expand a job-search brief into search facets:
short lowercase industry/company-universe labels (e.g. "market-data",
"payments-fintech", "credit-ratings", "regtech"). Facets name industries
or company clusters a recruiter would recognize — never job titles,
never companies. Reply with ONLY a JSON array of facet strings, max 12.`

// ExpandFacets turns the curation brief into facet labels via one LLM
// call. maxFacets caps the result. Parse errors surface with the raw
// output so the cause is diagnosable.
func ExpandFacets(ctx context.Context, brief string, c Completer, maxFacets int) ([]string, error) {
	out, err := c.Complete(ctx, facetSystemPrompt, brief)
	if err != nil {
		return nil, fmt.Errorf("facet expansion: %w", err)
	}
	var facets []string
	if jerr := json.Unmarshal([]byte(out), &facets); jerr != nil {
		return nil, fmt.Errorf("facet expansion: model returned non-JSON (%w); output: %s", jerr, truncate(out, 200))
	}
	clean := make([]string, 0, len(facets))
	for _, f := range facets {
		f = strings.ToLower(strings.TrimSpace(f))
		if f != "" {
			clean = append(clean, f)
		}
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("facet expansion: model returned no facets")
	}
	if len(clean) > maxFacets {
		clean = clean[:maxFacets]
	}
	return clean, nil
}

// Enumerate runs every facet through the enumerator and merges results
// into one deduped universe: same domain from multiple facets appears
// once, with all its facets listed (comma-joined). maxCalls bounds Exa
// cost — facets beyond the cap are skipped, not errored. Rate-limit and
// other enumeration failures abort with cause and a retry hint.
func Enumerate(ctx context.Context, facets []string, e Enumerator, maxCalls int) ([]Facet, error) {
	type entry struct {
		company Company
		facets  []string
	}
	byDomain := map[string]*entry{}
	var order []string

	calls := 0
	for _, facet := range facets {
		if calls >= maxCalls {
			break // cost bound: skip silently — the cap is the contract
		}
		calls++
		companies, err := e.Companies(ctx, facet)
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") {
				return nil, fmt.Errorf("enumeration rate-limited after %d facet(s): %w — wait a minute and retry", calls, err)
			}
			return nil, fmt.Errorf("enumerate %q: %w", facet, err)
		}
		for _, c := range companies {
			domain := strings.ToLower(strings.TrimSpace(c.Domain))
			if domain == "" {
				continue
			}
			if existing, ok := byDomain[domain]; ok {
				existing.facets = appendUnique(existing.facets, facet)
				continue
			}
			cp := c
			byDomain[domain] = &entry{company: cp, facets: []string{facet}}
			order = append(order, domain)
		}
	}

	merged := make([]Company, 0, len(order))
	for _, domain := range order {
		en := byDomain[domain]
		en.company.Facets = strings.Join(en.facets, ", ")
		merged = append(merged, en.company)
	}
	return []Facet{{Name: "enumerated", Companies: merged}}, nil
}

// BriefHash is the cache key for an expanded facet list: a stable digest
// of the brief text the facets were expanded from.
func BriefHash(brief string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(brief)))
	return hex.EncodeToString(sum[:])
}

func appendUnique(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
