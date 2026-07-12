# Scraping job postings

Scrape job portals for new postings, stage them for review, and promote
relevant ones into the tracked jobs table. Staging is the intermediate state
between discovery and commitment — results land in staging, get reviewed,
then get promoted or dismissed.

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

If `meta.truncated` is `true`, results are partial (rate-limited). Present
what was fetched and advise retrying later.

**Done when**: every selected scraper has returned results or reported an error.

### Step 3 — Present new results

Show the user a numbered list: title, company, location, date. Ask which to
track.

**Done when**: results presented, user has indicated their picks.

### Step 4 — Promote picks via jobs add

```bash
waypoint jobs add "<company>" "<position>" --url "<url>" --location "<location>"
```

Map result fields to job fields: `Title` → `position`, `Company` → `company`,
`URL` → `url`, `Location` → `location`. Confirm each with its job ID.

**Done when**: every user pick added as a job.

### Step 5 — Dismiss rejects

```bash
waypoint scrape dismiss "<url>"
```

Dismissed results don't reappear on the next scrape.

**Done when**: every reject dismissed.

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
