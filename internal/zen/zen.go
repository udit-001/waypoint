// Package zen is the curation client: one OpenAI-compatible
// chat-completions client with client-side session history and
// tool-call verdicts (WP-138).
//
// Wire research (docs/research/zen-api.md, WP-127): zen is an
// OpenAI-compatible gateway; free models are gated by the
// User-Agent header (omit it and every free call 429s). The
// conversation lives client-side — the wire is stateless per call;
// x-opencode-session only feeds metrics + provider stickiness.
//
// Tool schema is locked by the WP-131 curation prototype (14/14
// live verdicts): curate_posting{verdict, score, reasons}. No
// explicit tool_choice (mimo rejects it); max_tokens >= 1024.
package zen

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/udit-001/waypoint/internal/version"
)

// Verdict decisions.
const (
	DecisionShortlist = "shortlist"
	DecisionDismiss   = "dismiss"
)

// maxPostingChars caps the markdown body sent per posting turn
// (prototype budget: ~3.5k chars).
const maxPostingChars = 3500

// maxTokens is the per-request completion budget. Reasoning models burn
// tokens before the tool call — the old 1024 (calibrated on mimo) truncated
// deepseek-v4-flash mid-reasoning on empty-description postings (live-probed
// 2026-08-19: finish_reason=length, no tool call; worst observed completion
// 901 tokens). 4096 leaves headroom for toolLoop turns with history.
const maxTokens = 4096

// sleep is the test seam for backoff pauses.
var (
	sleep     = time.Sleep
	realSleep = time.Sleep
)

// Config shapes the client. Any OpenAI-compatible endpoint
// substitutes by config alone (zen, OpenRouter, LM Studio, Ollama).
type Config struct {
	BaseURL       string // scheme + host, no path (client appends /v1/chat/completions)
	APIKey        string
	Model         string
	FallbackModel string // opt-in paid fallback; empty = no fallback
	UserAgent     string // free-tier gate: "opencode/<version>"
	ProjectID     string // x-opencode-project; omitted when empty
	HTTPClient    *http.Client
}

// DefaultConfig returns the shipped configuration: the zen gateway,
// free-tier default model, no paid fallback.
//
// The UA gate (docs/research/zen-api.md) accepts opencode/<any-version>:
// verified live with "opencode/dev" against mimo-v2.5-free on 2026-08-18.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://opencode.ai/zen",
		Model:     "mimo-v2.5-free",
		UserAgent: "opencode/" + version.Version,
	}
}

// Client is one OpenAI-compatible chat-completions client. One per
// daemon process: it owns the process-stable session identity used
// for provider stickiness.
type Client struct {
	cfg           Config
	httpClient    *http.Client
	sessionID     string
	baseURL       string
	fallbackModel string
	searcher      CompanySearcher // optional company research
	pageFetcher   PageFetcher     // optional posting-page fetch
}

// New builds a Client with a fresh ses_<32hex> identity.
func New(cfg Config) *Client {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 120 * time.Second}
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("zen: crypto/rand unavailable: " + err.Error())
	}
	return &Client{
		cfg:           cfg,
		httpClient:    cfg.HTTPClient,
		sessionID:     "ses_" + hex.EncodeToString(b),
		baseURL:       strings.TrimRight(cfg.BaseURL, "/"),
		fallbackModel: cfg.FallbackModel,
	}
}

// SetCompanySearcher injects a company research backend.
func (c *Client) SetCompanySearcher(s CompanySearcher) {
	c.searcher = s
}

// SetPageFetcher injects a posting-page fetch backend.
func (c *Client) SetPageFetcher(f PageFetcher) {
	c.pageFetcher = f
}

// Posting is one curation input: a URL and its merged markdown.
type Posting struct {
	URL      string
	Markdown string
}

// Verdict is the locked curation output.
type Verdict struct {
	Decision string   `json:"verdict"`  // shortlist | dismiss
	Score    int      `json:"score"`    // 0-100, ranks the queue (never gates)
	Note     string   `json:"note"`     // scout's note: 1-2 sentences to the user, the skim layer
	Overview string   `json:"overview"` // neutral 2-3 sentence summary of the posting itself (what/where/stack) — replaces the verbatim body on the review page
	Reasons  []Reason `json:"reasons"`  // 1-3 tagged facts, one per fit dimension
}

// Reason is one skimmable fit fact. Kind says whether it supports the
// match or works against it; Field names the dimension it's about, so
// the UI can badge it (role/domain/level/location/company) instead of
// rendering a sentence. Text is a terse fragment (≤60 chars), not prose.
type Reason struct {
	Kind  string `json:"kind"`  // match | gap
	Field string `json:"field"` // role | domain | level | location | company
	Text  string `json:"text"`  // terse fragment, e.g. "Senior distributed-systems role"
}

// Error is a curation failure. Fatal errors (bad key, out of credits)
// must stop the cycle; recoverable ones leave the posting "new" for
// the next attempt.
type Error struct {
	Status int  // HTTP status; 0 for network/malformed-response errors
	Fatal  bool // true = retrying can't help
	Msg    string
}

func (e *Error) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("zen: HTTP %d: %s", e.Status, e.Msg)
	}
	return "zen: " + e.Msg
}

// ---- wire types ---------------------------------------------------------

type message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type completionResponse struct {
	Choices []struct {
		FinishReason string  `json:"finish_reason"`
		Message      message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// curateTool is the locked tool schema from the WP-131 prototype.
var curateTool = map[string]any{
	"type": "function",
	"function": map[string]any{
		"name":        "curate_posting",
		"description": "Record the curation verdict for one job posting against the user's brief.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"verdict": map[string]any{"type": "string", "enum": []string{DecisionShortlist, DecisionDismiss}},
				"score":   map[string]any{"type": "integer", "description": "0-100 fit score"},
				"note": map[string]any{
					"type":        "string",
					"description": "1-2 sentences the row doesn't already tell the user. The title, company, location, score, and reason chips are displayed elsewhere — surface what they can't: the stack and systems, the team's remit, the product it touches, the hiring bar, or the real dealbreaker behind a gap chip. Lead with the decisive fact. Max 200 chars.",
				},
				"overview": map[string]any{
					"type":        "string",
					"description": "Neutral 2-3 sentence summary of THIS posting only: what the team builds, core stack/systems, work model and location. Facts from the posting — no comparison to the person, no fit opinion (that's note/reasons). Max 60 words.",
				},
				"reasons": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"kind":  map[string]any{"type": "string", "enum": []string{"match", "gap"}},
							"field": map[string]any{"type": "string", "enum": []string{"role", "domain", "level", "location", "company"}},
							"text":  map[string]any{"type": "string", "description": "terse fragment, max 60 chars, no full sentences"},
						},
						"required": []string{"kind", "field", "text"},
					},
					"description": "1-3 fit facts: one per dimension (role, domain, level, location, company), each tagged match or gap",
				},
			},
			"required": []string{"verdict", "score", "note", "overview", "reasons"},
		},
	},
}

// searchCompanyTool lets the model research a company before judging.
var searchCompanyTool = map[string]any{
	"type": "function",
	"function": map[string]any{
		"name":        "search_company",
		"description": "Look up what a company does, its domain, size, and tech stack. Call this when the posting doesn't say what the company builds.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"company": map[string]any{"type": "string", "description": "Company name to research"},
			},
			"required": []string{"company"},
		},
	},
}

// CompanySearcher is the interface for looking up company descriptions.
// Injected by the caller; when nil, search_company returns a fallback.
type CompanySearcher interface {
	SearchCompany(ctx context.Context, name string) (string, error)
}

// fetchPostingTool lets the model pull the full posting page when the
// description provided is too thin to judge.
var fetchPostingTool = map[string]any{
	"type": "function",
	"function": map[string]any{
		"name":        "fetch_posting",
		"description": "Fetch the full job posting page as markdown. Call this when the posting description is empty or too thin to judge against the brief.",
		"parameters": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{"type": "string", "description": "The posting URL from the POSTING turn"},
			},
			"required": []string{"url"},
		},
	},
}

// ---- session --------------------------------------------------------------

// Session is one curation conversation: system prompt + turns, held
// client-side. Create one per cycle (session-per-cycle, WP-131).
// Session holds the system prompt shared by every posting. Each Curate
// call runs its own conversation — session-per-posting, not per-cycle:
// cross-posting history caused quadratic token growth, score anchoring,
// and protocol poisoning (an unanswered tool_calls message from posting
// N 400s posting N+1 — live on deepseek 2026-08-19). Company research
// caching lives in the injected CompanySearcher, not the conversation.
type Session struct {
	c            *Client
	systemPrompt string
}

// NewSession starts a session with the given system prompt (the brief
// portrait plus the judge instruction).
func (c *Client) NewSession(systemPrompt string) *Session {
	return &Session{
		c:            c,
		systemPrompt: systemPrompt,
	}
}

// Curate judges one posting in a fresh conversation and returns its
// verdict. The conversation is discarded afterwards.
//
// Failure ladder: retry x3 with backoff -> fallback model (if
// configured) x3 -> recoverable error. Each retry starts clean —
// there is no cross-posting state to poison. Fatal errors (401/402/403)
// return immediately.
func (s *Session) Curate(ctx context.Context, p Posting) (Verdict, error) {
	turn := message{Role: "user", Content: postingTurn(p)}

	models := []string{s.c.cfg.Model}
	if s.c.fallbackModel != "" {
		models = append(models, s.c.fallbackModel)
	}

	var lastErr *Error
	first := true // no backoff before the very first request
	for _, model := range models {
		for attempt := 0; attempt < 3; attempt++ {
			if !first {
				if err := ctx.Err(); err != nil {
					return Verdict{}, &Error{Msg: "context done: " + err.Error()}
				}
				sleep(time.Duration(attempt+1) * 2 * time.Second)
			}
			first = false

			// Fresh conversation per attempt: system + this posting.
			msgs := []message{{Role: "system", Content: s.systemPrompt}, turn}
			v, err := s.c.toolLoop(ctx, model, msgs)
			if err != nil {
				if err.Fatal {
					return Verdict{}, err
				}
				lastErr = err
				continue
			}
			return v, nil
		}
	}
	if lastErr == nil {
		lastErr = &Error{Msg: "no attempts made"}
	}
	return Verdict{}, lastErr
}

// postingTurn formats one posting as a user turn, URL first so the
// verdict stays anchored to it.
func postingTurn(p Posting) string {
	return fmt.Sprintf("POSTING\nURL: %s\n\n%s", p.URL, truncateChars(p.Markdown, maxPostingChars))
}

func truncateChars(s string, n int) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\n", " "), "\r", "")
	if len(s) <= n {
		return s
	}
	s = s[:n]
	// Back off to a rune boundary so JSON marshaling doesn't emit U+FFFD.
	for len(s) > 0 && !utf8.ValidString(s[len(s)-1:]) {
		s = s[:len(s)-1]
	}
	return s
}

// call performs one chat-completions request and returns the assistant
// message exactly as received (all tool calls intact — models may batch
// parallel tool calls), plus the validated curate_posting verdict if one
// is present. curateFound is false when the model called only research
// tools; that is not an error — toolLoop answers them and loops.
func (c *Client) call(ctx context.Context, model string, msgs []message) (message, Verdict, bool, *Error) {
	body := map[string]any{
		"model":      model,
		"messages":   msgs,
		"max_tokens": maxTokens,
		"tools":      []any{curateTool, searchCompanyTool, fetchPostingTool},
		// no tool_choice — mimo rejects it (WP-127)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return message{}, Verdict{}, false, &Error{Msg: "marshal request: " + err.Error()}
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return message{}, Verdict{}, false, &Error{Msg: "build request: " + err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("User-Agent", c.cfg.UserAgent) // free-tier gate
	req.Header.Set("x-opencode-client", "cli")
	req.Header.Set("x-opencode-session", c.sessionID)
	if c.cfg.ProjectID != "" {
		req.Header.Set("x-opencode-project", c.cfg.ProjectID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return message{}, Verdict{}, false, &Error{Msg: "request: " + err.Error()}
	}
	defer resp.Body.Close()

	// Classify by status BEFORE decoding the body: a proxy returning an
	// HTML error page on 401/402 must not downgrade into a retryable
	// decode error.
	if resp.StatusCode != 200 {
		var out completionResponse
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out) // best effort
		msg := "request failed"
		if out.Error != nil && out.Error.Message != "" {
			msg = out.Error.Message
		}
		fatal := resp.StatusCode == 401 || resp.StatusCode == 402 || resp.StatusCode == 403
		return message{}, Verdict{}, false, &Error{Status: resp.StatusCode, Fatal: fatal, Msg: msg}
	}

	var out completionResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return message{}, Verdict{}, false, &Error{Status: resp.StatusCode, Msg: "decode response: " + err.Error()}
	}

	if len(out.Choices) == 0 {
		return message{}, Verdict{}, false, &Error{Msg: "empty choices in response"}
	}
	msg := out.Choices[0].Message
	if len(msg.ToolCalls) == 0 {
		// finish_reason distinguishes truncation (length — max_tokens too
		// small, reasoning burned the budget) from refusal (stop — model
		// answered in prose instead of calling the tool).
		return message{}, Verdict{}, false, &Error{Msg: "no tool call in response (finish_reason=" + out.Choices[0].FinishReason + ")"}
	}

	// Validate the curate_posting call if the model made one (possibly
	// batched with research calls).
	var v Verdict
	found := false
	for _, tc := range msg.ToolCalls {
		if tc.Function.Name != "curate_posting" {
			continue
		}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &v); err != nil {
			return message{}, Verdict{}, false, &Error{Msg: "decode tool arguments: " + err.Error()}
		}
		if v.Decision != DecisionShortlist && v.Decision != DecisionDismiss {
			return message{}, Verdict{}, false, &Error{Msg: "invalid verdict " + fmt.Sprintf("%q", v.Decision)}
		}
		if len(v.Reasons) == 0 {
			return message{}, Verdict{}, false, &Error{Msg: "verdict has no reasons"}
		}
		if strings.TrimSpace(v.Overview) == "" {
			return message{}, Verdict{}, false, &Error{Msg: "verdict has no overview"}
		}
		// Normalize: clamp text, default unknown kinds/fields so a
		// sloppy verdict still renders (legacy string reasons from older
		// cycles arrive as gaps with the raw sentence as text).
		if len(v.Note) > 240 {
			v.Note = v.Note[:240]
		}
		for i := range v.Reasons {
			r := &v.Reasons[i]
			if r.Kind != "match" && r.Kind != "gap" {
				r.Kind = "gap"
			}
			if r.Field == "" {
				r.Field = "role"
			}
			if len(r.Text) > 80 {
				r.Text = r.Text[:80]
			}
		}
		if v.Score < 0 {
			v.Score = 0
		}
		if v.Score > 100 {
			v.Score = 100
		}
		found = true
		break
	}
	return msg, v, found, nil
}

// toolLoop runs one posting's conversation: system + posting turn, then
// answer the model's tool calls until it calls curate_posting. Every
// tool_call_id in an assistant message is answered with a tool response —
// including curate_posting itself, which gets a synthetic "recorded" ack —
// otherwise the next request 400s ("insufficient tool messages following
// tool_calls message"; models may batch parallel calls, seen live on
// deepseek 2026-08-19). Bounded to prevent runaway tool loops.
func (c *Client) toolLoop(ctx context.Context, model string, msgs []message) (Verdict, *Error) {
	const maxTurns = 6 // fetch_posting + search_company + curate_posting = 3 typical

	for turn := 0; turn < maxTurns; turn++ {
		if err := ctx.Err(); err != nil {
			return Verdict{}, &Error{Msg: "context done: " + err.Error()}
		}

		res, v, curateFound, err := c.call(ctx, model, msgs)
		if err != nil {
			return Verdict{}, err
		}

		// Record the assistant message verbatim (all tool calls intact),
		// then one tool response per call — protocol-valid for any batch.
		ext := []message{res}
		for _, tc := range res.ToolCalls {
			ext = append(ext, message{
				Role:       "tool",
				Content:    c.executeTool(ctx, tc),
				ToolCallID: tc.ID,
			})
		}
		msgs = append(msgs, ext...)

		if curateFound {
			return v, nil
		}
	}

	return Verdict{}, &Error{Msg: "exceeded max tool turns"}
}

// executeTool runs one tool call and returns the string the model sees
// as the tool response.
func (c *Client) executeTool(ctx context.Context, tc toolCall) string {
	switch tc.Function.Name {
	case "curate_posting":
		// The verdict is already captured by call(); this ack only keeps
		// the conversation protocol-valid.
		return `{"status": "recorded"}`

	case "search_company":
		var args struct {
			Company string `json:"company"`
		}
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		if c.searcher != nil {
			r, err := c.searcher.SearchCompany(ctx, args.Company)
			if err != nil {
				return fmt.Sprintf("No information found for %s: %v", args.Company, err)
			}
			return r
		}
		return fmt.Sprintf("Company research not available. Company: %s", args.Company)

	case "fetch_posting":
		var args struct {
			URL string `json:"url"`
		}
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		if c.pageFetcher != nil {
			r, err := c.pageFetcher.FetchPage(ctx, args.URL)
			if err != nil {
				return fmt.Sprintf("Could not fetch the posting page: %v — judge on the posting text alone.", err)
			}
			return r
		}
		return "Posting page fetch not available. Judge on the posting text alone."

	default:
		return "Unknown tool."
	}
}

// ---- key resolution -----------------------------------------------------------

// ResolveKey picks the API key: the stored value wins, ZEN_API_KEY
// env is the fallback (the daemon must not depend on the launching
// shell's env, but the env override eases local testing).
func ResolveKey(stored string) string {
	stored = strings.TrimSpace(stored)
	if stored != "" {
		return stored
	}
	return strings.TrimSpace(os.Getenv("ZEN_API_KEY"))
}

// ProjectID derives a stable per-install project identifier from the
// database path (research decision #4: sha1 of the db path). Feeds
// x-opencode-project; harmless if the gateway ignores it.
func ProjectID(dbPath string) string {
	h := sha1.Sum([]byte(dbPath))
	return hex.EncodeToString(h[:])
}

// Complete runs one user turn against a system prompt in a fresh,
// tool-free conversation and returns the assistant text. Generic
// single-turn completion for non-curation jobs — facet expansion
// (WP-152). Fatal errors (bad key, out of credits) surface as *Error.
func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	body := map[string]any{
		"model": c.cfg.Model,
		"messages": []message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		"max_tokens": maxTokens,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", &Error{Msg: "marshal request: " + err.Error()}
	}
	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", &Error{Msg: "build request: " + err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	if c.cfg.ProjectID != "" {
		req.Header.Set("x-opencode-project", c.cfg.ProjectID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", &Error{Msg: "request: " + err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var out completionResponse
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out)
		msg := "request failed"
		if out.Error != nil && out.Error.Message != "" {
			msg = out.Error.Message
		}
		fatal := resp.StatusCode == 401 || resp.StatusCode == 402 || resp.StatusCode == 403
		return "", &Error{Status: resp.StatusCode, Fatal: fatal, Msg: msg}
	}

	var out completionResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return "", &Error{Msg: "decode response: " + err.Error()}
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", &Error{Msg: "empty completion"}
	}
	return out.Choices[0].Message.Content, nil
}
