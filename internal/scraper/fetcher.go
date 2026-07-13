package scraper

import (
	"context"
	"fmt"
	"html"
	"io"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Fetcher retrieves HTML from a URL. The default adapter (HTTPFetcher)
// handles browser headers, exponential backoff on 429/5xx, and returns
// "" on 404. Tests inject a MockFetcher with canned HTML.
type Fetcher interface {
	Fetch(ctx context.Context, url string) (string, error)
}

const defaultUA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

var (
	sharedTagRE   = regexp.MustCompile(`<[^>]+>`)
	sharedSpaceRE = regexp.MustCompile(`\s+`)
)

// CleanHTML strips tags, decodes entities, and collapses whitespace.
// Shared across all HTML-parsing scrapers.
func CleanHTML(s string) string {
	s = sharedTagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = sharedSpaceRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// HTTPFetcher is the default Fetcher adapter.
type HTTPFetcher struct {
	Client    *http.Client
	UserAgent string // defaults to defaultUA when empty
}

// Fetch retrieves HTML with browser headers and exponential backoff.
func (f *HTTPFetcher) Fetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	ua := f.UserAgent
	if ua == "" {
		ua = defaultUA
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	return DoWithRetry(ctx, req)
}

// DoWithRetry executes an HTTP request with exponential backoff on 429/5xx.
// Returns ("", nil) on 404. Resets the request body between retries via
// req.GetBody when available. Shared by HTTPFetcher.Fetch and POST-based
// scrapers (bitspilani, vit) that build their own requests.
func DoWithRetry(ctx context.Context, req *http.Request) (string, error) {
	const maxRetries = 6
	delay := time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Reset body for retry if GetBody is available
		if attempt > 0 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return "", err
			}
			req.Body = body
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			resp.Body.Close()
			if attempt == maxRetries {
				return "", fmt.Errorf("request failed: %d %s", resp.StatusCode, resp.Status)
			}
			jitter := time.Duration(rand.Intn(500)) * time.Millisecond
			select {
			case <-time.After(delay + jitter):
			case <-ctx.Done():
				return "", ctx.Err()
			}
			delay *= 2
			if delay > 10*time.Second {
				delay = 10 * time.Second
			}
			continue
		}

		if resp.StatusCode == 404 {
			resp.Body.Close()
			return "", nil
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return "", fmt.Errorf("request failed: %d %s", resp.StatusCode, resp.Status)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return "", err
		}
		return string(body), nil
	}

	return "", fmt.Errorf("request failed after max retries")
}
