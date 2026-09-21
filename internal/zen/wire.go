package zen

// OpenCode wire parity (WP-162): the identity algorithms and free-tier
// request shape the zen gateway's free tier validates, mirrored from
// upstream opencode (captured live requests and 9router PR #4105). Everything
// here is internal to the package; callers see only the client's
// Config/Curate/Complete.

import (
	"crypto/sha1"
	"encoding/hex"
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

// openCodeMinUAVersion is the verified-good opencode version: the server
// serves the free tier only to User-Agent opencode/<version> with
// major > 1 or (major == 1 && minor >= 17). Stale catalog versions clamp
// here (the verified-good UA floor parity).
const openCodeMinUAVersion = "1.18.31"

// openCodeUASuffix is the AI-SDK suffix opencode's client appends.
const openCodeUASuffix = "ai-sdk/provider-utils/4.0.23 runtime/bun/1.3.14"

// The free-tier tool-name gate (verified by live bisection 2026-09-19,
// see docs/research/zen-api.md): the request's tools array must include at
// least TWO tool definitions named from the opencode client's tool set —
// otherwise 403 FreeTierError regardless of everything else. Custom tools
// (curate_posting, search_company, ...) ride along freely once the gate is
// satisfied. The gate validates the NAMES, not the schemas (verified live
// with empty properties), so the definitions are decoys: the descriptions
// tell the model they are unavailable, which keeps reasoning models on
// tool-less turns from wandering into a call (executeTool would answer
// harmlessly, but the detour burns a retry).
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

// opencodeUserAgent builds the UA opencode sends.
func opencodeUserAgent(version string) string {
	return "opencode/" + version + " " + openCodeUASuffix
}

var versionShape = regexp.MustCompile(`^(\d+)\.(\d+)(?:\.(\d+))?`)

// validOpencodeVersion clamps a catalog version to one the free tier
// accepts: untouched when major > 1 or (major == 1 && minor >= 17), the
// verified-good floor otherwise (including unparseable input).
func validOpencodeVersion(version string) string {
	m := versionShape.FindStringSubmatch(strings.TrimSpace(version))
	if m == nil {
		return openCodeMinUAVersion
	}
	major, majorErr := strconv.Atoi(m[1])
	minor, minorErr := strconv.Atoi(m[2])
	if majorErr != nil || minorErr != nil {
		return openCodeMinUAVersion
	}
	if major > 1 || (major == 1 && minor >= 17) {
		return m[0]
	}
	return openCodeMinUAVersion
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
		b.WriteString(fmt.Sprintf("%02x", byte(time48>>uint(shift))))
	}
	for i := 0; i < 14; i++ {
		b.WriteByte(opencodeIDAlphabet[random[i]%byte(len(opencodeIDAlphabet))])
	}
	return b.String()
}

// opencodeIDFromSeed derives a stable opencode-shaped id from a seed
// (sha1: 6 time bytes + 14 body bytes) — used for the process-stable
// session id so zen's sticky provider routing survives restarts.
func opencodeIDFromSeed(prefix, seed string) string {
	digest := sha1.Sum([]byte(seed))
	var b strings.Builder
	b.WriteString(prefix)
	b.WriteByte('_')
	b.WriteString(hex.EncodeToString(digest[:6]))
	for _, r := range digest[6:20] {
		b.WriteByte(opencodeIDAlphabet[r%byte(len(opencodeIDAlphabet))])
	}
	return b.String()
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

// ensureFreeTierShape guarantees the agent-turn shape the free tier
// requires: stream stays true and the tools array gains the gate-satisfying
// client tools wrapped for the payload's family when the caller passed
// none. Unknown families default to chat-completions (the family every
// free model uses); google-generative-ai is skipped entirely (verified
// parity — a different tool shape with no free models today, left untouched
// rather than mangled).
func ensureFreeTierShape(payload map[string]any, api string) {
	if api == apiGoogleGenerativeAI {
		return
	}
	if tools, ok := payload["tools"].([]any); ok && len(tools) > 0 {
		return
	}
	gate := []any{wireTool(gateToolDefs[0], api), wireTool(gateToolDefs[1], api)}
	payload["tools"] = gate
	// The decoys exist only to satisfy the gate; on chat-completions (and
	// unknown families, which default to its shape) force tool_choice "none"
	// so the model cannot call a tool it was told is unavailable — verified
	// 200 against the live gateway. (mimo's tool_choice rejection was
	// observed on real-tool curation turns, which never take this path).
	// Responses coerces to "auto" instead (see responsesWire.buildBody);
	// Anthropic has no "none".
	if api != apiOpenAIResponses && api != apiAnthropicMessages {
		if _, set := payload["tool_choice"]; !set {
			payload["tool_choice"] = "none"
		}
	}
}
