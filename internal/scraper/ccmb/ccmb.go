package ccmb

import (
	"context"
	"regexp"
	"strings"

	"github.com/udit-001/waypoint/internal/scraper"
)

const (
	listingURL = "https://ccmb.res.in/jobs/"
	sourceName = "CCMB (CSIR)"
)

type CCMB struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(CCMB{Fetcher: &scraper.HTTPFetcher{}})
}

func (CCMB) Name() string         { return "ccmb" }
func (CCMB) Source() string       { return sourceName }
func (CCMB) Categories() []string { return []string{"biotech", "academic"} }

func (n CCMB) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	f := n.Fetcher
	if f == nil {
		f = &scraper.HTTPFetcher{}
	}

	body, err := f.Fetch(ctx, listingURL)
	if err != nil {
		return nil, err
	}

	results := parseCards(body)

	results = scraper.FilterByQuery(results, opts.Query)

	return results, nil
}

func extractField(chunk, label string) string {
	re := regexp.MustCompile(`<strong>` + regexp.QuoteMeta(label) + `:</strong>\s*(.*?)</p>`)
	m := re.FindStringSubmatch(chunk)
	if m == nil {
		return ""
	}
	return scraper.CleanHTML(m[1])
}

func parseCards(body string) []scraper.Result {
	var results []scraper.Result

	chunks := strings.Split(body, `class="card mb-4 notification-card"`)
	for _, chunk := range chunks[1:] {
		titleRE := regexp.MustCompile(`class="card-title"[^>]*>([\s\S]*?)</h4>`)
		tm := titleRE.FindStringSubmatch(chunk)
		if tm == nil {
			continue
		}
		title := scraper.CleanHTML(tm[1])
		if title == "" {
			continue
		}

		lastDate := extractField(chunk, "Last Date to Apply")
		postedDate := extractField(chunk, "Posted Date")
		category := extractField(chunk, "Category")

		pdfLink := ""
		pdfRE := regexp.MustCompile(`href="(https?://[^"]+\.pdf)"`)
		if pm := pdfRE.FindStringSubmatch(chunk); pm != nil {
			pdfLink = pm[1]
		}

		id := regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(title[:min(30, len(title))], "-")

		results = append(results, scraper.Result{
			ID:       id,
			Title:    title,
			Company:  sourceName,
			Location: "Hyderabad, India",
			Date:     lastDate,
			URL:      pdfLink,
			Metadata: map[string]string{
				"posted_date": postedDate,
				"category":    category,
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
