# Exa MCP Setup

Exa enables company research, hiring signals, and people search. If not connected, offer to add it.

## Config

Add to agent's MCP config:

- **pi**: `~/.pi/agent/mcp.json`
- **Cursor**: `.cursor/mcp.json`
- **Claude Code**: `claude mcp add --transport http exa https://mcp.exa.ai/mcp`

```json
"exa": {
  "type": "streamable-http",
  "url": "https://mcp.exa.ai/mcp?tools=web_search_exa,web_fetch_exa,web_search_advanced_exa"
}
```

No API key needed (free tier). Restart agent after adding.

## Verify

After restart, check tools: `exa_web_search_exa`, `exa_web_fetch_exa`, `exa_web_search_advanced_exa` should appear.
