package zen

// OpenCode wire parity (WP-162): the identity algorithms and free-tier
// request shape the zen gateway's free tier validates, mirrored from
// upstream opencode (captured live requests and opencode's upstream identifier fix). Everything
// here is internal to the package; callers see only the client's
// Config/Curate/Complete.

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// API families the gateway serves (catalog metadata values).
const (
	apiChatCompletions    = "openai-completions"
	apiOpenAIResponses    = "openai-responses"
	apiAnthropicMessages  = "anthropic-messages"
	apiGoogleGenerativeAI = "google-generative-ai"
)

// OpenCode v2 identity (verified against the live gateway from a captured
// live 2.0.26 request — see docs/research/zen-api.md and pi-zen shared.ts):
// since OpenCode v2 (2026-10-14) the server enforces the UA shape
// opencode/<channel>/<version>/<client> with major >= 2. The v1 form
// (opencode/<version>, optionally with the AI-SDK suffix) and any
// x-opencode-request header are rejected with 403 FreeTierError.
const (
	openCodeUAChannel = "latest"
	openCodeUAClient  = "cli"

	// openCodeMinUAVersion is the verified-good opencode v2 version floor:
	// the UA version must have major >= 2 or the free tier 403s. Stale
	// catalog versions clamp here (the verified-good UA floor parity).
	openCodeMinUAVersion = "2.0.26"
)

// The free-tier tool-name gate (verified by live bisection 2026-09-19,
// see docs/research/zen-api.md): the request's tools array must include at
// least TWO tool definitions named from the opencode client's tool set —
// otherwise 403 FreeTierError regardless of everything else. Custom tools
// (curate_posting, search_company, ...) ride along freely once the gate is
// satisfied. The gate validates the NAMES, not the schemas (verified live
// with empty properties), so the definitions are decoys — the deliberate
// Waypoint choice, keeping the client tool-free: the descriptions tell the
// model they are unavailable, which keeps reasoning models from wandering
// into a call (executeTool would answer harmlessly, but the detour burns a
// retry).
//
// A v2-era report (pi-zen, Oct 2026) claimed the gate hardens to full
// definition comparison; a subsequent probe (2026-09-20) returned 200 on
// both routes with these stubs — keep the stubs, and if the gate ever
// hardens the ZEN_LIVE=1 probe is the canary that says so.
const gateToolDescription = "This tool is currently unavailable and must not be used. Do not call it, and do not mention it."

var gateToolDefs = []toolDef{
	{
		Name:        "read",
		Description: gateToolDescription,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
	{
		Name:        "bash",
		Description: gateToolDescription,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	},
}

// opencodeUserAgent builds the v2 UA opencode sends:
// opencode/<channel>/<version>/<client>.
func opencodeUserAgent(version string) string {
	return "opencode/" + openCodeUAChannel + "/" + validOpencodeVersion(version) + "/" + openCodeUAClient
}

// versionShape anchors a bare X.Y.Z version: the only form the v2 UA clamp
// accepts verbatim (a bare "2.1" or junk clamps to the floor).
var versionShape = regexp.MustCompile(`^(\d+)\.\d+\.\d+`)

// validOpencodeVersion clamps a configured opencode version to one the v2
// free tier accepts: untouched when major >= 2, the verified-good floor
// otherwise (including unparseable input) — the verified-good floor the
// gateway enforces: a v1 form or low version draws 403
// FreeTierError from OpenCode before the request is ever served.
func validOpencodeVersion(version string) string {
	m := versionShape.FindStringSubmatch(strings.TrimSpace(version))
	if m == nil {
		return openCodeMinUAVersion
	}
	major, err := strconv.Atoi(m[1])
	if err != nil || major < 2 {
		return openCodeMinUAVersion
	}
	return m[0]
}

// opencodeIDAlphabet is opencode's base62 id alphabet, upstream order.
const opencodeIDAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// idTimeShift is the 12-bit per-millisecond counter space upstream
// multiplies the timestamp by (BigInt(timestamp) * 0x1000n).
const idTimeShift = 1 << 12

// opencodeID mints an opencode-shaped id: `<prefix>_<12 hex time><14
// base62 random>`. ses encodes the bitwise-complemented timestamp
// (descending — the server validates the canonical descending ses_ shape);
// msg encodes it ascending. random must carry >= 14 bytes; counter must
// stay within one millisecond (< idTimeShift).
func opencodeID(prefix string, timestampMS int64, counter int, random []byte) string {
	current := timestampMS*idTimeShift + int64(counter)
	var time48 uint64
	if prefix == "ses" {
		time48 = uint64(^current) & 0xFFFFFFFFFFFF // two's-complement low 48 bits
	} else {
		time48 = uint64(current) & 0xFFFFFFFFFFFF
	}
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteByte('_')
	for shift := 40; shift >= 0; shift -= 8 {
		fmt.Fprintf(&b, "%02x", byte(time48>>uint(shift)))
	}
	for i := 0; i < 14; i++ {
		b.WriteByte(opencodeIDAlphabet[random[i]%byte(len(opencodeIDAlphabet))])
	}
	return b.String()
}

// withGateDecoys is the gate seam: the decoys ride first, then the
// caller's tools. Composition detail (which decoys, how many, what
// position) lives here — callers state intent, not recipe.
func withGateDecoys(defs []toolDef) []toolDef {
	out := make([]toolDef, 0, len(defs)+len(gateToolDefs))
	out = append(out, gateToolDefs...)
	return append(out, defs...)
}

// ---- free-tier request shape -------------------------------------------------

// toolDef is the family-neutral tool description every wire wrapper is
// built from.
type toolDef struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// wireTool wraps a neutral tool def for one API family:
//   - chat-completions: {type: function, function: {name, description, parameters}}
//   - openai-responses: {type: function, name, description, parameters}
//   - anthropic-messages: {name, description, input_schema}
func wireTool(def toolDef, api string) map[string]any {
	switch api {
	case apiOpenAIResponses:
		return map[string]any{
			"type": "function", "name": def.Name,
			"description": def.Description, "parameters": def.Parameters,
		}
	case apiAnthropicMessages:
		return map[string]any{
			"name": def.Name, "description": def.Description, "input_schema": def.Parameters,
		}
	default: // apiChatCompletions and unknown families
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": def.Name, "description": def.Description, "parameters": def.Parameters,
			},
		}
	}
}

// curateToolDef is the locked curation tool (WP-131 prototype, 14/14 live
// verdicts), family-neutral. The parameter schema is the locked wire shape —
// do not trim it.
var curateToolDef = toolDef{
	Name:        "curate_posting",
	Description: "Record the curation verdict for one job posting against the user's brief.",
	Parameters: map[string]any{
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
}

// ensureFreeTierShape guarantees the free-tier request shape:
//
//  1. an empty tools array gains the gate decoys (read/bash placeholders,
//     wrapped for the payload's family) — the deliberate Waypoint policy:
//     the client stays tool-free, the decoy names satisfy the tool-name
//     gate, and on chat-completions (and unknown families, which default
//     to its shape) tool_choice is forced to "none" so the model cannot
//     call a tool it was told is unavailable (verified 200 against the
//     live gateway 2026-09-20; mimo's tool_choice rejection was observed
//     on real-tool turns, which never take this path);
//  2. non-empty tools are left untouched, together with any tool_choice
//     already set (mimo — WP-127);
//  3. openai-responses: tool_choice is coerced to "auto" (the only value
//     the Responses lane accepts; store:false stays buildBody's own
//     invariant);
//  4. anthropic-messages: no tool_choice handling (no "none" exists);
//  5. chat-completions assistant history gains a reasoning_content
//     placeholder when one is missing — a v2-era wire rule (verified by
//     pi-zen Oct 2026): reasoning models 400 multi-turn replays without
//     it, and the response pipeline strips the field, so a placeholder can
//     never echo real reasoning back; non-reasoning models ignore the
//     field.
//
// The shape pass never mutates the caller's message slice in place — the
// system prompt and turn history are reused across attempts, and a mutated
// placeholder would compound. It copies first. google-generative-ai is
// skipped entirely (verified parity — a different tool shape with no free
// models today, left untouched rather than mangled).
func ensureFreeTierShape(payload map[string]any, api string) {
	if api == apiGoogleGenerativeAI {
		return
	}

	tools, _ := payload["tools"].([]any)
	chatLane := api != apiOpenAIResponses && api != apiAnthropicMessages
	if len(tools) == 0 {
		gate := []any{wireTool(gateToolDefs[0], api), wireTool(gateToolDefs[1], api)}
		payload["tools"] = gate
		if chatLane {
			if _, set := payload["tool_choice"]; !set {
				payload["tool_choice"] = "none"
			}
		}
	} else if choice, ok := payload["tool_choice"]; ok && choice == "none" {
		// Real tools must not ride a "none" choice (mimo — WP-127); delete
		// a lingered one defensively.
		delete(payload, "tool_choice")
	}

	if api == apiOpenAIResponses {
		payload["tool_choice"] = "auto"
	}
	if chatLane {
		injectReasoningPlaceholder(payload)
	}
}

// injectReasoningPlaceholder maps the chat-shaped messages so every
// assistant turn carries a nonempty reasoning_content — Zen reasoning
// models 400 multi-turn chats without it, and the response pipeline strips
// the field, so a placeholder can never echo real reasoning (verified
// live Oct 2026). Non-assistant turns and turns that already carry one pass
// through; the input slice is copied, never mutated.
func injectReasoningPlaceholder(payload map[string]any) {
	switch msgs := payload["messages"].(type) {
	case []message:
		out := make([]message, len(msgs))
		copy(out, msgs)
		changed := false
		for i := range out {
			if out[i].Role == "assistant" && strings.TrimSpace(out[i].ReasoningContent) == "" {
				out[i].ReasoningContent = " "
				changed = true
			}
		}
		if changed {
			payload["messages"] = out
		}
	case []map[string]any:
		out := make([]map[string]any, len(msgs))
		copy(out, msgs)
		changed := false
		for i := range out {
			if role, _ := out[i]["role"].(string); role == "assistant" {
				if rc, ok := out[i]["reasoning_content"].(string); !ok || strings.TrimSpace(rc) == "" {
					out[i]["reasoning_content"] = " "
					changed = true
				}
			}
		}
		if changed {
			payload["messages"] = out
		}
	}
}
