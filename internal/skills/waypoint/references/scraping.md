# Scraping job postings

Scrape job portals, review new results in **staging**, then promote picks to tracked jobs or dismiss rejects.

## Entry condition

Before running scrapers, check whether any are relevant:

```bash
waypoint scrape list --json
waypoint profile show --json
```

Match the user's `industry` against each scraper's `categories`.

- **Zero relevant scrapers** → stop. Fall back to Exa — `read` [data/exa-search](references/data/exa-search.md) and search manually.
- **Relevant scrapers exist** → proceed to Flow below. Fall back to Exa if any of these hold: every relevant scraper returns 0 results at Step 2, or the results they return don't match the user's curation brief (wrong level, poor quality, too sparse). Exa fills the gaps the scrapers can't.

**Done when**: entry gate passed (relevant scrapers found) or fallback decision made.

## Flow

### Step 1 — Select scrapers

```bash
waypoint scrape list --json
waypoint profile show --json
```

Match the user's `industry` against each scraper's `categories`. A scraper
with `["biotech", "academic"]` is relevant for `industry: "biotechnology"`.

**Done when**: every relevant scraper identified, every irrelevant one
explicitly excluded with a reason.

### Step 2 — Run each relevant scraper

```bash
waypoint scrape run <name> -q "<query>" -l "<location>" --json
```

Optional flags (supported by portals that offer them; ignored by the rest):

| Flag | What it does | Supported by |
|------|-------------|--------------|
| `-q, --query <text>` | Keyword search | All |
| `-l, --location <text>` | Location to search | LinkedIn (defaults to "India" if omitted), Indeed, Google Jobs |
| `--limit <n>` | Cap results (0 = all) | All |
| `--jobage <days>` | Posted within N days (default: 90; 0 = all) | All |
| `--remote <mode>` | `remote` / `hybrid` / `onsite` | LinkedIn |
| `--page <n>` | Page number, 1-indexed | LinkedIn, Indeed |
| `--today <YYYY-MM-DD>` | Reference date for recency (`meta.today`); filters client-side on listing scrapers, echoed on all | ipu + listing scrapers (client-side); echoed on LinkedIn/Indeed/Google |

**Anchor "today" explicitly.** The CLI can't see your clock context, so pass `--today` (use the real current date, not a placeholder). It's echoed back as **`meta.today`** in `--json` output — the reference date results were judged against. How it applies differs by scraper type:

- **Listing scrapers** (e.g. ipu, ncbs): `--today` filters `--jobage` recency client-side.
- **API scrapers** (LinkedIn, Indeed, Google Jobs): the portal's server-side filter governs what comes back — `--today` doesn't change it, but `meta.today` still tells you the reference date to reason about how fresh a result is when promoting.

Example — listing scraper with an explicit anchor:
```bash
waypoint scrape run ipu -q "engineer" --today <YYYY-MM-DD> --jobage 30 --json
```

Read **`meta.today`** from the response, then drop any result whose `date` is older than the brief's recency window relative to it.

Results are already filtered by query — no need to filter again.

Results are staged automatically — the CLI deduplicates against the staging
file and the jobs table by URL, so only new postings appear.

If `meta.count` is `0`, no new postings since the last run. Skip to the next
scraper — don't present an empty list.

If a scraper errors, skip it and continue with the remaining scrapers.
Mention the failure to the user after presenting results from the ones that
succeeded.

**Done when**: every selected scraper has been run.

### Step 3 — Present new results

Show the user a numbered list: title, company, location, date. Ask which
to track.

If results have `metadata` fields (qualification, salary, vacancy), include
them inline — they help the user decide.

**Done when**: results presented, user has indicated their picks.

### Step 4 — Promote picks

Each promoted result is **raw** — its URL content must be extracted before
adding. The result's URL type decides the extraction method:

| If the URL ends in… | The result is a… | Extract with |
|---------------------|------------------|--------------|
| `.pdf` | **PDF notification** | `read` [data/pdf-extract](references/data/pdf-extract.md) — extract position, deadline, salary, eligibility from the PDF text |
| `linkedin.com/jobs/…` | **LinkedIn posting** | `waypoint scrape detail linkedin <id> --json` — fetches description, seniority, employment type, job function, industries |
| Anything else | **Web page** | `read` [data/job-extract](references/data/job-extract.md) — fetch the page, parse structured fields |

Use the extracted data — not the generic scraper result fields — to
populate `jobs add`. Write it as a single line — bash's `\` line-continuation
is bash-only and breaks under Windows PowerShell. Keep the note **shell-safe**
(short, no `$`/quotes/backticks); a long or shell-unsafe extracted description
goes via `--notes-file` instead (see the shell-safe rule in SKILL.md Notes):

```
waypoint jobs add "<company>" "<position>" --url "<url>" --location "<location>" --date "<date>" --salary "<salary>" --notes "<short extracted description>"
```

Field mapping from result and extraction to `jobs add`:

| Source | `jobs add` flag | Notes |
|--------|-----------------|-------|
| Extracted title | `<position>` (2nd arg) | Override the scraper's generic title |
| `result.company` | `<company>` (1st arg) | |
| `result.url` | `--url` | |
| `result.location` | `--location` | |
| Extracted deadline | `--date` | The real deadline from the PDF/page — not a guess from filename |
| `result.metadata.salary` | `--salary` | If present |
| Extracted description | `--notes` | Real extracted content, summarised if long |

After adding, the job is enriched. Move to the next promoted result.

**Done when**: every promoted result enriched with real extracted data and
added with accurate fields. No result left with a generic title, empty date,
or "Check PDF" note.

### Step 5 — Dismiss rejects

```bash
waypoint scrape dismiss "<url>"
```

Dismissed results don't reappear on the next scrape. If the user is unsure
about a result, skip dismissal — it stays in staging as "new" and won't
reappear until pruned.

**Done when**: every explicit reject dismissed.

## Commands

| Command | What it does |
|---------|-------------|
| `scrape list [--json]` | List registered scrapers with categories |
| `scrape run <name> [flags]` | Fetch, stage, print new results (see Step 2; add `--today <YYYY-MM-DD>` to anchor recency) |
| `scrape staged [--status new\|dismissed] [--json]` | Review staged backlog |
| `scrape dismiss <url>` | Mark a staged result as dismissed |
| `scrape detail <name> <id> [--json]` | Fetch full description + metadata for a staged result (LinkedIn only) |
| `scrape prune [--days 30]` | Remove old staged entries |

## Notes

- `scrape run` writes to staging before printing. If interrupted, results are
  preserved — the next run deduplicates correctly.
- Results already tracked as jobs (via `jobs add --url`) are automatically
  filtered out by `scrape run` — no need to dismiss them.
