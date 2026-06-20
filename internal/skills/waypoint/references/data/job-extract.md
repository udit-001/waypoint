# Job Details Extraction

Parse job info from any source → `jobs add` flags.

```
input → extract text → parse fields → jobs add → optionally enrich via exa-search
```

## Input sources

**URL** — exa available:
```
exa_web_fetch_exa { urls: ["<url>"], maxCharacters: 5000 }
```
No exa:
```bash
curl -sL "<url>" | sed 's/<[^>]*>//g' | sed '/^$/d' | head -300 > /tmp/job-page.txt
```

**PDF** → `read` [pdf-extract](pdf-extract.md), then parse extracted text.

**Plain text** — user pastes job description → parse directly.

**Company name only** — "I'm applying to Google" → `read` [exa-search](exa-search.md) for company info + open roles.

## Field mapping

| Field | Flag | Look for |
|-------|------|----------|
| Company | arg 1 | company name, "at X", "X is hiring" |
| Position | arg 2 | job title, role |
| Status | `--status` | default "Not Applied" |
| Category | `--category` | match to existing: `categories list` |
| Salary | `--salary` | "$100k", "₹15 LPA", "€60k" |
| Location | `--location` | city, "Remote", "Hybrid" |
| Contact | `--contact` | hiring manager, recruiter email |
| URL | `--url` | source URL |
| Deadline | `--date` | "apply by", "closes on" |
| Applied | `--applied-date` | if already applied |
| Notes | `--notes` | requirements, tech stack, extras |

Ambiguous → ask user. Don't guess.

## After adding

- "Research company/people?" → [exa-search](exa-search.md)
- "Draft cover letter?" → [cover-letter](cover-letter.md)
- "More jobs to add?"
