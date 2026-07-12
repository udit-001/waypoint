package iisertirupati

import (
	"context"
	"regexp"
	"strings"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	listingURL = "https://www.iisertirupati.ac.in/jobs/"
	sourceName = "IISER Tirupati"
)

type IISERTirupati struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(IISERTirupati{Fetcher: &scraper.HTTPFetcher{}})
}

func (IISERTirupati) Name() string         { return "iisertirupati" }
func (IISERTirupati) Source() string       { return sourceName }
func (IISERTirupati) Categories() []string { return []string{"biotech", "academic"} }

func (n IISERTirupati) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	f := n.Fetcher
	if f == nil {
		f = &scraper.HTTPFetcher{}
	}

	body, err := f.Fetch(ctx, listingURL)
	if err != nil {
		return nil, err
	}

	results := parseJobs(body)

	return results, nil
}

func parseJobs(body string) []scraper.Result {
	var results []scraper.Result

	rowRE := regexp.MustCompile(`<tr[^>]*>([\s\S]*?)</tr>`)
	titleRE := regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>[\s\S]*?<span class="job-title"[^>]*>([\s\S]*?)</span>`)

	for _, m := range rowRE.FindAllStringSubmatch(body, -1) {
		row := m[1]
		if !strings.Contains(row, "job-title") {
			continue
		}

		tm := titleRE.FindStringSubmatch(row)
		if tm == nil {
			continue
		}
		url := tm[1]
		title := scraper.CleanHTML(tm[2])
		if title == "" {
			continue
		}

		id := url
		id = strings.TrimSuffix(id, "/")
		if idx := strings.LastIndex(id, "/"); idx >= 0 {
			id = id[idx+1:]
		}

		tdRE := regexp.MustCompile(`<td[^>]*>([\s\S]*?)</td>`)
		cells := tdRE.FindAllStringSubmatch(row, -1)
		date := ""
		if len(cells) >= 3 {
			date = scraper.CleanHTML(cells[2][1])
		}

		results = append(results, scraper.Result{
			ID:       id,
			Title:    title,
			Company:  sourceName,
			Location: "Tirupati, India",
			Date:     date,
			URL:      url,
		})
	}

	return results
}
