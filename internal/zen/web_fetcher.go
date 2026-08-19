package zen

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/udit-001/waypoint/internal/scraper"
)

// PageFetcher fetches a web page and returns its content as markdown.
// Injected by the caller; when nil, fetch_posting returns a fallback.
type PageFetcher interface {
	FetchPage(ctx context.Context, url string) (string, error)
}

// MarkdownFetcher renders a page to markdown in a headless browser
// (mdsvc). Optional fallback for pages plain HTTP cannot read — JS-heavy
// boards render as empty shells without it. Nil disables the fallback.
type MarkdownFetcher interface {
	Fetch(ctx context.Context, url string) (string, error)
}

// maxFetchBytes caps a fetched page (matches opencode's webfetch limit).
const maxFetchBytes = 5 << 20 // 5MB

// maxFetchResultChars caps what a tool result feeds back into the
// conversation — enough evidence for a verdict without blowing context.
const maxFetchResultChars = 6000

// WebFetcher implements PageFetcher with plain HTTP + HTML→markdown.
// Budgeted like ExaSearcher: cap fetches per cycle, cache per URL so
// repeat fetches are free.
type WebFetcher struct {
	cap     int // max live fetches (0 = unlimited)
	count   int
	fetcher *scraper.HTTPFetcher
	// Markdown is the rendering-service fallback when the direct fetch
	// fails or yields no text. May be nil.
	Markdown MarkdownFetcher
	cache    map[string]string
	mu       sync.Mutex
}

// NewWebFetcher creates a page fetcher with a per-cycle fetch budget.
// cap <= 0 means unlimited.
func NewWebFetcher(cap int) *WebFetcher {
	return &WebFetcher{
		cap:     cap,
		fetcher: &scraper.HTTPFetcher{},
		cache:   make(map[string]string),
	}
}

// FetchPage fetches url and returns markdown. Cached per URL.
func (w *WebFetcher) FetchPage(ctx context.Context, url string) (string, error) {
	w.mu.Lock()
	if md, ok := w.cache[url]; ok {
		w.mu.Unlock()
		return md, nil
	}
	w.mu.Unlock()

	if w.cap > 0 && w.count >= w.cap {
		return "", fmt.Errorf("fetch budget exhausted (%d/%d) — judge on the posting text alone", w.count, w.cap)
	}

	w.mu.Lock()
	w.count++
	w.mu.Unlock()

	fetchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	html, err := w.fetcher.Fetch(fetchCtx, url)
	var md string
	if err == nil {
		if len(html) > maxFetchBytes {
			html = html[:maxFetchBytes]
		}
		md = pageToMarkdown(html, contentTypeOf(html))
	}
	// Headless-rendering fallback: JS-heavy boards come back empty from
	// plain HTTP; the mdsvc tier renders them for real.
	if md == "" && w.Markdown != nil {
		if rendered, rerr := w.Markdown.Fetch(fetchCtx, url); rerr == nil {
			md = rendered
		}
	}
	if md == "" {
		if err != nil {
			return "", fmt.Errorf("fetch: %w", err)
		}
		return "", fmt.Errorf("page has no text content")
	}
	if len(md) > maxFetchResultChars {
		md = md[:maxFetchResultChars] + "\n…(truncated)"
	}

	w.mu.Lock()
	w.cache[url] = md
	w.mu.Unlock()
	return md, nil
}

// contentTypeOf sniffs text content types from the body — cheap and
// dependency-free for the one job of rejecting binary pages.
func contentTypeOf(body string) string {
	s := strings.TrimSpace(strings.ToLower(body))
	if strings.HasPrefix(s, "<!doctype html") || strings.HasPrefix(s, "<html") || (len(s) > 0 && s[0] == '<') {
		return "text/html"
	}
	return "text/plain"
}

// pageToMarkdown converts an HTML page to markdown, falling back to
// plain text. Non-HTML text passes through unchanged.
func pageToMarkdown(body, ct string) string {
	if ct == "text/html" {
		return scraper.HTMLToMarkdown(body)
	}
	return strings.TrimSpace(body)
}
