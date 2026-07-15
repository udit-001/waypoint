package linkedin

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	searchURL  = "https://www.linkedin.com/jobs-guest/jobs/api/seeMoreJobPostings/search"
	detailURL  = "https://www.linkedin.com/jobs-guest/jobs/api/jobPosting"
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

	location := opts.Location
	if location == "" {
		location = "India"
	}

	start := 0
	if opts.Page > 1 {
		start = (opts.Page - 1) * 10
	}

	params := url.Values{}
	if opts.Query != "" {
		params.Set("keywords", opts.Query)
	}
	params.Set("location", location)
	params.Set("start", strconv.Itoa(start))

	if tpr := jobageToTPR(opts.JobAge); tpr != "" {
		params.Set("f_TPR", tpr)
	}
	if wt := workTypeFlag(opts.Remote); wt != "" {
		params.Set("f_WT", wt)
	}

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

// workTypeFlag converts a workplace-type string to LinkedIn's f_WT value.
func workTypeFlag(mode string) string {
	switch strings.ToLower(mode) {
	case "remote":
		return "2"
	case "hybrid":
		return "3"
	case "onsite", "on-site":
		return "1"
	default:
		return ""
	}
}

// normalizeID extracts a numeric job ID from a raw ID, a job-view URL, or a URN.
func normalizeID(input string) string {
	if m := regexp.MustCompile(`urn:li:jobPosting:(\d+)`).FindStringSubmatch(input); m != nil {
		return m[1]
	}
	if m := regexp.MustCompile(`-(\d{6,})(?:\?|$)`).FindStringSubmatch(input); m != nil {
		return m[1]
	}
	if m := regexp.MustCompile(`/(\d{6,})(?:\?|$)`).FindStringSubmatch(input); m != nil {
		return m[1]
	}
	if regexp.MustCompile(`^\d{6,}$`).MatchString(input) {
		return input
	}
	return ""
}

func (n LinkedIn) Detail(ctx context.Context, id string) (*scraper.Result, error) {
	rawID := normalizeID(id)
	if rawID == "" {
		return nil, fmt.Errorf("could not parse a job ID from %q", id)
	}

	f := n.Fetcher
	if f == nil {
		f = &scraper.HTTPFetcher{}
	}

	body, err := f.Fetch(ctx, detailURL+"/"+rawID)
	if err != nil {
		return nil, err
	}
	if body == "" {
		return nil, fmt.Errorf("job %s not found", rawID)
	}

	result := parseJobDetail(body, rawID)
	return result, nil
}

func parseJobDetail(htmlBody, id string) *scraper.Result {
	r := &scraper.Result{
		ID:  id,
		URL: "https://www.linkedin.com/jobs/view/" + id,
	}

	// Title
	if m := regexp.MustCompile(`class="(?:top-card-layout__title|topcard__title)[^"]*"[^>]*>([\s\S]*?)</h[12]>`).FindStringSubmatch(htmlBody); m != nil {
		r.Title = scraper.CleanHTML(m[1])
	}

	// Company + company URL
	if m := regexp.MustCompile(`class="topcard__org-name-link[^"]*"[^>]*href="([^"]+)"[^>]*>([\s\S]*?)</a>`).FindStringSubmatch(htmlBody); m != nil {
		r.Company = scraper.CleanHTML(m[2])
		r.Metadata = map[string]string{
			"company_url": html.UnescapeString(m[1]),
		}
	}

	// Location
	if m := regexp.MustCompile(`class="topcard__flavor topcard__flavor--bullet"[^>]*>([\s\S]*?)</span>`).FindStringSubmatch(htmlBody); m != nil {
		r.Location = scraper.CleanHTML(m[1])
	}

	// Description
	if m := regexp.MustCompile(`class="(?:show-more-less-html__markup|description__text[^"]*)"[^>]*>([\s\S]*?)</div>`).FindStringSubmatch(htmlBody); m != nil {
		desc := m[1]
		desc = regexp.MustCompile(`<\s*br\s*/?>`).ReplaceAllString(desc, "\n")
		desc = regexp.MustCompile(`</(?:p|li|ul|ol|div|h\d)>`).ReplaceAllString(desc, "\n")
		r.Description = scraper.CleanHTML(desc)
	}

	// Job criteria (seniority, employment type, job function, industries)
	criteriaRE := regexp.MustCompile(`class="description__job-criteria-subheader"[^>]*>([\s\S]*?)</h3>[\s\S]*?class="description__job-criteria-text[^"]*"[^>]*>([\s\S]*?)</span>`)
	if r.Metadata == nil && criteriaRE.MatchString(htmlBody) {
		r.Metadata = map[string]string{}
	}
	for _, m := range criteriaRE.FindAllStringSubmatch(htmlBody, -1) {
		key := strings.ToLower(scraper.CleanHTML(m[1]))
		val := scraper.CleanHTML(m[2])
		if key != "" && val != "" {
			r.Metadata[key] = val
		}
	}

	// Apply URL
	if m := regexp.MustCompile(`class="topcard__link[^"]*"[^>]*href="([^"]+)"`).FindStringSubmatch(htmlBody); m != nil {
		if r.Metadata == nil {
			r.Metadata = map[string]string{}
		}
		r.Metadata["apply_url"] = html.UnescapeString(m[1])
	}

	return r
}
