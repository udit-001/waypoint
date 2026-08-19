// Package exa is the single seam to the hosted Exa MCP server. All Exa
// tool use — web_search_exa (company research) and web_fetch_exa
// (posting-page fetch) — goes through one client with one lifecycle
// (initialize once, session echoed on every call), one cache, and one
// rate-limit budget shared across tools: Exa limits the caller, not the
// tool, so fetches and searches draw down the same counter.
//
// Anonymous by default — the hosted MCP free tier needs no key, and an
// invalid Bearer token is a worse signal than none. Pass headers only
// when a real Exa credential exists.
package exa

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/udit-001/waypoint/internal/mcp"
)

// Endpoint is the hosted Exa MCP server with the tools waypoint uses
// whitelisted. The same set the LinkedIn fetcher exposes.
const Endpoint = "https://mcp.exa.ai/mcp?tools=web_search_exa,web_fetch_exa"

// maxSearchResults matches the Exa company-lookup pattern
// (patterns-companies.md: category:company <name>, numResults 5).
const maxSearchResults = 5

// maxFetchChars caps what web_fetch_exa extracts per page. Postings are
// shorter than profiles; 8k covers the longest job pages comfortably.
const maxFetchChars = 8000

// Client is one MCP session against the hosted Exa server. Safe for
// concurrent use. It implements zen.CompanySearcher (SearchCompany) and
// detail.ExaFetcher (FetchPage) directly — no adapters needed.
type Client struct {
	endpoint string
	headers  map[string]string

	// callTool is the seam under which the MCP protocol lives. Tests
	// inject a fake; production initializes lazily on first call.
	callTool func(ctx context.Context, tool string, args map[string]any) (string, error)

	budget    int // max live calls across all tools; <=0 = unlimited
	spent     int
	searches  map[string]string // lowercased company name → summary
	pages     map[string]string // url → markdown
	mu        sync.Mutex
	initOnce  sync.Once
	initErr   error
	sessionID string
	mcpClient *mcp.Client
}

// New creates an Exa client. Empty endpoint falls back to the hosted
// default; headers carries auth only when a real Exa key exists.
func New(endpoint string, headers map[string]string) *Client {
	if endpoint == "" {
		endpoint = Endpoint
	}
	c := &Client{
		endpoint: endpoint,
		headers:  headers,
		searches: make(map[string]string),
		pages:    make(map[string]string),
	}
	c.callTool = c.defaultCallTool
	return c
}

// SetBudget caps the number of live (uncached) Exa calls across all
// tools. Called per cycle by the autopilot; cache hits never spend.
func (c *Client) SetBudget(n int) {
	c.mu.Lock()
	c.budget = n
	c.spent = 0
	c.mu.Unlock()
}

// SearchCompany looks up what a company does using Exa's
// category:company pattern, returning a concise description suitable
// for an LLM brief. Cached per company name.
func (c *Client) SearchCompany(ctx context.Context, name string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return "", fmt.Errorf("exa: empty company name")
	}

	c.mu.Lock()
	if desc, ok := c.searches[key]; ok {
		c.mu.Unlock()
		return desc, nil
	}
	c.mu.Unlock()

	if err := c.spend(); err != nil {
		return "", err
	}

	raw, err := c.callTool(ctx, "web_search_exa", map[string]any{
		"query":      fmt.Sprintf("category:company %s", name),
		"numResults": maxSearchResults,
	})
	if err != nil {
		return "", fmt.Errorf("exa search: %w", err)
	}

	desc := Summarize(raw, 400)
	c.mu.Lock()
	c.searches[key] = desc
	c.mu.Unlock()
	return desc, nil
}

// Fetch fetches a URL's content as markdown via web_fetch_exa.
// Implements detail.ExaFetcher. Cached per URL.
func (c *Client) Fetch(ctx context.Context, rawURL string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("exa: empty url")
	}

	c.mu.Lock()
	if md, ok := c.pages[rawURL]; ok {
		c.mu.Unlock()
		return md, nil
	}
	c.mu.Unlock()

	if err := c.spend(); err != nil {
		return "", err
	}

	raw, err := c.callTool(ctx, "web_fetch_exa", map[string]any{
		"urls":          []string{rawURL},
		"maxCharacters": maxFetchChars,
	})
	if err != nil {
		return "", fmt.Errorf("exa fetch: %w", err)
	}
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("exa fetch: empty body")
	}

	c.mu.Lock()
	c.pages[rawURL] = raw
	c.mu.Unlock()
	return raw, nil
}

// spend consumes one unit of budget for a live call.
func (c *Client) spend() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.budget > 0 && c.spent >= c.budget {
		return fmt.Errorf("exa budget exhausted (%d/%d live calls this cycle)", c.spent, c.budget)
	}
	c.spent++
	return nil
}

// defaultCallTool performs the real MCP round trip: initialize once
// (negotiating protocol and session), then tools/call echoing the
// session header back — the same pattern as the LinkedIn fetcher.
// A failed initialize is retried on the next call (initErr resets).
func (c *Client) defaultCallTool(ctx context.Context, tool string, args map[string]any) (string, error) {
	if err := c.ensureInit(ctx); err != nil {
		return "", err
	}

	headers := map[string]string{}
	for k, v := range c.headers {
		headers[k] = v
	}
	if c.sessionID != "" {
		headers["Mcp-Session-Id"] = c.sessionID
	}
	return c.mcpClient.CallTool(ctx, headers, tool, args)
}

func (c *Client) ensureInit(ctx context.Context) error {
	c.mu.Lock()
	alreadyInit := c.mcpClient != nil && c.initErr == nil
	c.mu.Unlock()
	if alreadyInit {
		return nil
	}

	client := mcp.New(c.endpoint)
	headers := map[string]string{}
	for k, v := range c.headers {
		headers[k] = v
	}
	res, err := client.Initialize(ctx, headers)
	if err != nil {
		// Not sync.Once-permanent: next call retries initialize.
		return fmt.Errorf("exa init: %w", err)
	}

	c.mu.Lock()
	c.mcpClient = client
	c.sessionID = res.SessionID
	c.initErr = nil
	c.mu.Unlock()
	return nil
}

// Summarize extracts a concise description from raw Exa search output:
// first meaningful text lines up to limit chars, raw fallback.
func Summarize(raw string, limit int) string {
	if raw == "" {
		return "No information found."
	}
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "{") || strings.HasPrefix(line, "[") {
			continue
		}
		b.WriteString(line)
		b.WriteString(" ")
		if b.Len() > limit {
			break
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		out = raw
	}
	if len(out) > limit {
		out = out[:limit] + "…"
	}
	return out
}
