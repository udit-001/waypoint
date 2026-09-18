package zen

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// The family tests route through the client with a catalog whose metadata
// names the family, against SSE upstreams scripted per request — the same
// seam the chat-family tests use, per the ticket's fake-SSE-server rule.

// sseUpstream records requests and replays scripted SSE payloads (one per
// request; the last repeats). Non-empty payloads are emitted verbatim as
// data lines; "error:JSON" entries emit an error event instead.
type sseUpstream struct {
	mu       sync.Mutex
	requests []capturedSSE
	payloads []string
}

type capturedSSE struct {
	path    string
	auth    string
	ua      string
	session string
	request string
	body    map[string]any
}

func (u *sseUpstream) handler(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	json.Unmarshal(raw, &body)

	u.mu.Lock()
	u.requests = append(u.requests, capturedSSE{
		path:    r.URL.Path,
		auth:    r.Header.Get("Authorization"),
		ua:      r.Header.Get("User-Agent"),
		session: r.Header.Get("X-Opencode-Session"),
		request: r.Header.Get("X-Opencode-Request"),
		body:    body,
	})
	idx := len(u.requests) - 1
	if idx >= len(u.payloads) {
		idx = len(u.payloads) - 1
	}
	payload := u.payloads[idx]
	u.mu.Unlock()

	w.Header().Set("Content-Type", "text/event-stream")
	for _, line := range strings.Split(payload, "\n") {
		if line == "" {
			continue
		}
		w.Write([]byte("data: " + line + "\n\n"))
	}
	if !strings.Contains(payload, "[DONE]") && !strings.Contains(payload, `"message_stop"`) {
		w.Write([]byte("data: [DONE]\n\n"))
	}
}

func (u *sseUpstream) start(t *testing.T) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(u.handler))
	t.Cleanup(ts.Close)
	return ts
}

func (u *sseUpstream) at(i int) capturedSSE {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.requests[i]
}

// familyCatalog builds a warm catalog upstream that assigns model → api.
func familyCatalog(t *testing.T, api string) *Catalog {
	t.Helper()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"count":1,"opencodeVersion":"1.18.31","models":[{"id":"union-alpha-free","name":"Union Alpha Free","api":"` + api + `"}]}`))
	}))
	t.Cleanup(up.Close)
	cat := NewCatalog(CatalogConfig{URL: up.URL})
	if _, err := cat.Models(context.Background()); err != nil {
		t.Fatalf("warm catalog: %v", err)
	}
	return cat
}

func newFamilyClient(t *testing.T, sseURL string, cat *Catalog) *Client {
	t.Helper()
	return New(Config{
		BaseURL:   sseURL,
		APIKey:    "sk-test",
		Model:     "union-alpha-free",
		ProjectID: "proj-seed",
		Catalog:   cat,
	})
}

// TestCatalog_API: known model → its family; unknown → chat-completions.
func TestCatalog_API(t *testing.T) {
	cat := familyCatalog(t, apiAnthropicMessages)
	if got := cat.API("union-alpha-free"); got != apiAnthropicMessages {
		t.Errorf("API = %q, want anthropic-messages", got)
	}
	if got := cat.API("nope"); got != apiChatCompletions {
		t.Errorf("unknown model API = %q, want the chat-completions default", got)
	}
}

// TestCurate_responsesFamily: a responses-family model routes to
// /v1/responses with input items and flat tools; the completed output
// assembles the verdict.
func TestCurate_responsesFamily(t *testing.T) {
	args := `{"verdict":"shortlist","score":77,"note":"n","overview":"o","reasons":[{"kind":"match","field":"role","text":"Go role"}]}`
	completed := `{"type":"response.completed","response":{"status":"completed","output":[{"type":"function_call","call_id":"call_1","name":"curate_posting","arguments":` + quoteJSON(args) + `}]}}`
	u := &sseUpstream{payloads: []string{strings.Join([]string{
		`{"type":"response.output_text.delta","delta":"thinking..."}`,
		`{"type":"response.function_call_arguments_delta","item_id":"fc_1","delta":"{\"verdict\":"}`,
		completed,
	}, "\n")}}
	ts := u.start(t)
	c := newFamilyClient(t, ts.URL, familyCatalog(t, apiOpenAIResponses))

	v, err := c.NewSession(briefPrompt).Curate(context.Background(), Posting{URL: "https://x.io/1", Markdown: "Senior Go role"})
	if err != nil {
		t.Fatalf("Curate: %v", err)
	}
	if v.Decision != DecisionShortlist || v.Score != 77 {
		t.Fatalf("verdict = %+v", v)
	}

	req := u.at(0)
	if !strings.HasSuffix(req.path, "/v1/responses") {
		t.Errorf("endpoint = %q, want /v1/responses", req.path)
	}
	if req.body["stream"] != true {
		t.Errorf("stream = %v, want true", req.body["stream"])
	}
	input, _ := req.body["input"].([]any)
	if len(input) != 2 {
		t.Fatalf("input = %d items, want system + user", len(input))
	}
	sys := input[0].(map[string]any)
	if sys["role"] != "system" {
		t.Errorf("input[0] = %v, want the system item", sys)
	}
	tools, _ := req.body["tools"].([]any)
	t0 := tools[0].(map[string]any)
	if t0["name"] != "curate_posting" || t0["function"] != nil {
		t.Errorf("tool[0] = %v, want the flat responses wrapper", t0)
	}
}

// TestCurate_anthropicFamily: an anthropic-messages model routes to
// /v1/messages with a top-level system, input_schema tools, and tool_use
// assembly from input_json_delta partials.
func TestCurate_anthropicFamily(t *testing.T) {
	u := &sseUpstream{payloads: []string{strings.Join([]string{
		`{"type":"message_start"}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"curate_posting"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"verdict\":\"dismiss\",\"score\":12,"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"note\":\"thin\",\"overview\":\"contract gig\",\"reasons\":[{\"kind\":\"gap\",\"field\":\"level\",\"text\":\"contractor\"}]}"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
		`{"type":"message_stop"}`,
	}, "\n")}}
	ts := u.start(t)
	c := newFamilyClient(t, ts.URL, familyCatalog(t, apiAnthropicMessages))

	v, err := c.NewSession(briefPrompt).Curate(context.Background(), Posting{URL: "https://x.io/1", Markdown: "Contract gig"})
	if err != nil {
		t.Fatalf("Curate: %v", err)
	}
	if v.Decision != DecisionDismiss || v.Score != 12 || len(v.Reasons) != 1 {
		t.Fatalf("verdict = %+v, want assembled dismiss verdict", v)
	}

	req := u.at(0)
	if !strings.HasSuffix(req.path, "/v1/messages") {
		t.Errorf("endpoint = %q, want /v1/messages", req.path)
	}
	if req.body["system"] != briefPrompt {
		t.Errorf("system = %v, want the brief at top level", req.body["system"])
	}
	tools, _ := req.body["tools"].([]any)
	t0 := tools[0].(map[string]any)
	if t0["name"] != "curate_posting" {
		t.Errorf("tool[0] = %v", t0)
	}
	if _, ok := t0["input_schema"]; !ok {
		t.Error("anthropic tool must carry input_schema")
	}
}

// TestCurate_anthropicToolResultEncoding: a search_company detour comes
// back as a tool_result block in a user turn (with the tool_use id), then
// the curation call proceeds.
func TestCurate_anthropicToolResultEncoding(t *testing.T) {
	researchArgs := `{"company":"Acme"}`
	verdictArgs := `{"verdict":"shortlist","score":60,"note":"n","overview":"o","reasons":[{"kind":"match","field":"role","text":"Go"}]}`
	u := &sseUpstream{payloads: []string{
		// Turn 1: the model asks for company research.
		strings.Join([]string{
			`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_r1","name":"search_company"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":` + quoteJSON(researchArgs) + `}}`,
			`{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
			`{"type":"message_stop"}`,
		}, "\n"),
		// Turn 2: the model curates.
		strings.Join([]string{
			`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_c1","name":"curate_posting"}}`,
			`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":` + quoteJSON(verdictArgs) + `}}`,
			`{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
			`{"type":"message_stop"}`,
		}, "\n"),
	}}
	ts := u.start(t)
	c := newFamilyClient(t, ts.URL, familyCatalog(t, apiAnthropicMessages))

	if _, err := c.NewSession(briefPrompt).Curate(context.Background(), Posting{URL: "https://x.io/1", Markdown: "m"}); err != nil {
		t.Fatalf("Curate: %v", err)
	}
	if u.at(1).path == "" {
		t.Fatal("second request missing — the tool result was not answered")
	}
	body := u.at(1).body
	msgs, _ := body["messages"].([]any)
	var found bool
	for _, m := range msgs {
		mm := m.(map[string]any)
		if mm["role"] != "user" {
			continue
		}
		content, _ := mm["content"].([]any)
		for _, c := range content {
			block := c.(map[string]any)
			if block["type"] == "tool_result" && block["tool_use_id"] == "toolu_r1" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("no tool_result block for toolu_r1 in follow-up: %v", msgs)
	}
}

// TestClient_familyDefaultsToChat: without a catalog the client stays on
// the chat-completions wire.
func TestClient_familyDefaultsToChat(t *testing.T) {
	u := &sseUpstream{payloads: []string{strings.Join([]string{
		`{"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"curate_posting","arguments":"{\"verdict\":\"shortlist\",\"score\":50,\"note\":\"n\",\"overview\":\"o\",\"reasons\":[{\"kind\":\"match\",\"field\":\"role\",\"text\":\"r\"}]}"}}]},"finish_reason":null}]}`,
		`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
	}, "\n")}}
	ts := u.start(t)
	c := New(Config{BaseURL: ts.URL, APIKey: "k", Model: "m"}) // no Catalog
	if _, err := c.NewSession(briefPrompt).Curate(context.Background(), Posting{URL: "https://x.io/1", Markdown: "m"}); err != nil {
		t.Fatalf("Curate: %v", err)
	}
	if !strings.HasSuffix(u.at(0).path, "/v1/chat/completions") {
		t.Errorf("endpoint = %q, want chat-completions", u.at(0).path)
	}
	if !regexp.MustCompile(`^opencode/1\.18\.31 ai-sdk/provider-utils`).MatchString(u.at(0).ua) {
		t.Errorf("UA = %q, want the floored opencode UA", u.at(0).ua)
	}
}

// quoteJSON embeds a string as a JSON string literal.
func quoteJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
