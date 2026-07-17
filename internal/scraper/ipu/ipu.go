package ipu

import (
	"context"
	"regexp"
	"strings"

	"github.com/udit-001/waypoint/internal/scraper"
)

const (
	listingURL = "https://www.ipu.ac.in/careers.php"
	baseURL    = "https://www.ipu.ac.in"
	sourceName = "GGSIPU"
)

type IPU struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(IPU{Fetcher: &scraper.HTTPFetcher{}})
}

func (IPU) Name() string         { return "ipu" }
func (IPU) Source() string       { return sourceName }
func (IPU) Categories() []string { return []string{"academic"} }

func (n IPU) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	f := n.Fetcher
	if f == nil {
		f = &scraper.HTTPFetcher{}
	}

	body, err := f.Fetch(ctx, listingURL)
	if err != nil {
		return nil, err
	}

	results := parseTable(body)

	results = filterNonAds(results)
	results = scraper.FilterByRecency(results, opts.JobAge)
	results = scraper.FilterByQuery(results, opts.Query)

	return results, nil
}

func parseTable(body string) []scraper.Result {
	var results []scraper.Result

	rowRE := regexp.MustCompile(`<tr[^>]*>([\s\S]*?)</tr>`)
	tdRE := regexp.MustCompile(`<td[^>]*>([\s\S]*?)</td>`)
	anchorRE := regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>([\s\S]*?)</a>`)

	for _, m := range rowRE.FindAllStringSubmatch(body, -1) {
		row := m[1]
		cells := tdRE.FindAllStringSubmatch(row, -1)
		if len(cells) < 2 {
			continue
		}

		firstCell := cells[0][1]
		anchor := anchorRE.FindStringSubmatch(firstCell)
		if anchor == nil {
			continue
		}

		href := anchor[1]
		title := scraper.CleanHTML(anchor[2])
		if title == "" {
			continue
		}

		if strings.HasPrefix(href, "/") {
			href = baseURL + href
		}

		date := scraper.CleanHTML(cells[1][1])

		id := href
		if idx := strings.LastIndex(id, "/"); idx >= 0 {
			id = id[idx+1:]
		}

		results = append(results, scraper.Result{
			ID:       id,
			Title:    title,
			Company:  sourceName,
			Location: "New Delhi, India",
			Date:     date,
			URL:      href,
		})
	}

	return results
}

// classifyNotice returns the notice type derived from the title's keywords.
// The default "ad" covers advertisements, extensions, corrigenda, walk-in
// interviews, and any title that matches no non-ad pattern (conservative —
// better to include than miss).
func classifyNotice(title string) string {
	l := strings.ToLower(title)
	if strings.Contains(l, "schedule") {
		return "schedule"
	}
	if strings.Contains(l, "list of selected") ||
		strings.HasPrefix(l, "result of") ||
		strings.HasPrefix(l, "result for") {
		return "result"
	}
	if strings.Contains(l, "cancellation") {
		return "cancellation"
	}
	if strings.Contains(l, "postponement") {
		return "postponement"
	}
	if strings.Contains(l, "refund") {
		return "refund"
	}
	if strings.Contains(l, "empanelment") || strings.Contains(l, "empanel ") {
		return "empanelment"
	}
	if strings.Contains(l, "procurement") ||
		strings.Contains(l, "nit ") ||
		strings.Contains(l, "gem portal") ||
		strings.Contains(l, "notice inviting bid") {
		return "procurement"
	}
	if strings.Contains(l, "syllabus for") {
		return "syllabus"
	}
	if strings.Contains(l, "inviting objections") {
		return "objections"
	}
	return "ad"
}

// filterNonAds classifies each result by notice type, records the type in
// Metadata["notice_type"] for traceability, and drops non-actionable notices
// (schedules, results, cancellations, etc.).
func filterNonAds(results []scraper.Result) []scraper.Result {
	filtered := results[:0]
	for _, r := range results {
		typ := classifyNotice(r.Title)
		if typ != "ad" {
			continue
		}
		if r.Metadata == nil {
			r.Metadata = map[string]string{}
		}
		r.Metadata["notice_type"] = typ
		filtered = append(filtered, r)
	}
	return filtered
}
