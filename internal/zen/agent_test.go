package zen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---- scripted agent server -------------------------------------------------
//
// Unlike fakeZen (shaped to curate_posting), agentScriptServer replies from
// a queue of raw assistant messages — tool calls with arbitrary arguments,
// plain prose, or HTTP errors — which is what the generic AgentLoop needs.

type agentScriptServer struct {
	script []map[string]any // one assistant message per turn; nil = HTTP 500
	bodies []map[string]any // every request body received
	n      int
	t      *testing.T
}

func (s *agentScriptServer) handler(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	s.bodies = append(s.bodies, body)

	i := min(s.n, len(s.script)-1)
	s.n++
	resp := s.script[i]
	if resp == nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(errorBody("boom"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	sse(w, chunk(resp, ""))
	sseDone(w)
}

func (s *agentScriptServer) start(t *testing.T) *Client {
	t.Helper()
	s.t = t
	ts := httptest.NewServer(http.HandlerFunc(s.handler))
	t.Cleanup(ts.Close)
	return New(Config{BaseURL: ts.URL, APIKey: "k", Model: "test-model"})
}

func (s *agentScriptServer) req(t *testing.T, i int) map[string]any {
	t.Helper()
	if i >= len(s.bodies) {
		t.Fatalf("request %d not received (%d total)", i, len(s.bodies))
	}
	return s.bodies[i]
}

func (s *agentScriptServer) lastBodyTools(t *testing.T, i int) []any {
	t.Helper()
	msgs, _ := s.req(t, i)["messages"].([]any)
	if len(msgs) == 0 {
		t.Fatal("no messages in request")
	}
	_ = msgs
	tools, _ := s.req(t, i)["tools"].([]any)
	return tools
}

// toolMsg builds an assistant message carrying one tool call.
func toolMsg(name, args string) map[string]any {
	return map[string]any{
		"role": "assistant",
		"tool_calls": []any{map[string]any{
			"id":       "call-" + name,
			"type":     "function",
			"function": map[string]any{"name": name, "arguments": args},
		}},
	}
}

// lastUserText reads the last user-role message's content from a request.
func lastUserText(t *testing.T, body map[string]any) string {
	t.Helper()
	msgs, _ := body["messages"].([]any)
	for i := len(msgs) - 1; i >= 0; i-- {
		m, _ := msgs[i].(map[string]any)
		if role, _ := m["role"].(string); role == "user" {
			c, _ := m["content"].(string)
			return c
		}
	}
	return ""
}

const exaArgs = `{"query":"infrastructure devtools companies","numResults":10}`
const submitArgs = `{"companies":[{"name":"Vercel","domain":"vercel.com","why":"deploy platform"}],"clusters":["devtools"]}`

func discoveryTools() []AgentTool {
	return []AgentTool{
		{Name: "exa_search", Description: "search", Parameters: map[string]any{"type": "object", "properties": map[string]any{}}},
		{Name: "submit_companies", Description: "submit", Terminal: true, Parameters: map[string]any{"type": "object", "properties": map[string]any{}}},
	}
}

// TestAgentLoop_submitsUnderBudget: search turn then terminal submit turn;
// the terminal call's arguments come back, tool replies reach the model
// with the matching call id, and every tool_call is answered.
func TestAgentLoop_submitsUnderBudget(t *testing.T) {
	s := &agentScriptServer{script: []map[string]any{
		toolMsg("exa_search", exaArgs),
		toolMsg("submit_companies", submitArgs),
	}}
	c := s.start(t)

	var exaCalls int
	args, turns, err := c.AgentLoop(context.Background(), "system prompt", "user turn", discoveryTools(), AgentExec(func(name, a string) (string, bool) {
		switch name {
		case "exa_search":
			exaCalls++
			return `[{"title":"Vercel","url":"https://vercel.com"}]`, false
		case "submit_companies":
			return `{"status":"recorded"}`, true
		}
		return "unknown tool", false
	}), 5)
	if err != nil {
		t.Fatal(err)
	}
	if turns != 2 {
		t.Errorf("turns = %d, want 2", turns)
	}
	if args != submitArgs {
		t.Errorf("terminal args = %s", args)
	}
	if exaCalls != 1 {
		t.Errorf("exa calls = %d, want 1", exaCalls)
	}

	// Turn 2's request carries the assistant + tool messages, tool reply
	// bound to the call id.
	body := s.req(t, 1)
	msgs, _ := body["messages"].([]any)
	if len(msgs) < 4 {
		t.Fatalf("turn-2 messages = %d, want >= 4 (system, user, assistant, tool)", len(msgs))
	}
	last, _ := msgs[len(msgs)-1].(map[string]any)
	if role, _ := last["role"].(string); role != "tool" {
		t.Errorf("last message role = %v, want tool", last["role"])
	}
	if id, _ := last["tool_call_id"].(string); id != "call-exa_search" {
		t.Errorf("tool_call_id = %v, want call-exa_search", id)
	}
}

// TestAgentLoop_nudgesProseTurn: a prose answer gets the terminal tool
// named back at it once, then the loop continues.
func TestAgentLoop_nudgesProseTurn(t *testing.T) {
	s := &agentScriptServer{script: []map[string]any{
		map[string]any{"role": "assistant", "content": "Here is what I found so far..."},
		toolMsg("submit_companies", submitArgs),
	}}
	c := s.start(t)

	args, turns, err := c.AgentLoop(context.Background(), "system prompt", "user turn", discoveryTools(), AgentExec(func(name, a string) (string, bool) {
		return `{"status":"recorded"}`, name == "submit_companies"
	}), 5)
	if err != nil {
		t.Fatal(err)
	}
	if turns != 2 {
		t.Errorf("turns = %d, want 2", turns)
	}
	if args != submitArgs {
		t.Errorf("terminal args = %s", args)
	}
	if nudge := lastUserText(t, s.req(t, 1)); !strings.Contains(nudge, "submit_companies") {
		t.Errorf("nudge = %q, want it to name the terminal tool", nudge)
	}
}

// TestAgentLoop_exhaustsTurns: maxTurns without a terminal call is an
// error carrying the turn count.
func TestAgentLoop_exhaustsTurns(t *testing.T) {
	s := &agentScriptServer{script: []map[string]any{
		toolMsg("exa_search", exaArgs),
		toolMsg("exa_search", exaArgs),
	}}
	c := s.start(t)

	_, turns, err := c.AgentLoop(context.Background(), "system prompt", "user turn", discoveryTools(), AgentExec(func(name, a string) (string, bool) {
		return "[]", false
	}), 2)
	if err == nil {
		t.Fatal("want exhaustion error")
	}
	if turns != 2 {
		t.Errorf("turns = %d, want 2", turns)
	}
	if !strings.Contains(err.Error(), "exceeded 2 turns") {
		t.Errorf("error = %v, want turn count named", err)
	}
}

// TestAgentLoop_gateToolsRideFirst: the free-tier gate's decoy tool names
// lead the tools array, then the caller's tools in order.
func TestAgentLoop_gateToolsRideFirst(t *testing.T) {
	s := &agentScriptServer{script: []map[string]any{
		toolMsg("submit_companies", submitArgs),
	}}
	c := s.start(t)

	if _, _, err := c.AgentLoop(context.Background(), "system prompt", "user turn", discoveryTools(), AgentExec(func(name, a string) (string, bool) {
		return "{}", true
	}), 5); err != nil {
		t.Fatal(err)
	}
	tools := s.lastBodyTools(t, 0)
	if len(tools) < 4 {
		t.Fatalf("tools = %d, want >= 4 (2 gate + 2 caller)", len(tools))
	}
	first, _ := tools[0].(map[string]any)
	fn, _ := first["function"].(map[string]any)
	if name, _ := fn["name"].(string); name != "read" {
		t.Errorf("first tool = %v, want the gate decoy read", fn["name"])
	}
}
