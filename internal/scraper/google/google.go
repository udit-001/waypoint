package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/SwatiBio/waypoint/internal/scraper"
)

const (
	searchURL  = "https://www.google.com/search"
	jobsURL    = "https://www.google.com/async/callback:550"
	sourceName = "Google Jobs"
)

type Google struct {
	Fetcher scraper.Fetcher
}

func init() {
	scraper.Register(Google{})
}

func (Google) Name() string         { return "google" }
func (Google) Source() string       { return sourceName }
func (Google) Categories() []string { return []string{"tech", "biotech", "academic", "aggregator"} }

// Note: Google Jobs requires JavaScript execution for the initial page.
// The Python JobSpy version uses tls_client for TLS impersonation; plain
// net/http gets a JS-only page with no job data. This scraper may return
// 0 results depending on Google's bot detection. Use the Indeed scraper
// as a more reliable aggregator alternative.

func (n Google) Search(ctx context.Context, opts scraper.SearchOpts) ([]scraper.Result, error) {
	query := opts.Query
	if query == "" {
		query = "research jobs"
	}
	if opts.Location != "" {
		query += " near " + opts.Location
	} else {
		query += " in India"
	}

	// Phase 1: initial search page — get cursor + first batch of jobs
	htmlBody, cursor, err := fetchInitialPage(ctx, query)
	if err != nil {
		return nil, err
	}

	results := parseInitialJobs(htmlBody)

	// Phase 2: paginate via async cursor
	for page := 0; page < 5 && cursor != ""; page++ {
		nextResults, nextCursor, err := fetchNextPage(ctx, cursor)
		if err != nil {
			break
		}
		results = append(results, nextResults...)
		cursor = nextCursor
		time.Sleep(time.Duration(rand.Intn(1500)+500) * time.Millisecond)
	}

	if opts.Limit > 0 && len(results) > opts.Limit {
		results = results[:opts.Limit]
	}

	return results, nil
}

var initialHeaders = map[string]string{
	"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
	"Accept-Language":           "en-US,en;q=0.9",
	"Referer":                   "https://www.google.com/",
	"Sec-Ch-Ua":                 `"Chromium";v="130", "Google Chrome";v="130", "Not?A_Brand";v="99"`,
	"Sec-Ch-Ua-Mobile":          "?0",
	"Sec-Ch-Ua-Platform":        `"macOS"`,
	"Sec-Fetch-Dest":            "document",
	"Sec-Fetch-Mode":            "navigate",
	"Sec-Fetch-Site":            "same-origin",
	"Upgrade-Insecure-Requests": "1",
	"User-Agent":                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
}

var jobsHeaders = map[string]string{
	"Accept":           "*/*",
	"Accept-Language":  "en-US,en;q=0.9",
	"Referer":          "https://www.google.com/",
	"Sec-Ch-Ua":        `"Chromium";v="130", "Google Chrome";v="130", "Not?A_Brand";v="99"`,
	"Sec-Ch-Ua-Mobile": "?0",
	"Sec-Ch-Ua-Platform": `"macOS"`,
	"Sec-Fetch-Dest":   "empty",
	"Sec-Fetch-Mode":   "cors",
	"Sec-Fetch-Site":   "same-origin",
	"User-Agent":       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
}

func fetchInitialPage(ctx context.Context, query string) (string, string, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("udm", "8")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL+"?"+params.Encode(), nil)
	if err != nil {
		return "", "", err
	}
	for k, v := range initialHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("google search returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	htmlBody := string(body)

	// Extract cursor
	cursor := ""
	cursorRE := regexp.MustCompile(`data-async-fc="([^"]+)"`)
	if m := cursorRE.FindStringSubmatch(htmlBody); m != nil {
		cursor = m[1]
	}

	return htmlBody, cursor, nil
}

func fetchNextPage(ctx context.Context, cursor string) ([]scraper.Result, string, error) {
	params := url.Values{}
	params.Add("fc", cursor)
	params.Add("fcv", "3")
	params.Add("async", "_fmt:prog")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jobsURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, "", err
	}
	for k, v := range jobsHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("google async returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	return parseAsyncJobs(string(body))
}

// parseInitialJobs extracts job listings from the initial Google search page HTML.
// Google embeds job data as JSON with key "520084652".
func parseInitialJobs(htmlBody string) []scraper.Result {
	var results []scraper.Result

	// Find all JSON blocks with the job data key
	pattern := `520084652":(\[.*?\]\s*\])\s*\}\s*\]\s*\]\s*\]\s*\]\s*\]`
	re := regexp.MustCompile(pattern)

	for _, m := range re.FindAllStringSubmatch(htmlBody, -1) {
		if len(m) < 2 {
			continue
		}
		var jobData []interface{}
		if err := json.Unmarshal([]byte(m[1]), &jobData); err != nil {
			continue
		}
		if jobPost := parseJobArray(jobData); jobPost != nil {
			results = append(results, *jobPost)
		}
	}

	return results
}

// parseAsyncJobs extracts jobs from the async pagination response.
// Format: [[[key, json_string], ...]] with a data-async-fc cursor.
func parseAsyncJobs(body string) ([]scraper.Result, string, error) {
	// Extract cursor
	cursor := ""
	cursorRE := regexp.MustCompile(`data-async-fc="([^"]+)"`)
	if m := cursorRE.FindStringSubmatch(body); m != nil {
		cursor = m[1]
	}

	// Find the outer [[[...]]] JSON array
	startIdx := strings.Index(body, "[[[")
	if startIdx < 0 {
		return nil, cursor, nil
	}
	endIdx := strings.LastIndex(body, "]]]")
	if endIdx < 0 || endIdx <= startIdx {
		return nil, cursor, nil
	}

	jsonStr := body[startIdx : endIdx+3]

	var outer []interface{}
	if err := json.Unmarshal([]byte(jsonStr), &outer); err != nil {
		return nil, cursor, nil
	}

	var results []scraper.Result
	for _, item := range outer {
		arr, ok := item.([]interface{})
		if !ok || len(arr) < 2 {
			continue
		}

		// arr[1] is a JSON string containing nested job data
		jobJSON, ok := arr[1].(string)
		if !ok {
			continue
		}
		if !strings.HasPrefix(jobJSON, "[[[") {
			continue
		}

		var jobData []interface{}
		if err := json.Unmarshal([]byte(jobJSON), &jobData); err != nil {
			continue
		}

		if jobPost := parseJobArray(jobData); jobPost != nil {
			results = append(results, *jobPost)
		}
	}

	return results, cursor, nil
}

// parseJobArray extracts a job from Google's nested array format.
// Indices: title=0, company=1, location=2, url=3[0][0], days_ago=12, description=19, id=28
func parseJobArray(data []interface{}) *scraper.Result {
	getStr := func(idx int) string {
		if idx < len(data) {
			if s, ok := data[idx].(string); ok {
				return s
			}
		}
		return ""
	}

	title := getStr(0)
	if title == "" {
		return nil
	}
	company := getStr(1)
	location := getStr(2)

	// URL: data[3][0][0]
	jobURL := ""
	if len(data) > 3 {
		if arr, ok := data[3].([]interface{}); ok && len(arr) > 0 {
			if inner, ok := arr[0].([]interface{}); ok && len(inner) > 0 {
				if s, ok := inner[0].(string); ok {
					jobURL = s
				}
			}
		}
	}

	// Days ago: data[12]
	daysAgo := getStr(12)
	datePosted := ""
	if daysAgo != "" {
		if m := regexp.MustCompile(`\d+`).FindString(daysAgo); m != "" {
			var days int
			fmt.Sscanf(m, "%d", &days)
			datePosted = time.Now().AddDate(0, 0, -days).Format("2006-01-02")
		}
	}

	// Description: data[19]
	description := getStr(19)

	// ID: data[28]
	id := getStr(28)
	if id == "" {
		id = jobURL
	}

	return &scraper.Result{
		ID:          id,
		Title:       title,
		Company:     company,
		Location:    location,
		Date:        datePosted,
		URL:         jobURL,
		Description: description,
	}
}
