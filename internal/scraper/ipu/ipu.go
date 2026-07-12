package ipu

import (
	"context"
	"regexp"
	"strings"

	"github.com/SwatiBio/waypoint/internal/scraper"
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
