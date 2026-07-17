# Command Reference

All commands: `--json`.

## Jobs

| Cmd | Flags |
|-----|-------|
| `jobs add <co> <pos>` | `--status` `--category` `--salary` `--location` `--contact` `--url` `--notes` `--date` `--applied-date` `--reminder` |
| `jobs list` | `--status` `--category` `--search` `--limit` `--all` |
| `jobs get <id>` | `--history` |
| `jobs update <id>` | same flags as `add` |
| `jobs delete <id>` | `--force` |
| `jobs stats` | |

## Artifacts

| Cmd | Flags |
|-----|-------|
| `artifacts add` | `--skill` `--title` `--title-file` `-f` `--variant-content` `--variant-file` `--variant-label` `--variants` `--variants-file` `--options` `--options-file` `--job` |
| `artifacts list` | `--skill` `--job` `--all` |
| `artifacts get <id>` | |
| `artifacts delete <id>` | `--force` |
| `artifacts archive <id>` | |

## Other

| Cmd | Flags |
|-----|-------|
| `categories list\|add\|rename\|delete` | |
| `profile show\|set` | `--name` `--email` `--phone` `--title` `--skills` `--experience` `--education` `--industry` `--greeting-style` `--sign-off` |
| `start` | `--port` (8080) |
| `init` | `--force` |
