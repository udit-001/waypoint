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

// maxTokens is the per-request token budget. Reasoning models burn
// tokens before the tool call — below 1024 the call gets truncated.
const maxTokens = 1024

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

// Posting is one curation input: a URL and its merged markdown.
type Posting struct {
	URL      string
	Markdown string
}

// Verdict is the locked curation output.
type Verdict struct {
	Decision string   `json:"verdict"` // shortlist | dismiss
	Score    int      `json:"score"`   // 0-100, ranks the queue (never gates)
	Reasons  []string `json:"reasons"` // 1-3 short reasons, each citing a brief field
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
		Message message `json:"message"`
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
				"reasons": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "1-3 short reasons, each citing a brief field",
				},
			},
			"required": []string{"verdict", "score", "reasons"},
		},
	},
}

// ---- session --------------------------------------------------------------

// Session is one curation conversation: system prompt + turns, held
// client-side. Create one per cycle (session-per-cycle, WP-131).
type Session struct {
	c       *Client
	history []message
}

// NewSession starts a conversation with the given system prompt
// (the brief JSON verbatim plus the judge instruction).
func (c *Client) NewSession(systemPrompt string) *Session {
	return &Session{
		c:       c,
		history: []message{{Role: "system", Content: systemPrompt}},
	}
}

// Curate sends one posting as a turn and returns its verdict.
//
// Failure ladder: retry x3 with backoff -> fallback model (if
// configured) x3 -> recoverable error. The failed turn is dropped
// from history so the session self-heals — the next posting starts
// from clean state. Fatal errors (401/402/403) return immediately.
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
				// A cancelled cycle must not burn backoff sleeps.
				if err := ctx.Err(); err != nil {
					return Verdict{}, &Error{Msg: "context done: " + err.Error()}
				}
				sleep(time.Duration(attempt+1) * 2 * time.Second)
			}
			first = false
			msgs := append(append([]message{}, s.history...), turn)
			v, tc, err := s.c.call(ctx, model, msgs)
			if err != nil {
				if err.Fatal {
					return Verdict{}, err
				}
				lastErr = err
				continue
			}
			s.history = append(s.history, turn, assistantMessage(tc))
			return v, nil
		}
	}
	if lastErr == nil {
		lastErr = &Error{Msg: "no attempts made"}
	}
	return Verdict{}, lastErr
}

// assistantMessage rebuilds the assistant turn recorded in history.
func assistantMessage(tc toolCall) message {
	return message{Role: "assistant", ToolCalls: []toolCall{tc}}
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

// ---- transport --------------------------------------------------------------

// call performs one chat-completions request and validates the tool
// call into a Verdict.
func (c *Client) call(ctx context.Context, model string, msgs []message) (Verdict, toolCall, *Error) {
	body := map[string]any{
		"model":      model,
		"messages":   msgs,
		"max_tokens": maxTokens,
		"tools":      []any{curateTool},
		// no tool_choice — mimo rejects it (WP-127)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Verdict{}, toolCall{}, &Error{Msg: "marshal request: " + err.Error()}
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return Verdict{}, toolCall{}, &Error{Msg: "build request: " + err.Error()}
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
		return Verdict{}, toolCall{}, &Error{Msg: "request: " + err.Error()}
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
		return Verdict{}, toolCall{}, &Error{Status: resp.StatusCode, Fatal: fatal, Msg: msg}
	}

	var out completionResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out); err != nil {
		return Verdict{}, toolCall{}, &Error{Status: resp.StatusCode, Msg: "decode response: " + err.Error()}
	}

	if len(out.Choices) == 0 || len(out.Choices[0].Message.ToolCalls) == 0 {
		return Verdict{}, toolCall{}, &Error{Msg: "no tool call in response"}
	}
	tc := out.Choices[0].Message.ToolCalls[0]

	var v Verdict
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &v); err != nil {
		return Verdict{}, toolCall{}, &Error{Msg: "decode tool arguments: " + err.Error()}
	}
	if v.Decision != DecisionShortlist && v.Decision != DecisionDismiss {
		return Verdict{}, toolCall{}, &Error{Msg: "invalid verdict " + fmt.Sprintf("%q", v.Decision)}
	}
	if len(v.Reasons) == 0 {
		return Verdict{}, toolCall{}, &Error{Msg: "verdict has no reasons"}
	}
	if v.Score < 0 {
		v.Score = 0
	}
	if v.Score > 100 {
		v.Score = 100
	}
	return v, tc, nil
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
