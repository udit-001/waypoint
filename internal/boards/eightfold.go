// Package boards implementation: Eightfold provider.
//
// Eightfold (eightfold.ai) is an ATS platform whose customers host their
// careers sites at <tenant>.eightfold.ai. The careers SPA loads postings
// from /api/pcsx/search and full descriptions from
// /api/pcsx/position_details — open JSON GETs, no cookies or session.
//
// Reverse-engineered via network capture + cross-tenant probing
// (2026-08-19), validated against citi, juniper, nvidia, ericsson:
//
//  1. The search API REQUIRES a `domain` param — the customer's primary
//     domain, which is NOT the hostname prefix (host "citi" → domain
//     "citi.com"). The board URL alone cannot derive it; the careers
//     page embeds it in a hidden <code id="pcsx-data"> block, so Fetch
//     probes the page once to discover it (mirrors Workday's instance
//     auto-probe).
//  2. `num` is hard-capped at 10 per request — pagination via
//     start/num is mandatory.
//  3. Empty query/location returns the whole board (count in
//     data.count); postedTs is epoch seconds.
//  4. position_details needs no domain — only the numeric position id.
package boards

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/udit-001/waypoint/internal/scraper"
)

// eightfoldHostRE matches a careers host of the form <tenant>.eightfold.ai.
var eightfoldHostRE = regexp.MustCompile(`^([a-z0-9-]+)\.eightfold\.ai$`)

// eightfoldPageSize is the server's hard cap on `num` per search request.
const eightfoldPageSize = 10

const (
	eightfoldMaxPages     = 500 // default sweep depth (boards without max_pages)
	eightfoldHardMaxPages = 500 // absolute guard: 500 pages × 10 = 5000 postings
)

// pcsxDataRE matches the hidden careers-page config block that embeds the
// tenant domain: <code id="pcsx-data">{"domain": "customer.com", ...}</code>.
// The JSON is HTML-entity-escaped (&#34; for quotes), so unescape before
// parsing.
var pcsxDataRE = regexp.MustCompile(`(?s)id="pcsx-data"[^>]*>(.*?)</code>`)

// Eightfold scrapes employers whose careers site runs on the Eightfold
// platform. Fetcher is the seam tests inject.
type Eightfold struct {
	Fetcher JSONFetcher
}

func init() {
	Register(Eightfold{})
}

func (Eightfold) Name() string { return "eightfold" }

// Detect claims a Board whose URL is a <tenant>.eightfold.ai host. The API
// pin is empty: the search endpoint needs the tenant domain, which is only
// discoverable by probing the careers page, so Fetch resolves it (mirrors
// Workday's unpinned-instance probe).
func (e Eightfold) Detect(b Board) (*DetectHit, error) {
	if b.URL == "" {
		return nil, nil
	}
	u, err := url.Parse(b.URL)
	if err != nil {
		return nil, err
	}
	if !eightfoldHostRE.MatchString(u.Hostname()) {
		return nil, nil
	}
	return &DetectHit{API: ""}, nil
}

// policy returns the SSRF host policy for Eightfold: only
// <tenant>.eightfold.ai hosts are ever fetched.
func (Eightfold) policy() HostPolicy {
	return func(raw string) error {
		u, err := url.Parse(raw)
		if err != nil {
			return err
		}
		if u.Scheme != "https" {
			return fmt.Errorf("eightfold: URL must use https")
		}
		if !eightfoldHostRE.MatchString(u.Hostname()) {
			return fmt.Errorf("eightfold: untrusted hostname %q", u.Hostname())
		}
		return nil
	}
}

// coord captures a board's careers host.
type eightfoldCoord struct {
	host   string
	tenant string
}

// coordFromBoard parses the <tenant>.eightfold.ai host out of a board URL.
func eightfoldCoordFromBoard(b Board) (eightfoldCoord, bool) {
	if b.URL == "" {
		return eightfoldCoord{}, false
	}
	u, err := url.Parse(b.URL)
	if err != nil {
		return eightfoldCoord{}, false
	}
	m := eightfoldHostRE.FindStringSubmatch(u.Hostname())
	if m == nil {
		return eightfoldCoord{}, false
	}
	return eightfoldCoord{host: u.Hostname(), tenant: m[1]}, true
}

// domainFromCareers extracts the tenant `domain` from the careers page
// HTML. The SPA embeds the board config in <code id="pcsx-data"> as
// HTML-escaped JSON, e.g. {&#34;domain&#34;: &#34;citi.com&#34;, ...}.
func domainFromCareers(page []byte) (string, error) {
	m := pcsxDataRE.FindSubmatch(page)
	if m == nil {
		return "", fmt.Errorf("eightfold: careers page has no pcsx-data config")
	}
	raw := html.UnescapeString(string(m[1]))
	var cfg struct {
		Domain string `json:"domain"`
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return "", fmt.Errorf("eightfold: parse pcsx-data config: %w", err)
	}
	if cfg.Domain == "" {
		return "", fmt.Errorf("eightfold: pcsx-data config has no domain")
	}
	return cfg.Domain, nil
}

// searchURL builds the pcsx search endpoint for a page offset.
func (c eightfoldCoord) searchURL(domain string, start int) string {
	return fmt.Sprintf("https://%s/api/pcsx/search?domain=%s&query=&location=&start=%d&num=%d&sort_by=newest",
		c.host, domain, start, eightfoldPageSize)
}

// searchPosition is one row in the pcsx search response.
type searchPosition struct {
	ID               int64    `json:"id"`
	Name             string   `json:"name"`
	StandardizedLocs []string `json:"standardizedLocations"`
	PostedTS         int64    `json:"postedTs"`
	Department       string   `json:"department"`
	ATSJobID         string   `json:"atsJobId"`
	WorkLocationOpt  string   `json:"workLocationOption"`
}

// searchResponse is the pcsx search envelope.
type searchResponse struct {
	Status int `json:"status"`
	Data   struct {
		Count     int              `json:"count"`
		Positions []searchPosition `json:"positions"`
	} `json:"data"`
}

// Fetch probes the careers page for the tenant domain, then paginates the
// search endpoint (num capped at 10/server) until the board's page budget or
// the full posting count is reached. Filtering is server-side only — empty
// query/location returns the whole board; recency is applied here both as a
// pagination early-stop (pages sort newest-first, so once a full page
// predates the window nothing later can be fresh) and a final trim.
func (e Eightfold) Fetch(ctx context.Context, b Board, hit DetectHit, opts FetchOpts) ([]scraper.Result, error) {
	f := e.Fetcher
	if f == nil {
		f = &HTTPFetcher{}
	}
	coord, ok := eightfoldCoordFromBoard(b)
	if !ok {
		return nil, fmt.Errorf("eightfold: board URL lost its host")
	}

	// Probe the careers page for the tenant domain (the search API 400s
	// without it and the hostname prefix is never the domain).
	page, err := f.GetJSON(ctx, b.URL, e.policy())
	if err != nil {
		return nil, fmt.Errorf("eightfold: probe careers page: %w", err)
	}
	domain, err := domainFromCareers(page)
	if err != nil {
		return nil, err
	}

	maxPages := opts.MaxPages
	if maxPages <= 0 {
		maxPages = b.MaxPages
	}
	if maxPages <= 0 {
		maxPages = eightfoldMaxPages
	}
	if maxPages > eightfoldHardMaxPages {
		maxPages = eightfoldHardMaxPages
	}

	var results []scraper.Result
	fetched := 0

	// Recency cutoff: pages sort newest-first, so once a full page's newest
	// posting predates the window, every later page would be filtered out
	// too — stop paging and let FilterByRecency do final trimming.
	var cutoff time.Time
	if opts.JobAgeDays > 0 {
		cutoff = time.Now().UTC().AddDate(0, 0, -opts.JobAgeDays)
	}

	for page := 0; page < maxPages; page++ {
		start := page * eightfoldPageSize
		respBytes, err := f.GetJSON(ctx, coord.searchURL(domain, start), e.policy())
		if err != nil {
			return nil, fmt.Errorf("eightfold: search page %d: %w", page, err)
		}
		var resp searchResponse
		if err := json.Unmarshal(respBytes, &resp); err != nil {
			return nil, fmt.Errorf("eightfold: decode search page %d: %w", page, err)
		}
		if resp.Status != 200 {
			return nil, fmt.Errorf("eightfold: search returned status %d", resp.Status)
		}
		positions := resp.Data.Positions
		fetched += len(positions)
		for _, p := range positions {
			results = append(results, e.result(coord, b.Company, p))
		}
		if len(positions) < eightfoldPageSize || fetched >= resp.Data.Count {
			break
		}
		// Newest-first stop: a full page whose first row already predates the
		// window means every remaining page is older still.
		if !cutoff.IsZero() && len(positions) > 0 && time.Unix(positions[0].PostedTS, 0).Before(cutoff) {
			break
		}
	}

	results = scraper.FilterByRecency(results, opts.JobAgeDays, time.Time{})
	return scraper.Truncate(results, opts.Limit), nil
}

// result normalizes one search row. IDs are the numeric position id — the
// key for /careers/job/<id> and position_details.
func (e Eightfold) result(coord eightfoldCoord, company string, p searchPosition) scraper.Result {
	loc := strings.Join(p.StandardizedLocs, ", ")
	meta := map[string]string{}
	if p.Department != "" {
		meta["department"] = p.Department
	}
	if p.ATSJobID != "" {
		meta["ats_job_id"] = p.ATSJobID
	}
	if p.WorkLocationOpt != "" {
		meta["work_location"] = p.WorkLocationOpt
	}
	r := scraper.Result{
		ID:       strconv.FormatInt(p.ID, 10),
		Title:    strings.TrimSpace(p.Name),
		Company:  company,
		Location: loc,
		URL:      fmt.Sprintf("https://%s/careers/job/%d", coord.host, p.ID),
		Metadata: meta,
	}
	if p.PostedTS > 0 {
		r.Date = time.Unix(p.PostedTS, 0).Format("2006-01-02")
	}
	return r
}

// detailResponse is the position_details envelope.
type detailResponse struct {
	Data struct {
		Name             string   `json:"name"`
		JobDescription   string   `json:"jobDescription"`
		PublicURL        string   `json:"publicUrl"`
		StandardizedLocs []string `json:"standardizedLocations"`
	} `json:"data"`
}

// Detail fetches the full description for one posting via
// /api/pcsx/position_details — no domain needed, only the numeric id.
func (e Eightfold) Detail(ctx context.Context, b Board, id string) (scraper.Result, error) {
	coord, ok := eightfoldCoordFromBoard(b)
	if !ok {
		return scraper.Result{}, fmt.Errorf("eightfold: board URL lost its host")
	}
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		return scraper.Result{}, fmt.Errorf("eightfold: invalid position id %q", id)
	}
	f := e.Fetcher
	if f == nil {
		f = &HTTPFetcher{}
	}
	endpoint := fmt.Sprintf("https://%s/api/pcsx/position_details?position_id=%s&hl=en", coord.host, id)
	respBytes, err := f.GetJSON(ctx, endpoint, e.policy())
	if err != nil {
		return scraper.Result{}, fmt.Errorf("eightfold: detail: %w", err)
	}
	var d detailResponse
	if err := json.Unmarshal(respBytes, &d); err != nil {
		return scraper.Result{}, fmt.Errorf("eightfold: decode detail: %w", err)
	}
	if d.Data.Name == "" && d.Data.JobDescription == "" {
		return scraper.Result{}, fmt.Errorf("eightfold: position %s not found", id)
	}
	result := scraper.Result{
		ID:          id,
		Title:       strings.TrimSpace(d.Data.Name),
		Company:     b.Company,
		Location:    strings.Join(d.Data.StandardizedLocs, ", "),
		URL:         strings.TrimSpace(d.Data.PublicURL),
		Description: strings.TrimSpace(scraper.HTMLToMarkdown(d.Data.JobDescription)),
	}
	if result.URL == "" {
		result.URL = fmt.Sprintf("https://%s/careers/job/%s", coord.host, id)
	}
	return result, nil
}
