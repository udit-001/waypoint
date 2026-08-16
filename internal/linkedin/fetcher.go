package linkedin

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/udit-001/waypoint/internal/mcp"
)

// defaultEndpoint is the hosted Exa MCP server — the same "public free MCP
// setup" the waypoint skill documents for pi/opencode
// (internal/skills/waypoint/references/data/exa-setup.md). web_fetch_exa is a
// default tool, so no API key is required (anonymous free tier, rate-limited).
const defaultEndpoint = "https://mcp.exa.ai/mcp?tools=web_search_exa,web_fetch_exa,web_search_advanced_exa"

// fetchTool is the Exa tool that renders a URL's content as clean markdown.
const fetchTool = "web_fetch_exa"

// maxFetchChars caps how much of the profile page Exa extracts per page.
// LinkedIn profiles render long; 12k characters covers the core sections
// (verified against live output for several public profiles).
const maxFetchChars = 12000

// Fetcher fetches public LinkedIn profiles through Exa's hosted MCP server.
// It is stateless — each FetchProfile performs its own initialize + tool call.
type Fetcher struct {
	endpoint string
	// callTool is the seam under which the MCP protocol lives. Tests inject a
	// fake returning fixture markdown; production wires it to the MCP client
	// (initialize → call web_fetch_exa), mirroring how the income-tracker
	// adapters seam their MCP calls.
	callTool func(ctx context.Context, tool string, args map[string]any) (string, error)
}

// Option configures a Fetcher.
type Option func(*Fetcher)

// WithEndpoint overrides the MCP endpoint (tests).
func WithEndpoint(e string) Option { return func(f *Fetcher) { f.endpoint = e } }

// WithCallTool overrides the tool-call seam (tests).
func WithCallTool(fn func(ctx context.Context, tool string, args map[string]any) (string, error)) Option {
	return func(f *Fetcher) { f.callTool = fn }
}

// New creates a Fetcher against the hosted Exa MCP server.
func New(opts ...Option) *Fetcher {
	f := &Fetcher{endpoint: defaultEndpoint}
	for _, o := range opts {
		o(f)
	}
	if f.callTool == nil {
		f.callTool = func(ctx context.Context, tool string, args map[string]any) (string, error) {
			return callExaTool(ctx, f.endpoint, tool, args)
		}
	}
	return f
}

// FetchProfile fetches the public LinkedIn profile at rawURL and parses it
// into structured fields. The returned Profile may be partially empty (e.g.
// LinkedIn served a login wall) — callers check Empty() to surface that.
func (f *Fetcher) FetchProfile(ctx context.Context, rawURL string) (Profile, error) {
	u, err := ValidateURL(rawURL)
	if err != nil {
		return Profile{}, err
	}
	text, err := f.callTool(ctx, fetchTool, map[string]any{
		"urls":          []string{u},
		"maxCharacters": maxFetchChars,
	})
	if err != nil {
		return Profile{}, fmt.Errorf("exa fetch: %w", err)
	}
	return ParseProfile(text), nil
}

// ValidateURL accepts http(s) URLs on linkedin.com whose path starts with
// /in/ (a person profile). Company pages and other hosts are rejected with a
// clear message before any network call. Exported so the HTTP handler can
// classify validation failures (400) separately from fetch failures (502);
// FetchProfile re-validates internally so direct callers stay safe.
func ValidateURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("a LinkedIn profile URL is required")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q", rawURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("not a LinkedIn URL — expected https://www.linkedin.com/in/<username>")
	}
	host := strings.ToLower(u.Host)
	if host != "linkedin.com" && !strings.HasSuffix(host, ".linkedin.com") {
		return "", fmt.Errorf("not a LinkedIn URL — expected https://www.linkedin.com/in/<username>")
	}
	if !strings.HasPrefix(u.Path, "/in/") {
		return "", fmt.Errorf("not a LinkedIn profile URL — expected .../in/<username>")
	}
	return u.String(), nil
}

// callExaTool runs the real MCP round trip: initialize (negotiates protocol
// and any session), then call the tool echoing the session header back.
func callExaTool(ctx context.Context, endpoint, tool string, args map[string]any) (string, error) {
	client := mcp.New(endpoint, mcp.WithTimeout(50*time.Second))
	init, err := client.Initialize(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("initialize: %w", err)
	}
	headers := map[string]string{}
	if init.SessionID != "" {
		headers["Mcp-Session-Id"] = init.SessionID
	}
	text, err := client.CallTool(ctx, headers, tool, args)
	if err != nil {
		return "", fmt.Errorf("tools/call %q: %w", tool, err)
	}
	return text, nil
}
