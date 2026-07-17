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
- **Relevant scrapers exist** → proceed to Flow below. If every relevant scraper returns 0 results at Step 2, fall back to Exa the same way.

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
populate `jobs add`:

```bash
waypoint jobs add "<company>" "<position>" \
  --url "<url>" \
  --location "<location>" \
  --date "<date>" \
  --salary "<salary>" \
  --notes "<extracted description or key details>"
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
| `scrape run <name> [flags]` | Fetch, stage, print new results (see Step 2 for flags) |
| `scrape staged [--status new\|dismissed] [--json]` | Review staged backlog |
| `scrape dismiss <url>` | Mark a staged result as dismissed |
| `scrape detail <name> <id> [--json]` | Fetch full description + metadata for a staged result (LinkedIn only) |
| `scrape prune [--days 30]` | Remove old staged entries |

## Notes

- `scrape run` writes to staging before printing. If interrupted, results are
  preserved — the next run deduplicates correctly.
- Results already tracked as jobs (via `jobs add --url`) are automatically
  filtered out by `scrape run` — no need to dismiss them.
