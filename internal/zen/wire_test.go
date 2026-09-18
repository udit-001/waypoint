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

// TestOpencodeIDFromSeed_stable: same seed, same id; matches the documented
// sha1 split (6 time bytes + 14 body bytes).
func TestOpencodeIDFromSeed_stable(t *testing.T) {
	a := opencodeIDFromSeed("ses", "install-seed")
	b := opencodeIDFromSeed("ses", "install-seed")
	if a != b {
		t.Fatalf("seed ids differ: %q vs %q", a, b)
	}
	if !regexp.MustCompile(sesShape).MatchString(a) {
		t.Errorf("seed id = %q, want %s", a, sesShape)
	}
	if a == opencodeIDFromSeed("ses", "other-seed") {
		t.Error("different seeds produced the same id")
	}
}

// TestOpencodeUserAgent: full UA = opencode/<version> + AI-SDK suffix.
func TestOpencodeUserAgent(t *testing.T) {
	got := opencodeUserAgent("1.18.31")
	want := "opencode/1.18.31 ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14"
	if got != want {
		t.Errorf("UA = %q, want %q", got, want)
	}
}

// TestValidOpencodeVersion: stale/garbage versions clamp to the floor;
// fresh ones pass through untouched.
func TestValidOpencodeVersion(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", openCodeMinUAVersion},
		{"garbage", openCodeMinUAVersion},
		{"1.16.9", openCodeMinUAVersion}, // below the 1.17 server gate
		{"1.9", openCodeMinUAVersion},
		{"0.5.1", openCodeMinUAVersion},
		{"1.17.0", "1.17.0"},
		{"1.18.31", "1.18.31"},
		{"1.19.2", "1.19.2"},
		{"2.0.0", "2.0.0"},
		{"1.18.31\n", "1.18.31"},
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

// TestEnsureFreeTierShape: empty/missing tools gain the placeholder in the
// payload's family wrapper; non-empty tools are untouched.
func TestEnsureFreeTierShape(t *testing.T) {
	payload := map[string]any{"model": "m"}
	ensureFreeTierShape(payload, apiChatCompletions)
	tools, _ := payload["tools"].([]any)
	if len(tools) != 2 {
		t.Fatalf("tools = %v, want the two gate tools", payload["tools"])
	}
	fn0, _ := tools[0].(map[string]any)["function"].(map[string]any)
	fn1, _ := tools[1].(map[string]any)["function"].(map[string]any)
	if fn0["name"] != "read" || fn1["name"] != "bash" {
		t.Errorf("gate tools = %v, %v, want read + bash", fn0["name"], fn1["name"])
	}

	populated := map[string]any{"tools": []any{wireTool(curateToolDef, apiChatCompletions)}}
	ensureFreeTierShape(populated, apiChatCompletions)
	if got, _ := populated["tools"].([]any); len(got) != 1 {
		t.Errorf("populated tools changed: %v", populated["tools"])
	}

	// Unknown family defaults to chat-completions; the google family is
	// skipped entirely (pi-zen parity — different tool shape, no free models).
	unknown := map[string]any{}
	ensureFreeTierShape(unknown, "some-unknown-family")
	if got, _ := unknown["tools"].([]any); len(got) != 2 {
		t.Errorf("unknown family tools = %v, want the two chat-completions gate tools", unknown["tools"])
	}
	google := map[string]any{}
	ensureFreeTierShape(google, apiGoogleGenerativeAI)
	if _, ok := google["tools"]; ok {
		t.Error("google-generative-ai payload must be left untouched")
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
