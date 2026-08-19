// Package mdsvc fetches pages as markdown through Vercel's free
// conversion services. Both markdown.new and compress.new render pages
// in a headless browser, which recovers JS-heavy job boards a plain
// HTTP client cannot; both rate-limit by IP per day, so the client
// self-tracks a daily quota per service in an on-disk JSON state file
// (a fresh `autopilot run` process inherits the daemon's usage), marks
// a service exhausted on 429 or at the daily cap, and fails over to
// the next service. When every service is spent, Fetch returns an
// error and the caller (detail chain tier 3.5, zen fetch_posting
// fallback) degrades to its next tier.
//
// Call shapes (both GET, text/markdown response):
//
//	GET https://markdown.new/<url>                 preamble + frontmatter
//	GET https://compress.new/<url>?main_only=true  clean body
package mdsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// DailyQuota is the self-imposed per-service daily cap. Neither
	// service publishes a limit; ~500/day is the measured ceiling from
	// news-aggregator's production use. Waypoint's volume sits far below
	// it — the cap exists so a runaway cycle cannot burn goodwill.
	DailyQuota = 500

	// minContentChars rejects near-empty responses (error pages, SPA
	// shells) before they masquerade as posting bodies.
	minContentChars = 100

	// maxContentChars caps the returned markdown — 20k matches the
	// detail chain's own body cap; consumers cap tighter as needed.
	maxContentChars = 20000

	requestTimeout = 45 * time.Second
	maxBodyBytes   = 1 << 20
)

// errQuota signals a service has no quota left today. Unwrapped by the
// failover loop; surfaced to the caller only when every service has it.
var errQuota = errors.New("daily quota exhausted")

// service is one conversion endpoint. The order of services is the
// failover order: markdown.new first, compress.new behind it.
type service struct {
	name string
	url  func(rawURL string) string
}

var services = []service{
	{"markdown.new", func(u string) string { return "https://markdown.new/" + u }},
	{"compress.new", func(u string) string { return "https://compress.new/" + u + "?main_only=true" }},
}

// stateFile is the on-disk quota state. Day is a UTC date (2006-01-02);
// any other day resets every counter.
type stateFile struct {
	Day      string             `json:"day"`
	Services map[string]*svcUse `json:"services"`
}

type svcUse struct {
	Used      int  `json:"used"`
	Remaining int  `json:"remaining"` // last x-rate-limit-remaining; -1 unknown
	Exhausted bool `json:"exhausted"`
}

// Client fetches pages as markdown with quota-tracked failover between
// services. It implements detail.Fetcher and is safe for concurrent use.
type Client struct {
	statePath string

	// fetch is the seam under which HTTP lives; tests inject a fake.
	fetch func(ctx context.Context, url string) (*http.Response, error)

	mu    sync.Mutex
	state stateFile
}

// New creates a client whose quota state persists at statePath (the
// parent directory must exist — the data dir does). Empty statePath
// disables persistence: usage then lives only as long as the process.
func New(statePath string) *Client {
	c := &Client{
		statePath: statePath,
		state:     stateFile{Day: today(), Services: map[string]*svcUse{}},
	}
	c.fetch = c.httpFetch
	if statePath != "" {
		if b, err := os.ReadFile(statePath); err == nil {
			var s stateFile
			if json.Unmarshal(b, &s) == nil && s.Day == c.state.Day {
				if s.Services == nil {
					s.Services = map[string]*svcUse{}
				}
				c.state = s
			}
		}
	}
	return c
}

// Fetch returns the page at rawURL as markdown. markdown.new is tried
// first; on quota exhaustion, 429, or request failure it fails over to
// compress.new. An error means every service is spent or failed — the
// caller should degrade to its next tier.
func (c *Client) Fetch(ctx context.Context, rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("mdsvc: no url")
	}
	var errs []string
	for _, svc := range services {
		md, err := c.fetchService(ctx, svc, rawURL)
		if err == nil {
			return md, nil
		}
		errs = append(errs, fmt.Sprintf("%s: %v", svc.name, err))
	}
	return "", fmt.Errorf("mdsvc: %s", strings.Join(errs, "; "))
}

// fetchService runs one service. Quota is checked before the request;
// a 429 marks the service exhausted for the day, a success records
// usage and any service-reported remaining count.
func (c *Client) fetchService(ctx context.Context, svc service, rawURL string) (string, error) {
	if !c.available(svc.name) {
		return "", errQuota
	}

	reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	resp, err := c.fetch(reqCtx, svc.url(rawURL))
	if err != nil {
		return "", fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		c.exhaust(svc.name)
		return "", errQuota
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	content := Clean(string(body))
	if len(content) < minContentChars {
		return "", fmt.Errorf("content too short (%d chars)", len(content))
	}
	if len(content) > maxContentChars {
		content = content[:maxContentChars]
	}

	c.record(svc.name, headerRemaining(resp.Header))
	return content, nil
}

// available reports whether the service has quota left today. Races
// between the check and the next record can overshoot by a call or two
// under concurrency; the cap is goodwill, not billing.
func (c *Client) available(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	u := c.state.Services[name]
	return u == nil || (!u.Exhausted && u.Used < DailyQuota && u.Remaining != 0)
}

// exhaust marks a service done for the day and persists.
func (c *Client) exhaust(name string) {
	c.mu.Lock()
	c.use(name).Exhausted = true
	c.mu.Unlock()
	c.persist()
}

// record logs a successful call and the service-reported remaining
// quota (when the response carries x-rate-limit-remaining), then
// persists.
func (c *Client) record(name string, remaining int) {
	c.mu.Lock()
	u := c.use(name)
	u.Used++
	u.Remaining = remaining
	if remaining == 0 {
		u.Exhausted = true
	}
	c.mu.Unlock()
	c.persist()
}

// use returns (creating if needed) the state entry for name. The
// caller must hold c.mu.
func (c *Client) use(name string) *svcUse {
	u := c.state.Services[name]
	if u == nil {
		u = &svcUse{Remaining: -1}
		c.state.Services[name] = u
	}
	return u
}

// persist writes the state best-effort: quota tracking is advisory, so
// a failed write must not fail the fetch — worst case tomorrow
// over-spends by one session's usage.
func (c *Client) persist() {
	if c.statePath == "" {
		return
	}
	c.mu.Lock()
	b, err := json.Marshal(c.state)
	c.mu.Unlock()
	if err != nil {
		return
	}
	_ = os.WriteFile(c.statePath, b, 0o600)
}

// httpFetch performs the GET through the default client; the context
// bounds it.
func (c *Client) httpFetch(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	return http.DefaultClient.Do(req)
}

// Clean normalizes a service response into body markdown: drops the
// markdown.new preamble ("Title: …\nURL Source: …\nMarkdown Content:"),
// strips a leading YAML frontmatter block, and trims space. compress.new
// output passes through unchanged.
func Clean(s string) string {
	if i := strings.Index(s, "\nMarkdown Content:\n"); i >= 0 {
		s = s[i+len("\nMarkdown Content:\n"):]
	}
	if strings.HasPrefix(s, "---") {
		parts := strings.SplitN(s, "---", 3)
		if len(parts) == 3 {
			s = parts[2]
		}
	}
	return strings.TrimSpace(s)
}

// headerRemaining reads x-rate-limit-remaining from a response; -1 when
// absent or unparsable (the GET endpoints do not always send it).
func headerRemaining(h http.Header) int {
	v := strings.TrimSpace(h.Get("x-rate-limit-remaining"))
	if v == "" {
		return -1
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return -1
	}
	return n
}

func today() string { return time.Now().UTC().Format("2006-01-02") }
