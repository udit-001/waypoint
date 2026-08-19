// Package detail implements the detail chain that fills posting rows
// with full job-posting bodies. Tiers are tried in order — (1) board
// provider detail, (2) scraper detailer, (3) direct HTTP fetch,
// (3.5) markdown conversion services, (4) Exa fetch, then raw only.
// First success wins; provenance is recorded.
package detail

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/udit-001/waypoint/internal/scraper"
)

// DetailResult holds the fields extracted from a detail fetch. The chain
// merges these into the posting row using the precedence rules from the spec:
// sweep wins URL/date/company/title; detail wins description/salary/
// requirements/deadline/location; raw markdown always retained.
type DetailResult struct {
	// Body is the raw markdown fetched from the source (degradation path).
	Body string `json:"body"`
	// Source records which tier produced this result: "board", "scraper",
	// "exa", or "none" (raw only).
	Source string `json:"source"`

	// Structured fields extracted by parsers or provided by ATS APIs.
	Title        string            `json:"title,omitempty"`
	Company      string            `json:"company,omitempty"`
	Location     string            `json:"location,omitempty"`
	Date         string            `json:"date,omitempty"`
	Description  string            `json:"description,omitempty"`
	Salary       string            `json:"salary,omitempty"`
	Deadline     string            `json:"deadline,omitempty"`
	Requirements string            `json:"requirements,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// Fetcher fetches raw HTML/markdown from a URL. Used by the Exa tier.
type Fetcher interface {
	Fetch(ctx context.Context, url string) (string, error)
}

// BoardDetailer fetches structured detail from a board provider (ATS API).
// Wraps boards.Provider.Detail behind a narrower interface for testing.
type BoardDetailer interface {
	Detail(ctx context.Context, boardURL, id string) (scraper.Result, error)
}

// ScraperDetailer fetches detail from a scraper that has a detail endpoint.
// Wraps scraper.Detailer behind a narrower interface for testing.
type ScraperDetailer interface {
	Detail(ctx context.Context, id string) (*scraper.Result, error)
}

// ExaFetcher fetches raw markdown via the Exa MCP web_fetch_exa tool.
// Nil means Exa is unavailable (the chain skips the Exa tier).
type ExaFetcher interface {
	Fetch(ctx context.Context, rawURL string) (string, error)
}

// Parser extracts structured fields from raw markdown for a specific ATS
// family. Returns a DetailResult with only the fields the parser can
// extract; empty strings mean "not found".
type Parser interface {
	// Parse extracts structured fields from raw markdown.
	Parse(md string) DetailResult
	// Family returns the ATS family name (e.g. "greenhouse", "lever").
	Family() string
}

// DetectFamily detects the ATS family from a posting URL by examining
// the hostname. Returns "" when no family is recognized (generic page).
func DetectFamily(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())

	switch {
	case strings.Contains(host, "greenhouse.io"):
		return "greenhouse"
	case strings.Contains(host, "lever.co"):
		return "lever"
	case strings.Contains(host, "ashbyhq.com"):
		return "ashby"
	case strings.Contains(host, "linkedin.com") || strings.Contains(host, "in.linkedin.com"):
		return "linkedin"
	case strings.Contains(host, "workday.com") || strings.Contains(host, "myworkdayjobs.com"):
		return "workday"
	case strings.Contains(host, "bamboohr.com"):
		return "bamboohr"
	default:
		return ""
	}
}

// Merge merges listing fields (from the sweep) with detail fields
// (from the chain). Precedence:
//   - Listing wins: URL, Date, Company, Title (never overwritten)
//   - Detail wins when non-empty: Description, Salary, Requirements, Deadline, Location
//   - Union of non-empty for Metadata
//   - Raw body always retained from detail (degradation path)
//
// The original listing fields are passed as listingDesc/listingMeta to
// preserve them when the detail result has empty body fields.
func Merge(listing scraper.Result, detail DetailResult, listingDesc string, listingMeta map[string]string) scraper.Result {
	// Start from listing fields — listing wins for URL/date/company/title.
	merged := scraper.Result{
		ID:       listing.ID,
		Title:    listing.Title,
		Company:  listing.Company,
		Date:     listing.Date,
		URL:      listing.URL,
		Location: listing.Location,
	}

	// Detail wins for body fields when non-empty.
	if detail.Description != "" {
		merged.Description = detail.Description
	} else {
		merged.Description = listingDesc
	}

	if detail.Location != "" {
		merged.Location = detail.Location
	}

	// Union of metadata: listing base + detail overlay.
	merged.Metadata = mergeMetadata(listingMeta, detail.Metadata)

	// Add structured detail fields to metadata.
	if detail.Salary != "" {
		if merged.Metadata == nil {
			merged.Metadata = map[string]string{}
		}
		merged.Metadata["salary"] = detail.Salary
	}
	if detail.Deadline != "" {
		if merged.Metadata == nil {
			merged.Metadata = map[string]string{}
		}
		merged.Metadata["deadline"] = detail.Deadline
	}
	if detail.Requirements != "" {
		if merged.Metadata == nil {
			merged.Metadata = map[string]string{}
		}
		merged.Metadata["requirements"] = detail.Requirements
	}

	// Record provenance.
	if detail.Source != "" {
		if merged.Metadata == nil {
			merged.Metadata = map[string]string{}
		}
		merged.Metadata["detail_source"] = detail.Source
	}

	return merged
}

// mergeMetadata returns the union of two metadata maps, with b overlaying a.
func mergeMetadata(a, b map[string]string) map[string]string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	out := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// MergeDetailResultInto enriches a posting's description and metadata
// in-place, following the same precedence rules as Merge. This is the
// convenience version used when you already have a Posting and want to
// apply detail fields to it.
func MergeDetailResultInto(desc string, meta map[string]string, detail DetailResult) (string, map[string]string) {
	// Structured description wins; a raw body still beats empty — the
	// degradation path (generic page, no parser) must not be discarded.
	if detail.Description != "" {
		desc = detail.Description
	} else if detail.Body != "" && desc == "" {
		desc = detail.Body
	}

	meta = mergeMetadata(meta, detail.Metadata)

	if detail.Salary != "" {
		if meta == nil {
			meta = map[string]string{}
		}
		meta["salary"] = detail.Salary
	}
	if detail.Deadline != "" {
		if meta == nil {
			meta = map[string]string{}
		}
		meta["deadline"] = detail.Deadline
	}
	if detail.Requirements != "" {
		if meta == nil {
			meta = map[string]string{}
		}
		meta["requirements"] = detail.Requirements
	}
	if detail.Source != "" {
		if meta == nil {
			meta = map[string]string{}
		}
		meta["detail_source"] = detail.Source
	}

	return desc, meta
}

// RawDetail returns a DetailResult with only the raw body — used when all
// other tiers fail. The body must be provided by the caller (typically
// fetched via a scraper or the markdown services).
func RawDetail(body string) DetailResult {
	return DetailResult{
		Body:   body,
		Source: "none",
	}
}

// BodyFieldsFilled reports whether the listing already has all body
// fields populated (description, salary, deadline, requirements).
// When true, the Exa fetch tier can be skipped.
func BodyFieldsFilled(r scraper.Result) bool {
	if r.Description == "" {
		return false
	}
	if r.Metadata != nil {
		if r.Metadata["salary"] == "" || r.Metadata["deadline"] == "" {
			return false
		}
	} else {
		return false
	}
	return true
}

// IsLinkedInURL reports whether the URL belongs to LinkedIn (login-walled).
// LinkedIn URLs must never enter the Exa tier.
func IsLinkedInURL(rawURL string) bool {
	return DetectFamily(rawURL) == "linkedin"
}

// init registers the built-in parsers.
func init() {
	RegisterParser(&greenhouseParser{})
	RegisterParser(&leverParser{})
	RegisterParser(&ashbyParser{})
}

// parserRegistry holds all registered parsers, keyed by family.
var parserRegistry = map[string]Parser{}

// RegisterParser adds a parser to the registry.
func RegisterParser(p Parser) {
	parserRegistry[p.Family()] = p
}

// ParserFor returns the parser for the given family, or nil.
func ParserFor(family string) Parser {
	return parserRegistry[family]
}

// formatError is a helper for user-friendly error messages.
func formatError(msg string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", msg, err)
	}
	return fmt.Errorf("%s", msg)
}
