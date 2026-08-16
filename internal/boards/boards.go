// Package boards fetches job postings from company ATS boards
// (Greenhouse, Workday, Lever, BambooHR) and normalizes them into
// scraper.Result records. A board is one company's careers site behind one
// vendor. Vendors self-register through an interface; the registry is
// compile-time, mirroring the scraper registry in internal/scraper.
package boards

import (
	"context"
	"errors"
	"sort"

	"github.com/udit-001/waypoint/internal/scraper"
)

// Board is one company ATS board as stored in boards.toml.
type Board struct {
	Name     string
	Company  string
	URL      string
	MaxPages int
	Enabled  bool
}

// FetchOpts controls a single provider fetch.
type FetchOpts struct {
	// JobAgeDays limits results to postings from the last N days (0 = all).
	JobAgeDays int
	// MaxPages caps pagination. Liveness probes set this to 1.
	MaxPages int
	// Limit caps the number of results returned (0 = no cap).
	Limit int
}

// DetectHit is the result of a provider claiming a board.
type DetectHit struct {
	// API is the derived JSON API URL for the board. Empty when the
	// provider must probe before it knows (Workday without an instance).
	API string
}

// Provider is the seam: detect a board's API URL from its careers URL,
// fetch postings for it, and enrich one posting with its full body.
// Implementations live in this package; the interface keeps verification,
// sweep, and detail code blind to vendor specifics.
type Provider interface {
	// Name is the provider id, unique across registered providers.
	Name() string
	// Detect claims the board. Returns a DetectHit when the board matches,
	// or (nil, nil) when it does not.
	Detect(b Board) (*DetectHit, error)
	// Fetch fetches and normalizes postings from a claimed board. The
	// list is lean (title, location, date if the list exposes one), no
	// descriptions. Descriptions and richer metadata come from Detail.
	Fetch(ctx context.Context, b Board, hit DetectHit, opts FetchOpts) ([]scraper.Result, error)
	// Detail fetches the full body for one posting. The id is the
	// scraper.Result.ID returned by Fetch. Detail enrichment is a
	// deliberate, on-demand step the agent runs on the few postings it's
	// seriously considering — not an automatic part of sweep.
	Detail(ctx context.Context, b Board, id string) (scraper.Result, error)
}

// ErrNotMatched is returned by DetectProvider when no registered provider
// recognizes the board URL.
var ErrNotMatched = errors.New("no provider matches the board URL")

var registry = map[string]Provider{}

// Register adds a provider to the compile-time registry.
func Register(p Provider) { registry[p.Name()] = p }

// All returns every registered provider, by name.
func All() []Provider {
	out := make([]Provider, 0, len(registry))
	for _, p := range registry {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
	return out
}

// DetectProvider returns the provider whose Detect claims the board.
func DetectProvider(b Board) (Provider, *DetectHit, error) {
	for _, p := range All() {
		hit, err := p.Detect(b)
		if err != nil {
			continue // a provider may reject a parseable but unhandled host
		}
		if hit != nil {
			return p, hit, nil
		}
	}
	return nil, nil, ErrNotMatched
}
