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

// nonAdTypes are the notice types that filterNonAds drops. Anything not in
// this set is kept (conservative — better to include than miss a real ad).
var nonAdTypes = map[string]bool{
	"schedule": true, "result": true, "cancellation": true,
	"postponement": true, "refund": true, "empanelment": true,
	"procurement": true, "syllabus": true, "objections": true,
}

// classifyNotice returns the notice type derived from the title's keywords.
// Non-ad types (schedules, results, cancellations, …) are matched first and
// dropped by filterNonAds. Kept notices get a specific type (advertisement,
// extension, corrigendum, walk_in, employment_notice) for traceability; any
// title that matches no pattern returns "ad" (conservative — kept).
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
	if strings.Contains(l, "walk-in") || strings.Contains(l, "walk in") {
		return "walk_in"
	}
	if strings.Contains(l, "corrigendum") {
		return "corrigendum"
	}
	if strings.Contains(l, "extension") {
		return "extension"
	}
	if strings.Contains(l, "employment notice") {
		return "employment_notice"
	}
	if strings.Contains(l, "advertisement") {
		return "advertisement"
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
		if nonAdTypes[typ] {
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
