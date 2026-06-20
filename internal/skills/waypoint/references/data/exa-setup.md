# Exa MCP Setup

Exa enables company research, hiring signals, and people search. If not connected, offer to add it.

No API key needed (free tier). Restart agent after adding.

## pi

`~/.pi/agent/mcp.json`:
```json
"exa": {
  "type": "streamable-http",
  "url": "https://mcp.exa.ai/mcp?tools=web_search_exa,web_fetch_exa,web_search_advanced_exa"
}
```

## Cursor

`.cursor/mcp.json`:
```json
"exa": {
  "url": "https://mcp.exa.ai/mcp?tools=web_search_exa,web_fetch_exa,web_search_advanced_exa"
}
```

## Claude Code

```bash
claude mcp add --transport http exa https://mcp.exa.ai/mcp
```

## Codex

```bash
codex mcp add exa --url "https://mcp.exa.ai/mcp?tools=web_search_exa,web_fetch_exa,web_search_advanced_exa"
```

Or in `~/.codex/config.toml`:
```toml
[mcp_servers.exa]
url = "https://mcp.exa.ai/mcp?tools=web_search_exa,web_fetch_exa,web_search_advanced_exa"
enabled = true
```

## OpenCode

CLI:
```bash
opencode mcp add exa --url "https://mcp.exa.ai/mcp?tools=web_search_exa,web_fetch_exa,web_search_advanced_exa"
```

Or in `opencode.json` (global: `~/.config/opencode/opencode.json`, project: `./opencode.json`):
```json
"mcp": {
  "exa": {
    "type": "remote",
    "url": "https://mcp.exa.ai/mcp?tools=web_search_exa,web_fetch_exa,web_search_advanced_exa",
    "enabled": true
  }
}
```

## Verify

After restart, check for tools: `exa_web_search_exa`, `exa_web_fetch_exa`, `exa_web_search_advanced_exa`.
