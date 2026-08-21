// Package discovery enumerates candidate companies and maps their
// careers pages onto ATS boards. Discover is the module's whole
// interface: callers hand in a facet list (plain data — hardcoded
// facets today, Exa+zen enumeration later) and the set of already
// watched board URLs; back come verified, unwatched candidates ready
// for the discovery ledger.
//
// The pipeline per company mirrors the validated prototype:
//
//	fetch careers page(s) → extract ATS board links → verify liveness
//	through the boards registry → drop watched boards and empty companies
package discovery

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/udit-001/waypoint/internal/boards"
	"github.com/udit-001/waypoint/internal/scraper"
)

// Company is one facet entry: a company to probe.
type Company struct {
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

// Facet is a named slice of the company universe (e.g. "ratings",
// "payments-fintech"). Facets are plain data so any enumerator can
// produce them.
type Facet struct {
	Name      string    `json:"name"`
	Companies []Company `json:"companies"`
}

// BoardLink is one ATS board link extracted from a careers page.
type BoardLink struct {
	Provider string // greenhouse | lever | ashby | workday | eightfold
	URL      string // canonical board URL
}

// Candidate is a discovered company with its verified, unwatched boards.
type Candidate struct {
	Name   string      `json:"name"`
	Domain string      `json:"domain"`
	Facet  string      `json:"facet"`
	Boards []BoardLink `json:"boards"`
}

// Options tunes politeness and injects test seams.
type Options struct {
	// Fetcher fetches careers-page HTML. nil uses scraper.HTTPFetcher.
	Fetcher scraper.Fetcher
	// Concurrency caps parallel company probes. 0 defaults to 4.
	Concurrency int
}

// Extraction regexes. Greenhouse/Lever/Ashby capture the board token;
// Workday captures tenant + instance + site slug (an optional locale
// prefix is skipped); Eightfold captures the tenant subdomain. All are
// case-insensitive — careers pages mix cases freely.
var (
	greenhouseRE = regexp.MustCompile(`(?i)(?:job-boards|boards)\.greenhouse\.io/([a-z0-9_-]+)`)
	leverRE      = regexp.MustCompile(`(?i)jobs\.lever\.co/([a-z0-9_-]+)`)
	ashbyRE      = regexp.MustCompile(`(?i)jobs\.ashbyhq\.com/([a-z0-9_-]+)`)
	workdayRE    = regexp.MustCompile(`(?i)([a-z0-9_-]+)\.(wd[0-9]+)\.myworkdayjobs\.com/(?:[a-z]{2}-[A-Za-z]{2}/)?([A-Za-z0-9_-]+)`)
	eightfoldRE  = regexp.MustCompile(`(?i)([a-z0-9-]+)\.eightfold\.ai`)
)

// ExtractBoards finds ATS board links in careers-page HTML and returns
// one canonical URL per hit, deduplicated, in stable provider order.
// Pure: no network, fully fixture-testable.
func ExtractBoards(html string) []BoardLink {
	var out []BoardLink
	seen := map[string]bool{}
	add := func(provider, url string) {
		if seen[url] {
			return
		}
		seen[url] = true
		out = append(out, BoardLink{Provider: provider, URL: url})
	}

	for _, m := range greenhouseRE.FindAllStringSubmatch(html, -1) {
		add("greenhouse", fmt.Sprintf("https://job-boards.greenhouse.io/%s/", m[1]))
	}
	for _, m := range leverRE.FindAllStringSubmatch(html, -1) {
		add("lever", fmt.Sprintf("https://jobs.lever.co/%s/", m[1]))
	}
	for _, m := range ashbyRE.FindAllStringSubmatch(html, -1) {
		add("ashby", fmt.Sprintf("https://jobs.ashbyhq.com/%s/", m[1]))
	}
	for _, m := range workdayRE.FindAllStringSubmatch(html, -1) {
		add("workday", fmt.Sprintf("https://%s.%s.myworkdayjobs.com/%s/", m[1], strings.ToLower(m[2]), m[3]))
	}
	for _, m := range eightfoldRE.FindAllStringSubmatch(html, -1) {
		add("eightfold", fmt.Sprintf("https://%s.eightfold.ai/careers", m[1]))
	}
	return out
}

// careersURLs is the fallback ladder of likely careers-page locations.
func careersURLs(domain string) []string {
	return []string{
		fmt.Sprintf("https://%s/careers", domain),
		fmt.Sprintf("https://careers.%s/", domain),
	}
}

// quickWin providers short-circuit the ladder: once one of these shows
// up, further careers URLs are unlikely to add anything.
var quickWin = map[string]bool{"greenhouse": true, "lever": true, "ashby": true}

// verifyBoard probes one board link for liveness through the existing
// boards registry seam (Detect claims it, a first-page Fetch proves the
// API answers). Function variable so package tests can stub network
// verification.
var verifyBoard = func(ctx context.Context, l BoardLink) bool {
	b := boards.Board{URL: l.URL, Enabled: true}
	p, hit, err := boards.DetectProvider(b)
	if err != nil {
		return false // no provider claims it (e.g. ashby today)
	}
	pctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err = p.Fetch(pctx, b, *hit, boards.FetchOpts{MaxPages: 1, Limit: 5})
	return err == nil
}

// Discover runs the full pipeline over every facet: probe each company's
// careers pages, extract and verify board links, filter already-watched
// URLs, and drop companies left with nothing. Companies keep facet order;
// output is deterministic regardless of concurrency.
//
// watched holds board URLs already configured (boards.toml); comparison
// ignores trailing slashes and letter case.
func Discover(ctx context.Context, facets []Facet, watched map[string]bool, opts Options) ([]Candidate, error) {
	fetcher := opts.Fetcher
	if fetcher == nil {
		fetcher = &scraper.HTTPFetcher{}
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}

	normalize := func(u string) string {
		u = strings.TrimSpace(strings.ToLower(u))
		return strings.TrimSuffix(u, "/")
	}
	watchedSet := make(map[string]bool, len(watched))
	for u := range watched {
		watchedSet[normalize(u)] = true
	}

	type job struct {
		facet string
		c     Company
	}
	var jobs []job
	for _, f := range facets {
		for _, c := range f.Companies {
			jobs = append(jobs, job{facet: f.Name, c: c})
		}
	}

	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	out := make([]Candidate, 0, len(jobs))
	var wg sync.WaitGroup

	for _, j := range jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			cand := probeCompany(ctx, fetcher, j.facet, j.c, watchedSet, normalize)
			if cand == nil {
				return
			}
			mu.Lock()
			out = append(out, *cand)
			mu.Unlock()
		}(j)
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Deterministic output: facet order as given, then company name.
	rank := map[string]int{}
	for i, f := range facets {
		rank[f.Name] = i
	}
	sortCands(out, rank)
	return out, nil
}

// probeCompany fetches the careers ladder for one company and returns a
// candidate, or nil when nothing survives extraction, verification, and
// the watched filter.
func probeCompany(ctx context.Context, fetcher scraper.Fetcher, facet string, c Company, watched map[string]bool, normalize func(string) string) *Candidate {
	var links []BoardLink
	for i, u := range careersURLs(c.Domain) {
		html, err := fetcher.Fetch(ctx, u)
		if err != nil || html == "" {
			continue
		}
		links = append(links, ExtractBoards(html)...)
		if i == 0 && hasAny(links, quickWin) {
			break
		}
	}

	var verified []BoardLink
	for _, l := range links {
		if watched[normalize(l.URL)] {
			continue
		}
		if !verifyBoard(ctx, l) {
			continue
		}
		verified = append(verified, l)
	}
	if len(verified) == 0 {
		return nil
	}
	return &Candidate{Name: c.Name, Domain: c.Domain, Facet: facet, Boards: verified}
}

func hasAny(links []BoardLink, set map[string]bool) bool {
	for _, l := range links {
		if set[l.Provider] {
			return true
		}
	}
	return false
}

// sortCands orders candidates by facet rank then company name, in place.
func sortCands(cands []Candidate, rank map[string]int) {
	// insertion sort keeps this dependency-free and stable enough for
	// the small candidate counts involved
	for i := 1; i < len(cands); i++ {
		for k := i; k > 0 && less(cands[k], cands[k-1], rank); k-- {
			cands[k], cands[k-1] = cands[k-1], cands[k]
		}
	}
}

func less(a, b Candidate, rank map[string]int) bool {
	ra, rb := rank[a.Facet], rank[b.Facet]
	if ra != rb {
		return ra < rb
	}
	return a.Name < b.Name
}
