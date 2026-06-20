# Job Details Extraction

Parse job info from any source → `jobs add` flags.

## Flow

```
input → extract text → parse fields → jobs add → optionally enrich via exa-search
```

## Input sources

### URL (job posting page)

Exa MCP available:
```
exa_web_fetch_exa { urls: ["<url>"], maxCharacters: 5000 }
```

No exa:
```bash
curl -sL "<url>" | sed 's/<[^>]*>//g' | sed '/^$/d' | head -300 > /tmp/job-page.txt
```

### PDF (job posting / dossier)

`read` [pdf-extract](pdf-extract.md) for extraction steps, then parse the text.

### Plain text

User pastes job description → parse directly.

### Just a company name

"I'm applying to Google" with no details → `read` [exa-search](exa-search.md) for company info + open roles.

## Field mapping

| Field | Flag | Look for |
|-------|------|----------|
| Company | arg 1 | company name, "at X", "X is hiring" |
| Position | arg 2 | job title, role |
| Status | `--status` | default "Not Applied" |
| Category | `--category` | match to existing: `categories list` |
| Salary | `--salary` | "$100k", "₹15 LPA", "€60,000" |
| Location | `--location` | city, "Remote", "Hybrid" |
| Contact | `--contact` | hiring manager, recruiter email |
| URL | `--url` | source URL |
| Deadline | `--date` | "apply by", "closes on" |
| Applied | `--applied-date` | if user already applied |
| Notes | `--notes` | requirements, tech stack, extras |

Ambiguous? Ask user. Don't guess.

## Examples

URL:
```
exa_web_fetch_exa { urls: ["https://careers.google.com/jobs/123"] }
→ parse → waypoint jobs add "Google" "Senior SWE" --location "Mountain View" --url "..." --category Tech
→ exa people search → jobs update <id> --contact "..."
```

PDF:
```
pdftotext dossier.pdf - | head -200
→ parse → waypoint jobs add "GBRC" "Research Scientist" --location "Gujarat"
```

Company name only:
```
exa_web_search_advanced_exa { query: "category:company Stripe", numResults: 3 }
exa_web_search_exa { query: "Stripe careers open roles engineering", numResults: 5 }
→ ask which role → jobs add "Stripe" "<role>" --category Tech
→ exa people search → jobs update <id> --contact "..."
```

## After adding

- "Research the company/people?" → exa-search
- "Draft a cover letter?" → cover-letter
- "More jobs to add?"
