package jncasr

import (
	"context"
	"regexp"
	"strings"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	listingURL = "https://www.jncasr.ac.in/openings"
	baseURL    = "https://www.jncasr.ac.in"
	sourceName = "JNCASR"
)

// JNCASR scrapes the JNCASR openings page for current recruitments.
type JNCASR struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(JNCASR{Fetcher: &scraper.HTTPFetcher{}})
}

func (JNCASR) Name() string         { return "jncasr" }
func (JNCASR) Source() string       { return sourceName }
func (JNCASR) Categories() []string { return []string{"biotech", "academic"} }

func (n JNCASR) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
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

var commentRE = regexp.MustCompile(`<!--[\s\S]*?-->`)

func fieldText(chunk, fieldClass string) string {
	re := regexp.MustCompile(`class="views-field views-field-` + regexp.QuoteMeta(fieldClass) + `"[^>]*>([\s\S]*?)</td>`)
	m := re.FindStringSubmatch(chunk)
	if m == nil {
		return ""
	}
	return scraper.CleanHTML(m[1])
}

func fieldLink(chunk, fieldClass string) string {
	re := regexp.MustCompile(`class="views-field views-field-` + regexp.QuoteMeta(fieldClass) + `"[^>]*>([\s\S]*?)</td>`)
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

	// Strip Twig debug comments before parsing
	cleaned := commentRE.ReplaceAllString(htmlBody, "")

	chunks := strings.Split(cleaned, "<tr")
	for _, chunk := range chunks[1:] {
		if !strings.Contains(chunk, "views-field-title") {
			continue
		}

		// Title field: contains anchor (href + title) + type annotation in <i>
		titleCell := fieldText(chunk, "title")
		if titleCell == "" {
			continue
		}

		// Extract the anchor href for the detail page URL
		titleCellRaw := chunk
		titleRE := regexp.MustCompile(`class="views-field views-field-title"[^>]*>([\s\S]*?)</td>`)
		if m := titleRE.FindStringSubmatch(titleCellRaw); m != nil {
			titleCellRaw = m[1]
		}
		anchorRE := regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>([\s\S]*?)</a>`)
		anchor := anchorRE.FindStringSubmatch(titleCellRaw)
		if anchor == nil {
			continue
		}

		slug := anchor[1]
		if strings.HasPrefix(slug, "/") {
			slug = strings.TrimPrefix(slug, "/openings/")
		}
		if slug == "" {
			continue
		}

		// Title text: the anchor text + the type annotation
		titleText := scraper.CleanHTML(anchor[2])
		typeMatch := regexp.MustCompile(`<i[^>]*>([\s\S]*?)</i>`).FindStringSubmatch(titleCellRaw)
		if typeMatch != nil {
			titleText += " " + scraper.CleanHTML(typeMatch[1])
		}

		postedDate := fieldText(chunk, "field-in-date")
		deadline := fieldText(chunk, "field-end-date")
		pdfURL := fieldLink(chunk, "views-conditional-field")

		url := baseURL + "/openings/" + slug
		if pdfURL != "" {
			url = pdfURL
		}

		results = append(results, scraper.Result{
			ID:       slug,
			Title:    titleText,
			Company:  sourceName,
			Location: "Bengaluru, India",
			Date:     deadline,
			URL:      url,
			Metadata: map[string]string{
				"posted_date": postedDate,
			},
		})
	}

	return results
}
