package manipal

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/udit-001/waypoint/internal/scraper"
)

const (
	baseURL    = "https://www.manipal.edu"
	facultyURL = "https://www.manipal.edu/mu/important-links/careers-mu/jobs-at-mahe-current-vacancies-/faculty-positions.html"
	staffURL   = "https://www.manipal.edu/mu/important-links/careers-mu/jobs-at-mahe-current-vacancies-/staff-positions.html"
	sourceName = "MAHE (Manipal)"
)

type Manipal struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(Manipal{Fetcher: &scraper.HTTPFetcher{}})
}

func (Manipal) Name() string         { return "manipal" }
func (Manipal) Source() string       { return sourceName }
func (Manipal) Categories() []string { return []string{"academic", "biotech"} }

func (n Manipal) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	f := n.Fetcher
	if f == nil {
		f = &scraper.HTTPFetcher{}
	}

	var allResults []scraper.Result

	for _, pageURL := range []string{facultyURL, staffURL} {
		body, err := f.Fetch(ctx, pageURL)
		if err != nil {
			continue // one page failing shouldn't kill the whole scrape
		}
		allResults = append(allResults, parseTable(body)...)
	}

	allResults = scraper.FilterByQuery(allResults, opts.Query)

	return allResults, nil
}

func parseTable(body string) []scraper.Result {
	var results []scraper.Result

	rowRE := regexp.MustCompile(`<tr[^>]*>([\s\S]*?)</tr>`)
	tdRE := regexp.MustCompile(`<td[^>]*>([\s\S]*?)</td>`)
	anchorRE := regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>`)
	deadlineRE := regexp.MustCompile(`Apply on or before\s*([^<]+)`)

	for _, m := range rowRE.FindAllStringSubmatch(body, -1) {
		row := m[1]
		cells := tdRE.FindAllStringSubmatch(row, -1)
		if len(cells) < 4 {
			continue
		}

		postDate := scraper.CleanHTML(cells[0][1])
		department := scraper.CleanHTML(cells[1][1])
		position := scraper.CleanHTML(cells[2][1])
		if position == "" {
			continue
		}

		// PDF link from the 4th cell
		pdfURL := ""
		am := anchorRE.FindStringSubmatch(cells[3][1])
		if am != nil {
			href := am[1]
			if strings.HasPrefix(href, "/") {
				href = baseURL + href
			}
			pdfURL = href
		}

		// Deadline from the 4th cell text
		deadline := ""
		if dm := deadlineRE.FindStringSubmatch(cells[3][1]); dm != nil {
			deadline = strings.TrimSpace(dm[1])
		}

		// Decode the PDF filename for a clean ID
		id := position
		if pdfURL != "" {
			if decoded, err := url.QueryUnescape(pdfURL); err == nil {
				if idx := strings.LastIndex(decoded, "/"); idx >= 0 {
					id = strings.TrimSuffix(decoded[idx+1:], ".pdf")
				}
			}
		}

		results = append(results, scraper.Result{
			ID:       id,
			Title:    position + " — " + department,
			Company:  sourceName,
			Location: "Manipal, India",
			Date:     deadline,
			URL:      pdfURL,
			Metadata: map[string]string{
				"posted_date": postDate,
				"department":  department,
			},
		})
	}

	return results
}
