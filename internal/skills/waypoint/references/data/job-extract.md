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

## How to apply

Detect the **method** the posting asks for, then route by it. The method is how the applicant submits: email, form, portal, site, or other. Each method has a destination.

| Method | Detect | Route |
|--------|--------|-------|
| Email | "send your resume to", "email …@", an address near "apply" | `--contact` if it's a person; instructions → notes |
| Form | "fill out this form", `google.com/forms`, `typeform.com` | apply URL → notes |
| Portal / site | "apply at", `careers.`, an ATS domain (`greenhouse.io`, `lever.co`, `workday`) | apply URL → notes |
| Other | "in person", "referral", "by mail" | method + details → notes |

`url` is the **posting** (where you read the job). The **apply** link, if separate, goes in notes — never overwrite `url` with it.

Write apply details as a `## How to apply` section in notes (it renders as markdown — see `SKILL.md`). Method first, then destination and instructions:

```bash
waypoint jobs update 5 --contact "mike.r@stripe.com" --notes "## How to apply
Email **mike.r@stripe.com** — subject line 'SWE Application — [Name]'.

> Attach: resume, cover letter. Rolling deadline."
```

Form or portal (no contact person, apply link separate from posting):

```bash
waypoint jobs update 8 --notes "## How to apply
Submit via [Greenhouse form](https://boards.greenhouse.io/figma/jobs/123).

> Portfolio PDF required."
```

**Done when**: every detected apply piece routed — email to `contact` (if a person) else notes, apply URL and instructions to a `## How to apply` notes section; `url` unchanged as the posting. If no method is stated, skip.

## After adding

- "Research company/people?" → [exa-search](exa-search.md)
- "Draft cover letter?" → [cover-letter](cover-letter.md)
- "More jobs to add?"
