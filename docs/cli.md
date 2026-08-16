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
| `waypoint profile show` | Display your profile (`--json` for machine output — the document `set` accepts) |
| `waypoint profile schema` | Print the profile document schema as an empty template — the writable surface for `set` |
| `waypoint profile set --file <doc.json>` | Update profile fields from a JSON document (patch semantics: only keys present change; `-` reads stdin). Doc keys match `show --json` output; unknown keys are rejected. Structured entries: `experience` `{title, company, start, end, description}`, `education` `{institution, degree, start, end, description}`; dates `YYYY-MM` or `YYYY`, empty `end` = present, `description` optional free text. `salaryFloor` is `[{region, amount}]` |
| `waypoint profile brief` | Show the job-search curation brief (`--json` for machine output — the frontier the agent reads) |

## Scrapers

| Command | Description |
|---------|-------------|
| `waypoint scrape run <name>` | Run a job scraper and stage/print new results. Flags: `--query`, `--location`, `--limit`, `--jobage` (default 90), `--remote`, `--page`, `--today <YYYY-MM-DD>` (reference date for recency) |
| `waypoint scrape list` | List registered scrapers with categories |
| `waypoint scrape staged` | Review staged results. Flags: `--status new|dismissed`, `--limit` |
| `waypoint scrape dismiss <url>` | Mark a staged result as dismissed |
| `waypoint scrape promote [<url>]` | Move staged result(s) into tracked jobs; `--all` promotes every new entry |
| `waypoint scrape detail <name> <id>` | Fetch full posting details (LinkedIn) |
| `waypoint scrape prune` | Remove old staged entries. Flag: `--days` (default 30) |
| `waypoint scrape migrate` | Import legacy scrape-cache.json into the database |

## Boards

Company ATS boards (Greenhouse, Workday, Lever, BambooHR) — one company's careers site per board. Boards live in `boards.toml` inside `data_dir`, so they travel with the database in backups. The flow: find the company's careers URL (any search tool), `add` it (detect + verify + save), then `sweep` to stage postings.

| Command | Description |
|---------|-------------|
| `waypoint boards add <name> --url <url>` | Detect the provider for a careers URL, verify the board is live (first-page fetch), and save it enabled. Flag: `--company` (display name, defaults to `<name>`) |
| `waypoint boards list` | List saved boards |
| `waypoint boards remove <name>` | Remove a board |
| `waypoint boards enable <name>` | Include a board in sweeps |
| `waypoint boards disable <name>` | Skip a board in sweeps |
| `waypoint boards verify [<name>]` | Re-probe one board (or all enabled) for liveness; exits non-zero on failure |
| `waypoint boards sweep` | Fetch every enabled board, deduplicate against staging and tracked jobs, and stage new postings. Flags: `--jobage` (default 90, same as `scrape run`), `--limit` (per board). JSON: per-board `fetched`/`new`/`seen`/`failed` plus the `jobs` array staged this sweep — the completion contract for agents: `new==0` everywhere → done; any `failed` → verify/fix/disable that board |

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

## Resume

| Command | Description |
|---------|-------------|
| `waypoint resume extract <file.pdf>` | Extract redacted resume text for model use. Always JSON. Flag: `--no-redact` (raw, user's eyes only) |
| `waypoint resume doctor` | Report extraction backend availability (pdf_oxide / poppler). Always JSON |

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

- `--json` — Output as JSON (for scripting). `waypoint resume` commands are always JSON — the flag is accepted as a no-op for uniformity.
