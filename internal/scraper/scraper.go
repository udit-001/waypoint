package scraper

import (
	"context"
	"sort"
	"strings"
)

// Result is a single job posting returned by a scraper.
type Result struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Company     string            `json:"company"`
	Location    string            `json:"location"`
	Date        string            `json:"date"`
	URL         string            `json:"url"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SearchOpts controls a scraper search.
type SearchOpts struct {
	Query    string
	Location string
	Limit    int
	JobAge   int    // days (0 = all). Supported by scrapers whose portal offers a recency filter.
	Remote   string // "remote" | "hybrid" | "onsite" ("" = any). Supported by scrapers whose portal offers a workplace-type filter.
	Page     int    // 1-indexed page number (0 = page 1). Supported by scrapers whose portal offers pagination.
}

// Scraper fetches job postings from a specific portal.
type Scraper interface {
	Name() string
	Source() string
	Categories() []string
	Search(ctx context.Context, opts SearchOpts) ([]Result, error)
}

// Detailer fetches the full detail of a single job posting.
// Optional capability — scrapers implement this only if their portal
// has a detail endpoint. The CLI type-asserts: s, ok := scraper.(Detailer).
type Detailer interface {
	Detail(ctx context.Context, id string) (*Result, error)
}

// FilterByQuery keeps only results whose Title contains the query substring
// (case-insensitive). Used by listing scrapers that don't support server-side
// keyword search — API-based scrapers (LinkedIn, Indeed, Google Jobs) filter
// server-side and should NOT call this.
func FilterByQuery(results []Result, query string) []Result {
	if query == "" {
		return results
	}
	q := strings.ToLower(query)
	filtered := results[:0]
	for _, r := range results {
		if strings.Contains(strings.ToLower(r.Title), q) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// Truncate caps results to at most limit entries (0 = no cap).
func Truncate(results []Result, limit int) []Result {
	if limit > 0 && len(results) > limit {
		return results[:limit]
	}
	return results
}

// ApplyFilters applies query substring filtering and limit to results.
// Deprecated: listing scrapers should call FilterByQuery inside their Search
// method; the CLI should call Truncate. Kept for backward compatibility with
// existing tests.
func ApplyFilters(results []Result, opts SearchOpts) []Result {
	results = FilterByQuery(results, opts.Query)
	results = Truncate(results, opts.Limit)
	return results
}

var registry = map[string]Scraper{}

// Register adds a scraper to the compile-time registry.
// Called from each scraper package's init().
func Register(s Scraper) {
	registry[s.Name()] = s
}

// Get returns a scraper by name, or false if not registered.
func Get(name string) (Scraper, bool) {
	s, ok := registry[name]
	return s, ok
}

// All returns every registered scraper, sorted by name.
func All() []Scraper {
	out := make([]Scraper, 0, len(registry))
	for _, s := range registry {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name() < out[j].Name()
	})
	return out
}
