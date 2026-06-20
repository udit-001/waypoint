# Job Details Extraction

Parse job info from any source into structured `jobs add` / `jobs update` fields.

## Flow

```
User provides input → extract fields → jobs add with flags → optionally enrich via exa-search
```

## Input sources

### 1. URL (job posting page)

If `exa` MCP available:
```
exa_web_fetch_exa { urls: ["<url>"], maxCharacters: 5000 }
```

If no exa, use `curl`:
```bash
curl -sL "<url>" | sed 's/<[^>]*>//g' | sed '/^$/d' | head -300 > /tmp/job-page.txt
```

### 2. PDF (job posting / dossier)

`read` [pdf-extract](pdf-extract.md) for extraction steps. Then parse the extracted text.

### 3. Plain text

User pastes job description directly. No extraction needed — just parse.

### 4. Just a company name

User says "I'm applying to Google" with no details. `read` [exa-search](exa-search.md) to find company info and open roles.

## Parsing

From extracted text, identify these fields:

| Field | `--flag` | Look for |
|-------|---------|----------|
| Company | `--` arg 1 | company name, employer, "at X", "X is hiring" |
| Position | `--` arg 2 | job title, role, "Senior Engineer", "Product Manager" |
| Status | `--status` | default "Not Applied" unless user says otherwise |
| Category | `--category` | match to existing: `categories list` |
| Salary | `--salary` | "$100k", "₹15 LPA", "€60,000", compensation range |
| Location | `--location` | city, "Remote", "Hybrid", "NYC" |
| Contact | `--contact` | hiring manager name, recruiter email |
| URL | `--url` | the source URL |
| Deadline | `--date` | "apply by", "closes on", deadline date |
| Applied | `--applied-date` | if user says they already applied |
| Notes | `--notes` | requirements, tech stack, anything extra |

If a field is ambiguous, ask the user. Don't guess.

## Examples

User pastes a URL:
```
1. exa_web_fetch_exa { urls: ["https://careers.google.com/jobs/123"] }
2. Parse: company=Google, position="Senior SWE", location="Mountain View"...
3. waypoint jobs add "Google" "Senior SWE" --location "Mountain View" --url "https://careers.google.com/jobs/123" --category Tech --notes "Requires 5+ years Go experience"
4. Optionally: exa company/people research → jobs update <id> --contact "..."
```

User uploads a PDF dossier:
```
1. pdftotext dossier.pdf - | head -200
2. Parse extracted text into fields
3. waypoint jobs add "GBRC" "Research Scientist" --location "Gujarat" --notes "..."
```

User says "I'm applying to Stripe":
```
1. exa_web_search_advanced_exa { query: "category:company Stripe", numResults: 3 }
2. exa_web_search_exa { query: "Stripe careers open roles engineering", numResults: 5 }
3. Ask: "Which role at Stripe?"
4. waypoint jobs add "Stripe" "<role>" --category Tech
5. exa people search → jobs update <id> --contact "..."
```

## After adding

Once job is created, suggest:
- "Want me to research the company/people?" → exa-search
- "Should I draft a cover letter?" → cover-letter reference
- "Any other jobs to add?"
