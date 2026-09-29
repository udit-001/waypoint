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
| `waypoint scrape staged` | Review postings in the ledger. Flag: `--status new|shortlisted|dismissed|promoted` |
| `waypoint scrape dismiss <url>` | Mark a posting as dismissed |
| `waypoint scrape promote [<url>]` | Promote posting(s) into tracked jobs; `--all` promotes every new entry |
| `waypoint scrape detail <name> <id>` | Fetch full posting details (LinkedIn) |
| `waypoint scrape prune` | Remove old postings. Flag: `--days` (default 30) |
| `waypoint scrape migrate` | Import legacy scrape-cache.json into the database |

## Boards

Company ATS boards (Greenhouse, Workday, Lever, BambooHR, Eightfold) — one company's careers site per board. Boards live in `boards.toml` inside `data_dir`, so they travel with the database in backups. The flow: find the company's careers URL (any search tool), `add` it (detect + verify + save), then `sweep` to stage postings.

| Command | Description |
|---------|-------------|
| `waypoint boards add <name> --url <url>` | Detect the provider for a careers URL, verify the board is live (first-page fetch), and save it enabled. Flag: `--company` (display name, defaults to `<name>`) |
| `waypoint boards list` | List saved boards |
| `waypoint boards remove <name>` | Remove a board |
| `waypoint boards enable <name>` | Include a board in sweeps |
| `waypoint boards disable <name>` | Skip a board in sweeps |
| `waypoint boards verify [<name>]` | Re-probe one board (or all enabled) for liveness; exits non-zero on failure |
| `waypoint boards sweep` | Fetch every enabled board, deduplicate against the postings ledger and tracked jobs, and add new postings. Flags: `--jobage` (default 90, same as `scrape run`), `--limit` (per board). JSON: per-board `fetched`/`new`/`seen`/`failed` plus the `jobs` array added this sweep — the completion contract for agents: `new==0` everywhere → done; any `failed` → verify/fix/disable that board |

## Discover

Maps the company universe onto watchable boards automatically. With `zen_api_key` + `exa_api_key` set (Settings), the brief expands into facet labels via Zen (cached by brief hash — unchanged briefs skip the LLM call), each facet enumerates companies via Exa `category:company` search (≤60 calls/run), deduped by domain with all source facets listed. Without keys it falls back to the built-in starter facets. Each candidate company's careers pages are probed, ATS board links extracted (Greenhouse, Lever, Ashby, Workday with site slug, Eightfold), and survivors verified live through the same detection `boards add` uses. Already-watched boards are filtered out — a company you track never surfaces again. Candidates persist in the database across runs; re-runs never duplicate rows or reset review decisions.

| Command | Description |
|---------|-------------|
| `waypoint discover run` | Probe the facet list and persist discovered companies as candidates (`suggested`). Prints ID / Company / Facet / Boards / Status; `--json` candidates carry `id` + full board URLs. Already-decided companies (added/dismissed) are skipped on later runs |
| `waypoint discover add <id>` | Promote a candidate: verify gate (same as `boards add`), then into boards.toml enabled; candidate becomes `added`. Already-watched board → clear no-op. `--json` emits `{meta:{id, name, updated, status, detail}}` |
| `waypoint discover dismiss <id>` | Tombstone a candidate (`dismissed`) — discovery never suggests it again. `--json` same shape as add |

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
| `waypoint start` | Launch the web UI server. Flags: `--port` (default 8080), `--foreground`, `--no-open` |
| `waypoint stop` | Stop the server (or the supervisor, when one is running). Refuses while a service manager owns the server |
| `waypoint service install` | Register Waypoint to start automatically and start it now |
| `waypoint service uninstall` | Stop it and unregister (alias: `remove`) |
| `waypoint service start` \| `stop` \| `restart` | Control the service without unregistering it |
| `waypoint service status` | `running` \| `stopped` \| `not found` |
| `waypoint skills install` | Install agent skill for AI coding assistants. Flag: `--agent` |
| `waypoint upgrade` | Self-update to the latest release. Flags: `--force`, `--no-skills` |

### Background service

`waypoint service` uses whatever the platform provides, and never needs
administrator rights:

| Platform | Mechanism | Restarts on crash |
|----------|-----------|-------------------|
| Linux | systemd user unit | yes (`Restart=always`) |
| macOS | launchd LaunchAgent | yes |
| Windows | per-user `HKCU\...\CurrentVersion\Run` entry plus Waypoint's own supervisor (`service run` → `service supervise`) | yes (bounded: 5 starts/minute, then it gives up and says so in the log) |

On Windows a real service was rejected deliberately: registering one needs
admin, and a service runs in session 0, where it cannot open the browser,
cannot post desktop notifications, and resolves `%APPDATA%` to a different
profile — so it would read the wrong database.

Use `waypoint service stop` rather than `waypoint stop` for a running service.
The supervisor's own narration (and the server's stdout/stderr) is appended to
`service.log` in the config directory:

```
Linux/macOS  ~/.config/waypoint/service.log
```

## Common Options

Every command accepts:

- `--json` — Output as JSON (for scripting). `waypoint resume` commands are always JSON — the flag is accepted as a no-op for uniformity.
