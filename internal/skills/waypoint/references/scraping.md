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
waypoint scrape run <name> -q "<query>" --json
```

Results are staged automatically — the CLI deduplicates against the staging
file and the jobs table by URL, so only new postings appear.

If `meta.count` is `0`, no new postings since the last run. Skip to the next
scraper — don't present an empty list.

If a scraper errors, skip it and continue with the remaining scrapers.
Mention the failure to the user after presenting results from the ones that
succeeded.

**Done when**: every selected scraper has been run.

### Step 3 — Present new results

Show the user a numbered list: title, company, location, deadline. Ask which
to track.

If results have `metadata` fields (qualification, salary, vacancy), include
them inline — they help the user decide.

**Done when**: results presented, user has indicated their picks.

### Step 4 — Promote picks

```bash
waypoint jobs add "<company>" "<position>" \
  --url "<url>" \
  --location "<location>" \
  --date "<deadline>" \
  --salary "<salary>" \
  --notes "<description or key details>"
```

Field mapping from result to `jobs add`:

| Result field | Flag | Notes |
|-------------|------|-------|
| `title` | `<position>` (2nd arg) | |
| `company` | `<company>` (1st arg) | |
| `url` | `--url` | |
| `location` | `--location` | |
| `date` | `--date` | Deadline |
| `metadata.salary` | `--salary` | If present |
| `description` | `--notes` | Summarize if long |

After promoting, the job is in the pipeline — offer to enrich it (`jobs get`)
or generate materials (cover letter, email).

**Done when**: every user pick added as a job, ID confirmed.

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
| `scrape run <name> [-q "..."] [--json]` | Fetch, stage, print new results |
| `scrape staged [--status new\|dismissed] [--json]` | Review staged backlog |
| `scrape dismiss <url>` | Mark a staged result as dismissed |
| `scrape prune [--days 30]` | Remove old staged entries |

## Notes

- `scrape run` writes to staging before printing. If interrupted, results are
  preserved — the next run deduplicates correctly.
- Results already tracked as jobs (via `jobs add --url`) are automatically
  filtered out by `scrape run` — no need to dismiss them.
- Dismissed entries stay in staging (they don't reappear in `scrape run`).
  `prune` is the only command that removes entries, and only old ones.
