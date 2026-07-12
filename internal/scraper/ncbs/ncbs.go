package ncbs

import (
	"context"
	"regexp"
	"strings"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	listingURL = "https://www.ncbs.res.in/jobportal/"
	baseURL    = "https://www.ncbs.res.in"
	sourceName = "NCBS (TIFR)"
)

// NCBS scrapes the NCBS job portal for open positions.
type NCBS struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(NCBS{Fetcher: &scraper.HTTPFetcher{}})
}

func (NCBS) Name() string         { return "ncbs" }
func (NCBS) Source() string       { return sourceName }
func (NCBS) Categories() []string { return []string{"biotech", "academic"} }

func (n NCBS) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	f := n.Fetcher
	if f == nil {
		f = &scraper.HTTPFetcher{}
	}

	htmlBody, err := f.Fetch(ctx, listingURL)
	if err != nil {
		return nil, err
	}

	results := parseJobRows(htmlBody)

	return results, nil
}

// fieldText extracts the text content of a Drupal Views field cell by class name.
func fieldText(chunk, fieldClass string) string {
	re := regexp.MustCompile(`class="views-field views-field-` + regexp.QuoteMeta(fieldClass) + `"[^>]*>([\s\S]*?)</td>`)
	m := re.FindStringSubmatch(chunk)
	if m == nil {
		return ""
	}
	return scraper.CleanHTML(m[1])
}

// parseJobRows parses the NCBS listing page's Drupal Views table into Results.
func parseJobRows(htmlBody string) []scraper.Result {
	var results []scraper.Result

	chunks := strings.Split(htmlBody, "<tr")
	for _, chunk := range chunks[1:] {
		if !strings.Contains(chunk, "views-field-title") {
			continue
		}

		// Title cell: extract anchor text + href
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

		// Node ID from the View button: href="/jobportal/node/<id>"
		viewLinkRE := regexp.MustCompile(`href="/jobportal/node/(\d+)"`)
		viewLink := viewLinkRE.FindStringSubmatch(chunk)
		if viewLink == nil {
			continue
		}
		id := viewLink[1]

		deadline := fieldText(chunk, "field-job-date")

		results = append(results, scraper.Result{
			ID:       id,
			Title:    title,
			Company:  sourceName,
			Location: "Bengaluru, India",
			Date:     deadline,
			URL:      baseURL + "/jobportal/node/" + id,
			Metadata: map[string]string{
				"qualification": fieldText(chunk, "php"),
				"domain":        fieldText(chunk, "php-1"),
				"experience":    fieldText(chunk, "php-2"),
				"reservation":   fieldText(chunk, "php-3"),
				"vacancy":       fieldText(chunk, "field-job-vacancy"),
			},
		})
	}

	return results
}
