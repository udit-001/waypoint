package detail

import (
	"context"
	"fmt"
	"strings"

	"github.com/udit-001/waypoint/internal/scraper"
)

// Chain orchestrates the detail fetch for a single posting URL.
// Tiers are tried in order; first success wins. The chain records
// provenance via the detail_source metadata field.
//
// Tier order (cheap and precise first, rate-limited last):
//  1. Board provider detail — structured ATS JSON (free, precise).
//  2. Scraper detailer — portal detail endpoint (free).
//  3. Direct HTTP fetch — our own fetcher, per-family parsing (free).
//  4. Exa — hosted MCP web_fetch (anonymous tier ~50 calls/day/IP;
//     last resort by design).
type Chain struct {
	// BoardDetail fetches structured JSON from an ATS board provider.
	// May be nil (skips tier 1).
	BoardDetail BoardDetailer
	// ScraperDetail fetches from a scraper that has a detail endpoint.
	// May be nil (skips tier 2).
	ScraperDetail ScraperDetailer
	// Direct fetches raw HTML via plain HTTP (tier 3). May be nil.
	Direct Fetcher
	// Markdown fetches JS-rendered pages as markdown via markdown.new
	// (tier 3.5, free). May be nil.
	Markdown Fetcher
	// Exa fetches raw markdown via Exa's web_fetch_exa tool (tier 4,
	// rate-limited last resort). May be nil.
	Exa ExaFetcher

	// exaCap limits the number of Exa fetches per chain run (per-cycle).
	// 0 means unlimited (default; the caller should set this).
	exaCap   int
	exaCount int
}

// NewChain creates a detail chain with the given dependencies. Order:
// board, scraper, direct HTTP fetcher, markdown services (mdsvc), Exa
// (last resort).
func NewChain(board BoardDetailer, scraper ScraperDetailer, direct, markdown Fetcher, exa ExaFetcher) *Chain {
	return &Chain{
		BoardDetail:   board,
		ScraperDetail: scraper,
		Direct:        direct,
		Markdown:      markdown,
		Exa:           exa,
	}
}

// SetExaCap sets the maximum number of Exa fetches allowed. Must be
// called before Fetch.
func (c *Chain) SetExaCap(n int) {
	c.exaCap = n
	c.exaCount = 0
}

// FetchDetail runs the four-tier chain for a single posting.
//
// Parameters:
//   - rawURL: the posting URL (used to detect ATS family and as board ID)
//   - boardURL: the board's careers URL (used by board detailer to derive API)
//   - boardID: the posting ID on the board (used by board + scraper detailers)
//   - existingDesc: description already in the posting (from sweep)
//   - existingMeta: metadata already in the posting (from sweep)
//
// Returns a DetailResult with the fields the chain could extract. The
// caller should merge with the posting's existing fields using Merge().
func (c *Chain) FetchDetail(ctx context.Context, rawURL, boardURL, boardID, existingDesc string, existingMeta map[string]string) (DetailResult, error) {
	// Tier 1: Board provider detail (structured ATS JSON).
	if c.BoardDetail != nil && boardURL != "" && boardID != "" {
		if r, err := c.fetchBoard(ctx, boardURL, boardID); err == nil && r.Description != "" {
			return r, nil
		}
		// Board failed or returned no description — fall through.
	}

	// Tier 2: Scraper detailer (portal with detail endpoint).
	if c.ScraperDetail != nil && boardID != "" {
		if r, err := c.fetchScraper(ctx, boardID); err == nil && r.Description != "" {
			return r, nil
		}
	}

	// Tier 3: Direct HTTP fetch + per-family parsing (free, preferred).
	// Skip when: no fetcher, LinkedIn URL (login-walled), or body filled.
	if c.Direct != nil && !IsLinkedInURL(rawURL) && !BodyFieldsFilledFrom(existingDesc, existingMeta) {
		if body, err := c.Direct.Fetch(ctx, rawURL); err == nil && body != "" {
			return c.parseBody(rawURL, body, "direct"), nil
		}
	}

	// Tier 3.5: markdown.new — headless-browser rendering for JS-heavy
	// pages the direct fetcher can't parse (free, no budget).
	if c.Markdown != nil && !IsLinkedInURL(rawURL) && !BodyFieldsFilledFrom(existingDesc, existingMeta) {
		if body, err := c.Markdown.Fetch(ctx, rawURL); err == nil && body != "" {
			return c.parseBody(rawURL, body, "mdsvc"), nil
		}
	}

	// Tier 4: Exa fetch + per-family parsing (rate-limited; last resort).
	// Skip when: no Exa client, LinkedIn URL, or body already filled.
	if c.Exa != nil && !IsLinkedInURL(rawURL) && !BodyFieldsFilledFrom(existingDesc, existingMeta) {
		if c.exaCap > 0 && c.exaCount >= c.exaCap {
			// Exa cap reached — skip to raw.
		} else if r, err := c.fetchExa(ctx, rawURL); err == nil && r.Body != "" {
			c.exaCount++
			return r, nil
		}
	}

	// Tier 4: Raw only — no structured extraction, just the body if available.
	// The degradation path: empty structured fields + full text still lets
	// curation judge everything.
	return RawDetail(existingDesc), nil
}

// fetchBoard tries the board provider detail endpoint.
func (c *Chain) fetchBoard(ctx context.Context, boardURL, boardID string) (DetailResult, error) {
	result, err := c.BoardDetail.Detail(ctx, boardURL, boardID)
	if err != nil {
		return DetailResult{}, fmt.Errorf("board detail: %w", err)
	}
	return DetailResult{
		Body:        result.Description,
		Source:      "board",
		Title:       result.Title,
		Company:     result.Company,
		Location:    result.Location,
		Date:        result.Date,
		Description: result.Description,
		Metadata:    result.Metadata,
	}, nil
}

// fetchScraper tries the scraper's detail endpoint.
func (c *Chain) fetchScraper(ctx context.Context, id string) (DetailResult, error) {
	result, err := c.ScraperDetail.Detail(ctx, id)
	if err != nil {
		return DetailResult{}, fmt.Errorf("scraper detail: %w", err)
	}
	if result == nil {
		return DetailResult{}, fmt.Errorf("scraper detail: nil result")
	}
	return DetailResult{
		Body:        result.Description,
		Source:      "scraper",
		Title:       result.Title,
		Company:     result.Company,
		Location:    result.Location,
		Date:        result.Date,
		Description: result.Description,
		Metadata:    result.Metadata,
	}, nil
}

// maxDetailBodyChars caps a fetched posting body before it enters the
// DB. Some board pages inline nav/scripts that survive conversion;
// 20k chars is ~10x a real posting body and keeps rows lean.
const maxDetailBodyChars = 20000

// parseBody runs the per-family parser over a raw body and stamps the
// tier that fetched it as provenance (parsers no longer hardcode a
// source — the chain knows which tier ran).
func (c *Chain) parseBody(rawURL, body, source string) DetailResult {
	if len(body) > maxDetailBodyChars {
		body = body[:maxDetailBodyChars]
	}
	// Tiers fetch different media: direct HTTP returns raw HTML,
	// mdsvc/Exa return markdown. Parsers (and the description column)
	// speak markdown — convert before parsing so an HTML body never
	// reaches the parser or the DB.
	if looksLikeHTML(body) {
		body = scraper.HTMLToMarkdown(body)
		if len(body) > maxDetailBodyChars {
			body = body[:maxDetailBodyChars]
		}
	}
	family := DetectFamily(rawURL)
	if parser := ParserFor(family); parser != nil {
		r := parser.Parse(body)
		r.Body = body
		r.Source = source
		return r
	}
	return DetailResult{Body: body, Source: source}
}

// looksLikeHTML reports whether a fetched body is a full HTML page
// rather than converted markdown. Cheap prefix/structure sniff — the
// only decision it feeds is whether to run the HTML→markdown pass.
func looksLikeHTML(body string) bool {
	s := body
	if len(s) > 4096 {
		s = s[:4096]
	}
	t := strings.TrimLeft(s, " \t\r\n")
	low := strings.ToLower(t)
	if strings.HasPrefix(low, "<!doctype") || strings.HasPrefix(low, "<html") {
		return true
	}
	// Fragment heuristic: multiple block-level tags means HTML.
	return strings.Contains(low, "<div") && strings.Contains(low, "<p")
}

// fetchExa fetches raw markdown via Exa and parses it with a per-family parser.
func (c *Chain) fetchExa(ctx context.Context, rawURL string) (DetailResult, error) {
	body, err := c.Exa.Fetch(ctx, rawURL)
	if err != nil {
		return DetailResult{}, fmt.Errorf("exa fetch: %w", err)
	}
	if body == "" {
		return DetailResult{}, fmt.Errorf("exa fetch: empty body")
	}
	return c.parseBody(rawURL, body, "exa"), nil
}

// BodyFieldsFilledFrom checks if body fields (description + salary +
// deadline) are already present in the existing data. This is the
// extended version that checks metadata for salary/deadline.
func BodyFieldsFilledFrom(desc string, meta map[string]string) bool {
	if desc == "" {
		return false
	}
	if meta == nil {
		return false
	}
	return meta["salary"] != "" && meta["deadline"] != ""
}

// --- fake implementations for testing ---

// FakeBoardDetailer is a test double for BoardDetailer.
type FakeBoardDetailer struct {
	Results map[string]DetailResult // key: boardID
	Err     error
}

func (f *FakeBoardDetailer) Detail(ctx context.Context, boardURL, id string) (scraperResult interface{}, err error) {
	// This is intentionally not the real signature — we'll use the real
	// scraper.Result type. For testing, we return the stored result.
	if f.Err != nil {
		return nil, f.Err
	}
	r, ok := f.Results[id]
	if !ok {
		return nil, fmt.Errorf("board detail: unknown ID %q", id)
	}
	// Convert DetailResult to scraper.Result-like shape.
	return struct {
		Title, Company, Location, Date, URL, Description string
		Metadata                                         map[string]string
	}{
		Title:       r.Title,
		Company:     r.Company,
		Location:    r.Location,
		Date:        r.Date,
		Description: r.Description,
		Metadata:    r.Metadata,
	}, nil
}

// FormatError formats a user-friendly error message.
func FormatError(msg string, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", msg, err)
	}
	return fmt.Errorf("%s", msg)
}

// NormalizeBoardID extracts the posting ID from a URL. For board URLs,
// this is the numeric ID in the path. For other URLs, returns the full URL.
func NormalizeBoardID(rawURL string) string {
	// Try common patterns: /jobs/{id}, /{id}, last path segment
	parts := strings.Split(rawURL, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return rawURL
}
