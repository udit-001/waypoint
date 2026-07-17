# CLI Reference

All commands support `--json`.

## Jobs

| Command | Description |
|---------|-------------|
| `waypoint jobs add <company> <position>` | Add a job. Flags: `--status`, `--category`, `--salary`, `--location`, `--contact`, `--url`, `--notes`, `--date`, `--applied-date`, `--reminder` |
| `waypoint jobs list` | List jobs. Flags: `--status`, `--category`, `--search`, `--limit`, `--all` |
| `waypoint jobs get <id>` | Show job details. Flag: `--history` |
| `waypoint jobs update <id>` | Update job fields. Same flags as `add` |
| `waypoint jobs delete <id>` | Delete a job. Flag: `--force` |
| `waypoint jobs stats` | Show aggregate statistics |

## Categories

Alias: `waypoint cat`

| Command | Description |
|---------|-------------|
| `waypoint categories list` | List all categories with job counts |
| `waypoint categories add <name>` | Add a new category |
| `waypoint categories rename <id> <name>` | Rename a category by ID |
| `waypoint categories delete <id>` | Delete a category by ID (jobs move to General) |

## Profile

| Command | Description |
|---------|-------------|
| `waypoint profile show` | Display your profile (`--json` for machine output) |
| `waypoint profile set` | Update profile fields. Flags: `--name`, `--email`, `--phone`, `--title`, `--skills` (JSON array), `--experience`, `--education`, `--industry`, `--greeting-style`, `--sign-off` |

## Artifacts

Alias: `waypoint artifact`

| Command | Description |
|---------|-------------|
| `waypoint artifacts add` | Add an artifact. Flags: `--skill`, `--title`, `--title-file`, `-f`/`--variant-file`, `--variant-content`, `--variant-label`, `--variants`, `--variants-file`, `--options`, `--options-file`, `--job` |
| `waypoint artifacts list` | List generated content. Flags: `--skill`, `--job`, `--all` |
| `waypoint artifacts get <id>` | Show artifact with all variants |
| `waypoint artifacts delete <id>` | Delete an artifact. Flag: `--force` |
| `waypoint artifacts archive <id>` | Soft-delete (hide from default list) |

The `-f`/`--variant-file` flag reads content from a file. Ideal for multiline text and AI agent workflows:

```bash
waypoint artifacts add --skill cover-letter --title "Cover for Google" -f /tmp/cover.txt --job 3
waypoint artifacts add --skill email-generator --title "Follow-up" --variants-file /tmp/variants.json --job 3
```

## System

| Command | Description |
|---------|-------------|
| `waypoint init` | Initialize a new SQLite database. Flag: `--force` |
| `waypoint start` | Launch the web UI server. Flag: `--port` (default 8080) |
| `waypoint stop` | Stop the background web UI server |
| `waypoint skills install` | Install agent skill for AI coding assistants. Flag: `--agent` |
| `waypoint upgrade` | Self-update to the latest release |

## Common Options

Every command accepts:

- `--json` — Output as JSON (for scripting)
