package nabi

import (
	"context"
	"regexp"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	listingURL = "https://nabi.res.in/site/career"
	sourceName = "NABI"
)

type NABI struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(NABI{Fetcher: &scraper.HTTPFetcher{}})
}

func (NABI) Name() string         { return "nabi" }
func (NABI) Source() string       { return sourceName }
func (NABI) Categories() []string { return []string{"biotech", "academic"} }

func (n NABI) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	f := n.Fetcher
	if f == nil {
		f = &scraper.HTTPFetcher{}
	}

	body, err := f.Fetch(ctx, listingURL)
	if err != nil {
		return nil, err
	}

	results := parseTable(body)

	return results, nil
}

func parseTable(body string) []scraper.Result {
	var results []scraper.Result

	rowRE := regexp.MustCompile(`<tr[^>]*>([\s\S]*?)</tr>`)
	tdRE := regexp.MustCompile(`<td[^>]*>([\s\S]*?)</td>`)
	anchorRE := regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*title="([^"]*)"`)

	for _, m := range rowRE.FindAllStringSubmatch(body, -1) {
		row := m[1]
		cells := tdRE.FindAllStringSubmatch(row, -1)
		if len(cells) < 4 {
			continue
		}

		slNo := scraper.CleanHTML(cells[0][1])
		if slNo == "" {
			continue
		}

		title := scraper.CleanHTML(cells[1][1])
		if title == "" {
			continue
		}

		deadline := scraper.CleanHTML(cells[2][1])

		// Extract PDF link from the 4th cell
		pdfURL := ""
		am := anchorRE.FindStringSubmatch(cells[3][1])
		if am != nil {
			pdfURL = am[1]
		}

		id := slNo

		url := pdfURL
		if url == "" {
			url = listingURL
		}

		results = append(results, scraper.Result{
			ID:       id,
			Title:    title,
			Company:  sourceName,
			Location: "Mohali, India",
			Date:     deadline,
			URL:      url,
		})
	}

	return results
}
