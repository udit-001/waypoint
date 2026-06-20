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

## Examples

URL:
```
exa_web_fetch_exa { urls: ["https://careers.google.com/jobs/123"] }
→ parse → jobs add "Google" "Senior SWE" --location "Mountain View" --url "..." --category Tech
→ exa people/news → jobs update <id> --contact "..." / --notes "..."
```

PDF:
```
pdftotext dossier.pdf - | head -200
→ parse → jobs add "GBRC" "Research Scientist" --location "Gujarat"
```

Company name:
```
exa search for company + open roles → ask which role → jobs add "Stripe" "<role>"
→ exa people/news → jobs update <id> --contact / --notes
```

## After adding

- "Research company/people?" → exa-search
- "Draft cover letter?" → cover-letter
- "More jobs to add?"
