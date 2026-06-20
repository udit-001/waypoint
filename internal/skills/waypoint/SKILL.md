---
name: waypoint
description: Job application tracker CLI
---

`waypoint` CLI. Local SQLite.

## First-run

Run at start of any waypoint conversation:
```bash
waypoint jobs stats --json && waypoint profile show --json
```

- `total: 0` + empty `name` → fresh install. Ask conversational questions, run commands yourself:
  1. "What's your name and what roles are you targeting?" → `profile set --name "..." --title "..." --skills '["..."]'`
  2. "Any jobs you're already tracking?" → `jobs add "..." "..." --status "..."` for each
  3. "Want to see the dashboard?" → `start`
- `total: 0` + has name → no jobs yet, ask if they want to add one
- Profile incomplete but jobs exist → ask just the missing fields

## Before generating content

### 1. Resolve the job

No job ID given? Search:
```bash
waypoint jobs list --search "<company or role>" --json
```
Found → use ID. Multiple → ask user to pick. None → ask for details, `jobs add`.

### 2. Profile must be complete

`name`, `title`, `skills` must be non-empty. If missing → ask user to fill before generating. Content quality depends on profile data.

```bash
waypoint profile set --name "Jane Doe" --title "Senior Engineer" --skills '["Go","React","AWS"]'
```

Job resolved + profile complete → `read` the skill reference and generate.

### 3. After saving

Suggest a natural next step:
- Cover letter → "Follow-up email too?"
- Interview prep → "Career summary as well?"
- First artifact → "`waypoint start` to see it in the web UI"

# External Data Sources

## Exa search (if MCP connected)

`read` [exa-search](references/exa-search.md) for company/people research patterns. Use findings to personalize content, save contacts via `jobs update --contact` / `--notes`.

## PDFs

If `pdftotext` (poppler) is available, extract text from PDFs for job details:

```bash
pdftotext <file.pdf> - | head -200
```

Useful for job postings, company dossiers, research papers saved as PDFs. Pipe output to `--notes` or `jobs add`.

## Commands

| Cmd | Flags |
|-----|-------|
| `jobs add <co> <pos>` | `--status` `--category` `--salary` `--location` `--contact` `--url` `--notes` `--date` `--applied-date` `--reminder` |
| `jobs list` | `--status` `--category` `--search` `--limit` `--all` |
| `jobs get <id>` | `--history` |
| `jobs update <id>` | same as `add` |
| `jobs delete <id>` | `--force` |
| `jobs stats` | |
| `artifacts add` | `--skill` `--title` `--title-file` `-f` `--variant-content` `--variant-file` `--variant-label` `--variants` `--variants-file` `--options` `--options-file` `--job` |
| `artifacts list` | `--skill` `--job` `--all` |
| `artifacts get <id>` | |
| `artifacts delete <id>` | `--force` |
| `artifacts archive <id>` | |
| `categories list\|add\|rename\|delete` | |
| `profile show\|set` | `--name` `--email` `--phone` `--title` `--skills` `--experience` `--education` `--industry` `--greeting-style` `--sign-off` |
| `start` | `--port` (8080) |
| `init` | `--force` |

All: `--db <path>`, `--json`.

## Skill references

| Ref | Output |
|-----|--------|
| [email-generator](references/email-generator.md) | 4 email types × 4 tones |
| [cover-letter](references/cover-letter.md) | cover letter in 4 styles |
| [resume-optimizer](references/resume-optimizer.md) | match %, missing keywords, action verbs |
| [interview-prep](references/interview-prep.md) | role Q&A + research checklist |
| [career-summary](references/career-summary.md) | resume summary in 5 styles |
| [statement-of-purpose](references/statement-of-purpose.md) | SOP in 4 tones |
| [exa-search](references/exa-search.md) | company/people research (if exa MCP) |
| [pdf-extract](references/pdf-extract.md) | extract text from PDFs (if pdftotext) |

## Save as artifacts

Always use `-f` — no shell escaping, linked to job, visible in web UI.

```bash
waypoint artifacts add --skill cover-letter --title "Cover for Google" -f /tmp/cover.txt --job 3       # single variant
waypoint artifacts add --skill email-generator --title "Follow-up" -f /tmp/email.txt --variant-label Casual --job 3  # custom label
waypoint artifacts add --skill cover-letter --title "Cover" --variants-file /tmp/variants.json --job 3 # multi-variant JSON
waypoint artifacts add --skill interview-prep --title-file /tmp/title.txt -f /tmp/prep.md --job 3      # title from file
```

Skill IDs: `email-generator` `cover-letter` `resume-optimizer` `interview-prep` `career-summary` `statement-of-purpose`

View: `artifacts list` · `artifacts list --job 3` · `artifacts list --skill cover-letter` · `artifacts get 12`

## Quick ref
```
waypoint jobs add "Google" "SWE" --status Applied --date 2026-06-20
waypoint jobs list --search python --category Tech
waypoint jobs update 1 --status Rejected
waypoint jobs stats
waypoint artifacts add --skill cover-letter --title "Cover" -f /tmp/cover.txt --job 1
waypoint start --port 8080
```
