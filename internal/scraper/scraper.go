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
}

// Scraper fetches job postings from a specific portal.
type Scraper interface {
	Name() string
	Source() string
	Categories() []string
	Search(ctx context.Context, opts SearchOpts) ([]Result, error)
}

// ApplyFilters applies query substring filtering and limit to results.
// Called by the CLI after Search() returns — scrapers themselves don't filter.
func ApplyFilters(results []Result, opts SearchOpts) []Result {
	if opts.Query != "" {
		q := strings.ToLower(opts.Query)
		filtered := results[:0]
		for _, r := range results {
			if strings.Contains(strings.ToLower(r.Title), q) {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}
	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
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
