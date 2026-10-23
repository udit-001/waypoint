package zen

import (
	"encoding/json"
	"math/big"
	"reflect"
	"regexp"
	"testing"
)

// The wire tests pin the opencode parity algorithms against upstream's
// documented shapes (docs/research + captured live requests): id formats,
// the descending ses_/ascending msg_ timestamp encoding, the UA gate, and
// the per-family placeholder tool wrappers.

const sesShape = `^ses_[0-9a-f]{12}[0-9A-Za-z]{14}$`
const msgShape = `^msg_[0-9a-f]{12}[0-9A-Za-z]{14}$`

// TestOpencodeID_shapes: 12 hex time chars + 14 base62 chars, per prefix.
func TestOpencodeID_shapes(t *testing.T) {
	ses := opencodeID("ses", 1758240000000, 0, []byte("0123456789ABCDEFGHIJKLMNOP"))
	if !regexp.MustCompile(sesShape).MatchString(ses) {
		t.Errorf("ses id = %q, want %s", ses, sesShape)
	}
	msg := opencodeID("msg", 1758240000000, 0, []byte("0123456789ABCDEFGHIJKLMNOP"))
	if !regexp.MustCompile(msgShape).MatchString(msg) {
		t.Errorf("msg id = %q, want %s", msg, msgShape)
	}
}

// TestOpencodeID_descendingVsAscending: for the same instant, ses_ carries
// the bitwise complement of the timestamp (descending — starts f4f5… for a
// msg_ of 0b0a…, the live captured pair) while msg_ is ascending.
func TestOpencodeID_descendingVsAscending(t *testing.T) {
	const ts = 1758240000000
	ses := opencodeID("ses", ts, 0, make([]byte, 14))
	msg := opencodeID("msg", ts, 0, make([]byte, 14))
	// Decode both time fields as 48-bit big-endian. The wire carries 6
	// bytes, so the low 48 bits of `current` are what survives — upstream's
	// extraction drops the top bits the same way.
	sesTime := new(big.Int).SetBytes(hexBytes(t, ses[4:16]))
	msgTime := new(big.Int).SetBytes(hexBytes(t, msg[4:16]))
	mask48 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 48), big.NewInt(1))
	current := new(big.Int).Lsh(big.NewInt(ts), 12) // ts<<12 + 0 counter
	wantMsg := new(big.Int).And(current, mask48)
	wantSes := complement48(t, wantMsg)
	if sesTime.Cmp(wantSes) != 0 {
		t.Errorf("ses time = %s, want two's complement %s of current %s", sesTime, wantSes, current)
	}
	if msgTime.Cmp(wantMsg) != 0 {
		t.Errorf("msg time = %s, want low-48 current %s", msgTime, wantMsg)
	}
	// And the pair is complementary within the 48-bit field (the captured
	// live evidence: ses_ f4f5… vs msg_ 0b0a…).
	sum := new(big.Int).Add(sesTime, msgTime)
	if sum.Cmp(mask48) != 0 {
		t.Errorf("ses+msg = %s, want 2^48-1 (complementary pair)", sum)
	}
}

// TestOpencodeID_counterInTime: the counter rides in the low 12 bits.
func TestOpencodeID_counterInTime(t *testing.T) {
	const ts = 1758240000000
	a := opencodeID("msg", ts, 0, make([]byte, 14))
	b := opencodeID("msg", ts, 1, make([]byte, 14))
	ta := new(big.Int).SetBytes(hexBytes(t, a[4:16]))
	tb := new(big.Int).SetBytes(hexBytes(t, b[4:16]))
	if new(big.Int).Sub(tb, ta).Int64() != 1 {
		t.Errorf("counter+1 changed time field by %s, want 1", new(big.Int).Sub(tb, ta))
	}
}

// TestOpencodeUserAgent: full v2 UA = opencode/<channel>/<version>/<client>.
func TestOpencodeUserAgent(t *testing.T) {
	if got, want := opencodeUserAgent("2.1.3"), "opencode/latest/2.1.3/cli"; got != want {
		t.Errorf("UA = %q, want %q", got, want)
	}
	if got, want := opencodeUserAgent(openCodeMinUAVersion), "opencode/latest/"+openCodeMinUAVersion+"/cli"; got != want {
		t.Errorf("UA = %q, want %q", got, want)
	}
}

// TestValidOpencodeVersion: v1 versions (major < 2, the form the pre-v2
// curated lists carried) and garbage clamp to the floor; v2 versions pass
// through untouched.
func TestValidOpencodeVersion(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", openCodeMinUAVersion},
		{"garbage", openCodeMinUAVersion},
		{"1.19.3", openCodeMinUAVersion}, // a v1 catalogue version — clamps
		{"1.9", openCodeMinUAVersion},
		{"0.5.1", openCodeMinUAVersion},
		{"2.0.0", "2.0.0"},
		{"2.0.26", "2.0.26"},
		{"2.1.3\n", "2.1.3"},
	}
	for _, c := range cases {
		if got := validOpencodeVersion(c.in); got != c.want {
			t.Errorf("validOpencodeVersion(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestGateToolsPerFamily: the gate-satisfying client tools (read, bash)
// wrap per family — chat-completions nests under function; responses and
// anthropic are flat.
func TestGateToolsPerFamily(t *testing.T) {
	for _, def := range gateToolDefs {
		chat := wireTool(def, apiChatCompletions)
		resp := wireTool(def, apiOpenAIResponses)
		anth := wireTool(def, apiAnthropicMessages)

		if got, _ := chat["function"].(map[string]any)["name"].(string); got != def.Name {
			t.Errorf("chat wrapper name = %v", got)
		}
		if got, _ := resp["name"].(string); got != def.Name {
			t.Errorf("responses wrapper name = %v", got)
		}
		if got, _ := anth["name"].(string); got != def.Name {
			t.Errorf("anthropic wrapper name = %v", got)
		}
		if _, ok := resp["function"]; ok {
			t.Error("responses wrapper must be flat (no function nesting)")
		}
		if _, ok := anth["input_schema"]; !ok {
			t.Error("anthropic wrapper must carry input_schema")
		}
	}
}

// TestWithGateDecoys: the gate seam — decoys ride first, caller's tools
// follow, and the caller's slice is never aliased.
func TestWithGateDecoys(t *testing.T) {
	edefs := []toolDef{curateToolDef, searchCompanyToolDef}
	got := withGateDecoys(edefs)
	if len(got) != 4 {
		t.Fatalf("len = %d, want decoys + 2 caller tools", len(got))
	}
	if got[0].Name != gateToolDefs[0].Name || got[1].Name != gateToolDefs[1].Name {
		t.Errorf("gate decoys missing at the front: %v, %v", got[0].Name, got[1].Name)
	}
	if got[2].Name != curateToolDef.Name || got[3].Name != searchCompanyToolDef.Name {
		t.Errorf("caller tools not preserved: %v, %v", got[2].Name, got[3].Name)
	}
	edefs[0] = toolDef{Name: "mutated"}
	if got[2].Name != curateToolDef.Name {
		t.Error("result aliases the caller's slice")
	}
}

// TestEnsureFreeTierShape: empty tools gain the two gate decoys; populated
// tools are untouched; the decoy turn carries tool_choice "none".
func TestEnsureFreeTierShape(t *testing.T) {
	payload := map[string]any{"model": "m"}
	ensureFreeTierShape(payload, apiChatCompletions)
	tools, _ := payload["tools"].([]any)
	if len(tools) != 2 {
		t.Fatalf("tools = %v, want the two gate decoys", payload["tools"])
	}
	fn0, _ := tools[0].(map[string]any)["function"].(map[string]any)
	fn1, _ := tools[1].(map[string]any)["function"].(map[string]any)
	if fn0["name"] != "read" || fn1["name"] != "bash" {
		t.Errorf("gate tools = %v, %v, want read + bash", fn0["name"], fn1["name"])
	}
	if got, _ := payload["tool_choice"]; got != "none" {
		t.Errorf("tool_choice = %v, want \"none\" on the decoy turn", got)
	}

	populated := map[string]any{"tools": []any{wireTool(curateToolDef, apiChatCompletions)}}
	ensureFreeTierShape(populated, apiChatCompletions)
	if got, _ := populated["tools"].([]any); len(got) != 1 {
		t.Errorf("populated tools changed: %v", populated["tools"])
	}
	if _, ok := populated["tool_choice"]; ok {
		t.Error("tool_choice must be left untouched on turns with real tools (mimo rejects it)")
	}

	// Unknown family defaults to chat-completions; the google family is
	// skipped entirely (verified parity — different tool shape, no free
	// models on it today).
	unknown := map[string]any{}
	ensureFreeTierShape(unknown, "some-unknown-family")
	if got, _ := unknown["tools"].([]any); len(got) != 2 {
		t.Errorf("unknown family tools = %v, want the two chat-completions gate decoys", unknown["tools"])
	}
	google := map[string]any{}
	ensureFreeTierShape(google, apiGoogleGenerativeAI)
	if _, ok := google["tools"]; ok {
		t.Error("google-generative-ai payload must be left untouched")
	}
}

// TestEnsureFreeTierShape_toolChoice: tool-less turns gain tool_choice
// "none" on completions-shaped families (verified 200 against the live
// gateway 2026-09-20, so the model cannot call a decoy) — anthropic has no
// "none"; the Responses lane always carries "auto" (verified by pi-zen,
// Oct 2026); google-generative-ai is skipped.
func TestEnsureFreeTierShape_toolChoiceNoneOnInjectedDecoys(t *testing.T) {
	cases := []struct {
		api     string
		want    any
		wantSet bool
	}{
		{apiChatCompletions, "none", true},
		{"some-unknown-family", "none", true},
		{apiOpenAIResponses, "auto", true},
		{apiAnthropicMessages, nil, false}, // Anthropic has no "none"
		{apiGoogleGenerativeAI, nil, false},
	}
	for _, tc := range cases {
		payload := map[string]any{"model": "m"}
		ensureFreeTierShape(payload, tc.api)
		got, ok := payload["tool_choice"]
		if ok != tc.wantSet || got != tc.want {
			t.Errorf("api %q: tool_choice = %v (set=%v), want %v (set=%v)", tc.api, got, ok, tc.want, tc.wantSet)
		}
	}
	// Real tools must not ride a "none" choice — deleted (mimo rejects any
	// explicit choice on real-tool turns).
	populated := map[string]any{
		"tools":       []any{wireTool(curateToolDef, apiChatCompletions)},
		"tool_choice": "none",
	}
	ensureFreeTierShape(populated, apiChatCompletions)
	if _, ok := populated["tool_choice"]; ok {
		t.Error("real-tool turn must not carry tool_choice \"none\" (mimo rejects it)")
	}
}

// TestEnsureFreeTierShape_reasoningPlaceholder: chat-lane assistant turns
// gain the reasoning_content placeholder (a v2-era rule, verified by
// pi-zen, Oct 2026: reasoning models 400 multi-turn replays without it);
// non-assistant turns and turns that already carry one pass through; the
// responses/anthropic/google lanes are excluded.
func TestEnsureFreeTierShape_reasoningPlaceholder(t *testing.T) {
	payload := map[string]any{
		"messages": []message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "u1"},
			{Role: "assistant", Content: "a1", ToolCalls: []toolCall{{ID: "call_1"}}},
			{Role: "tool", Content: "t1"},
		},
	}
	turns, _ := payload["messages"].([]message) // the caller's original slice
	ensureFreeTierShape(payload, apiChatCompletions)
	msgs, _ := payload["messages"].([]message)
	if len(msgs) != 4 {
		t.Fatalf("messages = %d, want the same 4 turns", len(msgs))
	}
	if turns[0].ReasoningContent != "" || turns[1].ReasoningContent != "" || turns[3].ReasoningContent != "" {
		t.Errorf("non-assistant turns must not gain the placeholder: %+v", turns)
	}
	if msgs[2].ReasoningContent != " " {
		t.Errorf("assistant placeholder = %q, want single space", msgs[2].ReasoningContent)
	}
	// The caller's slice is never mutated in place (turn history is reused
	// across retry attempts).
	if turns[2].ReasoningContent != "" {
		t.Errorf("original assistant message was mutated in place: %+v", turns[2])
	} // A turn that already carries one keeps it.
	withRC := map[string]any{"messages": []message{{Role: "assistant", ReasoningContent: "real reasoning"}}}
	ensureFreeTierShape(withRC, apiChatCompletions)
	got, _ := withRC["messages"].([]message)
	if got[0].ReasoningContent != "real reasoning" {
		t.Errorf("existing reasoning_content overwritten: %q", got[0].ReasoningContent)
	}
	// Responses and anthropic lanes are excluded.
	resp := map[string]any{"input": []any{}}
	ensureFreeTierShape(resp, apiOpenAIResponses)
	if _, ok := resp["messages"]; ok {
		t.Error("responses lane must not gain chat messages")
	}
	anth := map[string]any{"messages": []message{{Role: "assistant", Content: "a"}}}
	ensureFreeTierShape(anth, apiAnthropicMessages)
	anthMsgs, _ := anth["messages"].([]message)
	if anthMsgs[0].ReasoningContent != "" {
		t.Error("anthropic lane must not gain the chat placeholder")
	}
}

// TestGateToolDefs_decoyShape: the gate tools are decoys — names carry the
// gate, but the descriptions must tell the model not to call them (a
// real-tool description invited reasoning models onto a decoy detour). The
// literal text is pinned so a quiet drift in the decoy wording surfaces
// here.
func TestGateToolDefs_decoyShape(t *testing.T) {
	const wantDescription = "This tool is currently unavailable and must not be used. Do not call it, and do not mention it."
	for _, def := range gateToolDefs {
		if def.Name != "read" && def.Name != "bash" {
			t.Errorf("gate tool name = %q, want read or bash", def.Name)
		}
		if def.Description != wantDescription {
			t.Errorf("gate tool %q description = %q, want the pinned decoy text", def.Name, def.Description)
		}
		// The gate validates names, not schemas — empty properties are
		// served by the gateway.
		if props, ok := def.Parameters["properties"].(map[string]any); !ok || len(props) != 0 {
			t.Errorf("gate tool %q properties = %v, want empty", def.Name, def.Parameters["properties"])
		}
	}
}

// TestCurateToolDefPerFamily: the real tools wrap per family too.
func TestCurateToolDefPerFamily(t *testing.T) {
	chat := wireTool(curateToolDef, apiChatCompletions)
	if _, ok := chat["function"]; !ok {
		t.Error("chat-completions tool must nest under function")
	}
	anth := wireTool(curateToolDef, apiAnthropicMessages)
	if _, ok := anth["input_schema"]; !ok {
		t.Error("anthropic tool must carry input_schema")
	}
	// Same tool name in both wrappers.
	fn, _ := chat["function"].(map[string]any)
	if fn["name"] != anth["name"] {
		t.Errorf("tool names diverge: %v vs %v", fn["name"], anth["name"])
	}
	// The parameters JSON round-trips identically (same schema both sides).
	pj, _ := json.Marshal(fn["parameters"])
	aj, _ := json.Marshal(anth["input_schema"])
	if !reflect.DeepEqual(pj, aj) {
		t.Error("chat function.parameters and anthropic input_schema diverge")
	}
}

// ---- helpers ----------------------------------------------------------------

func hexBytes(t *testing.T, s string) []byte {
	t.Helper()
	if len(s)%2 != 0 {
		t.Fatalf("odd hex %q", s)
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(out); i++ {
		hi := hexVal(t, s[2*i])
		lo := hexVal(t, s[2*i+1])
		out[i] = hi<<4 | lo
	}
	return out
}

func hexVal(t *testing.T, c byte) byte {
	t.Helper()
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	}
	t.Fatalf("non-hex char %q", c)
	return 0
}

func complement48(t *testing.T, v *big.Int) *big.Int {
	t.Helper()
	// ~(v) restricted to 48 bits: (2^48 - 1) ^ v
	mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 48), big.NewInt(1))
	return new(big.Int).Xor(mask, v)
}
