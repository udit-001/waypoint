package icgeb

import (
	"context"
	"regexp"
	"strings"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	listingURL  = "https://www.icgeb.org/category/vacancies/"
	sourceName  = "ICGEB"
	googlebotUA = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
)

// ICGEB scrapes the ICGEB vacancies page. The site's WAF blocks browser
// User-Agents with 403, so we use a Googlebot UA.
type ICGEB struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(ICGEB{Fetcher: &googlebotFetcher{}})
}

func (ICGEB) Name() string         { return "icgeb" }
func (ICGEB) Source() string       { return sourceName }
func (ICGEB) Categories() []string { return []string{"biotech", "academic"} }

func (n ICGEB) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	f := n.Fetcher
	if f == nil {
		f = &googlebotFetcher{}
	}

	body, err := f.Fetch(ctx, listingURL)
	if err != nil {
		return nil, err
	}

	results := parseArticles(body)

	return results, nil
}

// googlebotFetcher uses a Googlebot UA to bypass ICGEB's WAF.
type googlebotFetcher struct{}

func (g *googlebotFetcher) Fetch(ctx context.Context, url string) (string, error) {
	return (&scraper.HTTPFetcher{UserAgent: googlebotUA}).Fetch(ctx, url)
}

func parseArticles(body string) []scraper.Result {
	var results []scraper.Result

	articleRE := regexp.MustCompile(`<article[^>]*>([\s\S]*?)</article>`)
	for _, m := range articleRE.FindAllStringSubmatch(body, -1) {
		art := m[1]

		titleRE := regexp.MustCompile(`<h1 class="entry-title"[^>]*>([\s\S]*?)</h1>`)
		tm := titleRE.FindStringSubmatch(art)
		if tm == nil {
			continue
		}
		title := scraper.CleanHTML(tm[1])
		if title == "" {
			continue
		}

		linkRE := regexp.MustCompile(`<a href="([^"]+)"[^>]*>[\s\S]*?<h1 class="entry-title"`)
		lm := linkRE.FindStringSubmatch(art)
		url := ""
		if lm != nil {
			url = lm[1]
		}

		dateRE := regexp.MustCompile(`class="postdate"[^>]*>([\s\S]*?)</div>`)
		dm := dateRE.FindStringSubmatch(art)
		postDate := ""
		if dm != nil {
			postDate = scraper.CleanHTML(dm[1])
		}

		closingRE := regexp.MustCompile(`Closing date:\s*([^<]+)`)
		cm := closingRE.FindStringSubmatch(art)
		deadline := ""
		if cm != nil {
			deadline = strings.TrimSpace(cm[1])
		}

		locRE := regexp.MustCompile(`<b>([^<]+)</b>`)
		lm2 := locRE.FindStringSubmatch(art)
		location := "New Delhi, India"
		if lm2 != nil {
			location = scraper.CleanHTML(lm2[1])
		}

		id := url
		if idx := strings.LastIndex(id, "/"); idx >= 0 {
			id = strings.TrimSuffix(id[idx+1:], "/")
		}

		results = append(results, scraper.Result{
			ID:       id,
			Title:    title,
			Company:  sourceName,
			Location: location,
			Date:     deadline,
			URL:      url,
			Metadata: map[string]string{
				"post_date": postDate,
			},
		})
	}

	return results
}
