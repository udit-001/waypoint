package niab

import (
	"context"
	"regexp"
	"strings"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	listingURL = "https://www.niab.res.in/recruitment/"
	sourceName = "NIAB"
)

type NIAB struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(NIAB{Fetcher: &scraper.HTTPFetcher{}})
}

func (NIAB) Name() string         { return "niab" }
func (NIAB) Source() string       { return sourceName }
func (NIAB) Categories() []string { return []string{"biotech", "academic"} }

func (n NIAB) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	f := n.Fetcher
	if f == nil {
		f = &scraper.HTTPFetcher{}
	}

	body, err := f.Fetch(ctx, listingURL)
	if err != nil {
		return nil, err
	}

	results := parseListings(body)

	return results, nil
}

var advtRE = regexp.MustCompile(`Advt\.?\s*No\.?\s*(\d+/\d+)`)

func parseListings(body string) []scraper.Result {
	var results []scraper.Result

	liRE := regexp.MustCompile(`<li[^>]*>([\s\S]*?)</li>`)
	anchorRE := regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>([\s\S]*?)</a>`)

	for _, m := range liRE.FindAllStringSubmatch(body, -1) {
		liContent := m[1]
		anchor := anchorRE.FindStringSubmatch(liContent)
		if anchor == nil {
			continue
		}
		href := anchor[1]
		text := scraper.CleanHTML(anchor[2])
		if text == "" || text == "Result" {
			continue
		}

		id := href
		if idx := strings.LastIndex(href, "/"); idx >= 0 {
			id = href[idx+1:]
		}

		advtNo := ""
		if am := advtRE.FindStringSubmatch(liContent); am != nil {
			advtNo = am[1]
		}

		results = append(results, scraper.Result{
			ID:       id,
			Title:    text,
			Company:  sourceName,
			Location: "Hyderabad, India",
			URL:      href,
			Metadata: map[string]string{
				"advt_no": advtNo,
			},
		})
	}

	return results
}
