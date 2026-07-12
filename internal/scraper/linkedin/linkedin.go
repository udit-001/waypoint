package linkedin

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	searchURL  = "https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search"
	sourceName = "LinkedIn"
)

// LinkedIn scrapes LinkedIn's public jobs-guest endpoints.
// No authentication required. Location is required for search.
// Personal use only — automated access is against LinkedIn's ToS.
type LinkedIn struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(LinkedIn{Fetcher: &scraper.HTTPFetcher{}})
}

func (LinkedIn) Name() string         { return "linkedin" }
func (LinkedIn) Source() string       { return sourceName }
func (LinkedIn) Categories() []string { return []string{"tech", "biotech", "academic"} }

func (n LinkedIn) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	f := n.Fetcher
	if f == nil {
		f = &scraper.HTTPFetcher{}
	}

	// LinkedIn requires a location. Default to India if not specified.
	location := opts.Location
	if location == "" {
		location = "India"
	}

	params := url.Values{}
	if opts.Query != "" {
		params.Set("keywords", opts.Query)
	}
	params.Set("location", location)
	params.Set("start", "0")

	fetchURL := searchURL + "?" + params.Encode()

	body, err := f.Fetch(ctx, fetchURL)
	if err != nil {
		return nil, err
	}

	results := parseJobCards(body)

	return results, nil
}

func parseJobCards(htmlBody string) []scraper.Result {
	var results []scraper.Result

	// Split on the job-posting URN, parse each chunk independently
	chunks := strings.Split(htmlBody, `data-entity-urn="urn:li:jobPosting:`)
	for _, chunk := range chunks[1:] {
		idMatch := regexp.MustCompile(`^(\d+)`).FindStringSubmatch(chunk)
		if idMatch == nil {
			continue
		}
		id := idMatch[1]

		// Full link
		linkMatch := regexp.MustCompile(`class="base-card__full-link[^"]*"[^>]*href="([^"]+)"`).FindStringSubmatch(chunk)
		jobURL := ""
		if linkMatch != nil {
			jobURL = html.UnescapeString(linkMatch[1])
			if idx := strings.Index(jobURL, "?"); idx >= 0 {
				jobURL = jobURL[:idx]
			}
		}

		// Title
		var title string
		h3Match := regexp.MustCompile(`class="base-search-card__title"[^>]*>([\s\S]*?)</h3>`).FindStringSubmatch(chunk)
		if h3Match != nil {
			title = scraper.CleanHTML(h3Match[1])
		}
		if title == "" {
			srMatch := regexp.MustCompile(`class="sr-only"[^>]*>([\s\S]*?)</span>`).FindStringSubmatch(chunk)
			if srMatch != nil {
				title = scraper.CleanHTML(srMatch[1])
			}
		}
		if title == "" {
			continue
		}

		// Company
		var company string
		subMatch := regexp.MustCompile(`class="base-search-card__subtitle"[^>]*>([\s\S]*?)</h4>`).FindStringSubmatch(chunk)
		if subMatch != nil {
			company = scraper.CleanHTML(subMatch[1])
		}

		// Location
		var location string
		locMatch := regexp.MustCompile(`class="job-search-card__location"[^>]*>([\s\S]*?)</span>`).FindStringSubmatch(chunk)
		if locMatch != nil {
			location = scraper.CleanHTML(locMatch[1])
		}

		// Date
		var date string
		dtMatch := regexp.MustCompile(`class="job-search-card__listdate[^"]*"[^>]*datetime="([^"]+)"`).FindStringSubmatch(chunk)
		if dtMatch != nil {
			date = dtMatch[1]
		}

		if jobURL == "" {
			jobURL = "https://www.linkedin.com/jobs/view/" + id
		}

		results = append(results, scraper.Result{
			ID:       id,
			Title:    title,
			Company:  company,
			Location: location,
			Date:     date,
			URL:      jobURL,
		})
	}

	return results
}

// jobageToTPR converts a job-age in days to LinkedIn's f_TPR seconds value.
func jobageToTPR(days int) string {
	if days <= 0 || days >= 9999 {
		return ""
	}
	return "r" + strconv.Itoa(days*86400)
}
