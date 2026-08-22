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
	"time"
)

// ---- fake OpenAI-wire server ----------------------------------------------

// fakeZen is an in-process OpenAI-compatible chat-completions server.
// It records every request (headers + body) and replies from a scripted
// queue of responses; the last scripted response repeats.
type fakeZen struct {
	mu       sync.Mutex
	requests []capturedRequest
	script   []fakeResponse
	n        int
}

type capturedRequest struct {
	ua        string
	auth      string
	client    string
	project   string
	session   string
	body      map[string]any
	rawBody   string
	lastModel string
}

type fakeResponse struct {
	status   int
	toolCall bool // true: reply with a curate_posting tool call
	decision string
	score    int
	reasons  []string // raw text fragments; serialized as {kind:match, field:role, text}
}

func toolCallBody(model, decision string, score int, reasons []string) map[string]any {
	objs := make([]map[string]any, len(reasons))
	for i, r := range reasons {
		objs[i] = map[string]any{"kind": "match", "field": "role", "text": r}
	}
	args, _ := json.Marshal(map[string]any{
		"verdict": decision, "score": score, "reasons": objs,
	})
	return map[string]any{
		"id":     "chatcmpl-fake",
		"object": "chat.completion",
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role": "assistant",
				"tool_calls": []map[string]any{{
					"id":   "call-fake-1",
					"type": "function",
					"function": map[string]any{
						"name":      "curate_posting",
						"arguments": string(args),
					},
				}},
			},
			"finish_reason": "tool_calls",
		}},
	}
}

func plainBody(content string) map[string]any {
	return map[string]any{
		"id": "chatcmpl-fake",
		"choices": []map[string]any{{
			"message": map[string]any{"role": "assistant", "content": content},
		}},
	}
}

func errorBody(msg string) map[string]any {
	return map[string]any{"error": map[string]any{"message": msg}}
}

func (f *fakeZen) handler(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	var body map[string]any
	json.Unmarshal(raw, &body)

	model, _ := body["model"].(string)

	f.mu.Lock()
	f.requests = append(f.requests, capturedRequest{
		ua:        r.Header.Get("User-Agent"),
		auth:      r.Header.Get("Authorization"),
		client:    r.Header.Get("X-Opencode-Client"),
		project:   r.Header.Get("X-Opencode-Project"),
		session:   r.Header.Get("X-Opencode-Session"),
		body:      body,
		rawBody:   string(raw),
		lastModel: model,
	})
	resp := f.script[min(f.n, len(f.script)-1)]
	f.n++
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch {
	case resp.status != 0 && resp.status != 200:
		w.WriteHeader(resp.status)
		json.NewEncoder(w).Encode(errorBody("boom"))
	case resp.toolCall:
		json.NewEncoder(w).Encode(toolCallBody(model, resp.decision, resp.score, resp.reasons))
	default:
		json.NewEncoder(w).Encode(plainBody("no tool call here"))
	}
}

func (f *fakeZen) start(t *testing.T) *Client {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(f.handler))
	t.Cleanup(ts.Close)
	c := New(Config{
		BaseURL:   ts.URL,
		APIKey:    "test-key",
		Model:     "test-model",
		UserAgent: "opencode/test",
		ProjectID: "proj123",
	})
	return c
}

func (f *fakeZen) reqCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func (f *fakeZen) last() capturedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[len(f.requests)-1]
}

func (f *fakeZen) at(i int) capturedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[i]
}

// stubSleep replaces the package sleep seam and records backoff durations.
func stubSleep(t *testing.T) *[]time.Duration {
	var got []time.Duration
	sleep = func(d time.Duration) { got = append(got, d) }
	t.Cleanup(func() { sleep = realSleep })
	return &got
}

const briefPrompt = `BRIEF: {"facts":{"title":"Backend Engineer"}}`

// ---- happy path -------------------------------------------------------------

func TestCurate_sendsLockedWireShape(t *testing.T) {
	f := &fakeZen{script: []fakeResponse{{toolCall: true, decision: "shortlist", score: 88, reasons: []string{"Senior Go engineer role"}}}}
	c := f.start(t)

	sess := c.NewSession(briefPrompt)
	v, err := sess.Curate(context.Background(), Posting{
		URL:      "https://example.com/job/1",
		Markdown: "Senior Go engineer role.",
	})
	if err != nil {
		t.Fatalf("Curate: %v", err)
	}
	if v.Decision != DecisionShortlist || v.Score != 88 || len(v.Reasons) != 1 {
		t.Errorf("verdict = %+v", v)
	}

	// Endpoint: BaseURL + /v1/chat/completions
	if f.reqCount() != 1 {
		t.Fatalf("expected 1 request, got %d", f.reqCount())
	}

	// Identity headers on every call.
	req := f.last()
	if req.auth != "Bearer test-key" {
		t.Errorf("Authorization = %q", req.auth)
	}
	if req.ua != "opencode/test" {
		t.Errorf("User-Agent = %q", req.ua)
	}
	if req.client != "cli" {
		t.Errorf("x-opencode-client = %q", req.client)
	}
	if req.project != "proj123" {
		t.Errorf("x-opencode-project = %q", req.project)
	}
	if !regexp.MustCompile(`^ses_[0-9a-f]{32}$`).MatchString(req.session) {
		t.Errorf("x-opencode-session = %q, want ses_<32hex>", req.session)
	}

	// Request body: locked curation wire shape.
	if req.body["model"] != "test-model" {
		t.Errorf("model = %v", req.body["model"])
	}
	if mt, _ := req.body["max_tokens"].(float64); mt < 1024 {
		t.Errorf("max_tokens = %v, want >= 1024", req.body["max_tokens"])
	}
	if _, present := req.body["tool_choice"]; present {
		t.Error("tool_choice must NOT be sent (mimo rejects it)")
	}
	tools, _ := req.body["tools"].([]any)
	if len(tools) != 3 {
		t.Fatalf("tools = %v, want curate_posting + search_company + fetch_posting", req.body["tools"])
	}
	fn0 := tools[0].(map[string]any)["function"].(map[string]any)
	if fn0["name"] != "curate_posting" {
		t.Errorf("tool[0] name = %v", fn0["name"])
	}
	fn1 := tools[1].(map[string]any)["function"].(map[string]any)
	if fn1["name"] != "search_company" {
		t.Errorf("tool[1] name = %v", fn1["name"])
	}
	fn2 := tools[2].(map[string]any)["function"].(map[string]any)
	if fn2["name"] != "fetch_posting" {
		t.Errorf("tool[2] name = %v", fn2["name"])
	}

	// Messages: system prompt verbatim, then the posting turn.
	msgs, _ := req.body["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2 (system + user)", len(msgs))
	}
	sys := msgs[0].(map[string]any)
	if sys["role"] != "system" || sys["content"] != briefPrompt {
		t.Errorf("system message = %v", sys)
	}
	user := msgs[1].(map[string]any)
	if user["role"] != "user" ||
		!strings.Contains(user["content"].(string), "https://example.com/job/1") ||
		!strings.Contains(user["content"].(string), "Senior Go engineer role.") {
		t.Errorf("user message = %v", user)
	}
}

func TestCurate_freshConversationPerPosting(t *testing.T) {
	f := &fakeZen{script: []fakeResponse{{toolCall: true, decision: "dismiss", score: 20, reasons: []string{"wrong location"}}}}
	c := f.start(t)

	sess := c.NewSession(briefPrompt)
	if _, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: "one"}); err != nil {
		t.Fatalf("first Curate: %v", err)
	}
	if _, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/2", Markdown: "two"}); err != nil {
		t.Fatalf("second Curate: %v", err)
	}

	// Session-per-posting: the second request must start clean — system
	// + its own turn only. No inherited tool_calls (which 400 on the
	// follow-up when left unanswered), no prior posting text.
	msgs, _ := f.last().body["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("second request messages = %d, want 2 (fresh conversation)", len(msgs))
	}
	if strings.Contains(f.last().rawBody, "example.com/1") {
		t.Error("first posting leaked into second posting's conversation")
	}
	user := msgs[1].(map[string]any)
	if !strings.Contains(user["content"].(string), "https://example.com/2") {
		t.Errorf("messages[1] = %v, want posting 2 turn", user)
	}
}

func TestCurate_truncatesMarkdown(t *testing.T) {
	f := &fakeZen{script: []fakeResponse{{toolCall: true, decision: "shortlist", score: 50, reasons: []string{"ok"}}}}
	c := f.start(t)

	long := strings.Repeat("x", 10_000)
	sess := c.NewSession(briefPrompt)
	if _, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: long}); err != nil {
		t.Fatalf("Curate: %v", err)
	}

	user := f.last().body["messages"].([]any)[1].(map[string]any)
	content := user["content"].(string)
	if len(content) > 3600 { // 3500 cap + URL line + slack
		t.Errorf("posting content length = %d, want <= 3600", len(content))
	}
	if !strings.Contains(content, strings.Repeat("x", 100)) {
		t.Error("content should retain the markdown body")
	}
}

// ---- failure ladder -----------------------------------------------------------

func TestCurate_retriesThenSucceeds(t *testing.T) {
	sleeps := stubSleep(t)
	f := &fakeZen{script: []fakeResponse{
		{status: 500},
		{status: 429},
		{toolCall: true, decision: "shortlist", score: 70, reasons: []string{"fit"}},
	}}
	c := f.start(t)

	sess := c.NewSession(briefPrompt)
	v, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: "md"})
	if err != nil {
		t.Fatalf("Curate after retries: %v", err)
	}
	if v.Decision != DecisionShortlist {
		t.Errorf("verdict = %+v", v)
	}
	if f.reqCount() != 3 {
		t.Errorf("requests = %d, want 3", f.reqCount())
	}
	if len(*sleeps) != 2 {
		t.Errorf("backoff sleeps = %d, want 2", len(*sleeps))
	}
}

func TestCurate_fallbackModel(t *testing.T) {
	sleeps := stubSleep(t)
	f := &fakeZen{script: []fakeResponse{
		{status: 500}, {status: 500}, {status: 500}, // primary exhausted
		{toolCall: true, decision: "dismiss", score: 10, reasons: []string{"no"}},
	}}
	c := f.start(t)
	c.fallbackModel = "fallback-model"

	sess := c.NewSession(briefPrompt)
	v, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: "md"})
	if err != nil {
		t.Fatalf("Curate via fallback: %v", err)
	}
	if v.Decision != DecisionDismiss {
		t.Errorf("verdict = %+v", v)
	}
	if f.reqCount() != 4 {
		t.Errorf("requests = %d, want 4 (3 primary + 1 fallback)", f.reqCount())
	}
	if m := f.last().lastModel; m != "fallback-model" {
		t.Errorf("final request model = %q, want fallback-model", m)
	}
	if len(*sleeps) != 3 {
		t.Errorf("backoff sleeps = %d, want 3", len(*sleeps))
	}
}

func TestCurate_allFailSelfHeals(t *testing.T) {
	stubSleep(t)
	f := &fakeZen{script: []fakeResponse{{status: 503}}}
	c := f.start(t)
	c.fallbackModel = "fallback-model"

	sess := c.NewSession(briefPrompt)
	_, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: "md"})
	if err == nil {
		t.Fatal("expected error after primary + fallback exhausted")
	}
	ze, ok := err.(*Error)
	if !ok {
		t.Fatalf("error type = %T, want *zen.Error", err)
	}
	if ze.Fatal {
		t.Error("5xx exhaustion is recoverable, not fatal")
	}
	if f.reqCount() != 6 {
		t.Errorf("requests = %d, want 6 (3 primary + 3 fallback)", f.reqCount())
	}

	// Session self-heals: the failed turn is not in history, so the next
	// posting's request is system + its own turn only.
	f.script = []fakeResponse{{toolCall: true, decision: "shortlist", score: 60, reasons: []string{"fit"}}}
	if _, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/2", Markdown: "md"}); err != nil {
		t.Fatalf("post-failure Curate: %v", err)
	}
	msgs, _ := f.last().body["messages"].([]any)
	if len(msgs) != 2 {
		t.Errorf("post-heal messages = %d, want 2 (failed turn dropped)", len(msgs))
	}
	if strings.Contains(f.last().rawBody, "example.com/1") {
		t.Error("failed posting turn leaked into history")
	}
}

func TestCurate_fatalAuthStopsImmediately(t *testing.T) {
	f := &fakeZen{script: []fakeResponse{{status: 401}}}
	c := f.start(t)
	c.fallbackModel = "fallback-model"

	sess := c.NewSession(briefPrompt)
	_, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: "md"})
	ze, ok := err.(*Error)
	if !ok || !ze.Fatal {
		t.Fatalf("want fatal *zen.Error for 401, got %v (%T)", err, err)
	}
	if f.reqCount() != 1 {
		t.Errorf("requests = %d, want 1 (401: no retry, no fallback)", f.reqCount())
	}
}

func TestCurate_fatalCredits(t *testing.T) {
	f := &fakeZen{script: []fakeResponse{{status: 402}}}
	c := f.start(t)

	sess := c.NewSession(briefPrompt)
	_, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: "md"})
	ze, ok := err.(*Error)
	if !ok || !ze.Fatal || ze.Status != 402 {
		t.Fatalf("want fatal 402 *zen.Error, got %v", err)
	}
}

func TestCurate_noToolCallIsRetried(t *testing.T) {
	stubSleep(t)
	f := &fakeZen{script: []fakeResponse{ // 200s without tool calls
		{}, {}, {},
		// fallback succeeds
		{toolCall: true, decision: "shortlist", score: 55, reasons: []string{"ok"}},
	}}
	c := f.start(t)
	c.fallbackModel = "fallback-model"

	sess := c.NewSession(briefPrompt)
	if _, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: "md"}); err != nil {
		t.Fatalf("Curate: %v", err)
	}
	if f.reqCount() != 4 {
		t.Errorf("requests = %d, want 4 (3 malformed + 1 fallback)", f.reqCount())
	}
}

func TestCurate_invalidVerdictRetried(t *testing.T) {
	stubSleep(t)
	f := &fakeZen{script: []fakeResponse{
		{toolCall: true, decision: "maybe", score: 50, reasons: []string{"hmm"}},
		{toolCall: true, decision: "shortlist", score: 50, reasons: []string{"fit"}},
	}}
	c := f.start(t)

	sess := c.NewSession(briefPrompt)
	if _, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: "md"}); err != nil {
		t.Fatalf("Curate: %v", err)
	}
	if f.reqCount() != 2 {
		t.Errorf("requests = %d, want 2 (invalid verdict retried)", f.reqCount())
	}
}

func TestCurate_clampsScore(t *testing.T) {
	f := &fakeZen{script: []fakeResponse{{toolCall: true, decision: "shortlist", score: 150, reasons: []string{"fit"}}}}
	c := f.start(t)

	sess := c.NewSession(briefPrompt)
	v, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: "md"})
	if err != nil {
		t.Fatalf("Curate: %v", err)
	}
	if v.Score != 100 {
		t.Errorf("score = %d, want clamped to 100", v.Score)
	}
}

// ---- client identity + config -------------------------------------------------

func TestNew_sessionIDPerClient(t *testing.T) {
	f := &fakeZen{script: []fakeResponse{{toolCall: true, decision: "shortlist", score: 1, reasons: []string{"x"}}}}
	c1 := f.start(t)
	c2 := New(Config{BaseURL: c1.baseURL, APIKey: "k", Model: "m", UserAgent: "opencode/test"})

	s1a := c1.NewSession(briefPrompt)
	s1b := c1.NewSession(briefPrompt)
	s2 := c2.NewSession(briefPrompt)

	for _, s := range []*Session{s1a, s1b, s2} {
		if _, err := s.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: "md"}); err != nil {
			t.Fatalf("Curate: %v", err)
		}
	}
	// One session id per client (per daemon process), shared across sessions.
	id := f.at(0).session
	if id == "" || f.at(1).session != id || f.at(2).session == id {
		t.Errorf("session ids = %q %q %q; want c1 sessions equal, c2 different", f.at(0).session, f.at(1).session, f.at(2).session)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BaseURL != "https://opencode.ai/zen" {
		t.Errorf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.Model != "x-preview-f-free" {
		t.Errorf("Model = %q (default must be the free tier)", cfg.Model)
	}
	if cfg.FallbackModel != "" {
		t.Errorf("FallbackModel = %q, want empty (paid fallback is opt-in)", cfg.FallbackModel)
	}
	if !strings.HasPrefix(cfg.UserAgent, "opencode/") {
		t.Errorf("UserAgent = %q, want opencode/<version> (free-tier gate)", cfg.UserAgent)
	}
}

// ---- key resolution ------------------------------------------------------------

func TestResolveKey(t *testing.T) {
	t.Setenv("ZEN_API_KEY", "env-key")

	if got := ResolveKey(""); got != "env-key" {
		t.Errorf("ResolveKey(\"\") = %q, want env fallback", got)
	}
	if got := ResolveKey("stored-key"); got != "stored-key" {
		t.Errorf("ResolveKey(stored) = %q, want stored wins", got)
	}
	if got := ResolveKey("   "); got != "env-key" {
		t.Errorf("ResolveKey(whitespace) = %q, want env fallback", got)
	}
	t.Setenv("ZEN_API_KEY", "")
	if got := ResolveKey(""); got != "" {
		t.Errorf("ResolveKey with nothing set = %q, want empty", got)
	}
}

// ---- review follow-ups --------------------------------------------------------

// TestCurate_fatalWithNonJSONBody: a proxy-style 401 with an HTML body
// must still classify as fatal (status before decode, not after).
func TestCurate_fatalWithNonJSONBody(t *testing.T) {
	ts := newHTML401Server(t)
	c := New(Config{BaseURL: ts.URL, APIKey: "k", Model: "m", UserAgent: "opencode/test"})
	c.fallbackModel = "fb"

	sess := c.NewSession(briefPrompt)
	_, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: "md"})
	ze, ok := err.(*Error)
	if !ok || !ze.Fatal || ze.Status != 401 {
		t.Fatalf("want fatal 401, got %v (%T)", err, err)
	}
}

func newHTML401Server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(401)
		io.WriteString(w, "<html><body>proxy says no</body></html>")
	}))
}

// TestProjectID: stable per install, derived from the db path.
func TestProjectID(t *testing.T) {
	a := ProjectID("/home/u/.waypoint/waypoint.db")
	if a == "" || len(a) != 40 { // sha1 hex
		t.Errorf("ProjectID = %q, want 40-char sha1 hex", a)
	}
	if ProjectID("/home/u/.waypoint/waypoint.db") != a {
		t.Error("ProjectID must be stable for the same path")
	}
	if ProjectID("/other/waypoint.db") == a {
		t.Error("ProjectID must differ for different paths")
	}
}

// TestCurate_emptyReasonsRetried: the locked schema requires 1-3
// reasons; a verdict without them is malformed and gets retried.
func TestCurate_emptyReasonsRetried(t *testing.T) {
	stubSleep(t)
	f := &fakeZen{script: []fakeResponse{
		{toolCall: true, decision: "shortlist", score: 50},
		{toolCall: true, decision: "shortlist", score: 50, reasons: []string{"fit"}},
	}}
	c := f.start(t)

	sess := c.NewSession(briefPrompt)
	if _, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: "md"}); err != nil {
		t.Fatalf("Curate: %v", err)
	}
	if f.reqCount() != 2 {
		t.Errorf("requests = %d, want 2 (empty reasons retried)", f.reqCount())
	}
}

// TestCurate_runeSafeTruncation: truncation must not split a UTF-8
// rune — the marshaled body must not contain U+FFFD.
func TestCurate_runeSafeTruncation(t *testing.T) {
	f := &fakeZen{script: []fakeResponse{{toolCall: true, decision: "shortlist", score: 50, reasons: []string{"ok"}}}}
	c := f.start(t)

	// Multibyte text that certainly truncates mid-rune at 3500 bytes.
	md := strings.Repeat("…job…", 2000) // … is 3 bytes
	sess := c.NewSession(briefPrompt)
	if _, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: md}); err != nil {
		t.Fatalf("Curate: %v", err)
	}
	if strings.Contains(f.last().rawBody, "\ufffd") {
		t.Error("truncated body contains U+FFFD — rune was split")
	}
}

// TestCurate_cancelledContextStopsEarly: backoff between attempts
// must notice a cancelled context instead of sleeping on.
func TestCurate_cancelledContextStopsEarly(t *testing.T) {
	stubSleep(t)
	f := &fakeZen{script: []fakeResponse{{status: 500}}}
	c := f.start(t)
	c.fallbackModel = "fb"

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // dead before the first call

	sess := c.NewSession(briefPrompt)
	_, err := sess.Curate(ctx, Posting{URL: "https://example.com/1", Markdown: "md"})
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
	if n := f.reqCount(); n > 1 {
		t.Errorf("requests = %d, want <= 1 (cancel noticed after first failure)", n)
	}
}

// TestCurate_parallelToolCallsAllAnswered: deepseek batches research +
// verdict calls in one assistant message (live 2026-08-19). Every
// tool_call_id must get a tool response or the follow-up request 400s.
// curate_posting batched alongside search_company still yields the verdict.
func TestCurate_parallelToolCallsAllAnswered(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		msgs, _ := body["messages"].([]any)

		var resp map[string]any
		if len(msgs) == 2 { // first round: batch both calls
			resp = map[string]any{
				"id": "chatcmpl-parallel",
				"choices": []map[string]any{{
					"index":         0,
					"finish_reason": "tool_calls",
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{
							{"id": "call-1", "type": "function", "function": map[string]any{
								"name": "search_company", "arguments": `{"company":"Algolia"}`}},
							{"id": "call-2", "type": "function", "function": map[string]any{
								"name": "curate_posting", "arguments": `{"verdict":"shortlist","score":72,"reasons":[{"kind":"match","field":"role","text":"search API backend role"}]}`}},
						},
					},
				}},
			}
		} else { // second round: model closes out
			resp = toolCallBody("test-model", "shortlist", 72, []string{"search API backend role — matches Go/backend skills"})
			// verify the previous assistant message was fully answered
			assistant, _ := msgs[len(msgs)-2].(map[string]any)
			tcs, _ := assistant["tool_calls"].([]any)
			if len(tcs) == 2 {
				toolMsgs := 0
				for _, m := range msgs {
					if mm, ok := m.(map[string]any); ok && mm["role"] == "tool" {
						toolMsgs++
					}
				}
				if toolMsgs < 2 {
					t.Errorf("parallel tool_calls not all answered: %d tool messages", toolMsgs)
				}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := New(Config{BaseURL: ts.URL, APIKey: "k", Model: "m", UserAgent: "opencode/test"})
	sess := c.NewSession(briefPrompt)
	v, err := sess.Curate(context.Background(), Posting{URL: "https://example.com/1", Markdown: "md"})
	if err != nil {
		t.Fatalf("Curate with parallel tool calls: %v", err)
	}
	if v.Decision != DecisionShortlist || v.Score != 72 {
		t.Errorf("verdict = %+v", v)
	}
}
