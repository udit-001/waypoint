# Research: OpenCode Zen API for the curation seam

*Resolves WP-127. Sources: [Zen docs](https://opencode.ai/docs/zen/), [gateway handler source](https://github.com/sst/opencode/blob/47f33329/packages/console/app/src/routes/zen/util/handler.ts), [issue #42029](https://github.com/anomalyco/opencode/issues/42029), working clients `pi-zen/index.ts` + `news-aggregator/app/jobs/llm_classifier.py`.*

## The one-paragraph answer

Zen is an OpenAI-compatible gateway. Waypoint needs exactly **one HTTP client hitting `/v1/chat/completions`** — every model the curation seam would use (free tier + kimi/glm/deepseek) lives on that endpoint. Auth is `Authorization: Bearer <key>`. Free models are genuinely $0 but **gated by `User-Agent: opencode/<version>`** — omit it and every free-model call 429s regardless of key.

## Facts

| Question | Answer |
|---|---|
| Key acquisition | Sign in at opencode.ai/auth → billing → copy key (`oc_...`). Prepaid credits, pay-per-request |
| Auth header | `Authorization: Bearer <key>` (verified in llm_classifier.py:167) |
| Endpoint | `https://opencode.ai/zen/v1/chat/completions` (OpenAI chat-completions schema). GPT/Grok models use `/v1/responses`, Claude/Qwen use `/v1/messages` — **irrelevant for us**, don't target them |
| Model catalog | `GET /v1/models` — public, no key needed |
| Free models (all $0, all "limited time") | `big-pickle`, `mimo-v2.5-free`, `deepseek-v4-flash-free`, `hy3-free`, `laguna-s-2.1-free`, `nemotron-3-ultra-free`, `nemotron-3.5-lightning-free` |
| Structured output | Tool calling on chat/completions — proven with `mimo-v2.5-free` by news-aggregator at 1,383 labels. **No `tool_choice`** (mimo rejects); `max_tokens >= 1024` (reasoning burns tokens first) |
| Rate limits | Per-key + per-model RPM/TPM/TPS limiters server-side; 429s carry `retryInSec`-style messages. Server itself retries/failovers 3× on 429/provider errors |
| Retry semantics (client) | Proven pattern: 3 retries with backoff + **fallback model** (`mimo-v2.5-free` → `glm-5.1` in production classifier). Treat 401 (auth), 402/credits, 429 distinctly |

## ⚠️ The User-Agent gate (issue #42029)

Free-model capacity is reserved for the official client, enforced by **`User-Agent` alone**:
- `User-Agent: opencode/1.17.11` → 200 OK
- Any other UA → `429 FreeUsageLimitError`, regardless of key or IP
- `x-opencode-*` headers do **not** unlock it; the gateway strips them before forwarding upstream

**Consequence:** the Go client must send `User-Agent: opencode/<version>` (mirror pi-zen's exact string). Anti-abuse gate, may change any time → **paid/BYOK is the long-term stable path; free tier is the on-ramp.**

## What `x-opencode-session` actually does (gateway source)

Not conversation state. `sessionId` feeds: (a) metrics logging, (b) **sticky-provider selection** — same session id → same upstream provider across requests (cache friendliness). Sticky key = session ?? workspace ?? IP. So: waypoint sends one `ses_<32hex>` per daemon process; the "conversation" itself lives client-side in our message array (the wire is stateless per call).

## Design decisions this locks in

1. **`internal/zen` Go package**: single OpenAI-compatible chat-completions client. Config: `{BaseURL, APIKey, Model, UserAgent}`. Default model `mimo-v2.5-free`; fallback `glm-5.1` (~$0.2–0.3/1M — pennies per cycle).
2. **Provider abstraction for free**: any OpenAI-compatible endpoint substitutes by changing BaseURL/Key/Model (OpenRouter, LM Studio, Ollama). UA gate is zen-specific; others ignore it.
3. **Key storage**: waypoint config (`~/.waypoint/`), settable from the Settings UI — **not** env-only, because the daemon (`waypoint start`) must not depend on the launching shell's env. `ZEN_API_KEY` env as override/fallback (mirrors pi-zen resolution order: stored key wins, env fallback).
4. **Headers sent**: `Authorization`, `User-Agent: opencode/<ver>`, `x-opencode-client: cli`, `x-opencode-session: ses_<hex per process>`, `x-opencode-project: sha1(db path)` (stable per install, harmless if unused).
5. **Error handling**: 401 → surface "check key" in UI; 402/credits → surface; 429 → backoff w/ retry (server message carries reset time) then fallback model; 5xx → retry ×3 then cycle-fail (postings stay `new`, next cycle re-curates).

## Open (for WP-131, not this ticket)

- Verdict quality of free models for *curation specifically* — prototype with ~20 real merged postings.
- Batch vs per-posting turns (WP-131 already holds this question).
- `qwen3.6-plus-free` appears in pi-zen but not the docs table — verify at prototype time via `/v1/models`.
