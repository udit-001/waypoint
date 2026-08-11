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
No exa — Unix pipe targets `<temp-dir>` (`/tmp`; use `$TEMP` on Windows). If the shell lacks these tools, fetch with your file tool instead:
```
curl -sL "<url>" | sed 's/<[^>]*>//g' | sed '/^$/d' | head -300 > <temp-dir>/job-page.txt
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
| Salary | `--salary` | see [salary extraction](#salary-extraction) below |
| Location | `--location` | city, "Remote", "Hybrid" |
| Contact | `--contact` | hiring manager, recruiter email |
| URL | `--url` | source URL |
| Deadline | `--date` | "apply by", "closes on" |
| Applied | `--applied-date` | if already applied |
| Notes | `--notes` | requirements, tech stack, extras |

Ambiguous → ask user. Don't guess.

## Salary extraction

Parse the salary string from the job posting into a compact value for
`--salary`. The salary chart renders any string containing a parseable
number — keep it raw but clean, not normalised.

### Extraction order

Each step preserves enough context for the chart to detect whether the
salary is monthly or annual and normalise everything to monthly.

1. **Keep informative tokens** — retain the currency prefix (`Rs.`, `$`,
   `€`, `₹`) and Indian comma formatting (e.g. `37,000`). Keep annual
   markers (`$`, `LPA`, `PA`, `/yr`, `/year`, `lakh`). Only strip `/month`,
   `/mo` — monthly is the default unit, so they're redundant.
2. **Chop suffixes** — drop everything after `+` (removes `+ HRA`,
   `+ 27% HRA`, etc.) and everything in `()`, `[]`, `{}` that looks like a
   qualification note (`GATE/NET`, `NET/GATE`).
3. **Coalesce `OR` options** — if the string contains ` OR ` (case-insensitive),
   split and keep only the option with the highest numeric value. That's
   the one the applicant would target.
4. **Expand shorthand, standardise qualifier** — convert `k`/`L`/`lakh` to
   full integers. If the original was annual (detected by `$`, `LPA`, `PA`,
   `/yr`, `/year`), append `/yr` so the chart divides by 12:
   `$100k` → `$100000/yr`, `₹15 LPA` → `₹1500000/yr`, `€60k` → `€60000/yr`.
   If the original had no annual marker, output the clean integer with its
   currency prefix — the chart treats `Rs.`/`₹` as monthly by default.

### Examples

| Raw | Extracted | Notes |
|-----|-----------|-------|
| `$100k` | `$100000/yr` | Annual qualifier kept → chart divides by 12 |
| `€60k` | `€60000/yr` | Annual qualifier kept |
| `₹15 LPA` | `₹1500000/yr` | Standardised to /yr → chart divides by 12 |
| `Rs. 37,000 + HRA` | `Rs. 37000` | Monthly default, no qualifier needed |
| `Rs. 28,000 + HRA` | `Rs. 28000` | Same pattern |
| `Rs. 37,000 + 27% HRA` | `Rs. 37000` | +27% HRA chopped, prefix kept |
| `Rs. 37,000 + HRA (GATE/NET) OR Rs. 31,000 + HRA` | `Rs. 37000` | Coalesced highest, prefix kept |
| `Rs. 31,000 + 20% HRA (NET/GATE) OR Rs. 25,000 + HRA` | `Rs. 31000` | Same |
| `70k-100k` | `70000-100000/yr` | No currency prefix, but k-format → annual |
| `50000 - 80000` | `50000-80000` | No prefix, no qualifier → annual (default) |

### Completion criterion

Salary extraction is done when every salary-like number in the posting
has been parsed, and the extracted string fits one of the patterns in
the example table above. Check three properties before declaring done:

- **Prefix preserved** — `Rs.`, `$`, `€`, `₹` are still in the output.
  The chart uses them to detect monthly vs annual.
- **Annual qualifier standardised** — if the original was annual (`$`, `LPA`,
  `PA`, `/yr`, `/year`), the output has `/yr` appended. The chart divides
  annual values by 12 automatically. If the original had no annual marker,
  none is needed — the chart treats `Rs.`/`₹` as monthly.
- **Number expanded** — `k`/`L`/`lakh` expanded to full integer.

If a format doesn't match any example, keep the most salary-like
number (highest if multiple) as a full integer with its currency
prefix. If it looks annual, add `/yr`. Don't ask the user — extract
what's there and move on.

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
