package indeed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	apiURL     = "https://apis.indeed.com/graphql"
	baseURL    = "https://in.indeed.com"
	sourceName = "Indeed"
)

type Indeed struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(Indeed{})
}

func (Indeed) Name() string         { return "indeed" }
func (Indeed) Source() string       { return sourceName }
func (Indeed) Categories() []string { return []string{"tech", "biotech", "academic", "aggregator"} }

func (n Indeed) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	var allResults []scraper.Result
	cursor := ""

	for page := 0; page < 5; page++ {
		results, nextCursor, err := n.fetchPage(ctx, opts, cursor)
		if err != nil {
			if page == 0 {
				return nil, err
			}
			break
		}
		allResults = append(allResults, results...)
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
		time.Sleep(time.Duration(rand.Intn(2000)+1000) * time.Millisecond)
	}

	return allResults, nil
}

func (n Indeed) fetchPage(ctx context.Context, opts scraper.SearchOpts, cursor string) ([]scraper.Result, string, error) {
	searchTerm := opts.Query
	if searchTerm == "" {
		searchTerm = "research"
	}
	location := opts.Location
	if location == "" {
		location = "India"
	}

	cursorStr := ""
	if cursor != "" {
		cursorStr = fmt.Sprintf(`cursor: "%s"`, cursor)
	}

	query := fmt.Sprintf(`query GetJobData { jobSearch(what: "%s" location: {where: "%s" radius: 25 radiusUnit: MILES} limit: 100 %s sort: RELEVANCE) { pageInfo { nextCursor } results { job { key title datePublished description { html } location { city admin1Code countryCode formatted { long } } employer { name } recruit { viewJobUrl } attributes { key label } } } } }`,
		searchTerm, location, cursorStr)

	payload, _ := json.Marshal(map[string]string{"query": query})

	if n.Fetcher != nil {
		raw, err := n.Fetcher.Fetch(ctx, apiURL)
		if err != nil {
			return nil, "", err
		}
		return parseResponse([]byte(raw))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(payload))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Host", "apis.indeed.com")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Indeed-Api-Key", "161092c2017b5bbab13edb12461a62d5a833871e7cad6d9d475304573de67ac8")
	req.Header.Set("Indeed-Locale", "en-US")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 16_6_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Indeed App 193.1")
	req.Header.Set("Indeed-App-Info", "appv=193.1; appid=com.indeed.jobsearch; osv=16.6.1; os=ios; dtype=phone")
	req.Header.Set("Indeed-Co", "IN")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("indeed API returned %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	return parseResponse(raw)
}

func parseResponse(raw []byte) ([]scraper.Result, string, error) {
	var gqlResp struct {
		Data struct {
			JobSearch struct {
				PageInfo struct {
					NextCursor string `json:"nextCursor"`
				} `json:"pageInfo"`
				Results []struct {
					Job struct {
						Key           string `json:"key"`
						Title         string `json:"title"`
						DatePublished int64  `json:"datePublished"`
						Description   struct {
							HTML string `json:"html"`
						} `json:"description"`
						Location struct {
							City       string `json:"city"`
							Admin1Code string `json:"admin1Code"`
							Country    string `json:"countryCode"`
							Formatted  struct {
								Long string `json:"long"`
							} `json:"formatted"`
						} `json:"location"`
						Employer struct {
							Name string `json:"name"`
						} `json:"employer"`
						Recruit struct {
							ViewJobURL string `json:"viewJobUrl"`
						} `json:"recruit"`
						Attributes []struct {
							Key   string `json:"key"`
							Label string `json:"label"`
						} `json:"attributes"`
					} `json:"job"`
				} `json:"results"`
			} `json:"jobSearch"`
		} `json:"data"`
	}

	if err := json.Unmarshal(raw, &gqlResp); err != nil {
		return nil, "", fmt.Errorf("parse indeed response: %w", err)
	}

	var results []scraper.Result
	for _, r := range gqlResp.Data.JobSearch.Results {
		j := r.Job
		if j.Title == "" {
			continue
		}

		jobURL := fmt.Sprintf("%s/viewjob?jk=%s", baseURL, j.Key)
		if j.Recruit.ViewJobURL != "" {
			jobURL = j.Recruit.ViewJobURL
		}

		datePosted := ""
		if j.DatePublished > 0 {
			datePosted = time.UnixMilli(j.DatePublished).Format("2006-01-02")
		}

		loc := j.Location.Formatted.Long
		if loc == "" {
			loc = strings.TrimSpace(j.Location.City + ", " + j.Location.Admin1Code)
		}

		var attrLabels []string
		for _, a := range j.Attributes {
			if a.Label != "" {
				attrLabels = append(attrLabels, a.Label)
			}
		}

		results = append(results, scraper.Result{
			ID:          j.Key,
			Title:       j.Title,
			Company:     j.Employer.Name,
			Location:    loc,
			Date:        datePosted,
			URL:         jobURL,
			Description: stripHTML(j.Description.HTML),
			Metadata: map[string]string{
				"attributes": strings.Join(attrLabels, ", "),
			},
		})
	}

	return results, gqlResp.Data.JobSearch.PageInfo.NextCursor, nil
}

var (
	tagRE   = regexp.MustCompile(`<[^>]+>`)
	spaceRE = regexp.MustCompile(`\s+`)
)

func stripHTML(s string) string {
	s = regexp.MustCompile(`<br\s*/?>|</(p|li|div|h\d)>`).ReplaceAllString(s, "\n")
	s = tagRE.ReplaceAllString(s, " ")
	// Collapse spaces but preserve newlines
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = spaceRE.ReplaceAllString(line, " ")
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
