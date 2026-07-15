package instem

import (
	"context"
	"regexp"
	"strings"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	listingURL = "https://www.instem.res.in/jobportal/"
	baseURL    = "https://www.instem.res.in"
	sourceName = "inStem (BRIC)"
)

// InStem scrapes the inStem job portal for open positions.
// Uses the same Drupal jobportal system as NCBS.
type InStem struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(InStem{Fetcher: &scraper.HTTPFetcher{}})
}

func (InStem) Name() string         { return "instem" }
func (InStem) Source() string       { return sourceName }
func (InStem) Categories() []string { return []string{"biotech", "academic"} }

func (n InStem) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	f := n.Fetcher
	if f == nil {
		f = &scraper.HTTPFetcher{}
	}

	htmlBody, err := f.Fetch(ctx, listingURL)
	if err != nil {
		return nil, err
	}

	results := parseJobRows(htmlBody)

	results = scraper.FilterByQuery(results, opts.Query)

	return results, nil
}

func fieldText(chunk, fieldClass string) string {
	re := regexp.MustCompile(`class="views-field views-field-` + regexp.QuoteMeta(fieldClass) + `"[^>]*>([\s\S]*?)</td>`)
	m := re.FindStringSubmatch(chunk)
	if m == nil {
		return ""
	}
	return scraper.CleanHTML(m[1])
}

func parseJobRows(htmlBody string) []scraper.Result {
	var results []scraper.Result

	chunks := strings.Split(htmlBody, "<tr")
	for _, chunk := range chunks[1:] {
		if !strings.Contains(chunk, "views-field-title") {
			continue
		}

		titleCellRE := regexp.MustCompile(`class="views-field views-field-title"[^>]*>([\s\S]*?)</td>`)
		titleMatch := titleCellRE.FindStringSubmatch(chunk)
		if titleMatch == nil {
			continue
		}
		anchorRE := regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>([\s\S]*?)</a>`)
		anchor := anchorRE.FindStringSubmatch(titleMatch[1])
		if anchor == nil {
			continue
		}
		title := scraper.CleanHTML(anchor[2])
		if title == "" {
			continue
		}

		viewLinkRE := regexp.MustCompile(`href="/jobportal/node/(\d+)"`)
		viewLink := viewLinkRE.FindStringSubmatch(chunk)
		if viewLink == nil {
			continue
		}
		id := viewLink[1]

		results = append(results, scraper.Result{
			ID:       id,
			Title:    title,
			Company:  sourceName,
			Location: "Bengaluru, India",
			Date:     fieldText(chunk, "field-job-date"),
			URL:      baseURL + "/jobportal/node/" + id,
			Metadata: map[string]string{
				"vacancy": fieldText(chunk, "field-job-vacancy"),
			},
		})
	}

	return results
}
