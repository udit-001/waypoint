package iisc

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	listingURL = "https://www.iisc.ac.in/careers/contractual-positions/"
	baseURL    = "https://www.iisc.ac.in"
	sourceName = "IISc"
)

// IISc scrapes the IISc careers page for contractual positions.
type IISc struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(IISc{Fetcher: &scraper.HTTPFetcher{}})
}

func (IISc) Name() string         { return "iisc" }
func (IISc) Source() string       { return sourceName }
func (IISc) Categories() []string { return []string{"biotech", "academic"} }

func (n IISc) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
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

func columnText(chunk string, col int) string {
	re := regexp.MustCompile(`class="column-` + strconv.Itoa(col) + `"[^>]*>([\s\S]*?)</td>`)
	m := re.FindStringSubmatch(chunk)
	if m == nil {
		return ""
	}
	return scraper.CleanHTML(m[1])
}

func columnLink(chunk string, col int) string {
	re := regexp.MustCompile(`class="column-` + strconv.Itoa(col) + `"[^>]*>([\s\S]*?)</td>`)
	m := re.FindStringSubmatch(chunk)
	if m == nil {
		return ""
	}
	linkRE := regexp.MustCompile(`href="([^"]+)"`)
	link := linkRE.FindStringSubmatch(m[1])
	if link == nil {
		return ""
	}
	href := link[1]
	if strings.HasPrefix(href, "/") {
		href = baseURL + href
	}
	return href
}

func parseJobRows(htmlBody string) []scraper.Result {
	var results []scraper.Result

	chunks := strings.Split(htmlBody, "<tr")
	for _, chunk := range chunks[1:] {
		if !strings.Contains(chunk, "column-1") || strings.Contains(chunk, "<th") {
			continue
		}

		status := columnText(chunk, 10)
		if status != "Open" {
			continue
		}

		slNo := columnText(chunk, 1)
		if slNo == "" {
			continue
		}

		title := columnText(chunk, 3)
		if title == "" {
			continue
		}

		advtNo := columnText(chunk, 2)
		dept := columnText(chunk, 4)
		startDate := columnText(chunk, 6)
		deadline := columnText(chunk, 7)
		pdfURL := columnLink(chunk, 5)
		applyLink := columnLink(chunk, 8)

		results = append(results, scraper.Result{
			ID:       slNo,
			Title:    title,
			Company:  sourceName,
			Location: "Bengaluru, India",
			Date:     deadline,
			URL:      pdfURL,
			Metadata: map[string]string{
				"advt_no":          advtNo,
				"department":       dept,
				"status":           status,
				"start_date":       startDate,
				"application_link": applyLink,
			},
		})
	}

	return results
}
