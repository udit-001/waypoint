# Company ATS boards

A **board** is one company's careers site behind one vendor (Greenhouse, Workday, Lever, BambooHR). Boards answer a different question than scraping: not "what's new across portals?" but "what is *this company* hiring for right now?"

Use boards when the user names target companies — "check Stripe and Khan Academy", "add companies from my list" — or when a tracked job's company is worth watching for more openings.

## Entry condition

```bash
waypoint boards list --json
```

- **User names companies not yet boarded** → Step 1 (add), then Step 2 (sweep).
- **All named companies already boarded** → Step 2 (sweep) directly.
- **User asks what boards exist / to prune them** → `boards list` / `boards remove <name>` / `boards disable <name>`.

**Done when**: every named company is either boarded or the user said skip.

## Step 1 — Add a board

Find the company's careers URL, then add it:

```bash
waypoint boards add <name> --company "<display name>" --url "<careers-url>" --json
```

`add` is the **verify gate**: it detects the provider, fetches the board's first page, and saves the board only when a real job list answers. A typo'd or dead URL fails here — never save an unverified board.

The URL is the ATS board URL, not the marketing careers page. Find it in this order:

1. `read` [data/exa-search](data/exa-search.md) — search `"<company> careers site:myworkdayjobs.com OR site:job-boards.greenhouse.io OR site:jobs.lever.co OR site:bamboohr.com/careers"`.
2. Follow the company's own careers link — the ATS URL is where it lands.
3. Give the URL to `boards add`; the provider detection accepts any of the four vendor URL shapes.

Workday URLs may omit the instance (`https://tenant.myworkdayjobs.com/Site`) — the sweep probes instances automatically.

**Done when**: `meta.verified` is true for every named company, or the failure was reported to the user (wrong URL, unsupported vendor). `provider: null` means no vendor matched — re-find the URL, never guess slugs.

## Step 2 — Sweep

```bash
waypoint boards sweep --json
```

Sweep every enabled board: fetch, filter by `--jobage` (default 90 days, same as `scrape run`), deduplicate against staging and tracked jobs, stage the rest. Each board's entry in `results` carries:

- `jobs` — **the postings this sweep staged. Present these to the user**, numbered, exactly like scrape results (title, company, location, date).
- `failed` — the board errored. Report it; suggest `boards verify <name>`, a URL fix, or `boards disable <name>`.

All four vendors expose posting dates in the list response, so `--jobage` filters every board without extra requests. The list payload is lean (title, location, date); the full description and extra metadata (department, employment type, experience, compensation) live behind `boards detail` — fetch them on demand only for postings you're seriously considering.

**Done when**: every enabled board reports `new: 0`, or the user has reviewed everything the sweep staged. Any `failed: true` board is reported with a next step.

## Step 2½ — Enrich the few you're seriously considering

The sweep list is lean. Before you write a cover letter or judge fit, fetch the full posting:
```bash
waypoint boards detail <board> <id> --json
```
Returns the full description (HTML→markdown), the absolute `date`, and any metadata each vendor exposes (department, employment type, experience for BambooHR; department for Greenhouse; time type, remote type, reqId, country for Workday; Lever already ships the full body in the list). `detail` merges into the staged entry via `EnrichStaging` — URL-indexed, so it persists for the promote step.

Don't `detail` every staged posting — only the ones the user is seriously considering. The sweep already gave you enough to triage; `detail` is the deep-read step.

**Done when**: every posting you intend to promote has a full description.

## Step 3 — Promote picks

Promotion is **not board-specific** — staged postings from a sweep flow through the same staging review as scraped ones. `read` [scraping](scraping.md) Step 3 (present) and Step 4 (extract + `jobs add`) and follow them verbatim; the extraction table there covers PDF, LinkedIn, and web-page URLs, which is all a board result can be.

Rejects: `waypoint scrape dismiss "<url>"` — same rule as scraping: unsure means skip, it stays staged.

**Done when**: same as scraping Steps 3–4 — every promoted result enriched with real extracted data, every explicit reject dismissed.

## Commands

| Command | What it does |
|---------|--------------|
| `boards add <name> --url <url> [--company]` | Detect + verify + save a board (the verify gate) |
| `boards list [--json]` | Saved boards with provider + enabled state |
| `boards remove <name>` | Delete a board |
| `boards enable/disable <name>` | Include/skip in sweeps |
| `boards verify [<name>] [--json]` | Re-probe one or all boards; non-zero exit on failure |
| `boards sweep [--jobage N] [--limit N] [--json]` | Fetch all enabled boards, stage new postings |
| `boards detail <board> <id> [--json]` | Fetch full description + metadata for one posting; enriches staged entry |

## Notes

- Boards live in `boards.toml` inside data_dir — they travel with the DB, and only the CLI writes them.
- A first sweep of a large board stages its whole first window (subject to `--jobage`); later sweeps stage only what's new.
- Workday instance auto-probe: if a Workday board starts failing after a tenant migration, re-verify it; probing usually heals the instance drift.
