package nipgr

import (
	"context"
	"regexp"
	"strings"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	listingURL = "https://nipgr.ac.in/nipgrv4/latest/"
	sourceName = "NIPGR"
)

type NIPGR struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(NIPGR{Fetcher: &scraper.HTTPFetcher{}})
}

func (NIPGR) Name() string         { return "nipgr" }
func (NIPGR) Source() string       { return sourceName }
func (NIPGR) Categories() []string { return []string{"biotech", "academic"} }

func (n NIPGR) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	f := n.Fetcher
	if f == nil {
		f = &scraper.HTTPFetcher{}
	}

	body, err := f.Fetch(ctx, listingURL)
	if err != nil {
		return nil, err
	}

	results := parseTable(body)

	results = scraper.FilterByQuery(results, opts.Query)

	return results, nil
}

func parseTable(body string) []scraper.Result {
	var results []scraper.Result

	rowRE := regexp.MustCompile(`<tr[^>]*>([\s\S]*?)</tr>`)
	tdRE := regexp.MustCompile(`<td[^>]*class="column-(\d)"[^>]*>([\s\S]*?)</td>`)
	anchorRE := regexp.MustCompile(`<a[^>]*href="([^"]+)"`)

	for _, m := range rowRE.FindAllStringSubmatch(body, -1) {
		row := m[1]
		cells := tdRE.FindAllStringSubmatch(row, -1)
		if len(cells) < 3 {
			continue
		}

		title := scraper.CleanHTML(cells[0][2])
		if title == "" {
			continue
		}

		deadline := scraper.CleanHTML(cells[1][2])

		pdfURL := ""
		am := anchorRE.FindStringSubmatch(cells[2][2])
		if am != nil {
			pdfURL = am[1]
		}

		appFormURL := ""
		if len(cells) >= 4 {
			am2 := anchorRE.FindStringSubmatch(cells[3][2])
			if am2 != nil {
				appFormURL = am2[1]
			}
		}

		id := pdfURL
		if idx := strings.LastIndex(id, "/"); idx >= 0 {
			id = id[idx+1:]
		}
		if id == "" {
			id = title[:min(30, len(title))]
		}

		url := pdfURL
		if url == "" {
			url = listingURL
		}

		results = append(results, scraper.Result{
			ID:       id,
			Title:    title,
			Company:  sourceName,
			Location: "New Delhi, India",
			Date:     deadline,
			URL:      url,
			Metadata: map[string]string{
				"application_form": appFormURL,
			},
		})
	}

	return results
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
